package registry

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// repoRoot — корень репо относительно каталога пакета (server/internal/registry).
const repoRoot = "../../../"

func loadReal(t *testing.T) *Registry {
	t.Helper()
	r, err := Load(filepath.Join(repoRoot, "registry"))
	if err != nil {
		t.Fatalf("Load(registry): %v", err)
	}
	return r
}

func TestLoad_OK(t *testing.T) {
	_ = loadReal(t)
}

func TestLoad_MissingDir_HonestError(t *testing.T) {
	if _, err := Load(t.TempDir()); err == nil {
		t.Fatal("ожидалась честная ошибка на отсутствующем honest_states.json, got nil")
	}
}

func TestLoad_Desync_HonestError(t *testing.T) {
	// Подменяем honest_states.json в tmp registry лишним членом → загрузчик обязан упасть.
	dir := t.TempDir()
	vals := filepath.Join(dir, "values")
	if err := os.MkdirAll(vals, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(vals, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("honest_states.json", `{"value_state":["ok"],"flag_state":["raised","not_raised","insufficient_data","not_published"]}`)
	write("glossary-kk.json", `{}`)
	write("glossary-ru.json", `{}`)
	write("taboo_lexicon.json", `{"ru":[],"kk":[]}`)
	if _, err := Load(dir); err == nil {
		t.Fatal("ожидался рассинхрон value_state (файл != Go-консты), got nil")
	}
}

// TestLoad_GlossaryIncomplete_HonestError — известное состояние без glossary-метки в локали → honest-fail (AC1).
func TestLoad_GlossaryIncomplete_HonestError(t *testing.T) {
	dir := t.TempDir()
	vals := filepath.Join(dir, "values")
	if err := os.MkdirAll(vals, 0o755); err != nil {
		t.Fatal(err)
	}
	writeJSONFile := func(name string, v any) {
		b, _ := json.Marshal(v)
		if err := os.WriteFile(filepath.Join(vals, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	full := func() map[string]string {
		m := map[string]string{"frame.signal": "рамка", "state.unknown": "неизвестно"}
		for _, s := range AllValueStates() {
			m["value_state."+string(s)] = "v"
		}
		for _, s := range AllFlagStates() {
			m["flag_state."+string(s)] = "f"
		}
		return m
	}
	writeJSONFile("honest_states.json", honestStatesFile{ValueState: toStrings(AllValueStates()), FlagState: flagsToStrings(AllFlagStates())})
	writeJSONFile("glossary-kk.json", full())
	ru := full()
	delete(ru, "value_state.stale") // пропущенный перевод известного состояния
	writeJSONFile("glossary-ru.json", ru)
	writeJSONFile("taboo_lexicon.json", tabooFile{RU: []string{}, KK: []string{}})

	if _, err := Load(dir); err == nil {
		t.Fatal("ожидался honest-fail: glossary-ru без ключа value_state.stale, got nil")
	}
}

// TestCrossAxis_ValueState — registry ↔ Go-консты ↔ OpenAPI: множества value_state РАВНЫ (AC4).
func TestCrossAxis_ValueState(t *testing.T) {
	r := loadReal(t)
	_ = r

	fromCode := toStrings(AllValueStates())
	fromOpenAPI := openapiEnum(t, "StringField")

	assertSameSet(t, "value_state Go↔OpenAPI", fromCode, fromOpenAPI)

	// И registry-файл (через загрузчик он уже сверен с Go-константами в Load — то есть транзитивно равен).
	// Явная сверка файла напрямую:
	var hs honestStatesFile
	if err := readJSON(filepath.Join(repoRoot, "registry", "values", "honest_states.json"), &hs); err != nil {
		t.Fatal(err)
	}
	assertSameSet(t, "value_state registry↔Go", hs.ValueState, fromCode)
}

// TestCrossAxis_FlagState — registry ↔ Go-консты ↔ OpenAPI (FlagField) РАВНЫ (AC4).
func TestCrossAxis_FlagState(t *testing.T) {
	fromCode := flagsToStrings(AllFlagStates())
	fromOpenAPI := openapiEnum(t, "FlagField")
	assertSameSet(t, "flag_state Go↔OpenAPI", fromCode, fromOpenAPI)

	var hs honestStatesFile
	if err := readJSON(filepath.Join(repoRoot, "registry", "values", "honest_states.json"), &hs); err != nil {
		t.Fatal(err)
	}
	assertSameSet(t, "flag_state registry↔Go", hs.FlagState, fromCode)
}

// TestNeutrality_GlossaryNoTaboo — doc-нейтральность: в glossary обеих локалей НЕТ taboo-строк (AC3).
func TestNeutrality_GlossaryNoTaboo(t *testing.T) {
	r := loadReal(t)
	for _, loc := range AllLocales() {
		for _, v := range r.GlossaryValues(loc) {
			if hits := r.FindTaboo(loc, v); len(hits) > 0 {
				t.Errorf("[%s] glossary-строка %q содержит taboo-корни %v", loc, v, hits)
			}
		}
	}
}

// TestFindTaboo_Morphology — матчер ловит склонения/мн.ч. (RU), KK-формы и гомоглифы; без ложных срабатываний.
func TestFindTaboo_Morphology(t *testing.T) {
	r := loadReal(t)
	cases := []struct {
		loc   Locale
		text  string
		taboo bool
	}{
		{RU, "выявлено нарушение сроков", true},
		{RU, "несколько нарушений графика", true}, // мн.ч. через корень
		{RU, "признаки коррупции", true},
		{RU, "нaрушение", true},                   // гомоглиф: лат. 'a'
		{RU, "сигнал, требующий проверки", false}, // нейтральная рамка
		{RU, "отклонение цены за км", false},
		{KK, "бұзушылықтар анықталды", true}, // мн.ч.
		{KK, "сыбайлас жемқорлық белгілері", true},
		{KK, "тексеруді талап ететін сигнал", false},
	}
	for _, c := range cases {
		hits := r.FindTaboo(c.loc, c.text)
		if got := len(hits) > 0; got != c.taboo {
			t.Errorf("[%s] FindTaboo(%q) = %v (hits=%v), ожидалось taboo=%v", c.loc, c.text, got, hits, c.taboo)
		}
	}
}

// --- helpers ---

func openapiEnum(t *testing.T, schema string) []string {
	t.Helper()
	doc, err := openapi3.NewLoader().LoadFromFile(filepath.Join(repoRoot, "docs", "api-contracts", "openapi.yaml"))
	if err != nil {
		t.Fatalf("load openapi: %v", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("openapi невалиден: %v", err)
	}
	ref := doc.Components.Schemas[schema]
	if ref == nil || ref.Value == nil {
		t.Fatalf("схема %q не найдена в openapi", schema)
	}
	stateRef := ref.Value.Properties["state"]
	if stateRef == nil || stateRef.Value == nil {
		t.Fatalf("у схемы %q нет свойства state", schema)
	}
	out := make([]string, 0, len(stateRef.Value.Enum))
	for _, e := range stateRef.Value.Enum {
		s, ok := e.(string)
		if !ok {
			t.Fatalf("enum-значение не строка: %T", e)
		}
		out = append(out, s)
	}
	return out
}

func assertSameSet(t *testing.T, label string, a, b []string) {
	t.Helper()
	x := append([]string(nil), a...)
	y := append([]string(nil), b...)
	sort.Strings(x)
	sort.Strings(y)
	if len(x) != len(y) {
		t.Fatalf("%s: разный размер: %v vs %v", label, x, y)
	}
	for i := range x {
		if x[i] != y[i] {
			t.Fatalf("%s: рассинхрон: %v vs %v", label, x, y)
		}
	}
}
