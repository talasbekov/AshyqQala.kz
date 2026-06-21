package main

import (
	"fmt"
	"math"
)

// MethodologyVersion пинит ФОРМУЛУ вердикта + значения порогов (GeoGate, MinSample,
// MonopolyShare, DeviationFactor). Иммутабелен: менять при ЛЮБОМ изменении логики/порогов,
// чтобы внешний пересчёт по тем же входам давал тот же вердикт.
// [Source: data_model §0/§2 — methodology_params версионируем-иммутабелен; architecture.md:108,124–127]
const MethodologyVersion = "stage0-1.0"

// wilsonZ95 — z-квантиль нормального распределения для 95% доверительного интервала.
const wilsonZ95 = 1.96

// ciMethodWilson95 — метка метода CI в артефакте (фиксирует выбор формулы).
const ciMethodWilson95 = "wilson_0.95"

// minGeoSample — нижняя граница размера выборки гео, при которой замер считается значимым.
// n ∈ {0,1,2} → «недостаточно сопоставимых данных», не выдавать долю за измерение.
// [Source: architecture.md:160 «n=0/1/2 → недостаточно, не NaN»]
const minGeoSample = 3

// Состояния честности (двухосевой enum value_state, AR-16).
const (
	stateOK           = "ok"
	stateInsufficient = "insufficient_sample"
	stateNoData       = "no_data"
)

// Значения вердикта (AC1: verdict ∈ go | go_with_fallback | no_go).
const (
	verdictGo         = "go"
	verdictGoFallback = "go_with_fallback"
	verdictNoGo       = "no_go"
)

// Verdict — машиночитаемый Go/No-Go-контракт гейта №0. Топ-уровень — struct (НЕ map),
// поэтому порядок ключей в JSON стабилен (определяется порядком полей) → чистый git-diff
// для коммитимого baseline-артефакта. Все имена — snake_case на проводе.
// [Source: architecture.md:523–524 (JSON snake_case); :348 (стабильность ключей)]
type Verdict struct {
	MethodologyVersion string         `json:"methodology_version"`
	GeneratedAt        string         `json:"generated_at"`
	DataSource         string         `json:"data_source"`
	WindowMonths       int            `json:"window_months"`
	GeoGateThreshold   float64        `json:"geo_gate_threshold"`
	OQ1Volume          OQ1Volume      `json:"oq1_volume"`
	OQ4GeoCoverage     OQ4GeoCoverage `json:"oq4_geo_coverage"`
	OQ6Sample          OQ6Sample      `json:"oq6_sample"`
	Verdict            string         `json:"verdict"`
	VerdictReason      string         `json:"verdict_reason"`
}

// OQ1Volume — объём и полнота (Шаг A). Репортится, авто-гейт на него НЕ вешается
// (порог объёма = «≥ согласованного с PM», не фиксированное число). [Source: architecture.md:170]
type OQ1Volume struct {
	ContractsInWindow       int     `json:"contracts_in_window"`
	ContractsTotal          int     `json:"contracts_total"`
	SumCompleteness         float64 `json:"sum_completeness"`
	SupplierBINCompleteness float64 `json:"supplier_bin_completeness"`
}

// OQ4GeoCoverage — автопокрытие геокодером (Шаг B+, НЕ прокси Шага B). Несущий гейт (AC2).
// Coverage/CILower/CIUpper — указатели: null при state != ok (честность: «нет замера» ≠ 0%).
type OQ4GeoCoverage struct {
	Coverage   *float64 `json:"coverage"`
	SampleSize int      `json:"sample_size"`
	Successes  int      `json:"successes"`
	Errors     int      `json:"errors"`
	CILower    *float64 `json:"ci_lower"`
	CIUpper    *float64 `json:"ci_upper"`
	CIMethod   string   `json:"ci_method"`
	State      string   `json:"state"`
}

// OQ6Sample — достаточность выборок для медиан/флагов (Шаг D). Репортится, не гейтится.
type OQ6Sample struct {
	ComparabilityKey string `json:"comparability_key"`
	MinSample        int    `json:"min_sample"`
	GroupsTotal      int    `json:"groups_total"`
	GroupsSufficient int    `json:"groups_sufficient"`
	AnnoMatched      int    `json:"anno_matched"`
	State            string `json:"state"`
}

// verdictOpts — внешний контекст, не выводимый из Report/Config (источник данных, время,
// owner-override fallback). Вынесен в opts, чтобы deriveVerdict оставалась чистой/тестируемой.
type verdictOpts struct {
	allowFallback bool   // -allow-fallback: частичный Go (owner sign-off, Story 0.4)
	dataSource    string // src.Name(): ows | file:<dir> | scrape:...
	generatedAt   string // RFC3339; в тестах фиксируется для детерминизма
}

// wilsonInterval — доверительный интервал Уилсона для доли successes/n (биномиальная
// пропорция). Устойчив при малых n и долях у границ 0/1 (в отличие от нормального
// приближения). При n ≤ 0 → честное состояние полной неопределённости [0,1], НЕ [0,0].
// Только math из stdlib.
func wilsonInterval(successes, n int, z float64) (lo, hi float64) {
	if n <= 0 {
		return 0, 1
	}
	nf := float64(n)
	phat := float64(successes) / nf
	z2 := z * z
	denom := 1 + z2/nf
	center := (phat + z2/(2*nf)) / denom
	margin := (z * math.Sqrt((phat*(1-phat)+z2/(4*nf))/nf)) / denom
	lo = center - margin
	hi = center + margin
	if lo < 0 {
		lo = 0
	}
	if hi > 1 {
		hi = 1
	}
	return lo, hi
}

