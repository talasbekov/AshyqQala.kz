package main

import (
	"fmt"
	"strings"
)

// Metrics — метрики этапа 5 (Story 5.6 AC-3): N размечено, M поднято, X пересчитываемых по методике.
type Metrics struct {
	Marked       int64 // N — размеченные контракты
	ActiveFlags  int64 // M — активные сигналы
	Recomputable int64 // X — активные сигналы с полным evidence+version (пересчитываемы третьим лицом)
}

// VerdictSummary — выжимка вердикта Stage-0 (Epic 0) из docs/ops/stage0-verdict-*.json. Только нужные поля.
type VerdictSummary struct {
	Verdict            string   // go | go_with_fallback | no_go
	Reason             string   // verdict_reason
	GeoGateThreshold   float64  // geo_gate_threshold (доля)
	GeoCoverage        *float64 // oq4 coverage; nil при insufficient
	GeoState           string   // oq4 state
	ContractsInWindow  int64    // oq1 contracts_in_window
	DataSource         string   // data_source (file:… / ows…)
	MethodologyVersion string   // версия методики Stage-0 (НЕ карточки)
}

// renderOnePager — детерминированный Markdown pilot-one-pager. Нейтральная лексика (нет оценочных слов);
// честные состояния; источник данных штампуется (синтетику не выдаём за живой пилот). Пустая проекция
// (Marked==0) → честный «данные не загружены», НЕ таблица фабрикованных нулей (§7.4 честность над домыслом).
func renderOnePager(m Metrics, v VerdictSummary, generatedAt string) string {
	var b strings.Builder
	b.WriteString("# Pilot one-pager — AshyqQala.kz (этап 5)\n\n")
	fmt.Fprintf(&b, "_Сгенерировано: %s · Источник данных: %s · Методика Stage-0: %s_\n\n",
		generatedAt, dataSourceLabel(v.DataSource), nz(v.MethodologyVersion))

	if m.Marked == 0 {
		b.WriteString("> **Недостаточно данных для пилотных метрик:** контракты ещё не загружены в проекцию.\n")
		b.WriteString("> Метрики появятся после импорта (живой `ows_v2` — Epic 2, либо интерим-парсер). Цифры не фабрикуются.\n\n")
	} else {
		b.WriteString("## Метрики этапа 5\n\n")
		b.WriteString("| Метрика | Значение |\n| --- | --- |\n")
		fmt.Fprintf(&b, "| Контрактов размечено (N) | %d |\n", m.Marked)
		fmt.Fprintf(&b, "| Сигналов поднято (M) | %d |\n", m.ActiveFlags)
		fmt.Fprintf(&b, "| Из них пересчитываемых по методике (X) | %d |\n\n", m.Recomputable)
		b.WriteString("_Каждый сигнал — «сигнал, требующий проверки», не обвинение. X пересчитываются третьим лицом " +
			"по опубликованной методике (evidence + версия)._\n\n")
		b.WriteString("Доказательства любого поднятого сигнала экспортируются для цитирования (AC-2): " +
			"`GET /api/contracts/{goszakup_id}/flags/{flag_type}/evidence.json` (+ `.txt`).\n\n")
	}

	b.WriteString("## Вердикт готовности данных (Stage-0, Epic 0)\n\n")
	fmt.Fprintf(&b, "- **Вердикт:** %s\n", verdictLabel(v.Verdict))
	if v.Reason != "" {
		fmt.Fprintf(&b, "- **Причина:** %s\n", v.Reason)
	}
	fmt.Fprintf(&b, "- **Контрактов в окне (OQ1):** %s\n", honestCount(v.ContractsInWindow))
	fmt.Fprintf(&b, "- **Гео-покрытие (OQ4, %s):** %s (состояние: %s)\n",
		gateLabel(v.GeoGateThreshold), geoCoverageLabel(v.GeoCoverage), nz(v.GeoState))
	b.WriteString("\n_Источник вердикта — закоммиченный артефакт Stage-0 (пересчитываемо)._\n")
	return b.String()
}

// dataSourceLabel — честная пометка источника (синтетику/интерим/нераспознанное НЕ выдаём за живой пилот).
// Живой — только `ows`; всё остальное помечается явно ([[data-source-interim-parser-first]]: основной источник
// до токена — `scrape`/интерим-парсер).
func dataSourceLabel(s string) string {
	switch {
	case s == "":
		return "нет данных"
	case strings.HasPrefix(s, "ows"):
		return s + " (живой источник)"
	case strings.HasPrefix(s, "file:"):
		return s + " (синтетика/демо)"
	case strings.HasPrefix(s, "scrape:") || strings.HasPrefix(s, "interim"):
		return s + " (интерим-парсер, не живой)"
	default:
		return s + " (источник не классифицирован — не подтверждён как живой)"
	}
}

// gateLabel — порог гео-гейта или честное «гейт не задан» при отсутствии/нуле (не выдаём «≥ 0%» за реальный порог).
func gateLabel(threshold float64) string {
	if threshold <= 0 {
		return "гейт не задан"
	}
	return fmt.Sprintf("гейт ≥ %.0f%%", threshold*100)
}

// verdictLabel — нейтральная расшифровка вердикта (без оценочных слов).
func verdictLabel(v string) string {
	switch v {
	case "go":
		return "go — данные достаточны для запуска"
	case "go_with_fallback":
		return "go_with_fallback — достаточно с оговоркой (по решению владельца)"
	case "no_go":
		return "no_go — данные пока недостаточны (требуется дозамер)"
	case "":
		return "нет данных"
	default:
		return v
	}
}

// honestCount — 0 → «нет данных» (не выдуманный «0» как факт), иначе число.
func honestCount(n int64) string {
	if n == 0 {
		return "нет данных"
	}
	return fmt.Sprintf("%d", n)
}

// geoCoverageLabel — nil → «недостаточно сопоставимых данных»; иначе процент.
func geoCoverageLabel(c *float64) string {
	if c == nil {
		return "недостаточно сопоставимых данных"
	}
	return fmt.Sprintf("%.1f%%", *c*100)
}

// nz — непустая строка или честное «нет данных».
func nz(s string) string {
	if strings.TrimSpace(s) == "" {
		return "нет данных"
	}
	return s
}
