//go:build integration

// Интеграционный тест агрегатов района (Story 6.3, FR-17) против реальной БД (миграции 0001-0018).
// Проверяет ПОВЕДЕНИЕ БД, не покрываемое юнитом с mock: членство по КАТО-ПРЕФИКСУ (партиционирование района),
// исключение чужого района и удалённых (is_deleted), подсчёт ТОЛЬКО активных флагов (снятый не считается),
// честный amount_known (NULL-суммы не суммируются). Самодостаточен и БЕЗ зависимости от seed: вставляет свои
// строки с синтетическим КАТО-префиксом «710512» (seed использует «710000000» → изоляция по префиксу) в
// транзакцию, проверяет членство своих ключей, затем откатывает tx. Гоняется:
// `go test -tags=integration -count=1 ./internal/store/...`.
package store

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/store/gen"
)

func TestDistrictAggregates_Integration(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL не задан — пропуск интеграционного теста")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("подключение к БД: %v", err)
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // изоляция: всё откатываем, БД остаётся чистой

	// Синтетический район «710512…» (изолирован от seed «710000000» по префиксу). Коды НЕ продакшн — это
	// тестовая синтетика для проверки МЕХАНИЗМА префикс-членства (реальные коды подтверждает Story 0.1).
	type cSeed struct {
		gid, kato   string
		amount      int64
		amountValid bool
		deleted     bool
	}
	seeds := []cSeed{
		{"ZZD63-A1", "710512000001", 100, true, false}, // в районе A, сумма известна, будет активный флаг
		{"ZZD63-A2", "710512000002", 0, false, false},  // в районе A, сумма NULL (не суммируется)
		{"ZZD63-A3", "710512000003", 999, true, true},  // в районе A, но удалён → исключён
		{"ZZD63-B1", "720000000001", 500, true, false}, // ДРУГОЙ район → не должен попасть в агрегат A
	}
	idByGid := map[string]int64{}
	for _, s := range seeds {
		var id int64
		if err := tx.QueryRow(ctx,
			`INSERT INTO contracts (goszakup_contract_id, subject_ru, amount_tng, status, direction, kato_code, is_deleted)
			 VALUES ($1, 'Ремонт автодороги Zztest63', CASE WHEN $3 THEN $4::bigint ELSE NULL END, 'active', 'road', $2, $5)
			 RETURNING id`,
			s.gid, s.kato, s.amountValid, s.amount, s.deleted).Scan(&id); err != nil {
			t.Fatalf("insert contract %s: %v", s.gid, err)
		}
		idByGid[s.gid] = id
	}

	// Флаги: A1 — АКТИВНЫЙ single_participant + СНЯТЫЙ price_per_km (не должен считаться); B1 — активный monopoly
	// (другой район, не должен считаться в A).
	insFlag := func(contractID int64, flagType string, active bool) {
		t.Helper()
		var clearedExpr string
		if active {
			clearedExpr = "NULL"
		} else {
			clearedExpr = "now()"
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO risk_flags (flag_type, subject_type, contract_id, evidence, methodology_version, is_active, cleared_at)
			 VALUES ($1, 'contract', $2, '{}'::jsonb, '1.0', $3, `+clearedExpr+`)`,
			flagType, contractID, active); err != nil {
			t.Fatalf("insert flag %s active=%v: %v", flagType, active, err)
		}
	}
	insFlag(idByGid["ZZD63-A1"], "single_participant", true)
	insFlag(idByGid["ZZD63-A1"], "price_per_km", false) // СНЯТЫЙ — негативный контроль (не должен считаться)
	insFlag(idByGid["ZZD63-B1"], "monopoly", true)      // другой район — не должен считаться в A

	q := gen.New(tx)
	const prefixA = "710512%"

	// --- Агрегаты района A ---
	agg, err := q.DistrictAggregates(ctx, prefixA)
	if err != nil {
		t.Fatalf("DistrictAggregates: %v", err)
	}
	// Членство по префиксу: A1+A2 (A3 удалён, B1 чужой район — ИСКЛЮЧЕНЫ). Негативный контроль партиционирования.
	if agg.ContractCount != 2 {
		t.Errorf("contract_count = %d, ожидалось 2 (A1+A2; A3 удалён и B1 чужой район исключены)", agg.ContractCount)
	}
	if agg.TotalAmountTng != 100 {
		t.Errorf("total_amount_tng = %d, ожидалось 100 (A1=100; A2 NULL не суммируется; B1=500 в другом районе)", agg.TotalAmountTng)
	}
	if agg.AmountKnownCount != 1 {
		t.Errorf("amount_known_count = %d, ожидалось 1 (только A1 с известной суммой)", agg.AmountKnownCount)
	}

	// Exact-full-код 720000000001 (район B) → ровно B1. Доказывает, что A-агрегат не «всё подряд».
	aggB, err := q.DistrictAggregates(ctx, "720000000001%")
	if err != nil {
		t.Fatalf("DistrictAggregates B: %v", err)
	}
	if aggB.ContractCount != 1 || aggB.TotalAmountTng != 500 {
		t.Errorf("район B: count=%d sum=%d, ожидалось 1/500", aggB.ContractCount, aggB.TotalAmountTng)
	}

	// --- Активные флаги района A по типу ---
	flags, err := q.DistrictActiveFlagsByType(ctx, prefixA)
	if err != nil {
		t.Fatalf("DistrictActiveFlagsByType: %v", err)
	}
	byType := map[string]int64{}
	for _, f := range flags {
		byType[f.FlagType] = f.N
	}
	if byType["single_participant"] != 1 {
		t.Errorf("single_participant = %d, ожидалось 1 (активный на A1)", byType["single_participant"])
	}
	// Негативный контроль ПО ПРИЧИНЕ: снятый price_per_km (is_active=false) НЕ считается.
	if _, ok := byType["price_per_km"]; ok {
		t.Errorf("снятый флаг price_per_km НЕ должен считаться активным, получено %v", byType)
	}
	// Негативный контроль партиционирования: monopoly из района B НЕ попал в район A.
	if _, ok := byType["monopoly"]; ok {
		t.Errorf("флаг monopoly из ЧУЖОГО района B НЕ должен считаться в районе A, получено %v", byType)
	}

	// --- Список объектов района A ---
	objs, err := q.ListContractsByDistrict(ctx, gen.ListContractsByDistrictParams{KatoPrefix: prefixA, Lim: 50})
	if err != nil {
		t.Fatalf("ListContractsByDistrict: %v", err)
	}
	gotFlag := map[string]bool{}
	for _, o := range objs {
		gotFlag[o.GoszakupContractID] = o.HasActiveFlag
	}
	if len(objs) != 2 {
		t.Fatalf("объектов района A = %d, ожидалось 2 (A1, A2)", len(objs))
	}
	if !gotFlag["ZZD63-A1"] {
		t.Error("A1 имеет активный флаг → has_active_flag должен быть true")
	}
	if gotFlag["ZZD63-A2"] {
		t.Error("A2 без активного флага → has_active_flag должен быть false")
	}
	if _, ok := gotFlag["ZZD63-A3"]; ok {
		t.Error("удалённый A3 не должен быть в списке объектов")
	}
	if _, ok := gotFlag["ZZD63-B1"]; ok {
		t.Error("B1 из чужого района не должен быть в списке объектов района A")
	}
}
