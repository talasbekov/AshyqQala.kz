package render

import (
	"ashyqqala/server/internal/registry"
)

// Evidence — входы рендера (КАРКАС). Полная иммутабельная модель (все входы расчёта +
// snapshot_id) — Epic 4; здесь минимум для каркаса нейтральности.
type Evidence struct {
	FlagState          registry.FlagState
	MethodologyVersion string
}

// Params — версионируемый иммутабельный конфиг методики на стороне рендера (Story 4.1), согласован
// с flags.Params. Несёт каноническую methodology_version (протекает в Evidence — перекрёстный инвариант
// версии, AC5б). Числовые пороги в прозу пойдут плейсхолдерами из этого источника (числовых литералов
// в шаблонах быть НЕ должно); per-flag форматирование — Epic 4 (флаги 4.2–4.5).
type Params struct {
	MethodologyVersion string
}

// Renderer — единственная точка публичной прозы (AR-14). Держит ссылку на рантайм-registry
// (glossary). Render остаётся ЧИСТЫМ: одни входы + один Registry → один текст; без IO/побочных
// эффектов (адаптеры web/OG/Telegram — Epic 5/7 — берут прозу ТОЛЬКО отсюда).
type Renderer struct {
	Reg *registry.Registry
}

// Render — каркас прозы флага: нейтральная рамка + состояние флага. flagID/params — задел
// (per-flag шаблоны и числа из methodology_params — Epic 4; числовые литералы в прозе запрещены).
func (rd Renderer) Render(flagID string, ev Evidence, params Params, loc registry.Locale) string {
	return rd.glossaryOr(loc, "frame.signal") + ": " + rd.RenderFlagState(ev.FlagState, loc)
}

// FlagName — отображаемое имя флага (glossary `flag.<id>.name`), единый источник (Story 5.5, вариант A).
// Никогда пусто: отсутствие ключа → честный fallback «неизвестно, см. методику».
func (rd Renderer) FlagName(flagID string, loc registry.Locale) string {
	return rd.glossaryOr(loc, "flag."+flagID+".name")
}

// FlagLine — полная нейтральная строка флага для презентационных поверхностей (web-бейдж и OG), AC-2/AC-4.
// СОВПАДАЕТ с web FlagBadge (web/src/features/flag/FlagBadge.tsx): raised → "<summary> — <рамка>";
// иначе → "<summary>: <недостаточно сопоставимых данных>". summary/рамка — из glossary (единый источник,
// вариант A); поэтому смысловое ядро web и OG идентично (cross-surface golden, Story 5.7). Никогда пусто.
// Прим.: "недостаточно сопоставимых данных" = value_state.insufficient_sample (== web `flag.insufficient`).
func (rd Renderer) FlagLine(flagID string, fs registry.FlagState, loc registry.Locale) string {
	summary := rd.glossaryOr(loc, "flag."+flagID+".summary")
	if fs == registry.FlagRaised {
		return summary + " — " + rd.glossaryOr(loc, "frame.signal")
	}
	return summary + ": " + rd.glossaryOr(loc, "value_state.insufficient_sample")
}

// RenderFlagState — ЗАКРЫТЫЙ union по flag_state с ОБЯЗАТЕЛЬНОЙ default-веткой.
// Новый/неизвестный член enum → честный текст «неизвестно, см. методику», НИКОГДА пусто/паника.
func (rd Renderer) RenderFlagState(s registry.FlagState, loc registry.Locale) string {
	switch s {
	case registry.FlagRaised, registry.FlagNotRaised, registry.FlagInsufficientData, registry.FlagNotPublished:
		return rd.glossaryOr(loc, "flag_state."+string(s))
	default:
		return rd.unknown(loc)
	}
}

// Text — публичный доступ к строке glossary по ключу с честным fallback (для адаптеров OG/Telegram —
// Epic 5/7). Никогда не возвращает пустую строку (отсутствие ключа → «неизвестно, см. методику»).
func (rd Renderer) Text(loc registry.Locale, key string) string { return rd.glossaryOr(loc, key) }

// RenderValueState — ЗАКРЫТЫЙ union по value_state с ОБЯЗАТЕЛЬНОЙ default-веткой.
func (rd Renderer) RenderValueState(s registry.ValueState, loc registry.Locale) string {
	switch s {
	case registry.StateOK, registry.StateNoData, registry.StateInsufficientSample,
		registry.StateNotComparable, registry.StateStale, registry.StateGeocodePending,
		registry.StateGeocodeFailed, registry.StateSourceConflict, registry.StateRedacted,
		registry.StateNotApplicable, registry.StateError:
		return rd.glossaryOr(loc, "value_state."+string(s))
	default:
		return rd.unknown(loc)
	}
}

// glossaryOr — строка glossary по ключу; при отсутствии — честный fallback на «неизвестно»
// (НИКОГДА не возвращает пустую строку).
func (rd Renderer) glossaryOr(loc registry.Locale, key string) string {
	if rd.Reg != nil {
		if v := rd.Reg.Lookup(loc, key); v != "" {
			return v
		}
	}
	return rd.unknown(loc)
}

// unknown — текст default-ветки; из registry, с зашитым fallback (на случай отсутствия registry).
func (rd Renderer) unknown(loc registry.Locale) string {
	if rd.Reg != nil {
		if v := rd.Reg.Lookup(loc, "state.unknown"); v != "" {
			return v
		}
	}
	return "неизвестно, см. методику"
}
