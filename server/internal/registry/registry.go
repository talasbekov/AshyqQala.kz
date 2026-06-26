// Package registry — рантайм-загрузчик единого источника истины (repo-root registry/values).
// Несущий принцип AR-12: человек правит ТОЛЬКО registry/; Go читает значения В РАНТАЙМЕ (НЕ codegen).
// Типы-enum здесь рукописные (закрытый union); равенство Go-констант с registry-файлом и OpenAPI
// гарантирует перекрёстный тест (Story 1.4 AC4), а загрузчик ЧЕСТНО падает на старте при рассинхроне
// (честность над домыслом — не «пусто», а явная ошибка).
package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ValueState — почему у значения нет ЗНАЧЕНИЯ (ось 1). Закрытый union. На проводе — lower_snake-строка.
// Честное состояние — НЕ ошибка: всегда HTTP 200 + типизированный enum (AR-16).
type ValueState string

const (
	StateOK                 ValueState = "ok"
	StateNoData             ValueState = "no_data"
	StateInsufficientSample ValueState = "insufficient_sample"
	StateNotComparable      ValueState = "not_comparable"
	StateStale              ValueState = "stale"
	StateGeocodePending     ValueState = "geocode_pending"
	StateGeocodeFailed      ValueState = "geocode_failed"
	StateSourceConflict     ValueState = "source_conflict"
	StateRedacted           ValueState = "redacted"
	StateNotApplicable      ValueState = "not_applicable"
	StateError              ValueState = "error"
)

// AllValueStates — закрытый список членов (источник для тестов полноты и перекрёстного теста).
func AllValueStates() []ValueState {
	return []ValueState{
		StateOK, StateNoData, StateInsufficientSample, StateNotComparable, StateStale,
		StateGeocodePending, StateGeocodeFailed, StateSourceConflict, StateRedacted,
		StateNotApplicable, StateError,
	}
}

// FlagState — почему флага нет/есть (ось 2). Закрытый union.
type FlagState string

const (
	FlagRaised           FlagState = "raised"
	FlagNotRaised        FlagState = "not_raised"
	FlagInsufficientData FlagState = "insufficient_data"
	FlagNotPublished     FlagState = "not_published"
)

// AllFlagStates — закрытый список членов flag_state.
func AllFlagStates() []FlagState {
	return []FlagState{FlagRaised, FlagNotRaised, FlagInsufficientData, FlagNotPublished}
}

// Locale — локаль UI/прозы. KK — по умолчанию. Никогда не "kz" (kz — страна, не язык; суффикс _kk/_ru).
type Locale string

const (
	KK Locale = "kk"
	RU Locale = "ru"
)

// AllLocales — поддерживаемые локали (KK первой — дефолт).
func AllLocales() []Locale { return []Locale{KK, RU} }

// honestStatesFile — форма registry/values/honest_states.json.
type honestStatesFile struct {
	ValueState []string `json:"value_state"`
	FlagState  []string `json:"flag_state"`
}

// tabooFile — форма registry/values/taboo_lexicon.json (корни по локалям).
type tabooFile struct {
	RU []string `json:"ru"`
	KK []string `json:"kk"`
}

// Registry — загруженный рантайм-источник истины (glossary + taboo по локалям).
type Registry struct {
	glossary map[Locale]map[string]string
	taboo    map[Locale][]string
}

// Load читает registry/values/* из каталога root (например "registry" или "../../../registry" в тестах).
// Честно падает: отсутствующий/битый файл → ошибка; множество honest_states != Go-консты → ошибка.
func Load(root string) (*Registry, error) {
	valuesDir := filepath.Join(root, "values")

	var hs honestStatesFile
	if err := readJSON(filepath.Join(valuesDir, "honest_states.json"), &hs); err != nil {
		return nil, err
	}
	if err := checkSet("value_state", hs.ValueState, toStrings(AllValueStates())); err != nil {
		return nil, err
	}
	if err := checkSet("flag_state", hs.FlagState, flagsToStrings(AllFlagStates())); err != nil {
		return nil, err
	}

	r := &Registry{
		glossary: map[Locale]map[string]string{},
		taboo:    map[Locale][]string{},
	}
	for _, loc := range AllLocales() {
		g := map[string]string{}
		if err := readJSON(filepath.Join(valuesDir, "glossary-"+string(loc)+".json"), &g); err != nil {
			return nil, err
		}
		r.glossary[loc] = g
	}
	if err := r.checkGlossaryComplete(); err != nil {
		return nil, err
	}

	var tab tabooFile
	if err := readJSON(filepath.Join(valuesDir, "taboo_lexicon.json"), &tab); err != nil {
		return nil, err
	}
	r.taboo[RU] = tab.RU
	r.taboo[KK] = tab.KK

	return r, nil
}

