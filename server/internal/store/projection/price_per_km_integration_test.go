//go:build integration

// Интеграционный тест флага «аномальная цена за км» (Story 4.3, AC2/AC4): медиана сидится в кэш
// price_benchmarks (BenchmarkStore, 4.1) → RecomputePricePerKM читает её → raised/not_raised/insufficient +
// авто-снятие. Гоняется: `go test -tags=integration ./internal/store/...` (миграции 0001-0007).
package projection_test

import (
	"context"
	"encoding/json"
	"testing"

	"ashyqqala/server/internal/flags"
	"ashyqqala/server/internal/store/projection"
)

var ppkmParamsInt = flags.Params{MethodologyVersion: "v1.0", MinSample: 5, PricePerKMDeviationFactor: 1.5}

const (
	ppkmKey = "direction=road|kato=710000000"
	cAnom   = int64(95001) // цена 2M > порог 1.5M → raised
	cNorm   = int64(95002) // цена 1.2M < порог → not_raised
	cNoMed  = int64(95003) // ключ без медианы → insufficient (флаг не строится)
)

func TestPricePerKMFlag_RaiseAutoClear(t *testing.T) {
	pool := riskFlagPool(t)
	defer pool.Close()
	ctx := context.Background()
	bench := projection.NewBenchmarkStore(pool, ppkmParamsInt)
	store := projection.NewRiskFlagStore(pool, ppkmParamsInt)

	clean := func() {
		_, _ = pool.Exec(ctx, "DELETE FROM risk_flags WHERE contract_id = ANY($1)", []int64{cAnom, cNorm, cNoMed})
		_ = bench.ReplaceSnapshot(ctx, nil) // очистить кэш медиан
	}
	clean()
	defer clean()

	// Seed медиану группы road×Astana: median 1_000_000, sample 5 (ok). cNoMed-ключа (water) НЕТ → нет медианы.
	if err := bench.ReplaceSnapshot(ctx, []projection.BenchmarkRow{
		{ComparabilityKey: ppkmKey, MedianPricePerKM: i64(1_000_000), SampleSize: 5},
	}); err != nil {
		t.Fatalf("seed median: %v", err)
	}

	inputs := []projection.PricePerKMInput{
		{ContractID: cAnom, PricePerKM: i64(2_000_000), ComparabilityKey: ppkmKey},
		{ContractID: cNorm, PricePerKM: i64(1_200_000), ComparabilityKey: ppkmKey},
		{ContractID: cNoMed, PricePerKM: i64(2_000_000), ComparabilityKey: "direction=water|kato=710000000"},
	}
	if err := store.RecomputePricePerKM(ctx, bench, inputs); err != nil {
		t.Fatalf("recompute #1: %v", err)
	}

	// cAnom → активный флаг; evidence хранит цену/медиану; версия каноническая.
	row, ok, err := store.GetPricePerKM(ctx, cAnom)
	if err != nil || !ok || !row.IsActive {
		t.Fatalf("cAnom: ожидался активный price-флаг (ok=%v active=%v err=%v)", ok, row.IsActive, err)
	}
	var ev flags.PricePerKMEvidence
	if err := json.Unmarshal(row.Evidence, &ev); err != nil {
		t.Fatalf("cAnom evidence невалиден: %v", err)
	}
	if ev.PricePerKM == nil || *ev.PricePerKM != 2_000_000 || ev.Median == nil || *ev.Median != 1_000_000 {
		t.Errorf("cAnom evidence не сохранил цену/медиану: %+v", ev)
	}
	if row.MethodologyVersion != "v1.0" {
		t.Errorf("cAnom methodology_version=%q, ожидалось v1.0", row.MethodologyVersion)
	}

	// cNorm (ниже порога) и cNoMed (нет медианы → insufficient) → флага НЕТ.
	for _, id := range []int64{cNorm, cNoMed} {
		if _, ok, _ := store.GetPricePerKM(ctx, id); ok {
			t.Errorf("contract %d: price-флаг не должен существовать (not_raised/insufficient)", id)
		}
	}
	if n, _ := store.CountActivePricePerKM(ctx); n != 1 {
		t.Fatalf("после recompute #1 активных price-флагов = %d, ожидалось 1 (только cAnom)", n)
	}

	// АВТО-СНЯТИЕ: цена cAnom падает до 1.2M (< порог) → not_raised → флаг снят (is_active=false, cleared_at).
	if err := store.RecomputePricePerKM(ctx, bench, []projection.PricePerKMInput{
		{ContractID: cAnom, PricePerKM: i64(1_200_000), ComparabilityKey: ppkmKey},
	}); err != nil {
		t.Fatalf("recompute auto-clear: %v", err)
	}
	row, ok, _ = store.GetPricePerKM(ctx, cAnom)
	if !ok || row.IsActive || !row.ClearedAt.Valid {
		t.Fatalf("cAnom: ожидался снятый флаг (is_active=false, cleared_at), got active=%v cleared=%v", row.IsActive, row.ClearedAt.Valid)
	}
	if n, _ := store.CountActivePricePerKM(ctx); n != 0 {
		t.Fatalf("после авто-снятия активных price-флагов = %d, ожидалось 0", n)
	}
}
