//go:build integration

// Интеграционный тест risk_flags (Story 4.2, AC2/AC3): пересчёт флага «единственный участник» на
// СИНТЕТИЧЕСКИХ участниках → raised; авто-снятие при смене данных; идемпотентность; evidence + version.
// Гоняется: `go test -tags=integration ./internal/store/...` с DATABASE_URL (миграции 0001-0007).
package projection_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/flags"
	"ashyqqala/server/internal/store/projection"
)

func riskFlagPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL не задан — пропуск интеграционного теста")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("подключение к БД: %v", err)
	}
	return pool
}

func cnt(n int) *int { return &n }

// rfParams — канонические methodology_params для теста (single-participant включён, один исключённый способ).
var rfParams = flags.Params{
	MethodologyVersion:              "v1.0",
	SingleParticipantEnabled:        true,
	SingleParticipantExcludeMethods: []string{"из_одного_источника"},
}

// тестовые contract_id (высокие, чтобы не пересекаться с seed).
const (
	cRaise  = int64(94001) // 1 участник, способ не исключён → raised
	cExcl   = int64(94002) // 1 участник, способ исключён → not_raised
	cMany   = int64(94003) // 3 участника → not_raised
	cNoData = int64(94004) // нет данных → insufficient
)

func TestSingleParticipantFlag_RaiseAutoClearIdempotent(t *testing.T) {
	pool := riskFlagPool(t)
	defer pool.Close()
	ctx := context.Background()
	store := projection.NewRiskFlagStore(pool, rfParams)

	clean := func() {
		_, _ = pool.Exec(ctx, "DELETE FROM risk_flags WHERE contract_id = ANY($1)",
			[]int64{cRaise, cExcl, cMany, cNoData})
	}
	clean()       // перезапускаемость
	defer clean() // откат тестовых данных

	inputs := []projection.SingleParticipantInput{
		{ContractID: cRaise, ParticipantCount: cnt(1), ProcurementMethod: "открытый_конкурс"},
		{ContractID: cExcl, ParticipantCount: cnt(1), ProcurementMethod: "из_одного_источника"},
		{ContractID: cMany, ParticipantCount: cnt(3), ProcurementMethod: "открытый_конкурс"},
		{ContractID: cNoData, ParticipantCount: nil, ProcurementMethod: "открытый_конкурс"},
	}
	if err := store.RecomputeSingleParticipant(ctx, inputs); err != nil {
		t.Fatalf("recompute #1: %v", err)
	}

	// cRaise → активный флаг с evidence; methodology_version == каноническая (AC5б).
	row, ok, err := store.Get(ctx, cRaise)
	if err != nil || !ok {
		t.Fatalf("cRaise: ожидался флаг, ok=%v err=%v", ok, err)
	}
	if !row.IsActive {
		t.Errorf("cRaise: ожидался активный флаг")
	}
	if row.MethodologyVersion != "v1.0" {
		t.Errorf("cRaise: methodology_version=%q, ожидалось v1.0 (==params)", row.MethodologyVersion)
	}
	var ev flags.SingleParticipantEvidence
	if err := json.Unmarshal(row.Evidence, &ev); err != nil {
		t.Fatalf("cRaise: evidence невалиден: %v", err)
	}
	if ev.ParticipantCount == nil || *ev.ParticipantCount != 1 || ev.MethodologyVersion != "v1.0" {
		t.Errorf("cRaise: evidence не сохранил входы: %+v", ev)
	}

	// cExcl/cMany/cNoData → флаг НЕ активен (исключённый способ / >1 / нет данных).
	for _, id := range []int64{cExcl, cMany, cNoData} {
		if _, ok, _ := store.Get(ctx, id); ok {
			t.Errorf("contract %d: флаг не должен существовать (не raised)", id)
		}
	}
	if n, _ := store.CountActive(ctx); n != 1 {
		t.Fatalf("после recompute #1 активных флагов = %d, ожидалось 1 (только cRaise)", n)
	}

	// АВТО-СНЯТИЕ (AC2): cRaise теперь 2 участника → not_raised → флаг снимается (is_active=false, cleared_at).
	if err := store.RecomputeSingleParticipant(ctx, []projection.SingleParticipantInput{
		{ContractID: cRaise, ParticipantCount: cnt(2), ProcurementMethod: "открытый_конкурс"},
	}); err != nil {
		t.Fatalf("recompute auto-clear: %v", err)
	}
	row, ok, _ = store.Get(ctx, cRaise)
	if !ok || row.IsActive || !row.ClearedAt.Valid {
		t.Fatalf("cRaise: ожидался снятый флаг (is_active=false, cleared_at set), got active=%v cleared=%v", row.IsActive, row.ClearedAt.Valid)
	}
	if n, _ := store.CountActive(ctx); n != 0 {
		t.Fatalf("после авто-снятия активных = %d, ожидалось 0", n)
	}

	// ИДЕМПОТЕНТНОСТЬ: повторный raise того же контракта (×2) → ровно один активный флаг, ре-активация.
	reraise := []projection.SingleParticipantInput{{ContractID: cRaise, ParticipantCount: cnt(1), ProcurementMethod: "открытый_конкурс"}}
	if err := store.RecomputeSingleParticipant(ctx, reraise); err != nil {
		t.Fatalf("re-raise #1: %v", err)
	}
	if err := store.RecomputeSingleParticipant(ctx, reraise); err != nil {
		t.Fatalf("re-raise #2: %v", err)
	}
	row, ok, _ = store.Get(ctx, cRaise)
	if !ok || !row.IsActive || row.ClearedAt.Valid {
		t.Fatalf("cRaise после ре-активации: ожидался активный без cleared_at, got active=%v cleared=%v", row.IsActive, row.ClearedAt.Valid)
	}
	if n, _ := store.CountActive(ctx); n != 1 {
		t.Fatalf("после двойного re-raise активных = %d, ожидалось 1 (без дублей)", n)
	}
}
