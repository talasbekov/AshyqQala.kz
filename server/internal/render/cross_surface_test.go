package render

import (
	"strings"
	"testing"

	"ashyqqala/server/internal/registry"
)

// TestCrossSurfaceContract_SemanticCore — Story 5.7 (AC-1 + AC-2): явная КАРТА cross-surface контракта
// нейтральности web↔OG. Смысловое ядро {рамка, число, flag_id, methodology_version, locale} (architecture.md:625-632).
// Карта показывает, ЧЕМ энфорсится каждый элемент: ЧАСТЬ — ЗДЕСЬ (golden+taboo+рамка-позитив+число), ЧАСТЬ —
// делегирована (ссылки ниже). Telegram-плечо — Epic 7 (Story 7.1); здесь web↔OG. «Прогон CI» (AC-2): go test ./...
// (ci-server.yml) + vitest (ci-web.yml); golden-тесты читают glossary с диска → check-registry гоняет с `-count=1`
// (кэш-маскировка, [[guards-must-prove-red]]).
//
// КАРТА ядро → enforcing-гард:
//
//	рамка (frame.signal)  → ЗДЕСЬ (golden-литерал raised + позитив присутствия) ⊕ web flagGlossaryParity.test.ts (web==registry)
//	flag_id               → ЗДЕСЬ (golden-литерал привязан к flag_id)
//	locale                → ЗДЕСЬ (обе локали) ⊕ render_property_test.go (все локали)
//	web↔OG равенство ядра  → ЗДЕСЬ (golden-литерал == OG FlagLine) ⊕ web badgeFramePresence.test.ts (web-бейдж == ТОТ ЖЕ литерал)
//	methodology_version   → ДЕЛЕГИРОВАНО og/meta_test.go:TestMetaHandler_MethodologyVersion_Pinned (OG-тег; web читает ту же params)
//	ЧИСЛО                 → N/A в текущей прозе (FlagLine без чисел; проверяется hasThresholdLiteral ниже + глобально
//	                        render_property_test.go:TestNoNumericLiterals_InGlossary; числа — Epic 4 плейсхолдерами).
func TestCrossSurfaceContract_SemanticCore(t *testing.T) {
	rd := newRenderer(t)

	// golden — смысловое ядро строки флага. ЛИТЕРАЛ (вписан руками, не из кода-под-тестом): это РОВНО текст
	// web-бейджа (FlagBadge.tsx:30 raised `${summary} — ${frame.signal}` / иначе `${summary}: ${insufficient}`) И
	// вывод OG render.FlagLine → ядро идентично на обеих поверхностях. Покрыты raised И не-raised (обе ветки FlagLine).
	type core struct {
		flagID string
		fs     registry.FlagState
		loc    registry.Locale
		out    string
	}
	golden := []core{
		// raised: "<summary> — <рамка>" (рамка-токен присутствует).
		{"single_participant", registry.FlagRaised, registry.RU, "Зафиксирован один участник — сигнал, требующий проверки"},
		{"single_participant", registry.FlagRaised, registry.KK, "Бір қатысушы тіркелді — тексеруді талап ететін сигнал"},
		{"price_per_km", registry.FlagRaised, registry.RU, "Цена за км выше медианы — сигнал, требующий проверки"},
		{"price_per_km", registry.FlagRaised, registry.KK, "Шақырым құны медианадан жоғары — тексеруді талап ететін сигнал"},
		// не-raised: "<summary>: <недостаточно сопоставимых данных>" (рамки-токена нет by design — честное состояние).
		{"single_participant", registry.FlagInsufficientData, registry.RU, "Зафиксирован один участник: недостаточно сопоставимых данных"},
		{"single_participant", registry.FlagInsufficientData, registry.KK, "Бір қатысушы тіркелді: салыстыруға жеткілікті дерек жоқ"},
		{"price_per_km", registry.FlagNotRaised, registry.RU, "Цена за км выше медианы: недостаточно сопоставимых данных"},
	}
	for _, g := range golden {
		got := rd.FlagLine(g.flagID, g.fs, g.loc)
		// (рамка ⊕ flag_id ⊕ locale ⊕ web↔OG равенство): ядро == golden-литерал — краснеет при ЛЮБОМ расхождении.
		if got != g.out {
			t.Errorf("[%s/%s/%s] cross-surface ядро разошлось: FlagLine = %q; golden %q", g.loc, g.flagID, g.fs, got, g.out)
		}
		// AC-1: ноль taboo на поверхности (обе локали, все состояния).
		if hits := rd.Reg.FindTaboo(g.loc, got); len(hits) > 0 {
			t.Errorf("[%s/%s/%s] taboo %v: %q", g.loc, g.flagID, g.fs, hits, got)
		}
		// AC-2 «ЧИСЛО» — N/A: в прозе НЕТ числового литерала-порога (числа из methodology_params плейсхолдерами, Epic 4).
		if hasThresholdLiteral(got) {
			t.Errorf("[%s/%s/%s] числовой литерал в прозе (запрещено; «число» — из methodology_params, Epic 4): %q", g.loc, g.flagID, g.fs, got)
		}
		// AC-1: рамка-токен ПРИСУТСТВУЕТ — позитив ТОЛЬКО для raised (у не-raised рамки нет: честное состояние).
		if g.fs == registry.FlagRaised {
			frame := rd.Reg.Lookup(g.loc, "frame.signal")
			if frame == "" || !strings.Contains(got, frame) {
				t.Errorf("[%s/%s] рамка-токен %q отсутствует в raised-выводе: %q", g.loc, g.flagID, frame, got)
			}
		}
	}

	// negative-control 1: taboo-матчер краснеет по ПРИЧИНЕ в ОБЕИХ локалях (не только RU).
	if hits := rd.Reg.FindTaboo(registry.RU, "это нарушение в контракте"); len(hits) == 0 {
		t.Error("negative-control (ru): FindTaboo не покраснел на «нарушение» — страж фиктивен")
	}
	if hits := rd.Reg.FindTaboo(registry.KK, "келісімшарттағы бұзушылық"); len(hits) == 0 {
		t.Error("negative-control (kk): FindTaboo не покраснел на «бұзушылық» — kk-страж фиктивен")
	}

	// negative-control 2: golden-сравнение ДИСКРИМИНИРУЕТ (не тавтология). Реальный вывод РАВЕН своему golden, но
	// мутированный на 1 хвост — НЕ равен → доказывает, что `got != g.out` в цикле поймал бы любое расхождение.
	real := rd.FlagLine(golden[0].flagID, golden[0].fs, golden[0].loc)
	if real != golden[0].out {
		t.Errorf("negative-control: вывод ≠ собственному golden[0]: %q vs %q", real, golden[0].out)
	}
	if real == golden[0].out+" (мутировано)" {
		t.Error("negative-control: вывод совпал с мутированным golden — сравнение фиктивно")
	}
}
