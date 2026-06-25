package projection

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/methodology"
	"ashyqqala/server/internal/store/gen"
)

// SeedMethodologyParams ИДЕМПОТЕНТНО и АТОМАРНО засевает пороги версии в methodology_params (append-only immutable,
// 0005). Все вставки версии — в ОДНОЙ транзакции (all-or-nothing): частичный сбой в середине НЕ оставляет
// недозасеянную immutable-версию (её нельзя дочистить триггером, а повтор сделал бы skip-if-present → версия
// навсегда неполна). Существование версии проверяется ВНУТРИ транзакции: при гонке двух seed второй упадёт на
// UNIQUE и откатится целиком (без частичного набора). Источник — methodology.Flatten(Load). seeded=true если вставил.
func SeedMethodologyParams(ctx context.Context, pool *pgxpool.Pool, version string, kvs []methodology.ParamKV) (bool, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("methodology_params: begin tx (версия %s): %w", version, err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op после Commit; откатывает при раннем return/skip

	q := gen.New(tx)
	existing, err := q.GetMethodologyParamsByVersion(ctx, version)
	if err != nil {
		return false, fmt.Errorf("methodology_params: чтение версии %s: %w", version, err)
	}
	if len(existing) > 0 {
		return false, nil // версия уже засеяна (immutable B-4) → skip; tx откатится (ничего не писали)
	}
	for _, kv := range kvs {
		if err := q.InsertMethodologyParam(ctx, gen.InsertMethodologyParamParams{
			Version:     version,
			Key:         kv.Key,
			Value:       kv.Value,
			Description: pgtype.Text{},
		}); err != nil {
			return false, fmt.Errorf("methodology_params: вставка %s/%s: %w", version, kv.Key, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("methodology_params: commit (версия %s): %w", version, err)
	}
	return true, nil
}

// VerifyMethodologyParams сверяет DB-реестр версии с ОЖИДАЕМЫМ набором порогов (Flatten(Load) из YAML): тот же
// набор key→value. Рассинхрон (дрейф YAML↔DB: правка YAML без новой версии ИЛИ неполный seed) → ошибка. Несущий
// инвариант пересчитываемости (FR-23): пороги в evidence воспроизводимы по ПУБЛИЧНОМУ YAML == DB-реестру. Зовётся
// на старте (cmd/api) после seed для fail-fast при дрейфе.
func VerifyMethodologyParams(ctx context.Context, pool *pgxpool.Pool, version string, kvs []methodology.ParamKV) error {
	rows, err := gen.New(pool).GetMethodologyParamsByVersion(ctx, version)
	if err != nil {
		return fmt.Errorf("methodology_params: чтение версии %s: %w", version, err)
	}
	db := make(map[string]string, len(rows))
	for _, r := range rows {
		db[r.Key] = r.Value
	}
	if len(db) != len(kvs) {
		return fmt.Errorf("methodology_params: дрейф YAML↔DB версии %s — в БД %d порогов, в YAML(Flatten) %d", version, len(db), len(kvs))
	}
	for _, kv := range kvs {
		if got, ok := db[kv.Key]; !ok || got != kv.Value {
			return fmt.Errorf("methodology_params: дрейф YAML↔DB версии %s для %q — DB=%q, YAML=%q", version, kv.Key, got, kv.Value)
		}
	}
	return nil
}
