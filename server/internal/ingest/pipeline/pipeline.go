// Package pipeline — постимпортный конвейер (Story 2.4): зафиксированный порядок
// нормализация (FR-2) → хук пересчёта (benchmark swap → flags) → атомарная публикация снапшота.
//
// Шов наблюдаем БЕЗ реального потребителя (анти-фиктивность, Amelia): RecalcHook — типизированный
// интерфейс с дефолтом noopRecalc; реальный потребитель RecalcRunHook оборачивает уже существующий
// recalc.Run (контракт порядка benchmark→flags + atomic swap построены в Epic 4 — Story 4.1).
//
// Публикация снапшота в MVP — ЛЁГКАЯ (AR-5): атомарность «читатель видит снапшот целиком (старый ИЛИ
// новый), не полупересчёт» обеспечивается per-table транзакционным swap внутри пересчёта
// (BenchmarkStore.ReplaceSnapshot, applyFlagOutcomes — MVCC), без глобальной таблицы-указателя/event-
// sourcing (отложено до пилота). RunPostImport ЗАДУМАН как единственный постимпортный джоб (не запускать
// конкурентно): порядок benchmark→flags зашит (recalc.Run), а одиночность СМЯГЧАЕТ долг «флаг против
// устаревшего/полупересчитанного benchmark» (deferred-work.md:201). Одиночность ENFORCED через
// WithSingleJobLock (pg advisory-lock, Story 2.6) в проводке cmd/importer — конкурентный второй джоб честно
// отказывается. Сам RunPostImport лок НЕ берёт (оставлен композируемым/тестируемым без БД).
package pipeline

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/clock"
	"ashyqqala/server/internal/recalc"
)

// RecalcHook — постимпортный шов пересчёта + публикации снапшота (benchmark swap → flags). Типизирован,
// чтобы порядок и наблюдаемость были частью контракта (не свободная функция).
type RecalcHook interface {
	Recalc(ctx context.Context) error
}

// noopRecalc — дефолтный хук: вызываемый no-op (НЕ nil). Делает шов наблюдаемым БЕЗ реального потребителя
// (анти-фиктивность): конвейер всегда вызывает хук, даже когда реальный пересчёт ещё не подключён.
type noopRecalc struct{}

func (noopRecalc) Recalc(context.Context) error { return nil }

// Compile-time: дефолт удовлетворяет шов (анти-фиктивность — шов реален без потребителя).
var _ RecalcHook = noopRecalc{}

// Normalizer — шаг нормализации (FR-2), выполняемый ПЕРЕД пересчётом. Реализация — orgnorm поверх
// декодированных организаций (Story 2.3); живое наполнение источника — Story 2.2.
type Normalizer interface {
	Normalize(ctx context.Context) error
}

// RunPostImport выполняет постимпортный конвейер в ЗАФИКСИРОВАННОМ порядке (AC1):
//
//	нормализация (FR-2) → хук пересчёта (benchmark swap → flags) → публикация снапшота (атомарный swap внутри хука).
//
// hook вызывается РОВНО один раз за публикацию. nil hook → noopRecalc (анти-фиктивность: шов всегда
// наблюдаем). nil norm → шаг нормализации пропускается. Падение нормализации НЕ пускает пересчёт (снапшот
// не публикуется на неполностью нормализованных данных).
func RunPostImport(ctx context.Context, norm Normalizer, hook RecalcHook) error {
	if hook == nil {
		hook = noopRecalc{}
	}
	if norm != nil {
		if err := norm.Normalize(ctx); err != nil {
			return fmt.Errorf("pipeline: нормализация (FR-2): %w", err)
		}
	}
	if err := hook.Recalc(ctx); err != nil {
		return fmt.Errorf("pipeline: пересчёт + публикация снапшота: %w", err)
	}
	return nil
}

