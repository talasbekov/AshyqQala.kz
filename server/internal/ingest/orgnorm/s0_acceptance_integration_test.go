//go:build integration

// S-0 приёмочный тест (Story 2.6, AC1): сценарий `импорт → курация(R) → импорт` доказывает инвариант
// идемпотентности+выживания+детерминизма ОДНИМ прогоном:
//   - правка R цела (manual-псевдоним пережил ре-импорт);
//   - дублей нет (счётчики проекций идентичны после импорта #1 и #2 — UPSERT ON CONFLICT);
//   - медианы БИТ-в-БИТ идентичны для того же снапшота (RunPostImport дважды → ε=0; инъекция Clock — pipeline).
//
// Очищает свои таблицы TRUNCATE … CASCADE (детерминированные счётчики, независимость от seed). ВНИМАНИЕ:
// CASCADE по FK задевает и contracts/acts (organizations→contracts→acts) — поэтому `make test-integration ./...`
// на ЗАСЕЯННОЙ БД ломает downstream seed-зависимые store/httpapi-тесты (пред-существующий долг clean-vs-seed,
// deferred-work.md; CI-job 2.6 обходит его, гоняя только import/normalize+projection на ЧИСТОЙ БД).
// Гоняется: `go test -tags=integration ./internal/ingest/...` с DATABASE_URL (миграции 0001-0015), -p 1.
package orgnorm_test

import (
	"context"
	"os"
	"sort"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/flags"
	"ashyqqala/server/internal/ingest/orgnorm"
	"ashyqqala/server/internal/ingest/pipeline"
	"ashyqqala/server/internal/lexicon"
	"ashyqqala/server/internal/store/curation"
	"ashyqqala/server/internal/store/gen"
	"ashyqqala/server/internal/store/projection"
)

