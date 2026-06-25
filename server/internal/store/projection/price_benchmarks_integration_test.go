//go:build integration

// Интеграционный тест кэша price_benchmarks (Story 4.1, AC3) + иммутабельности methodology_params (AC1).
// Гоняется отдельно: `go test -tags=integration ./internal/store/...` с DATABASE_URL (миграции 0001-0006).
// CI (`go test ./...` без тега) этот файл не компилирует → не требует БД.
package projection_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/flags"
	"ashyqqala/server/internal/registry"
	"ashyqqala/server/internal/store/gen"
	"ashyqqala/server/internal/store/projection"
)

func i64(v int64) *int64 { return &v }

func benchmarkPool(t *testing.T) *pgxpool.Pool {
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

// testParams — канонические methodology_params для теста (версия снапшота).
var testParams = flags.Params{MethodologyVersion: "v1.0", MinSample: 5}

// TestPriceBenchmarks_AtomicSwap — AC3: пересчёт «в сторону → атомарный swap»; читатель видит снапшот
// ЦЕЛИКОМ (старый ключ исчезает, новый появляется); честные состояния; sample_size фиксируется; детерминизм.
func TestPriceBenchmarks_AtomicSwap(t *testing.T) {
	pool := benchmarkPool(t)
	defer pool.Close()
	ctx := context.Background()
	store := projection.NewBenchmarkStore(pool, testParams)

	// Чистый старт (на случай прежних прогонов).
	if err := store.ReplaceSnapshot(ctx, nil); err != nil {
		t.Fatalf("очистка: %v", err)
	}

	// Снапшот 1: ключ A (ok, медиана 300), ключ C (insufficient — median NULL).
	keyA, keyB, keyC := "direction=road|kato=710000000", "direction=water|kato=710000000", "direction=road|kato=750000000"
	snap1 := []projection.BenchmarkRow{
		{ComparabilityKey: keyA, MedianPricePerKM: i64(300), SampleSize: 5},
		{ComparabilityKey: keyC, MedianPricePerKM: nil, SampleSize: 2},
	}
	if err := store.ReplaceSnapshot(ctx, snap1); err != nil {
		t.Fatalf("swap snap1: %v", err)
	}
	if v, st, err := store.Lookup(ctx, keyA); err != nil || st != registry.StateOK || v == nil || *v != 300 {
		t.Fatalf("keyA: ожидалось (300, ok), получено (%v, %s, %v)", v, st, err)
	}
	if v, st, err := store.Lookup(ctx, keyC); err != nil || st != registry.StateInsufficientSample || v != nil {
		t.Fatalf("keyC (median NULL): ожидалось (nil, insufficient_sample), получено (%v, %s, %v)", v, st, err)
	}
	if v, st, err := store.Lookup(ctx, keyB); err != nil || st != registry.StateNotComparable || v != nil {
		t.Fatalf("keyB (нет строки): ожидалось (nil, not_comparable), получено (%v, %s, %v)", v, st, err)
	}

	// Снапшот 2 (atomic swap): теперь только ключ B (ok, 200). Ключ A/C исчезают ЦЕЛИКОМ.
	snap2 := []projection.BenchmarkRow{{ComparabilityKey: keyB, MedianPricePerKM: i64(200), SampleSize: 6}}
	if err := store.ReplaceSnapshot(ctx, snap2); err != nil {
		t.Fatalf("swap snap2: %v", err)
	}
	if _, st, _ := store.Lookup(ctx, keyA); st != registry.StateNotComparable {
		t.Fatalf("после swap keyA должен исчезнуть (not_comparable), получено %s", st)
	}
	if v, st, _ := store.Lookup(ctx, keyB); st != registry.StateOK || v == nil || *v != 200 {
		t.Fatalf("keyB после swap: ожидалось (200, ok), получено (%v, %s)", v, st)
	}
	if n, err := store.Count(ctx); err != nil || n != 1 {
		t.Fatalf("Count после swap2 = %d (err %v), ожидалось 1", n, err)
	}

	// Детерминизм: повтор того же снапшота → тот же набор (median/sample_size совпадают).
	if err := store.ReplaceSnapshot(ctx, snap2); err != nil {
		t.Fatalf("повторный swap: %v", err)
	}
	rows, err := gen.New(pool).ListPriceBenchmarks(ctx)
	if err != nil || len(rows) != 1 {
		t.Fatalf("List после повтора: len=%d err=%v", len(rows), err)
	}
	if rows[0].SampleSize != 6 || !rows[0].MedianPricePerKm.Valid || rows[0].MedianPricePerKm.Int64 != 200 {
		t.Fatalf("детерминизм нарушен: %+v", rows[0])
	}
	if rows[0].MethodologyVersion != "v1.0" {
		t.Fatalf("methodology_version = %q, ожидалось каноническое v1.0", rows[0].MethodologyVersion)
	}
}

// TestRecalcBenchmarks_EmptyOnNoLength — 4.1 без geo_objects.length_km: RecalcBenchmarks честно публикует
// ПУСТОЙ снапшот (нет вычислимых price/km) → любой ключ not_comparable; никаких выдуманных медиан.
func TestRecalcBenchmarks_EmptyOnNoLength(t *testing.T) {
	pool := benchmarkPool(t)
	defer pool.Close()
	ctx := context.Background()
	store := projection.NewBenchmarkStore(pool, testParams)

	if err := store.RecalcBenchmarks(ctx); err != nil {
		t.Fatalf("RecalcBenchmarks: %v", err)
	}
	if n, err := store.Count(ctx); err != nil || n != 0 {
		t.Fatalf("после recalc Count = %d (err %v), ожидалось 0 (нет length_km → нет бенчмарков)", n, err)
	}
	if _, st, _ := store.Lookup(ctx, "direction=road|kato=710000000"); st != registry.StateNotComparable {
		t.Fatalf("после пустого recalc ожидалось not_comparable, получено %s", st)
	}
}

// TestMethodologyParams_Immutable — AC1/B-4: триггер запрещает UPDATE/DELETE строки версии (append-only).
func TestMethodologyParams_Immutable(t *testing.T) {
	pool := benchmarkPool(t)
	defer pool.Close()
	ctx := context.Background()

	// Очистка тестовых строк vTEST.0: DELETE/TRUNCATE запрещены триггерами → временно отключаем USER-триггеры.
	// Зовётся ПЕРЕД вставкой (перезапускаемость — иначе UNIQUE(version,key) на повторном прогоне) и в defer.
	cleanup := func() {
		_, _ = pool.Exec(ctx, "ALTER TABLE methodology_params DISABLE TRIGGER USER")
		_, _ = pool.Exec(ctx, "DELETE FROM methodology_params WHERE version = 'vTEST.0'")
		_, _ = pool.Exec(ctx, "ALTER TABLE methodology_params ENABLE TRIGGER USER")
	}
	cleanup()       // пречистка от мусора прошлого прогона
	defer cleanup() // откат после теста

	// Append: вставка строки версии — разрешена.
	if err := gen.New(pool).InsertMethodologyParam(ctx, gen.InsertMethodologyParamParams{
		Version: "vTEST.0", Key: "median.min_sample", Value: "5",
	}); err != nil {
		t.Fatalf("append методики (должно быть разрешено): %v", err)
	}

	// UPDATE — запрещён триггером (иммутабельность).
	if _, err := pool.Exec(ctx, "UPDATE methodology_params SET value = '4' WHERE version = 'vTEST.0'"); err == nil {
		t.Error("UPDATE methodology_params прошёл — ожидался запрет триггером (B-4 иммутабельность)")
	}
	// DELETE — запрещён триггером.
	if _, err := pool.Exec(ctx, "DELETE FROM methodology_params WHERE version = 'vTEST.0'"); err == nil {
		t.Error("DELETE methodology_params прошёл — ожидался запрет триггером (B-4 иммутабельность)")
	}
}
