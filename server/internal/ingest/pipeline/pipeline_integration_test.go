//go:build integration

// Интеграционный тест постимпортного конвейера (Story 2.4, AC2/AC3): RunPostImport поверх реальной БД
// выполняет хук пересчёта (benchmark swap → flags) и публикует снапшот. Тест проверяет ПОЛНУЮ замену
// (старый набор исчезает целиком) + идемпотентность повтора. Конкурентную изоляцию «читатель не видит
// полупересчёта» гарантирует переиспользуемый BenchmarkStore.ReplaceSnapshot (одна tx, MVCC) — она покрыта
// Epic 4 (TestPriceBenchmarks_AtomicSwap); здесь — именно публикация ЧЕРЕЗ конвейер. Гоняется:
// `go test -tags=integration ./internal/ingest/...` с DATABASE_URL (миграции 0001-0014). CI без тега не компилирует.
package pipeline_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/flags"
	"ashyqqala/server/internal/ingest/pipeline"
	"ashyqqala/server/internal/store/projection"
)

func i64(v int64) *int64 { return &v }

func integrationPool(t *testing.T) *pgxpool.Pool {
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

var testParams = flags.Params{MethodologyVersion: "v1.0", MinSample: 5}

// TestRunPostImport_AtomicSnapshotPublish — AC2/AC3: предзаполненный benchmark-снапшот заменяется ПОЛНОСТЬЮ
// постимпортным пересчётом (RecalcBenchmarks публикует пустой снапшот честно — нет length_km). Проверяет
// wholesale-замену (старый ключ исчезает целиком) и идемпотентность повтора. Атомарность swap для
// КОНКУРЕНТНОГО читателя гарантирует одна-tx ReplaceSnapshot (Epic 4 TestPriceBenchmarks_AtomicSwap), не этот тест.
func TestRunPostImport_AtomicSnapshotPublish(t *testing.T) {
	pool := integrationPool(t)
	defer pool.Close()
	ctx := context.Background()

	benchStore := projection.NewBenchmarkStore(pool, testParams)

	// Предусловие: непустой снапшот (как будто от прежнего пересчёта).
	seed := []projection.BenchmarkRow{{ComparabilityKey: "direction=road|kato=710000000", MedianPricePerKM: i64(300), SampleSize: 5}}
	if err := benchStore.ReplaceSnapshot(ctx, seed); err != nil {
		t.Fatalf("seed снапшота: %v", err)
	}
	if n, err := benchStore.Count(ctx); err != nil || n != 1 {
		t.Fatalf("предусловие: Count=%d err=%v, ожидалось 1", n, err)
	}

	// Постимпортный конвейер: реальный хук (benchmark swap → flags). Flags — пустой композит (no-op):
	// провайдеры живых флагов подключаются в Story 2.2; здесь проверяется именно публикация снапшота.
	hook := pipeline.RecalcRunHook{Bench: benchStore, Flags: pipeline.CompositeFlags(nil)}
	if err := pipeline.RunPostImport(ctx, nil, hook); err != nil {
		t.Fatalf("RunPostImport: %v", err)
	}

	// AC2/AC3: атомарный swap опубликовал новый (пустой) снапшот ЦЕЛИКОМ — старый ключ исчез.
	if n, err := benchStore.Count(ctx); err != nil || n != 0 {
		t.Fatalf("после RunPostImport Count=%d err=%v, ожидалось 0 (пустой снапшот честно, нет length_km)", n, err)
	}

	// Идемпотентность одиночного джоба: повтор безопасен.
	if err := pipeline.RunPostImport(ctx, nil, hook); err != nil {
		t.Fatalf("повторный RunPostImport: %v", err)
	}
	if n, err := benchStore.Count(ctx); err != nil || n != 0 {
		t.Fatalf("после повтора Count=%d err=%v, ожидалось 0", n, err)
	}
}
