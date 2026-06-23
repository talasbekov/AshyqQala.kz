package decode_test

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"ashyqqala/server/internal/ingest/decode"
)

func loadGolden(t *testing.T, name string) map[string]any {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("чтение %s: %v", name, err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("парс %s: %v", name, err)
	}
	return m
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestSchemaHash_OrderIndependent(t *testing.T) {
	if decode.SchemaHash([]string{"b", "a", "c"}) != decode.SchemaHash([]string{"c", "b", "a"}) {
		t.Fatal("schema_hash зависит от порядка полей (должен быть key-sorted)")
	}
	if decode.SchemaHash([]string{"a"}) == decode.SchemaHash([]string{"a", "b"}) {
		t.Fatal("разные наборы полей дали одинаковый schema_hash")
	}
}

// goldenSchemaHash — ЗАМОРОЖЕННЫЙ эталон хеша набора полей contract.json. Пин ловит дрейф самого
// алгоритма SchemaHash (тест, пересчитывающий pinned из входа, изменение алгоритма не заметил бы).
const goldenSchemaHash = "a98d9070b33119db1f705effe9e3f4664b0f18535001e179ba35553dc4dc0d11"

func TestSchemaHash_GoldenPinned(t *testing.T) {
	got := decode.SchemaHash(keys(loadGolden(t, "contract.json")))
	if got != goldenSchemaHash {
		t.Fatalf("schema_hash golden = %s, замороженный эталон %s (алгоритм SchemaHash изменился?)", got, goldenSchemaHash)
	}
}

func TestDecode_Golden(t *testing.T) {
	obj := loadGolden(t, "contract.json")
	pinned := decode.SchemaHash(keys(obj))

	c, err := decode.Decode(obj, pinned)
	if err != nil {
		t.Fatalf("декод golden-объекта: %v", err)
	}
	// golden-снапшот ожидаемого вывода декодера:
	if c.GoszakupContractID != "DEMO-0001" {
		t.Errorf("GoszakupContractID = %q, ожидалось DEMO-0001", c.GoszakupContractID)
	}
	if c.AmountTng == nil || *c.AmountTng != 123456789 {
		t.Errorf("AmountTng = %v, ожидалось 123456789", c.AmountTng)
	}
}

func TestDecode_Drift_StopNotSilentZero(t *testing.T) {
	pinned := decode.SchemaHash(keys(loadGolden(t, "contract.json")))
	drifted := loadGolden(t, "drifted.json") // поле amount переименовано → набор полей изменился

	_, err := decode.Decode(drifted, pinned)
	var drift decode.ErrSchemaDrift
	if !errors.As(err, &drift) {
		t.Fatalf("дрейф схемы: ожидался ErrSchemaDrift (стоп+алерт, не тихий 0), получено err=%v", err)
	}
}
