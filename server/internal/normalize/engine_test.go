package normalize_test

import (
	"testing"

	"ashyqqala/server/internal/normalize"
)

// testLex — минимальный лексикон для движка (гомоглиф лат 'o'→кир 'о' + орг-форма-синоним).
func testLex() normalize.Lexicon {
	return normalize.Lexicon{
		Version:    "v1",
		Homoglyphs: map[rune]rune{'o': 'о', 'a': 'а', 'e': 'е'},
		Synonyms: map[string]string{
			"товарищество с ограниченной ответственностью": "тоо",
		},
	}
}

func TestNormalizeBIN(t *testing.T) {
	cases := []struct{ in, want string }{
		{"123 456 789 012", "123456789012"},
		{"БИН: 000111222333", "000111222333"}, // ведущие нули сохранены
		{"222222222222", "222222222222"},
		{"нет", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := normalize.NormalizeBIN(c.in); string(got) != c.want {
			t.Errorf("NormalizeBIN(%q) = %q, ожидалось %q", c.in, got, c.want)
		}
	}
}

func TestCanonicalKey(t *testing.T) {
	lex := testLex()
	cases := []struct{ in, want string }{
		{"ТОО  Астана-Жол", "тоо астана жол"},                   // casefold + схлопывание + дефис
		{"ТОО \"Астана-Жол\"", "тоо астана жол"},                // кавычки/дефис как разделители
		{"ТОО Астана Жoл", "тоо астана жол"},                    // гомоглиф лат 'o' → кир 'о'
		{"Товарищество с ограниченной ответственностью", "тоо"}, // пофразовый синоним
		{"", ""},
	}
	for _, c := range cases {
		if got := normalize.CanonicalKey(c.in, lex); got != c.want {
			t.Errorf("CanonicalKey(%q) = %q, ожидалось %q", c.in, got, c.want)
		}
	}
}

// TestResolve_Auto_ByBIN — новый валидный БИН без конфликта имени → auto (регистрируется под своим БИН).
func TestResolve_Auto_ByBIN(t *testing.T) {
	d := normalize.Resolve(normalize.Input{RawBIN: "222222222222", RawName: "ТОО Астана Жол"}, nil, testLex())
	if d.Status != normalize.StatusAuto || d.BIN != "222222222222" {
		t.Fatalf("auto by bin: got status=%s bin=%q", d.Status, d.BIN)
	}
}

// TestResolve_Auto_VariantSpellingSameBIN — вариативное написание того же БИН → auto (объединение).
func TestResolve_Auto_VariantSpellingSameBIN(t *testing.T) {
	lex := testLex()
	existing := []normalize.Candidate{{BIN: "222222222222", NameKey: normalize.CanonicalKey("ТОО Астана Жол", lex)}}
	// гомоглиф + кавычки + дефис — то же каноническое имя, тот же БИН.
	d := normalize.Resolve(normalize.Input{RawBIN: "222222222222", RawName: "ТОО \"Астана-Жoл\""}, existing, lex)
	if d.Status != normalize.StatusAuto || d.BIN != "222222222222" {
		t.Fatalf("auto variant: got status=%s bin=%q", d.Status, d.BIN)
	}
}

// TestResolve_Conflict_NameClaimedByOtherBIN — то же имя под ДРУГИМ БИН → conflict («БИН↔наименование»).
func TestResolve_Conflict_NameClaimedByOtherBIN(t *testing.T) {
	lex := testLex()
	existing := []normalize.Candidate{{BIN: "222222222222", NameKey: normalize.CanonicalKey("ТОО Астана Жол", lex)}}
	d := normalize.Resolve(normalize.Input{RawBIN: "999999999999", RawName: "ТОО Астана Жол"}, existing, lex)
	if d.Status != normalize.StatusConflict {
		t.Fatalf("conflict: got status=%s (%s)", d.Status, d.Reason)
	}
}

// TestResolve_Manual_NameOnlySingleMatch — имя без БИН с ОДНИМ совпадением → manual (НЕ auto): без БИН
// авто-объединение = домысел (одно совпадение по имени может быть другой орг). [decision code-review 2026-06-27]
func TestResolve_Manual_NameOnlySingleMatch(t *testing.T) {
	lex := testLex()
	existing := []normalize.Candidate{{BIN: "222222222222", NameKey: normalize.CanonicalKey("ТОО Астана Жол", lex)}}
	d := normalize.Resolve(normalize.Input{RawBIN: "", RawName: "ТОО Астана Жол"}, existing, lex)
	if d.Status != normalize.StatusManual {
		t.Fatalf("name-only одно совпадение должно быть manual, got status=%s", d.Status)
	}
}

// TestValidBIN / TestCanonicalBIN — валиден только 12-значный БИН; мусор/неполный → "" (не фантомная орг).
func TestValidBIN(t *testing.T) {
	if !normalize.ValidBIN("222222222222") {
		t.Error("12 цифр должны быть валидны")
	}
	for _, bad := range []normalize.BIN{"", "12345", "2222222222222", "22222222222a"} {
		if normalize.ValidBIN(bad) {
			t.Errorf("ValidBIN(%q) = true, ожидалось false", bad)
		}
	}
}

func TestCanonicalBIN(t *testing.T) {
	if got := normalize.CanonicalBIN("222 222 222 222"); got != "222222222222" {
		t.Errorf("CanonicalBIN(валидный с пробелами) = %q", got)
	}
	// мусор/неполный → "" (резолв уйдёт в ветку «без БИН» → manual, не фантомная организация)
	for _, raw := range []string{"ул. Абая 12, оф 345", "Д-2024/00123", "1", ""} {
		if got := normalize.CanonicalBIN(raw); got != "" {
			t.Errorf("CanonicalBIN(%q) = %q, ожидалось \"\"", raw, got)
		}
	}
}

// TestResolve_InvalidBIN_NotPhantom — невалидный «БИН» (мусор) → ветка «без БИН» → manual, без авто-регистрации.
func TestResolve_InvalidBIN_NotPhantom(t *testing.T) {
	d := normalize.Resolve(normalize.Input{RawBIN: "Д-2024/00123", RawName: "ТОО Что-то"}, nil, testLex())
	if d.Status != normalize.StatusManual || d.BIN != "" {
		t.Fatalf("мусорный БИН не должен авто-регистрироваться: status=%s bin=%q", d.Status, d.BIN)
	}
}

// TestResolve_Manual_NameOnlyNoMatch — имя без БИН без совпадений → manual (очередь оператору, не домысел).
func TestResolve_Manual_NameOnlyNoMatch(t *testing.T) {
	d := normalize.Resolve(normalize.Input{RawBIN: "", RawName: "ТОО Неизвестная Фирма"}, nil, testLex())
	if d.Status != normalize.StatusManual || d.BIN != "" {
		t.Fatalf("manual: got status=%s bin=%q", d.Status, d.BIN)
	}
}

// TestResolve_Conflict_NameOnlyMultiBIN — имя без БИН совпало с ≥2 БИН → conflict.
func TestResolve_Conflict_NameOnlyMultiBIN(t *testing.T) {
	lex := testLex()
	k := normalize.CanonicalKey("ТОО Дубль", lex)
	existing := []normalize.Candidate{{BIN: "222222222222", NameKey: k}, {BIN: "333333333333", NameKey: k}}
	d := normalize.Resolve(normalize.Input{RawBIN: "", RawName: "ТОО Дубль"}, existing, lex)
	if d.Status != normalize.StatusConflict {
		t.Fatalf("conflict multi: got status=%s (%s)", d.Status, d.Reason)
	}
}
