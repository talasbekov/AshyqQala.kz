package arch_test

import (
	"os/exec"
	"strings"
	"testing"
)

// pureCorePackages — чистое ядро (AR-13/AR-27): чистые функции без store/IO/реального времени.
var pureCorePackages = []string{
	"ashyqqala/server/internal/median",
	"ashyqqala/server/internal/flags",
	"ashyqqala/server/internal/normalize",
	"ashyqqala/server/internal/benchmark",
}

// forbidden — запрещённые ПРЯМЫЕ импорты чистого ядра. "time" — реальное время (только через
// internal/clock); store/httpapi/goszakup — слои БД/доставки; драйверы pgx/chi.
var forbidden = []string{
	"ashyqqala/server/internal/store",
	"ashyqqala/server/internal/httpapi",
	"ashyqqala/server/internal/goszakup",
	"time",
	"github.com/jackc/pgx",
	"github.com/go-chi/chi",
}

// isForbidden — чистый предикат запрета для ПРЯМОГО импорта (точное имя или префикс пакета). Вынесен
// отдельно, чтобы negative-control тест доказал: страж умеет КРАСНЕТЬ (различает запрещённое/разрешённое).
func isForbidden(imp string) bool {
	for _, bad := range forbidden {
		if imp == bad || strings.HasPrefix(imp, bad+"/") {
			return true
		}
	}
	return false
}

// TestCoreImportBoundaries — исполняемая граница (AR-27): красный при нарушении. Проверяет ПРЯМЫЕ
// импорты (.Imports, не транзитивные .Deps) каждого чистого пакета через `go list`.
//
// ВАЖНО: результат `exec go list` НЕ входит в инпуты тест-кэша → при тёплом кэше нарушение границы
// в ядре маскируется (`ok (cached)`). Поэтому в CI этот пакет гоняется ОТДЕЛЬНЫМ шагом с `-count=1`
// (ci-server.yml), а локально — `make check-core` (тоже `-count=1`). Эмпирически проверено: под
// `-count=1` внедрённый `import "time"` в median даёт FAIL, под обычным `go test` — нет.
func TestCoreImportBoundaries(t *testing.T) {
	for _, pkg := range pureCorePackages {
		out, err := exec.Command("go", "list", "-f", "{{ range .Imports }}{{ . }}\n{{ end }}", pkg).CombinedOutput()
		if err != nil {
			t.Fatalf("go list %s: %v\n%s", pkg, err, out)
		}
		for imp := range strings.FieldsSeq(string(out)) {
			if isForbidden(imp) {
				t.Errorf("ядро %s импортирует запрещённое %q (AR-27: чистота = пересчитываемость без БД; время — через clock)", pkg, imp)
			}
		}
	}
}

// TestIsForbidden — negative/positive control: страж реально различает запрещённое и разрешённое,
// т.е. способен покраснеть (а не «всегда зелёный из-за сломанной логики предиката»). В отличие от
// TestCoreImportBoundaries, этот тест чист (без exec) → кэшируется и стабилен.
func TestIsForbidden(t *testing.T) {
	forbid := []string{
		"time", "time/tzdata",
		"ashyqqala/server/internal/store", "ashyqqala/server/internal/store/curation",
		"ashyqqala/server/internal/httpapi", "ashyqqala/server/internal/goszakup",
		"github.com/jackc/pgx/v5", "github.com/go-chi/chi/v5",
	}
	for _, imp := range forbid {
		if !isForbidden(imp) {
			t.Errorf("isForbidden(%q) = false, ожидалось true", imp)
		}
	}
	allow := []string{
		"slices", "strings", "crypto/sha256",
		"ashyqqala/server/internal/registry", "ashyqqala/server/internal/clock",
		"timezone-not-time", // содержит "time", но не пакет time
	}
	for _, imp := range allow {
		if isForbidden(imp) {
			t.Errorf("isForbidden(%q) = true, ожидалось false", imp)
		}
	}
}
