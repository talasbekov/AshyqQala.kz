//go:build scrape

package scrape

import (
	"os"
	"strings"
	"testing"

	"ashyqqala/server/internal/ingest/decode"
)

// goldenScrapeSchemaHash — ЗАМОРОЖЕННЫЙ хеш набора полей scrape-записи (RecordKeys). Ловит дрейф:
// если scrape начнёт эмитить другой набор полей, decode.DecodeLot вернёт ErrSchemaDrift, а этот
// golden — провалится первым (явно покажет смену контракта). Значение — из decode.SchemaHash.
const goldenScrapeSchemaHash = "548e4755d6836e7b91d6bce8b58e33e5b89337a86e423b49c48b7b56d3af5ca9"

func loadFixture(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("testdata/search-result.html")
	if err != nil {
		t.Fatalf("чтение фикстуры: %v", err)
	}
	return string(b)
}

// TestParseRows_Fixture — детерминированный разбор записанной HTML-страницы (без сети).
func TestParseRows_Fixture(t *testing.T) {
	s := New("", "", 0, 0)
	seen := map[string]bool{}
	rows := s.parseRows(loadFixture(t), AstanaKATO, seen)

	if len(rows) != 2 {
		t.Fatalf("ожидалось 2 лота из фикстуры (служебная строка отброшена), получено %d: %+v", len(rows), rows)
	}

	r0 := rows[0]
	if got := r0["name_ru"]; got != "Реконструкция автодороги по ул. Абая" {
		t.Errorf("name_ru[0] = %q (ожидалась обрезка хвоста «История»)", got)
	}
	if got := r0["trd_buy_number_anno"]; got != "123456-1" {
		t.Errorf("anno[0] = %q, ожидалось 123456-1", got)
	}
	if got, ok := r0["amount"].(float64); !ok || got != 240000000 {
		t.Errorf("amount[0] = %v (ok=%v), ожидалось 240000000", r0["amount"], ok)
	}
	if got := r0["ref_kato"]; got != AstanaKATO {
		t.Errorf("ref_kato[0] = %q, ожидалось %s", got, AstanaKATO)
	}
	if id, _ := r0["id"].(string); !strings.HasPrefix(id, "scrape-") {
		t.Errorf("id[0] = %q, ожидался префикс scrape-", id)
	}

	r1 := rows[1]
	if got := r1["name_ru"]; got != "Капитальный ремонт водопровода мкр. Сарыарка" {
		t.Errorf("name_ru[1] = %q", got)
	}
	if got, ok := r1["amount"].(float64); !ok || got != 89500000 {
		t.Errorf("amount[1] = %v (ok=%v), ожидалось 89500000", r1["amount"], ok)
	}

	// Дедуп: повторный разбор той же страницы с общим seen → 0 новых записей.
	if again := s.parseRows(loadFixture(t), AstanaKATO, seen); len(again) != 0 {
		t.Errorf("дедуп: повторный разбор дал %d записей, ожидалось 0", len(again))
	}
}

// TestSyntheticLotID_Stable — натуральный ключ стабилен (идемпотентность UPSERT) и различает лоты.
func TestSyntheticLotID_Stable(t *testing.T) {
	a := syntheticLotID("123456-1", "Реконструкция автодороги")
	b := syntheticLotID("123456-1", "Реконструкция автодороги")
	if a != b {
		t.Errorf("один и тот же лот → разные id: %q ≠ %q (ломает идемпотентность)", a, b)
	}
	if !strings.HasPrefix(a, "scrape-") {
		t.Errorf("id = %q, ожидался префикс scrape-", a)
	}
	if c := syntheticLotID("999999-9", "Реконструкция автодороги"); c == a {
		t.Errorf("разные объявления дали одинаковый id %q", c)
	}
}

func TestParseMoney(t *testing.T) {
	ok := map[string]float64{
		"240 000 000,00": 240000000,
		"89 500 000":     89500000,
		"1 234,56":       1234.56,
		"1,234,567.89":   1234567.89,
	}
	for in, want := range ok {
		if got, ok := parseMoney(in); !ok || got != want {
			t.Errorf("parseMoney(%q) = %v,%v, ожидалось %v,true", in, got, ok, want)
		}
	}
	// Нераспознаваемое → ok=false (вызывающий → amount=nil/NULL, НЕ 0): гардрейл честности.
	for _, in := range []string{"", "-", "н/д", "1.234.567"} {
		if got, ok := parseMoney(in); ok {
			t.Errorf("parseMoney(%q) = %v,true, ожидалось ok=false (→ NULL, не 0)", in, got)
		}
	}
}