// Lookup возвращает строку glossary по локали и ключу; "" если ключа нет
// (вызывающий — render — обязан иметь fallback на state.unknown; "пусто" недопустимо в выводе).
func (r *Registry) Lookup(loc Locale, key string) string {
	if m, ok := r.glossary[loc]; ok {
		return m[key]
	}
	return ""
}

// GlossaryValues — все строки glossary локали (для doc-теста нейтральности).
func (r *Registry) GlossaryValues(loc Locale) []string {
	m := r.glossary[loc]
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("registry: чтение %s: %w", path, err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("registry: парс %s: %w", path, err)
	}
	return nil
}

func toStrings(in []ValueState) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = string(s)
	}
	return out
}

func flagsToStrings(in []FlagState) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = string(s)
	}
	return out
}

// requiredGlossaryKeys — ключи, которые glossary ОБЯЗАН содержать в каждой локали:
// нейтральная рамка, текст default-ветки и метка КАЖДОГО honest-состояния. Отсутствие/пустое
// → honest-fail на старте (AC1), чтобы пропущенный перевод не маскировался под «неизвестно».
func requiredGlossaryKeys() []string {
	// frame.signal/state.unknown — несущие; signals.none + link.* — нейтральная навигация/агрегат OG-поверхности
	// (Story 5.5): единый источник прозы, обязателен в каждой локали (забытый перевод → honest-fail, не «пусто»).
	keys := []string{"frame.signal", "state.unknown", "signals.none", "link.source", "link.methodology", "link.report_error"}
	for _, s := range AllValueStates() {
		keys = append(keys, "value_state."+string(s))
	}
	for _, s := range AllFlagStates() {
		keys = append(keys, "flag_state."+string(s))
	}
	// Проза флагов — единый источник (Story 5.5, вариант A): name/summary для презентационных
	// поверхностей (web-бейдж + OG-рендер) живут ТОЛЬКО здесь; web сверяется parity-тестом. Должны
	// присутствовать в КАЖДОЙ локали (забытый kk-перевод → honest-fail на старте, не «неизвестно» в OG).
	// Список флагов MVP синхронизирован с httpapi.contractFlagTypes (FR-19 single_participant, FR-20 price_per_km).
	for _, f := range []string{"single_participant", "price_per_km"} {
		keys = append(keys, "flag."+f+".name", "flag."+f+".summary")
	}
	return keys
}

// checkGlossaryComplete честно падает, если в какой-либо локали отсутствует/пуст обязательный ключ.
func (r *Registry) checkGlossaryComplete() error {
	for _, loc := range AllLocales() {
		m := r.glossary[loc]
		for _, k := range requiredGlossaryKeys() {
			if strings.TrimSpace(m[k]) == "" {
				return fmt.Errorf("registry: glossary-%s: отсутствует/пустой обязательный ключ %q", loc, k)
			}
		}
	}
	return nil
}

// checkSet честно падает, если множества (без учёта порядка) различаются.
func checkSet(axis string, fromFile, fromCode []string) error {
	a := append([]string(nil), fromFile...)
	b := append([]string(nil), fromCode...)
	sort.Strings(a)
	sort.Strings(b)
	if len(a) != len(b) {
		return fmt.Errorf("registry: %s рассинхрон файл/консты: %v vs %v", axis, a, b)
	}
	for i := range a {
		if a[i] != b[i] {
			return fmt.Errorf("registry: %s рассинхрон файл/консты: %v vs %v", axis, a, b)
		}
	}
	return nil
}
