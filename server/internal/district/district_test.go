package district

import (
	"os"
	"path/filepath"
	"testing"
)

// realRoot — корень registry в репозитории (из server/internal/district → 3 уровня вверх).
const realRoot = "../../../registry"

func TestLoad_RealRegistry_FiveDistricts_AllCodesNull(t *testing.T) {
	c, err := Load(realRoot)
	if err != nil {
		t.Fatalf("Load(%q): %v", realRoot, err)
	}
	all := c.All()
	if len(all) != 5 {
		t.Fatalf("районов = %d, ожидалось 5 (Алматы/Сарыарка/Есиль/Байконыр/Нура)", len(all))
	}
	// Гардрейл «коды не выдумываем»: в S-0 ВСЕ kato=null (ждут Story 0.1). Если кто-то впишет код —
	// это осознанное действие (0.1), тест-страж напоминает о происхождении.
	for _, d := range all {
		if d.Kato != nil {
			t.Errorf("район %q: kato=%q, но в S-0 коды НЕ подтверждены (ожидался null до Story 0.1)", d.Slug, *d.Kato)
		}
		if d.NameKk == "" || d.NameRu == "" {
			t.Errorf("район %q: пустое имя kk/ru", d.Slug)
		}
	}
}

func TestNameByKATO_NoConfirmedCodes_NotFound(t *testing.T) {
	c, err := Load(realRoot)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Все коды null → имя НЕ резолвится (честный no_data у вызывающего, НЕ выдуманное имя).
	if kk, ru, ok := c.NameByKATO("710000000"); ok {
		t.Errorf("NameByKATO без подтверждённых кодов вернул ok=true (%q/%q) — должно быть not found", kk, ru)
	}
}

// writeCatalog — временный реестр с ПОДТВЕРЖДЁННЫМИ кодами (синтетика теста; в проде коды ставит 0.1).
func writeCatalog(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "values"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "values", DistrictsFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestNameByKATO_ConfirmedCodes_LongestPrefixWins(t *testing.T) {
	root := writeCatalog(t, `{
	  "version": "v1",
	  "districts": [
	    { "slug": "city",  "name_kk": "Қала",  "name_ru": "Город",  "kato": "710" },
	    { "slug": "esil",  "name_kk": "Есіл",  "name_ru": "Есиль",  "kato": "710512" }
	  ]
	}`)
	c, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// "710512100" префиксуется обоими ("710" и "710512") → выигрывает самый длинный (самый специфичный).
	_, ru, ok := c.NameByKATO("710512100")
	if !ok || ru != "Есиль" {
		t.Fatalf("longest-prefix: ok=%v ru=%q, ожидалось Есиль", ok, ru)
	}
	// "710999" префиксуется только "710" → Город.
	_, ru2, ok2 := c.NameByKATO("710999")
	if !ok2 || ru2 != "Город" {
		t.Fatalf("prefix: ok=%v ru=%q, ожидалось Город", ok2, ru2)
	}
	// "999" не префиксуется ничем → not found (не выдумываем).
	if _, _, ok3 := c.NameByKATO("999"); ok3 {
		t.Error("NameByKATO('999') вернул ok=true — должно быть not found")
	}
}

func TestValidKATO(t *testing.T) {
	good := []string{"71", "710000000", "710512", "71051210099"}
	for _, s := range good {
		if !ValidKATO(s) {
			t.Errorf("ValidKATO(%q)=false, ожидалось true", s)
		}
	}
	// Негативный контроль: страж краснеет на не-цифрах, метасимволах LIKE и границах длины.
	bad := []string{"", "7", "710000000000", "71%", "71_", "71\\", "abc", "71 0", "-71"}
	for _, s := range bad {
		if ValidKATO(s) {
			t.Errorf("ValidKATO(%q)=true, ожидалось false (цифры-онли 2..11; метасимволы LIKE запрещены)", s)
		}
	}
}

func TestLoad_HonestFail(t *testing.T) {
	cases := map[string]string{
		"bad_version":  `{"version":"1","districts":[{"slug":"a","name_kk":"А","name_ru":"А"}]}`,
		"empty_slug":   `{"version":"v1","districts":[{"slug":"","name_kk":"А","name_ru":"А"}]}`,
		"empty_name":   `{"version":"v1","districts":[{"slug":"a","name_kk":"","name_ru":"А"}]}`,
		"dup_slug":     `{"version":"v1","districts":[{"slug":"a","name_kk":"А","name_ru":"А"},{"slug":"a","name_kk":"Б","name_ru":"Б"}]}`,
		"no_districts": `{"version":"v1","districts":[]}`,
		"bad_kato":     `{"version":"v1","districts":[{"slug":"a","name_kk":"А","name_ru":"А","kato":"71x"}]}`,
		"dup_kato":     `{"version":"v1","districts":[{"slug":"a","name_kk":"А","name_ru":"А","kato":"710"},{"slug":"b","name_kk":"Б","name_ru":"Б","kato":"710"}]}`,
		"unknown_key":  `{"version":"v1","oops":1,"districts":[{"slug":"a","name_kk":"А","name_ru":"А"}]}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			root := writeCatalog(t, body)
			if _, err := Load(root); err == nil {
				t.Errorf("Load(%s): ожидалась честная ошибка, got nil", name)
			}
		})
	}
}

func TestLoad_MissingFile(t *testing.T) {
	if _, err := Load(t.TempDir()); err == nil {
		t.Error("Load несуществующего файла: ожидалась ошибка")
	}
}