// RecalcRunHook — РЕАЛЬНЫЙ хук пересчёта: оборачивает recalc.Run (зашитый порядок benchmark→flags + atomic
// swap, Story 4.1). Bench/Flags — реализации store-слоя (BenchmarkStore + композит флаг-рекалькуляторов).
type RecalcRunHook struct {
	Bench recalc.BenchmarkRecalculator
	Flags recalc.FlagRecalculator
}

// Recalc делегирует recalc.Run. nil-зависимости → честная ошибка (не nil-паника).
func (h RecalcRunHook) Recalc(ctx context.Context) error {
	if h.Bench == nil || h.Flags == nil {
		return fmt.Errorf("pipeline: RecalcRunHook требует Bench и Flags (benchmark→flags)")
	}
	return recalc.Run(ctx, h.Bench, h.Flags)
}

// CompositeFlags объединяет несколько recalc.FlagRecalculator в ОДИН (четыре флага Epic 4 — single/price/
// monopoly/rnu — пересчитываются одним проходом после benchmark). Сам реализует recalc.FlagRecalculator,
// поэтому подставляется в RecalcRunHook.Flags / recalc.Run. Порядок суб-рекалькуляторов сохраняется;
// первая ошибка прокидывается и останавливает дальнейшие (как и одиночный пересчёт).
type CompositeFlags []recalc.FlagRecalculator

// RecalcFlags вызывает суб-рекалькуляторы по порядку. Пустой композит — no-op.
func (c CompositeFlags) RecalcFlags(ctx context.Context) error {
	for i, r := range c {
		if r == nil {
			return fmt.Errorf("pipeline: композит флагов (шаг %d): nil суб-рекалькулятор", i)
		}
		if err := r.RecalcFlags(ctx); err != nil {
			return fmt.Errorf("pipeline: композит флагов (шаг %d): %w", i, err)
		}
	}
	return nil
}

// FreezeClock снимает «сейчас» ОДИН раз и возвращает фиксированные часы — чтобы один проход пересчёта
// использовал единое время (долг RNU: иначе длинный батч с живыми часами мог бы пересечь границу суток —
// deferred-work.md:218). Вызывать на старте постимпортного джоба, передавать результат в дата-зависимые
// рекалькуляторы (RNU).
func FreezeClock(c clock.Clock) clock.Clock {
	if c == nil {
		c = clock.Real{} // защита от nil: снимаем реальное «сейчас», а не паникуем
	}
	return clock.Fixed{T: c.Now()}
}

// PostImportLockKey — стабильный ключ pg advisory-lock одиночности постимпортного джоба (Story 2.6).
// Константа (НЕ random) → детерминированная сериализация между процессами importer.
const PostImportLockKey int64 = 0x4153485150535431 // "ASHQPST1"

// WithSingleJobLock ENFORCE одиночность постимпортного джоба через pg advisory-lock (Story 2.6, AC3; долг
// code-review 2.4): берёт ВЫДЕЛЕННОЕ соединение и pg_try_advisory_lock(key). Если лок занят другим джобом —
// run НЕ выполняется, возвращается честная ошибка (а не молчаливый конкурентный пересчёт против чужого
// снапшота). Lock/unlock на ОДНОМ соединении (advisory-lock сессионный); соединение держится на время run.
func WithSingleJobLock(ctx context.Context, pool *pgxpool.Pool, key int64, run func(context.Context) error) error {
	if pool == nil {
		return fmt.Errorf("pipeline: WithSingleJobLock: pool не задан")
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("pipeline: advisory-lock: захват соединения: %w", err)
	}
	defer conn.Release()
	var got bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&got); err != nil {
		return fmt.Errorf("pipeline: pg_try_advisory_lock(%d): %w", key, err)
	}
	if !got {
		return fmt.Errorf("pipeline: постимпортный джоб уже выполняется (advisory-lock %d занят) — одиночность джоба", key)
	}
	// Unlock через WithoutCancel: гарантированно освобождаем сессионный лок ДАЖЕ если run превысил ctx-таймаут
	// (иначе release зависел бы от teardown соединения). Соединение всё равно вернётся в пул через conn.Release.
	defer func() { _, _ = conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", key) }()
	return run(ctx)
}
