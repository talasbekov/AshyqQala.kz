//go:build integration

// Интеграционный тест флага «монополия в регионе» (Story 4.4, AC2/AC4): пересчёт по СИНТЕТИЧЕСКИМ групповым
// агрегатам на CONTRACTOR-субъекте (organization_id) → raised/not_raised/insufficient (малая группа /
// неразрешённый БИН); авто-снятие при падении доли; идемпотентность (org_uniq — повтор не плодит дубли);
// evidence + version. Гоняется: `go test -tags=integration ./internal/store/...` (миграции 0001-0007).
package projection_test

import (
	"context"
	"encoding/json"
	"testing"

	"ashyqqala/server/internal/flags"
	"ashyqqala/server/internal/store/projection"
)

var monParamsInt = flags.Params{MethodologyVersion: "v1.0", MonopolyConcentrationShare: 0.5, MonopolyMinGroupContracts: 5}

const monKey = "direction=road|kato=710000000"

// тестовые organization_id (высокие, чтобы не пересекаться с seed).
const (
	oMono  = int64(96001) // доля 0.6 ≥ 0.5, 5 контрактов, resolved → raised
	oNorm  = int64(96002) // доля 0.3 < 0.5 → not_raised
	oSmall = int64(96003) // 4 контракта < min(5) → insufficient (флаг не строится)
	oUnres = int64(96004) // доля 0.7, но БИН не разрешён (manual/conflict) → insufficient (не строится)
)

func TestMonopolyFlag_RaiseAutoClearIdempotent(t *testing.T) {
	pool := riskFlagPool(t)
	defer pool.Close()
	ctx := context.Background()
	store := projection.NewRiskFlagStore(pool, monParamsInt)

	clean := func() {
		_, _ = pool.Exec(ctx, "DELETE FROM risk_flags WHERE organization_id = ANY($1)",
			[]int64{oMono, oNorm, oSmall, oUnres})
	}
	clean()       // перезапускаемость
	defer clean() // изоляция от других тестов

	inputs := []projection.MonopolyInput{
		{OrganizationID: oMono, SupplierBIN: "111111111111", SupplierSum: i64(600_000), GroupTotalSum: i64(1_000_000), GroupContracts: 5, SupplierBINResolved: true, ComparabilityKey: monKey},
		{OrganizationID: oNorm, SupplierBIN: "222222222222", SupplierSum: i64(300_000), GroupTotalSum: i64(1_000_000), GroupContracts: 6, SupplierBINResolved: true, ComparabilityKey: monKey},
		{OrganizationID: oSmall, SupplierBIN: "333333333333", SupplierSum: i64(900_000), GroupTotalSum: i64(1_000_000), GroupContracts: 4, SupplierBINResolved: true, ComparabilityKey: monKey},
		{OrganizationID: oUnres, SupplierBIN: "444444444444", SupplierSum: i64(700_000), GroupTotalSum: i64(1_000_000), GroupContracts: 5, SupplierBINResolved: false, ComparabilityKey: monKey},
	}
	if err := store.RecomputeMonopoly(ctx, inputs); err != nil {
		t.Fatalf("recompute #1: %v", err)
	}

	// oMono → активный contractor-флаг; evidence хранит долю/суммы; версия каноническая.
	row, ok, err := store.GetMonopoly(ctx, oMono)
	if err != nil || !ok || !row.IsActive {
		t.Fatalf("oMono: ожидался активный monopoly-флаг (ok=%v active=%v err=%v)", ok, row.IsActive, err)
	}
	if row.SubjectType != "contractor" || !row.OrganizationID.Valid || row.OrganizationID.Int64 != oMono {
		t.Errorf("oMono: ожидался subject_type=contractor, organization_id=%d; got type=%q org=%+v", oMono, row.SubjectType, row.OrganizationID)
	}
	if row.ContractID.Valid {
		t.Errorf("oMono: contract_id должен быть NULL у contractor-флага, got %+v", row.ContractID)
	}
	var ev flags.MonopolyEvidence
	if err := json.Unmarshal(row.Evidence, &ev); err != nil {
		t.Fatalf("oMono evidence невалиден: %v", err)
	}
	if ev.SupplierSum == nil || *ev.SupplierSum != 600_000 || ev.GroupTotalSum == nil || *ev.GroupTotalSum != 1_000_000 {
		t.Errorf("oMono evidence не сохранил суммы: %+v", ev)
	}
	if ev.GroupContracts != 5 || ev.ConcentrationShare != 0.5 || ev.SupplierBIN != "111111111111" || !ev.SupplierResolved {
		t.Errorf("oMono evidence неполон: %+v", ev)
	}
	if row.MethodologyVersion != "v1.0" {
		t.Errorf("oMono methodology_version=%q, ожидалось v1.0", row.MethodologyVersion)
	}

	// oNorm (ниже порога), oSmall (мало контрактов), oUnres (неразрешён) → флага НЕТ.
	for _, id := range []int64{oNorm, oSmall, oUnres} {
		if _, ok, _ := store.GetMonopoly(ctx, id); ok {
			t.Errorf("org %d: monopoly-флаг не должен существовать (not_raised/insufficient)", id)
		}
	}
	if n, _ := store.CountActiveMonopoly(ctx); n != 1 {
		t.Fatalf("после recompute #1 активных monopoly-флагов = %d, ожидалось 1 (только oMono)", n)
	}

	// ИДЕМПОТЕНТНОСТЬ: повтор того же входа → по-прежнему 1 активный (org_uniq, без дублей).
	if err := store.RecomputeMonopoly(ctx, inputs); err != nil {
		t.Fatalf("recompute #2 (идемпотентность): %v", err)
	}
	if n, _ := store.CountActiveMonopoly(ctx); n != 1 {
		t.Fatalf("после повторного recompute активных = %d, ожидалось 1 (org_uniq)", n)
	}

	// АВТО-СНЯТИЕ: доля oMono падает до 0.3 (< 0.5) → not_raised → флаг снят (is_active=false, cleared_at).
	if err := store.RecomputeMonopoly(ctx, []projection.MonopolyInput{
		{OrganizationID: oMono, SupplierBIN: "111111111111", SupplierSum: i64(300_000), GroupTotalSum: i64(1_000_000), GroupContracts: 5, SupplierBINResolved: true, ComparabilityKey: monKey},
	}); err != nil {
		t.Fatalf("recompute auto-clear: %v", err)
	}
	row, ok, _ = store.GetMonopoly(ctx, oMono)
	if !ok || row.IsActive || !row.ClearedAt.Valid {
		t.Fatalf("oMono: ожидался снятый флаг (is_active=false, cleared_at), got active=%v cleared=%v", row.IsActive, row.ClearedAt.Valid)
	}
	if n, _ := store.CountActiveMonopoly(ctx); n != 0 {
		t.Fatalf("после авто-снятия активных monopoly-флагов = %d, ожидалось 0", n)
	}
}
