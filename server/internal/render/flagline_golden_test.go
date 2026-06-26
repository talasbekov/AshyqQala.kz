package render

import (
	"testing"

	"ashyqqala/server/internal/registry"
)

// TestFlagLine_CrossSurfaceGolden — ЗОЛОТОЙ СНАПШОТ смыслового ядра строки флага (Story 5.5, AC-4; задел
// под per-surface тест 5.7). Ожидаемые строки запинены ЛИТЕРАЛОМ (не выведены из кода-под-тестом —
// [[guards-must-prove-red]]) и ДОЛЖНЫ совпадать с тем, что собирает web FlagBadge
// (web/src/features/flag/FlagBadge.tsx: raised → `${summary} — ${frame.signal}`; иначе → `${summary}: ${insufficient}`).
//
// Кросс-поверхностное равенство web↔OG гарантируется транзитивно:
//
//	(1) этот golden фиксирует OG-вывод (render.FlagLine) литералом == текст web-бейджа;
//	(2) parity-тест web (web/src/features/contract/flagGlossaryParity.test.ts) фиксирует, что строки флагов
//	    в web-i18n РАВНЫ строкам в registry/values/glossary (единый источник, вариант A);
//	(3) FlagLine собирает строку ИЗ registry/glossary.
//
// → смысловое ядро {summary, рамка, flag_id, locale} идентично на web и OG. Прогон некэшируемо: `-count=1`.
//
// methodology_version (5-й элемент ядра AC-4) НЕ проходит через FlagLine (рамка+саммари без версии) —
// он версионирует OG-картинку/тег и пинится литералом в og.TestMetaHandler_MethodologyVersion_Pinned
// (та же каноническая params.MethodologyVersion, что читает web → расхождения версий между поверхностями нет).
func TestFlagLine_CrossSurfaceGolden(t *testing.T) {
	rd := newRenderer(t)

	type want struct {
		flagID string
		fs     registry.FlagState
		loc    registry.Locale
		out    string
	}
	golden := []want{
		// raised: "<summary> — <рамка>"
		{"single_participant", registry.FlagRaised, registry.RU, "Зафиксирован один участник — сигнал, требующий проверки"},
		{"single_participant", registry.FlagRaised, registry.KK, "Бір қатысушы тіркелді — тексеруді талап ететін сигнал"},
		{"price_per_km", registry.FlagRaised, registry.RU, "Цена за км выше медианы — сигнал, требующий проверки"},
		{"price_per_km", registry.FlagRaised, registry.KK, "Шақырым құны медианадан жоғары — тексеруді талап ететін сигнал"},
		// не-raised: "<summary>: <недостаточно сопоставимых данных>" (== web flag.insufficient)
		{"single_participant", registry.FlagInsufficientData, registry.RU, "Зафиксирован один участник: недостаточно сопоставимых данных"},
		{"single_participant", registry.FlagInsufficientData, registry.KK, "Бір қатысушы тіркелді: салыстыруға жеткілікті дерек жоқ"},
		{"price_per_km", registry.FlagNotRaised, registry.RU, "Цена за км выше медианы: недостаточно сопоставимых данных"},
	}
	for _, g := range golden {
		got := rd.FlagLine(g.flagID, g.fs, g.loc)
		if got != g.out {
			t.Errorf("FlagLine(%q,%s,%s) = %q; golden %q", g.flagID, g.fs, g.loc, got, g.out)
		}
	}
}

// TestFlagName_Golden — отображаемое имя флага (glossary flag.<id>.name) запинено литералом.
func TestFlagName_Golden(t *testing.T) {
	rd := newRenderer(t)
	cases := map[registry.Locale]map[string]string{
		registry.RU: {"single_participant": "Единственный участник", "price_per_km": "Цена за км"},
		registry.KK: {"single_participant": "Бір қатысушы", "price_per_km": "Шақырым құны"},
	}
	for loc, m := range cases {
		for id, want := range m {
			if got := rd.FlagName(id, loc); got != want {
				t.Errorf("FlagName(%q,%s) = %q; golden %q", id, loc, got, want)
			}
		}
	}
}

// TestFlagLine_NeverEmpty_UnknownFlag — неизвестный flag_id НИКОГДА не даёт пустую строку (честный fallback).
func TestFlagLine_NeverEmpty_UnknownFlag(t *testing.T) {
	rd := newRenderer(t)
	for _, loc := range registry.AllLocales() {
		if got := rd.FlagLine("nonexistent_flag", registry.FlagRaised, loc); got == "" {
			t.Errorf("[%s] FlagLine для неизвестного флага вернул пустую строку", loc)
		}
	}
}
