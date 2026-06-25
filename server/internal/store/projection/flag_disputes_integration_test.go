//go:build integration

// Интеграционный тест flag_disputes (Story 4.6, AC3): статус-переходы верификации (raised→confirmed/withdrawn,
// resolved_at-инвариант) + SM-C1 DB-derived из flag_disputes (повтор/откат статуса не двоит срез) + counts.
// Гоняется: `go test -tags=integration ./internal/store/...` (миграции 0001-0009).
package projection_test

import (
	"context"
	"testing"

	"ashyqqala/server/internal/store/projection"
)

const (
	fdFlag1 = int64(98001) // raised → confirmed (корректный)
	fdFlag2 = int64(98002) // withdrawn (некорректный)
)

func TestFlagDisputes_TransitionsAndSMC1(t *testing.T) {
	pool := riskFlagPool(t)
	defer pool.Close()
	ctx := context.Background()
	store := projection.NewFlagDisputeStore(pool)

	clean := func() {
		_, _ = pool.Exec(ctx, "DELETE FROM flag_disputes WHERE risk_flag_id = ANY($1)", []int64{fdFlag1, fdFlag2})
	}
	clean()
	defer clean()

	before, err := store.SMC1Counts(ctx)
	if err != nil {
		t.Fatalf("SMC1Counts before: %v", err)
	}

	// flag1: raised → resolved_at NULL (не финальный).
	if err := store.RecordDispute(ctx, projection.DisputeInput{RiskFlagID: fdFlag1, FlagType: "price_per_km", SubjectType: "contract", SubjectID: 111, Status: projection.DisputeRaised}); err != nil {
		t.Fatalf("raised: %v", err)
	}
	d, ok, err := store.Get(ctx, fdFlag1)
	if err != nil || !ok || d.Status != projection.DisputeRaised || d.ResolvedAt.Valid {
		t.Fatalf("raised: ожидался status=raised, resolved_at NULL; got %+v (ok=%v err=%v)", d, ok, err)
	}

	// flag1: confirmed → resolved_at задан; SM-C1 reviewed++ (корректный, incorrect не двигается).
	if err := store.RecordDispute(ctx, projection.DisputeInput{RiskFlagID: fdFlag1, FlagType: "price_per_km", SubjectType: "contract", SubjectID: 111, Status: projection.DisputeConfirmed}); err != nil {
		t.Fatalf("confirmed: %v", err)
	}
	d, _, _ = store.Get(ctx, fdFlag1)
	if d.Status != projection.DisputeConfirmed || !d.ResolvedAt.Valid {
		t.Fatalf("confirmed: ожидался status=confirmed, resolved_at задан; got %+v", d)
	}

	// flag2: withdrawn (некорректный) → reviewed++ + incorrect++.
	if err := store.RecordDispute(ctx, projection.DisputeInput{RiskFlagID: fdFlag2, FlagType: "monopoly", SubjectType: "contractor", SubjectID: 222, Status: projection.DisputeWithdrawn}); err != nil {
		t.Fatalf("withdrawn: %v", err)
	}

	// ИДЕМПОТЕНТНОСТЬ метрики: повтор confirmed для flag1 → НЕ двоит reviewed (инкремент только на переходе).
	if err := store.RecordDispute(ctx, projection.DisputeInput{RiskFlagID: fdFlag1, FlagType: "price_per_km", SubjectType: "contract", SubjectID: 111, Status: projection.DisputeConfirmed}); err != nil {
		t.Fatalf("confirmed повтор: %v", err)
	}

	after, err := store.SMC1Counts(ctx)
	if err != nil {
		t.Fatalf("SMC1Counts after: %v", err)
	}
	if got := after.Reviewed - before.Reviewed; got != 2 {
		t.Fatalf("reviewed Δ = %v, ожидалось 2 (confirmed flag1 + withdrawn flag2; DB-derived/повтор НЕ двоит)", got)
	}
	if got := after.Incorrect - before.Incorrect; got != 1 {
		t.Fatalf("incorrect Δ = %v, ожидалось 1 (только withdrawn flag2)", got)
	}

	if n, _ := store.CountByStatus(ctx, projection.DisputeConfirmed); n != 1 {
		t.Errorf("confirmed count = %d, ожидалось 1", n)
	}
	if n, _ := store.CountByStatus(ctx, projection.DisputeWithdrawn); n != 1 {
		t.Errorf("withdrawn count = %d, ожидалось 1", n)
	}
}
