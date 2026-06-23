package decode

import "ashyqqala/server/internal/goszakup"

// DecodeLotsFrom тянет лоты из источника (file/ows) и декодирует каждый при совпадении schema_hash;
// дрейф любого лота → ErrSchemaDrift (стоп, не тихий 0). `decode` — ЕДИНСТВЕННЫЙ пакет, импортирующий
// goszakup (AR-27, go-list-страж). Источник file — Story 2.0 (без токена); ows — Story 2.1.
func DecodeLotsFrom(src goszakup.Source, pinnedSchemaHash string, max int) ([]Lot, error) {
	var out []Lot
	err := src.Fetch(string(goszakup.ResourceLots), nil, max, func(items []map[string]any) error {
		for _, raw := range items {
			l, err := DecodeLot(raw, pinnedSchemaHash)
			if err != nil {
				return err
			}
			out = append(out, l)
		}
		return nil
	})
	return out, err
}

// Lot — доменная проекция лота ows_v2 (КАРКАС S-0; БЕЗ flags/geo). Полная модель — Epic 2.
// ПРИМЕЧАНИЕ: колонки quantity/unit/announcement_id ЕСТЬ в проекции lots (миграция 0003) и в UpsertLot,
// но S-0-декод их НАМЕРЕННО не читает (как Contract несёт лишь id+amount) — наполняются в Epic 2, не баг декода.
type Lot struct {
	GoszakupLotID string
	TitleRu       string
	TitleKk       string
	Amount        *int64
	KatoCode      string
}

// lotFieldCandidates — логическое поле → кандидаты реальных имён ows_v2 (первый присутствующий побеждает).
// Концепт fieldCandidates перенесён из stage0-audit (модуль не импортируется).
var lotFieldCandidates = map[string][]string{
	"goszakup_lot_id": {"lot_id", "id"},
	"title_ru":        {"name_ru", "lot_name_ru", "descr_ru"},
	"title_kk":        {"name_kk", "lot_name_kk"},
	"amount":          {"amount", "summ", "sum", "total_sum"},
	"kato_code":       {"ref_kato", "kato", "kato_code", "delivery_kato", "ref_kato_id"},
}

// DecodeLot декодирует raw-лот ТОЛЬКО при совпадении schema_hash с зафиксированным контрактом; дрейф
// набора полей → ErrSchemaDrift (стоп+алерт, НЕ тихий 0). Живой ows_v2 — Story 2.1.
func DecodeLot(raw map[string]any, pinnedSchemaHash string) (Lot, error) {
	if got := SchemaHash(keysOf(raw)); got != pinnedSchemaHash {
		return Lot{}, ErrSchemaDrift{Expected: pinnedSchemaHash, Got: got}
	}
	var l Lot
	l.GoszakupLotID = lotString(raw, "goszakup_lot_id")
	l.TitleRu = lotString(raw, "title_ru")
	l.TitleKk = lotString(raw, "title_kk")
	if n, ok := lotInt(raw, "amount"); ok {
		l.Amount = &n
	}
	l.KatoCode = lotString(raw, "kato_code")
	return l, nil
}

func lotPick(raw map[string]any, logical string) (any, bool) {
	for _, k := range lotFieldCandidates[logical] {
		if v, ok := raw[k]; ok && v != nil {
			return v, true
		}
	}
	return nil, false
}

func lotString(raw map[string]any, logical string) string {
	if v, ok := lotPick(raw, logical); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func lotInt(raw map[string]any, logical string) (int64, bool) {
	if v, ok := lotPick(raw, logical); ok {
		switch n := v.(type) {
		case float64: // JSON-числа после json.Unmarshal
			return int64(n), true
		case int64:
			return n, true
		case int:
			return int64(n), true
		}
	}
	return 0, false
}