func countRows(t *testing.T, pool *pgxpool.Pool, table string) int64 {
	t.Helper()
	var n int64
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestS0_ImportCurationReimport(t *testing.T) {
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

	// Самоизоляция: тест владеет этими таблицами (детерминированные счётчики, независимость от seed).
	if _, err := pool.Exec(ctx, "TRUNCATE org_name_aliases, organizations, price_benchmarks, risk_flags RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("TRUNCATE (применены ли миграции 0001-0015?): %v", err)
	}

	lex, err := lexicon.Load("../../../../registry")
	if err != nil {
		t.Fatalf("lexicon.Load: %v", err)
	}
	orgStore := projection.NewOrgStore(pool)
	aliasStore := curation.NewAliasStore(pool)

	doImport := func(label string) {
		existing, err := orgStore.ListOrganizations(ctx)
		if err != nil {
			t.Fatalf("%s: ListOrganizations: %v", label, err)
		}
		plan := orgnorm.PlanNormalization(buildAppearances(t), orgnorm.BuildCandidates(existing, lex), lex)
		if err := orgnorm.Apply(ctx, pool, plan); err != nil {
			t.Fatalf("%s: Apply: %v", label, err)
		}
	}

	// --- Импорт #1 ---
	doImport("import#1")
	orgs1 := countRows(t, pool, "organizations")
	aliases1 := countRows(t, pool, "org_name_aliases")

	// --- Курация строки R: оператор разрешает manual-запись (привязывает к 555) ---
	org555, err := orgStore.GetOrganizationByBIN(ctx, "555555555555")
	if err != nil {
		t.Fatalf("GetOrganizationByBIN(555): %v", err)
	}
	if err := aliasStore.ResolveAliasManually(ctx, gen.ResolveAliasManuallyParams{
		RawName:        "ТОО Безбинная Фирма",
		Source:         "trd-buy",
		OrganizationID: pgtype.Int8{Int64: org555.ID, Valid: true},
	}); err != nil {
		t.Fatalf("ResolveAliasManually (курация R): %v", err)
	}

	// --- Импорт #2 (ре-импорт) ---
	doImport("import#2")

	// AC1 «правка R цела»: manual-псевдоним пережил ре-импорт (org_id + status='manual').
	resolved, err := aliasStore.GetAlias(ctx, gen.GetAliasParams{RawName: "ТОО Безбинная Фирма", Source: "trd-buy"})
	if err != nil {
		t.Fatalf("GetAlias(R после ре-импорта): %v", err)
	}
	if resolved.ResolveStatus != "manual" || !resolved.OrganizationID.Valid || resolved.OrganizationID.Int64 != org555.ID {
		t.Fatalf("правка R НЕ пережила ре-импорт: status=%s org_id=%+v, ожидалось manual→%d", resolved.ResolveStatus, resolved.OrganizationID, org555.ID)
	}

	// AC1 «дублей нет»: счётчики проекций идентичны после #1 и #2 (UPSERT идемпотентен).
	if orgs2 := countRows(t, pool, "organizations"); orgs2 != orgs1 {
		t.Fatalf("ре-импорт расплодил организации: было %d, стало %d", orgs1, orgs2)
	}
	if aliases2 := countRows(t, pool, "org_name_aliases"); aliases2 != aliases1 {
		t.Fatalf("ре-импорт расплодил псевдонимы: было %d, стало %d", aliases1, aliases2)
	}

	// Идемпотентность пересчёта: RunPostImport дважды → снапшот стабилен. price_benchmarks пуст (нет
	// length_km/Epic 3) → Count 0 оба раза (честно, без выдуманных медиан). Инъекция Clock (один Now() на
	// проход) покрыта pipeline-тестами (FreezeClock) + cmd/importer; здесь композит без дата-зависимых флагов.
	params := flags.Params{MethodologyVersion: "v1.0", MinSample: 5}
	benchStore := projection.NewBenchmarkStore(pool, params)
	hook := pipeline.RecalcRunHook{Bench: benchStore, Flags: pipeline.CompositeFlags(nil)}
	for i := 1; i <= 2; i++ {
		if err := pipeline.RunPostImport(ctx, nil, hook); err != nil {
			t.Fatalf("RunPostImport #%d: %v", i, err)
		}
		if n, err := benchStore.Count(ctx); err != nil || n != 0 {
			t.Fatalf("RunPostImport #%d: Count=%d err=%v, ожидалось 0 (пустой снапшот честно)", i, n, err)
		}
	}

	// AC1 «медианы БИТ-в-БИТ для того же снапшота» — FIRING-guard детерминизма публикации: ReplaceSnapshot
	// ОДНОГО известного набора дважды → результат ε=0 идентичен. Проверка на НЕПУСТОМ наборе (price_benchmarks
	// из RunPostImport пуст до Epic 3 → контракт детерминизма проверяем на seeded-данных; guard способен
	// покраснеть на 2 строках). Сортировка по ключу: ListPriceBenchmarks без гарантированного ORDER BY.
	med := int64(300)
	seed := []projection.BenchmarkRow{
		{ComparabilityKey: "direction=road|kato=710000000", MedianPricePerKM: &med, SampleSize: 5},
		{ComparabilityKey: "direction=water|kato=710000000", MedianPricePerKM: nil, SampleSize: 2},
	}
	if err := benchStore.ReplaceSnapshot(ctx, seed); err != nil {
		t.Fatalf("seed snap1 ReplaceSnapshot: %v", err)
	}
	snap1, err := gen.New(pool).ListPriceBenchmarks(ctx)
	if err != nil {
		t.Fatalf("ListPriceBenchmarks #1: %v", err)
	}
	if err := benchStore.ReplaceSnapshot(ctx, seed); err != nil {
		t.Fatalf("seed snap2 ReplaceSnapshot: %v", err)
	}
	snap2, err := gen.New(pool).ListPriceBenchmarks(ctx)
	if err != nil {
		t.Fatalf("ListPriceBenchmarks #2: %v", err)
	}
	sort.Slice(snap1, func(a, b int) bool { return snap1[a].ComparabilityKey < snap1[b].ComparabilityKey })
	sort.Slice(snap2, func(a, b int) bool { return snap2[a].ComparabilityKey < snap2[b].ComparabilityKey })
	if len(snap1) != 2 || len(snap2) != 2 {
		t.Fatalf("ожидалось 2 строки в снапшоте, got %d / %d", len(snap1), len(snap2))
	}
	for i := range snap1 {
		if snap1[i].ComparabilityKey != snap2[i].ComparabilityKey ||
			snap1[i].MedianPricePerKm != snap2[i].MedianPricePerKm ||
			snap1[i].SampleSize != snap2[i].SampleSize ||
			snap1[i].MethodologyVersion != snap2[i].MethodologyVersion {
			t.Fatalf("снапшоты НЕ бит-в-бит идентичны на строке %d: %+v != %+v (детерминизм нарушен)", i, snap1[i], snap2[i])
		}
	}
	t.Logf("S-0 OK: R цела (org_id=%d), дублей нет (orgs=%d aliases=%d), снапшот ε=0 детерминирован (rows=%d)", org555.ID, orgs1, aliases1, len(snap1))
}
