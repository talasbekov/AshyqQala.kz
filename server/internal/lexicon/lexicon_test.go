package lexicon_test

import (
	"os"
	"path/filepath"
	"testing"

	"ashyqqala/server/internal/lexicon"
)

// repoRoot — корень репо относительно каталога пакета (server/internal/lexicon).
const repoRoot = "../../../"

// TestLoad_Real — реальный registry/values/normalize_lexicon.v1.json грузится; версия и пара значений
// пиннятся литералом (drift лексикона → красный тест).
func TestLoad_Real(t *testing.T) {
	lex, err := lexicon.Load(filepath.Join(repoRoot, "registry"))
	if err != nil {
		t.Fatalf("Load(registry): %v", err)
	}
	if lex.Version != "v1" {
		t.Errorf("version = %q, ожидалось v1", lex.Version)
	}
	if got := lex.Homoglyphs['o']; got != 'о' {
		t.Errorf("гомоглиф 'o' → %q, ожидалось кир 'о'", string(got))
	}
	if got := lex.Synonyms["товарищество с ограниченной ответственностью"]; got != "тоо" {
		t.Errorf("синоним длинной формы = %q, ожидалось тоо", got)
	}
}

// TestLoad_MissingFile_HonestError — отсутствующий файл → честная ошибка (не тихий пустой лексикон).
func TestLoad_MissingFile_HonestError(t *testing.T) {
	if _, err := lexicon.Load(t.TempDir()); err == nil {
		t.Fatal("ожидалась честная ошибка на отсутствующем normalize_lexicon.v1.json, got nil")
	}
}

// writeLex — пишет временный registry/values/normalize_lexicon.v1.json, возвращает root.
func writeLex(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	vals := filepath.Join(root, "values")
	if err := os.MkdirAll(vals, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vals, lexicon.LexiconFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

const validBody = `{"version":"v1","homoglyphs":{"o":"о"},"synonyms":{"тов":"тоо"}}`

// TestLoad_ValidTmp_OK — позитивный контроль: корректное тело грузится (страж не «всегда красный»).
func TestLoad_ValidTmp_OK(t *testing.T) {
	if _, err := lexicon.Load(writeLex(t, validBody)); err != nil {
		t.Fatalf("корректный лексикон должен грузиться, got %v", err)
	}
}

// TestLoad_HonestErrors — negative-control: загрузчик ОБЯЗАН покраснеть на каждом классе невалидности.
// См. [[guards-must-prove-red]].
func TestLoad_HonestErrors(t *testing.T) {
	cases := []struct{ name, body string }{
		{"невалидная версия", `{"version":"1","homoglyphs":{},"synonyms":{}}`},
		{"многосимвольный гомоглиф", `{"version":"v1","homoglyphs":{"ab":"в"},"synonyms":{}}`},
		{"пустой синоним-значение", `{"version":"v1","homoglyphs":{},"synonyms":{"тов":""}}`},
		{"неизвестное поле", `{"version":"v1","unknown":1,"homoglyphs":{},"synonyms":{}}`},
		{"битый JSON", `{"version":"v1",`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := lexicon.Load(writeLex(t, c.body)); err == nil {
				t.Errorf("ожидалась честная ошибка (%s), got nil", c.name)
			}
		})
	}
}
