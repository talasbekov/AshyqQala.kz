//go:build integration

// Тест enforcement одиночности постимпортного джоба (Story 2.6, AC3; долг code-review 2.4): pg advisory-lock
// сериализует джобы — пока лок занят конкурентом, WithSingleJobLock НЕ выполняет работу и честно ошибается.
// Краснеет, если лок не enforced (run выполнился бы под занятым локом). Гоняется: `go test -tags=integration
// ./internal/ingest/...` с DATABASE_URL.
package pipeline_test

import (
	"context"
	"strings"
	"testing"

	"ashyqqala/server/internal/ingest/pipeline"
)

func TestWithSingleJobLock_Enforced(t *testing.T) {
	pool := integrationPool(t)
	defer pool.Close()
	ctx := context.Background()
	const key = pipeline.PostImportLockKey

	// Имитация конкурентного джоба: держим тот же advisory-lock в ОТДЕЛЬНОМ соединении.
	holder, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire holder conn: %v", err)
	}
	defer holder.Release() // release на ВСЕХ путях (иначе pool.Close в defer завис бы на t.Fatalf ниже)
	var locked bool
	if err := holder.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&locked); err != nil || !locked {
		t.Fatalf("holder не взял лок: locked=%v err=%v", locked, err)
	}

	// Лок занят → WithSingleJobLock НЕ выполняет run и честно ошибается ИМЕННО про занятость лока.
	ran := false
	busyErr := pipeline.WithSingleJobLock(ctx, pool, key, func(context.Context) error { ran = true; return nil })
	if busyErr == nil {
		t.Fatal("ожидалась ошибка: постимпортный лок занят конкурентным джобом")
	}
	if !strings.Contains(busyErr.Error(), "одиночность джоба") {
		t.Fatalf("ошибка не про занятость лока (мог пройти посторонний сбой): %v", busyErr)
	}
	if ran {
		t.Fatal("run выполнен под занятым локом — одиночность джоба НЕ enforced")
	}

	// Освобождаем лок → WithSingleJobLock выполняет run. holder остаётся checked-out (release — в defer).
	if _, err := holder.Exec(ctx, "SELECT pg_advisory_unlock($1)", key); err != nil {
		t.Fatalf("unlock holder: %v", err)
	}

	ran2 := false
	if err := pipeline.WithSingleJobLock(ctx, pool, key, func(context.Context) error { ran2 = true; return nil }); err != nil {
		t.Fatalf("WithSingleJobLock после освобождения: %v", err)
	}
	if !ran2 {
		t.Fatal("run не выполнен после освобождения лока")
	}
}
