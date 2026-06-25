package render

import (
	"path/filepath"
	"strings"
	"testing"

	"ashyqqala/server/internal/registry"
)

func newRenderer(t *testing.T) Renderer {
	t.Helper()
	reg, err := registry.Load(filepath.Join("../../../", "registry"))
	if err != nil {
		t.Fatalf("registry.Load: %v", err)
	}
	return Renderer{Reg: reg}
}

// TestRender_Frame — вывод начинается с нейтральной рамки + несёт текст состояния флага.
func TestRender_Frame(t *testing.T) {
	rd := newRenderer(t)
	for _, loc := range registry.AllLocales() {
		out := rd.Render("flag.demo", Evidence{FlagState: registry.FlagRaised, MethodologyVersion: "v1"}, Params{}, loc)
		frame := rd.Reg.Lookup(loc, "frame.signal")
		if frame == "" || !strings.HasPrefix(out, frame) {
			t.Errorf("[%s] вывод %q не начинается с рамки %q", loc, out, frame)
		}
		if out == "" {
			t.Errorf("[%s] пустой вывод Render", loc)
		}
	}
}

// TestRenderFlagState_AllKnown — каждый известный flag_state даёт КОНКРЕТНУЮ glossary-строку
// (не default-fallback), непустую, в обеих локалях (closed union покрыт полностью).
func TestRenderFlagState_AllKnown(t *testing.T) {
	rd := newRenderer(t)
	for _, loc := range registry.AllLocales() {
		unknown := rd.Reg.Lookup(loc, "state.unknown")
		for _, s := range registry.AllFlagStates() {
			got := rd.RenderFlagState(s, loc)
			want := rd.Reg.Lookup(loc, "flag_state."+string(s))
			if got == "" {
				t.Errorf("[%s] RenderFlagState(%s) пусто", loc, s)
			}
			if want == "" {
				t.Errorf("[%s] glossary не содержит flag_state.%s", loc, s)
			}
			if got != want {
				t.Errorf("[%s] RenderFlagState(%s)=%q, ожидалось glossary %q", loc, s, got, want)
			}
			if got == unknown {
				t.Errorf("[%s] flag_state.%s упал в default-ветку (нет своей glossary-строки)", loc, s)
			}
		}
	}
}

// TestRenderValueState_AllKnown — то же для value_state (11 членов).
func TestRenderValueState_AllKnown(t *testing.T) {
	rd := newRenderer(t)
	for _, loc := range registry.AllLocales() {
		unknown := rd.Reg.Lookup(loc, "state.unknown")
		for _, s := range registry.AllValueStates() {
			got := rd.RenderValueState(s, loc)
			want := rd.Reg.Lookup(loc, "value_state."+string(s))
			if got == "" {
				t.Errorf("[%s] RenderValueState(%s) пусто", loc, s)
			}
			if want == "" {
				t.Errorf("[%s] glossary не содержит value_state.%s", loc, s)
			}
			if got != want {
				t.Errorf("[%s] RenderValueState(%s)=%q, ожидалось glossary %q", loc, s, got, want)
			}
			if got == unknown {
				t.Errorf("[%s] value_state.%s упал в default-ветку (нет своей glossary-строки)", loc, s)
			}
		}
	}
}

// TestDefaultBranch — ОБЯЗАТЕЛЬНАЯ default-ветка: неизвестный член enum → текст «неизвестно»,
// непусто, без паники (AC2).
func TestDefaultBranch(t *testing.T) {
	rd := newRenderer(t)
	for _, loc := range registry.AllLocales() {
		unknown := rd.Reg.Lookup(loc, "state.unknown")
		if got := rd.RenderFlagState(registry.FlagState("__bogus__"), loc); got != unknown || got == "" {
			t.Errorf("[%s] default flag_state=%q, ожидалось %q", loc, got, unknown)
		}
		if got := rd.RenderValueState(registry.ValueState("__bogus__"), loc); got != unknown || got == "" {
			t.Errorf("[%s] default value_state=%q, ожидалось %q", loc, got, unknown)
		}
	}
}

// TestDefaultBranch_NoRegistry — без registry default-ветка всё равно не пустая и не паникует.
func TestDefaultBranch_NoRegistry(t *testing.T) {
	rd := Renderer{} // Reg == nil
	if got := rd.RenderFlagState(registry.FlagState("x"), registry.RU); got == "" {
		t.Fatal("default-ветка без registry вернула пусто")
	}
}

// TestRender_SingleParticipant_Neutral — флаг FR-19 (Story 4.2) рендерится нейтрально: начинается с рамки
// «сигнал, требующий проверки», без taboo («нарушение»/«коррупция»). Проза С ЧИСЛАМИ evidence (participant_count
// и т.п.) — Epic 5 (presentation); здесь backend-only — нейтральная рамка + состояние флага.
func TestRender_SingleParticipant_Neutral(t *testing.T) {
	rd := newRenderer(t)
	for _, loc := range registry.AllLocales() {
		for _, fs := range []registry.FlagState{registry.FlagRaised, registry.FlagNotRaised, registry.FlagInsufficientData} {
			out := rd.Render("flag.single_participant", Evidence{FlagState: fs, MethodologyVersion: "v1.0"}, Params{MethodologyVersion: "v1.0"}, loc)
			frame := rd.Reg.Lookup(loc, "frame.signal")
			if frame == "" || !strings.HasPrefix(out, frame) {
				t.Errorf("[%s/%s] вывод %q не начинается с нейтральной рамки %q", loc, fs, out, frame)
			}
			if hits := rd.Reg.FindTaboo(loc, out); len(hits) > 0 {
				t.Errorf("[%s/%s] вывод FR-19 содержит taboo %v: %q", loc, fs, hits, out)
			}
		}
	}
}