// round4 округляет до 4 знаков — для читаемого baseline-артефакта без «плавающего хвоста».
// Округление применяется и к coverage, по которой принимается вердикт → JSON и verdict
// всегда согласованы (внешний пересчёт по опубликованной доле даёт тот же исход).
func round4(x float64) float64 { return math.Round(x*1e4) / 1e4 }

func ptr(f float64) *float64 { return &f }

// deriveVerdict — ЧИСТАЯ функция: читает готовые поля Report (ничего не пересчитывает
// из сырых данных) и оформляет машиночитаемый Verdict + exit-код для CI. БЕЗ os.Exit
// внутри (тестируемость). Гео — несущий гейт (AC2): coverage < GeoGate (или замер
// недостаточен) → verdict ≠ go → exit 1.
func deriveVerdict(r Report, cfg Config, opts verdictOpts) (Verdict, int) {
	v := Verdict{
		MethodologyVersion: MethodologyVersion,
		GeneratedAt:        opts.generatedAt,
		DataSource:         opts.dataSource,
		WindowMonths:       cfg.WindowMonths,
		GeoGateThreshold:   cfg.GeoGate,
		OQ1Volume:          deriveOQ1(r),
		OQ6Sample:          deriveOQ6(r, cfg),
	}

	// ---- OQ4 (гео) ----
	oq4 := OQ4GeoCoverage{
		SampleSize: r.geocodeAttempted,
		Successes:  r.geocodeSuccess,
		Errors:     r.geocodeErrors,
		CIMethod:   ciMethodWilson95,
	}
	geoMeasured := r.geocodeEnabled && r.geocodeAttempted >= minGeoSample
	if geoMeasured {
		cov := round4(float64(r.geocodeSuccess) / float64(r.geocodeAttempted))
		lo, hi := wilsonInterval(r.geocodeSuccess, r.geocodeAttempted, wilsonZ95)
		oq4.Coverage = ptr(cov)
		oq4.CILower = ptr(round4(lo))
		oq4.CIUpper = ptr(round4(hi))
		oq4.State = stateOK
	} else {
		// Гео-замер не прогонялся / выборка n ∈ {0,1,2} → честное insufficient,
		// coverage = null. НЕ выдавать 0% за измерение. [Source: prd.md:328–330]
		oq4.State = stateInsufficient
	}
	v.OQ4GeoCoverage = oq4

	// ---- Логика вердикта (детерминированная таблица; гео — несущий гейт) ----
	switch {
	case oq4.State != stateOK:
		v.Verdict = verdictNoGo
		v.VerdictReason = fmt.Sprintf("oq4_geo_coverage: недостаточная выборка (sample_size=%d, state=%s) → гео-замер не подтверждён", oq4.SampleSize, oq4.State)
		return v, 1
	case *oq4.Coverage >= cfg.GeoGate:
		v.Verdict = verdictGo
		v.VerdictReason = fmt.Sprintf("oq4_geo_coverage %.4f ≥ gate %.2f", *oq4.Coverage, cfg.GeoGate)
		return v, 0
	case opts.allowFallback:
		// Частичный Go — осознанное решение владельца (Story 0.4 фиксирует владельца+дату),
		// доступен ТОЛЬКО через явный -allow-fallback, не выдаётся автоматически.
		v.Verdict = verdictGoFallback
		v.VerdictReason = fmt.Sprintf("oq4_geo_coverage %.4f < gate %.2f; частичный Go через -allow-fallback (требует sign-off владельца, Story 0.4)", *oq4.Coverage, cfg.GeoGate)
		return v, 0
	default:
		v.Verdict = verdictNoGo
		v.VerdictReason = fmt.Sprintf("oq4_geo_coverage %.4f < gate %.2f", *oq4.Coverage, cfg.GeoGate)
		return v, 1
	}
}

func deriveOQ1(r Report) OQ1Volume {
	return OQ1Volume{
		ContractsInWindow:       r.contractsInWindow,
		ContractsTotal:          r.contractsTotal,
		SumCompleteness:         round4(frac(r.sumPresent, r.contractsTotal)),
		SupplierBINCompleteness: round4(frac(r.supplierPresent, r.contractsTotal)),
	}
}

func deriveOQ6(r Report, cfg Config) OQ6Sample {
	sufficient := 0
	for _, n := range r.medianGroups {
		if n >= cfg.MinSample {
			sufficient++
		}
	}
	total := len(r.medianGroups)
	state := stateOK
	switch {
	case total == 0:
		state = stateNoData
	case sufficient == 0:
		state = stateInsufficient
	}
	return OQ6Sample{
		ComparabilityKey: fmt.Sprintf("direction × kato × %dmo", cfg.WindowMonths),
		MinSample:        cfg.MinSample,
		GroupsTotal:      total,
		GroupsSufficient: sufficient,
		AnnoMatched:      r.annoMatched,
		State:            state,
	}
}

// frac — доля n/d в [0,1] (0 при d=0). В отличие от pct() (0–100), вердикт оперирует долями.
func frac(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d)
}
