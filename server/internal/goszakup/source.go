// Package goszakup — клиент ows_v2 + Source (точка swap ows|file|scrape). Story 2.0: интерфейс Source +
// file-реализация (ТОКЕН-НЕЗАВИСИМО). Боевая ows-реализация (живой ows_v2, нужен GOSZAKUP_TOKEN) — Story 2.1.
// Граница AR-27: этот пакет импортируется ТОЛЬКО из internal/ingest/decode (go-list-страж internal/arch).
package goszakup

// Resource — логический ресурс ows_v2. Точные REST-пути — в ows-реализации (Story 2.1).
type Resource string

const (
	ResourceContract Resource = "contract"
	ResourceLots     Resource = "lots"
	ResourceTrdBuy   Resource = "trd-buy"
	ResourceRNU      Resource = "rnu"
	ResourceJournal  Resource = "journal"
)

// Source — абстракция источника данных (единая точка swap scrape→ows|file). Сигнатура совместима с
// stage0-audit/source.go (концепт перенесён; модуль stage0 НЕ импортируется). Реализации:
//   - FileSource (Story 2.0) — локальные дампы, без токена;
//   - owsSource (Story 2.1) — живой ows_v2 по Bearer-токену.
type Source interface {
	Name() string
	// Fetch отдаёт записи ресурса в handle (пачкой). max<=0 — без лимита; scopeBINs — фильтр по БИН (для ows).
	Fetch(resource string, scopeBINs []string, max int, handle func(items []map[string]any) error) error
}