// TestRender_PricePerKM_Neutral — флаг FR-20 (Story 4.3) рендерится нейтрально: рамка «сигнал, требующий
// проверки» + состояние, без taboo. Проза С ЧИСЛАМИ (цена/км, медиана, порог) — Epic 5 (presentation).
func TestRender_PricePerKM_Neutral(t *testing.T) {
	rd := newRenderer(t)
	for _, loc := range registry.AllLocales() {
		for _, fs := range []registry.FlagState{registry.FlagRaised, registry.FlagNotRaised, registry.FlagInsufficientData} {
			out := rd.Render("flag.price_per_km", Evidence{FlagState: fs, MethodologyVersion: "v1.0"}, Params{MethodologyVersion: "v1.0"}, loc)
			if frame := rd.Reg.Lookup(loc, "frame.signal"); frame == "" || !strings.HasPrefix(out, frame) {
				t.Errorf("[%s/%s] вывод %q не начинается с нейтральной рамки %q", loc, fs, out, frame)
			}
			if hits := rd.Reg.FindTaboo(loc, out); len(hits) > 0 {
				t.Errorf("[%s/%s] вывод FR-20 содержит taboo %v: %q", loc, fs, hits, out)
			}
		}
	}
}

// TestRender_Monopoly_Neutral — флаг FR-21 (Story 4.4) рендерится нейтрально: рамка «сигнал, требующий
// проверки» + состояние, без taboo. Проза С ЧИСЛАМИ (доля, знаменатель выборки) — Epic 5 (presentation).
func TestRender_Monopoly_Neutral(t *testing.T) {
	rd := newRenderer(t)
	for _, loc := range registry.AllLocales() {
		for _, fs := range []registry.FlagState{registry.FlagRaised, registry.FlagNotRaised, registry.FlagInsufficientData} {
			out := rd.Render("flag.monopoly", Evidence{FlagState: fs, MethodologyVersion: "v1.0"}, Params{MethodologyVersion: "v1.0"}, loc)
			if frame := rd.Reg.Lookup(loc, "frame.signal"); frame == "" || !strings.HasPrefix(out, frame) {
				t.Errorf("[%s/%s] вывод %q не начинается с нейтральной рамки %q", loc, fs, out, frame)
			}
			if hits := rd.Reg.FindTaboo(loc, out); len(hits) > 0 {
				t.Errorf("[%s/%s] вывод FR-21 содержит taboo %v: %q", loc, fs, hits, out)
			}
		}
	}
}

// TestRender_RNU_Neutral — флаг FR-22 (Story 4.5) рендерится нейтрально: рамка «сигнал, требующий проверки» +
// состояние, без taboo. «Недобросовестный» — только цитата названия реестра, атрибуция государству; проза со
// ссылкой на реестр (source_url) — Epic 5 (presentation).
func TestRender_RNU_Neutral(t *testing.T) {
	rd := newRenderer(t)
	for _, loc := range registry.AllLocales() {
		for _, fs := range []registry.FlagState{registry.FlagRaised, registry.FlagNotRaised, registry.FlagNotPublished} {
			out := rd.Render("flag.rnu", Evidence{FlagState: fs, MethodologyVersion: "v1.0"}, Params{MethodologyVersion: "v1.0"}, loc)
			if frame := rd.Reg.Lookup(loc, "frame.signal"); frame == "" || !strings.HasPrefix(out, frame) {
				t.Errorf("[%s/%s] вывод %q не начинается с нейтральной рамки %q", loc, fs, out, frame)
			}
			if hits := rd.Reg.FindTaboo(loc, out); len(hits) > 0 {
				t.Errorf("[%s/%s] вывод FR-22 содержит taboo %v: %q", loc, fs, hits, out)
			}
		}
	}
}

// TestNeutrality_RenderOutputs_NoTaboo — вся сгенерированная проза (все состояния × локали)
// свободна от taboo-лексикона (AC3, doc-нейтральность на выходе render).
func TestNeutrality_RenderOutputs_NoTaboo(t *testing.T) {
	rd := newRenderer(t)
	for _, loc := range registry.AllLocales() {
		var outs []string
		for _, fs := range registry.AllFlagStates() {
			outs = append(outs, rd.Render("flag.demo", Evidence{FlagState: fs}, Params{}, loc))
		}
		for _, s := range registry.AllFlagStates() {
			outs = append(outs, rd.RenderFlagState(s, loc))
		}
		for _, s := range registry.AllValueStates() {
			outs = append(outs, rd.RenderValueState(s, loc))
		}
		for _, o := range outs {
			if hits := rd.Reg.FindTaboo(loc, o); len(hits) > 0 {
				t.Errorf("[%s] вывод render %q содержит taboo %v", loc, o, hits)
			}
		}
	}
}
