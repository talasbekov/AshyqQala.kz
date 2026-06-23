package decode_test

import (
	"errors"
	"testing"

	"ashyqqala/server/internal/ingest/decode"
)

// goldenLotSchemaHash — ЗАМОРОЖЕННЫЙ эталон хеша набора полей lot.json (ловит дрейф алгоритма SchemaHash).
const goldenLotSchemaHash = "0dcd872b0001371952b0046ae6fbff89954c45ee4d72644519a5eb53767a4b20"

func TestDecodeLot_Golden(t *testing.T) {
	obj := loadGolden(t, "lot.json")
	pinned := decode.SchemaHash(keys(obj))
	if pinned != goldenLotSchemaHash {
		t.Fatalf("schema_hash lot = %s, замороженный эталон %s (алгоритм изменился?)", pinned, goldenLotSchemaHash)
	}

	l, err := decode.DecodeLot(obj, pinned)
	if err != nil {
		t.Fatalf("декод golden-лота: %v", err)
	}
	if l.GoszakupLotID != "LOT-0001" {
		t.Errorf("GoszakupLotID = %q, ожидалось LOT-0001", l.GoszakupLotID)
	}
	if l.TitleRu == "" || l.TitleKk == "" {
		t.Errorf("title пуст: ru=%q kk=%q", l.TitleRu, l.TitleKk)
	}
	if l.Amount == nil || *l.Amount != 240000000 {
		t.Errorf("Amount = %v, ожидалось 240000000", l.Amount)
	}
	if l.KatoCode != "710000000" {
		t.Errorf("KatoCode = %q, ожидалось 710000000", l.KatoCode)
	}
}

func TestDecodeLot_Drift_StopNotSilentZero(t *testing.T) {
	pinned := decode.SchemaHash(keys(loadGolden(t, "lot.json")))
	drifted := loadGolden(t, "lot_drifted.json") // amount → amount_total: набор полей изменился

	_, err := decode.DecodeLot(drifted, pinned)
	var d decode.ErrSchemaDrift
	if !errors.As(err, &d) {
		t.Fatalf("дрейф схемы lot: ожидался ErrSchemaDrift (стоп, не тихий 0), получено %v", err)
	}
}
