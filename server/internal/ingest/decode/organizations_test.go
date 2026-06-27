package decode_test

import (
	"testing"

	"ashyqqala/server/internal/goszakup"
	"ashyqqala/server/internal/ingest/decode"
)

// Golden schema_hash фикстур testdata/orgs (набор ключей записи). Дрейф ключей → красный тест (как lots).
const (
	goldenContractOrgsHash = "9e6463f619991ba9d7944a3968006dcab3d2adb9bd05f679871112efcec0737f"
	goldenTrdBuyHash       = "554289cda264df36bf44036ef640ef060be561e33d9864aa50e1cd13a62e78f8"
)

// TestDecodeOrgsFromContracts — извлечение заказчика+подрядчика из каждой записи contract (file, без токена).
func TestDecodeOrgsFromContracts(t *testing.T) {
	orgs, err := decode.DecodeOrgsFromContracts(goszakup.NewFileSource("testdata/orgs"), goldenContractOrgsHash, 0)
	if err != nil {
		t.Fatalf("DecodeOrgsFromContracts: %v", err)
	}
	// 2 контракта × (заказчик + подрядчик) = 4 появления (дедуп — забота orgnorm, не декода).
	if len(orgs) != 4 {
		t.Fatalf("ожидалось 4 появления организаций, получено %d", len(orgs))
	}
	// Первая запись: заказчик 111…, подрядчик 222….
	cust, supp := orgs[0], orgs[1]
	if cust.BIN != "111111111111" || !cust.IsCustomer || cust.IsSupplier {
		t.Errorf("заказчик: bin=%q is_customer=%v is_supplier=%v", cust.BIN, cust.IsCustomer, cust.IsSupplier)
	}
	if cust.NameRu != "Акимат Астаны" || cust.NameKk != "Астана әкімдігі" {
		t.Errorf("заказчик имена: ru=%q kk=%q", cust.NameRu, cust.NameKk)
	}
	if supp.BIN != "222222222222" || supp.IsCustomer || !supp.IsSupplier {
		t.Errorf("подрядчик: bin=%q is_customer=%v is_supplier=%v", supp.BIN, supp.IsCustomer, supp.IsSupplier)
	}
	if supp.RegKato != "710000000" {
		t.Errorf("подрядчик kato=%q, ожидалось 710000000", supp.RegKato)
	}
}

// TestDecodeOrgsFromTrdBuy — извлечение участника; победитель помечается подрядчиком (is_supplier).
func TestDecodeOrgsFromTrdBuy(t *testing.T) {
	orgs, err := decode.DecodeOrgsFromTrdBuy(goszakup.NewFileSource("testdata/orgs"), goldenTrdBuyHash, 0)
	if err != nil {
		t.Fatalf("DecodeOrgsFromTrdBuy: %v", err)
	}
	if len(orgs) != 2 {
		t.Fatalf("ожидалось 2 участника, получено %d", len(orgs))
	}
	if !orgs[0].IsSupplier {
		t.Errorf("участник-победитель должен быть is_supplier=true (bin=%q)", orgs[0].BIN)
	}
	if orgs[1].IsSupplier {
		t.Errorf("участник-непобедитель должен быть is_supplier=false (bin=%q)", orgs[1].BIN)
	}
}

// TestDecodeOrgs_SchemaDrift — дрейф зафиксированного контракта схемы → ErrSchemaDrift (стоп, не тихий 0).
func TestDecodeOrgs_SchemaDrift(t *testing.T) {
	_, err := decode.DecodeOrgsFromContracts(goszakup.NewFileSource("testdata/orgs"), "deadbeef", 0)
	if err == nil {
		t.Fatal("ожидался ErrSchemaDrift при неверном pinnedSchemaHash, got nil")
	}
	if _, ok := err.(decode.ErrSchemaDrift); !ok {
		t.Fatalf("ожидался decode.ErrSchemaDrift, got %T (%v)", err, err)
	}
}