// TestParseRows_UnparseableAmount_NULL — гардрейл честности: нераспознанная сумма → amount=nil (→ NULL),
// НЕ 0; при этом ключ "amount" остаётся → schema_hash стабилен, decode даёт Amount=nil.
func TestParseRows_UnparseableAmount_NULL(t *testing.T) {
	html := `<table id="search-result"><tr><td>555555-5</td><td>Лот без суммы</td><td>1</td><td>-</td><td>x</td><td>y</td></tr></table>`
	rows := New("", "", 0, 0).parseRows(html, AstanaKATO, map[string]bool{})
	if len(rows) != 1 {
		t.Fatalf("ожидалась 1 запись, получено %d", len(rows))
	}
	if rows[0]["amount"] != nil {
		t.Errorf("amount = %v, ожидалось nil (нераспознанная сумма → NULL, не 0)", rows[0]["amount"])
	}
	lot, err := decode.DecodeLot(rows[0], decode.SchemaHash(RecordKeys))
	if err != nil {
		t.Fatalf("decode записи с nil-amount: %v (ключ amount должен сохраняться → schema_hash стабилен)", err)
	}
	if lot.Amount != nil {
		t.Errorf("Amount = %v, ожидалось nil (NULL)", *lot.Amount)
	}
}

// TestScrapeSchemaHash_RedOnDrift — negative-control golden: ИНОЙ набор полей → ИНОЙ хеш, т.е. страж
// schema_hash способен покраснеть (а не «всегда равен golden»). См. [[guards-must-prove-red]].
func TestScrapeSchemaHash_RedOnDrift(t *testing.T) {
	withExtra := append(append([]string(nil), RecordKeys...), "extra_field")
	if decode.SchemaHash(withExtra) == goldenScrapeSchemaHash {
		t.Error("набор полей +1 дал тот же schema_hash — страж дрейфа не различает изменение")
	}
	if decode.SchemaHash(RecordKeys[:len(RecordKeys)-1]) == goldenScrapeSchemaHash {
		t.Error("набор полей −1 дал тот же schema_hash — страж дрейфа не различает изменение")
	}
}

// TestScrapeRecord_DecodesToLot — scrape-запись проходит ТОТ ЖЕ decode, что и боевой ows (AC1/AC3):
// schema_hash совпадает с golden, DecodeLot заполняет Lot (GoszakupLotID = синтетический id).
func TestScrapeRecord_DecodesToLot(t *testing.T) {
	hash := decode.SchemaHash(RecordKeys)
	if hash != goldenScrapeSchemaHash {
		t.Fatalf("schema_hash scrape = %s, замороженный golden %s (контракт полей изменился?)", hash, goldenScrapeSchemaHash)
	}

	rows := New("", "", 0, 0).parseRows(loadFixture(t), AstanaKATO, map[string]bool{})
	if len(rows) == 0 {
		t.Fatal("нет записей из фикстуры")
	}
	lot, err := decode.DecodeLot(rows[0], hash)
	if err != nil {
		t.Fatalf("decode scrape-записи: %v", err)
	}
	if !strings.HasPrefix(lot.GoszakupLotID, "scrape-") {
		t.Errorf("GoszakupLotID = %q, ожидался синтетический scrape-ключ", lot.GoszakupLotID)
	}
	if lot.TitleRu != "Реконструкция автодороги по ул. Абая" {
		t.Errorf("TitleRu = %q", lot.TitleRu)
	}
	if lot.Amount == nil || *lot.Amount != 240000000 {
		t.Errorf("Amount = %v, ожидалось 240000000", lot.Amount)
	}
	if lot.KatoCode != AstanaKATO {
		t.Errorf("KatoCode = %q, ожидалось %s", lot.KatoCode, AstanaKATO)
	}
	// Честность: отсутствующих полей (title_kk) scrape не выдумывает → пусто (→ NULL в проекции).
	if lot.TitleKk != "" {
		t.Errorf("TitleKk = %q, ожидалось пусто (scrape не отдаёт kk)", lot.TitleKk)
	}
}
