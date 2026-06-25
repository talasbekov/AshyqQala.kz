package render

import (
	"regexp"
	"testing"

	"ashyqqala/server/internal/registry"
)

// allFlagIDs — все 4 флага Эпика 4 (FR-19…FR-22). Render игнорирует flagID (рамка+состояние), но property-тест
// surface-agnostic: проверяем КАЖДЫЙ flag_type × состояние × локаль.
var allFlagIDs = []string{"flag.single_participant", "flag.price_per_km", "flag.monopoly", "flag.rnu"}

// TestNeutrality_AllFlags_SurfaceAgnostic — AC2(а): ноль taboo для ВСЕХ flag_type × ВСЕХ FlagState × ОБЕИХ
// локалей; вывод начинается с нейтральной рамки. Консолидирует per-flag нейтральность-тесты 4.2–4.5.
func TestNeutrality_AllFlags_SurfaceAgnostic(t *testing.T) {
	rd := newRenderer(t)
	for _, loc := range registry.AllLocales() {
		frame := rd.Reg.Lookup(loc, "frame.signal")
		if frame == "" {
			t.Fatalf("[%s] нет нейтральной рамки frame.signal", loc)
		}
		for _, id := range allFlagIDs {
			for _, fs := range registry.AllFlagStates() {
				out := rd.Render(id, Evidence{FlagState: fs, MethodologyVersion: "v1.0"}, Params{MethodologyVersion: "v1.0"}, loc)
				if hits := rd.Reg.FindTaboo(loc, out); len(hits) > 0 {
					t.Errorf("[%s/%s/%s] taboo %v: %q", loc, id, fs, hits, out)
				}
				if len(out) < len(frame) || out[:len(frame)] != frame {
					t.Errorf("[%s/%s/%s] вывод не начинается с рамки %q: %q", loc, id, fs, frame, out)
				}
			}
		}
	}
}

// TestRenderFlagState_StatesDistinct — AC2(б) на уровне render: insufficient_data НЕ конфлантится с другими
// состояниями — все FlagState рендерятся в РАЗЛИЧНЫЕ непустые строки (однозначность честных состояний).
func TestRenderFlagState_StatesDistinct(t *testing.T) {
	rd := newRenderer(t)
	for _, loc := range registry.AllLocales() {
		seen := map[string]registry.FlagState{}
		for _, fs := range registry.AllFlagStates() {
			out := rd.RenderFlagState(fs, loc)
			if out == "" {
				t.Errorf("[%s/%s] пустой рендер состояния", loc, fs)
			}
			if prev, dup := seen[out]; dup {
				t.Errorf("[%s] состояния %s и %s рендерятся одинаково %q (конфлантятся)", loc, prev, fs, out)
			}
			seen[out] = fs
		}
	}
}

// thresholdLiteralRe / versionRe / placeholderRe — линтер числовых литералов-порогов в прозе (AC2в,
// architecture:594): числа в шаблоны приходят ТОЛЬКО из methodology_params через плейсхолдеры; «голая» цифра в
// glossary/render = протёкший литерал-порог. Разрешено: версия vN.N и плейсхолдер {имя}. Плейсхолдер ОБЯЗАН
// начинаться с буквы/_ — иначе литерал в скобках ({1.5}) обошёл бы стража (находка ревью 4.6).
var (
	versionLiteralRe     = regexp.MustCompile(`v\d+\.\d+`)
	placeholderLiteralRe = regexp.MustCompile(`\{[A-Za-z_][^}]*\}`)
	bareDigitRe          = regexp.MustCompile(`\d`)
)

// hasThresholdLiteral — есть ли «голая» цифра-литерал (после вычитания версии и плейсхолдеров).
func hasThresholdLiteral(s string) bool {
	s = versionLiteralRe.ReplaceAllString(s, "")
	s = placeholderLiteralRe.ReplaceAllString(s, "")
	return bareDigitRe.MatchString(s)
}

// TestNoNumericLiterals_InGlossary — AC2(в): ни одна строка glossary (источник прозы render) не содержит
// числового литерала-порога (числа — из methodology_params плейсхолдерами). Страж краснеет на протёкшей цифре.
func TestNoNumericLiterals_InGlossary(t *testing.T) {
	rd := newRenderer(t)
	for _, loc := range registry.AllLocales() {
		for _, v := range rd.Reg.GlossaryValues(loc) {
			if hasThresholdLiteral(v) {
				t.Errorf("[%s] glossary-строка содержит числовой литерал-порог (запрещено, число — из methodology_params): %q", loc, v)
			}
		}
	}
}

// TestThresholdLiteralLinter_Discriminates — negative/positive control линтера (страж ОБЯЗАН краснеть):
// внедрённая «голая» цифра → true; нейтральная проза + версия + плейсхолдер → false. [[guards-must-prove-red]]
func TestThresholdLiteralLinter_Discriminates(t *testing.T) {
	if !hasThresholdLiteral("отклонение более чем в 1.5 раза") {
		t.Error("линтер НЕ покраснел на числовом литерале «1.5» — страж бесполезен")
	}
	// Обход через фигурные скобки: {1.5} НЕ настоящий плейсхолдер (плейсхолдер начинается с буквы/_) → краснеет.
	if !hasThresholdLiteral("порог обёрнут в скобки {1.5}") {
		t.Error("линтер НЕ покраснел на литерале-в-скобках «{1.5}» — обход стража через {…}")
	}
	if hasThresholdLiteral("сигнал, требующий проверки (методика v1.0): отклонение более чем в {factor} раза") {
		t.Error("линтер ложно покраснел на версии vN.N + плейсхолдере {factor}")
	}
}
