//go:build integration

// Интеграционный тест methodology_params DB-реестра (Story 4.6, AC1): seed v1.0 из YAML (идемпотентно, append-only
// immutable) + перекрёстный инвариант YAML↔DB (Flatten(Load) == GetMethodologyParamsByVersion). Гоняется:
// `go test -tags=integration ./internal/store/...` (миграции 0001-0009; methodology_params — 0005).
package projection_test

import (
	"context"
	"path/filepath"
	"testing"

	"ashyqqala/server/internal/methodology"
	"ashyqqala/server/internal/store/gen"
	"ashyqqala/server/internal/store/projection"
)

func TestMethodologyParams_SeedAndYAMLInvariant(t *testing.T) {
	pool := riskFlagPool(t)
	defer pool.Close()
	ctx := context.Background()

	p, err := methodology.Load(filepath.Join("../../../../", "registry")) // projection — на уровень глубже methodology
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	kvs := methodology.Flatten(p)

	// Seed v1.0 (идемпотентно: на свежем контейнере вставит, на повторе skip — append-only immutable не нарушаем).
	if _, err := projection.SeedMethodologyParams(ctx, pool, p.MethodologyVersion, kvs); err != nil {
		t.Fatalf("seed #1: %v", err)
	}
	// Идемпотентность: повтор → seeded=false, БЕЗ ошибки (триггер immutability не сработал).
	if seeded, err := projection.SeedMethodologyParams(ctx, pool, p.MethodologyVersion, kvs); err != nil || seeded {
		t.Fatalf("seed #2 (идемпотентность): seeded=%v err=%v, ожидалось false/nil", seeded, err)
	}

	// Перекрёстный инвариант YAML↔DB: DB-реестр версии v1.0 СОВПАДАЕТ с Flatten(Load) (множество key→value).
	rows, err := gen.New(pool).GetMethodologyParamsByVersion(ctx, p.MethodologyVersion)
	if err != nil {
		t.Fatalf("GetMethodologyParamsByVersion: %v", err)
	}
	db := make(map[string]string, len(rows))
	for _, r := range rows {
		db[r.Key] = r.Value
	}
	if len(db) != len(kvs) {
		t.Fatalf("DB-реестр содержит %d порогов, YAML(Flatten) — %d (рассинхрон)", len(db), len(kvs))
	}
	for _, kv := range kvs {
		if db[kv.Key] != kv.Value {
			t.Errorf("инвариант YAML↔DB нарушен для %q: DB=%q, YAML=%q", kv.Key, db[kv.Key], kv.Value)
		}
	}
}
