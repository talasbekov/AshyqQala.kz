// Package recalc — оркестратор постимпортного пересчёта (Story 4.1, AC4). КОНТРАКТ ПОРЯДКА
// (architecture.md:880–881): кэш price_benchmarks пересчитывается и публикуется (атомарный swap) ПЕРЕД
// флагами риска — флаг НИКОГДА не считается против устаревшего benchmark.
//
// ДЕСКОУП (Q1, как паттерн Story 0.4): живой постимпортный хук импорт-конвейера + сам atomic swap снапшота
// строит Story 2.4. Здесь — ВЫЗЫВАЕМЫЙ оркестратор с зашитым порядком + юнит-тест порядка; реальные
// реализации шагов (store-слой / флаги 4.2–4.5) подключаются через интерфейсы, без выдуманного хука.
package recalc

import (
	"context"
	"fmt"
)

// BenchmarkRecalculator — пересчёт кэша price_benchmarks с атомарной публикацией (store-слой; Story 4.1
// Task 3 даёт реализацию, живой swap снапшота — 2.4).
type BenchmarkRecalculator interface {
	RecalcBenchmarks(ctx context.Context) error
}

// FlagRecalculator — пересчёт флагов риска (формулы — Epic 4 флаги 4.2–4.5; здесь только контракт порядка).
type FlagRecalculator interface {
	RecalcFlags(ctx context.Context) error
}

// Run выполняет пересчёт в ЕДИНСТВЕННО допустимом порядке: benchmark (+ swap) → flags. Если пересчёт
// benchmark падает, флаги НЕ пересчитываются (иначе считались бы против устаревшего/полупересчитанного
// кэша). Порядок зашит и покрыт юнит-тестом (AC4).
func Run(ctx context.Context, b BenchmarkRecalculator, f FlagRecalculator) error {
	if err := b.RecalcBenchmarks(ctx); err != nil {
		return fmt.Errorf("recalc: пересчёт benchmark (до флагов): %w", err)
	}
	if err := f.RecalcFlags(ctx); err != nil {
		return fmt.Errorf("recalc: пересчёт флагов: %w", err)
	}
	return nil
}
