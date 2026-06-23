// Package median — чистая медиан-сигнатура ядра (Story 1.10). Без store/IO/времени (AR-13):
// одни входы → один выход бит-в-бит. Реальное наполнение price_benchmarks/групп сопоставимости — Epic 4.
package median

import (
	"slices"

	"ashyqqala/server/internal/registry"
)

// MinSample — минимальный размер выборки для медианы (methodology_params, дефолт 5). При меньшей
// выборке медиана НЕ показывается — честное состояние insufficient_sample. [data-model min_sample=5]
const MinSample = 5

// Median — чистая функция: выборка целых (₸/км и т.п.) → (значение, честное состояние).
//   - len(samples) < MinSample → (nil, insufficient_sample): медиана НЕ показывается (не 0/NaN).
//   - иначе → (медиана, ok).
//
// Детерминирована: копирует и сортирует вход, не мутируя его; без store/IO/времени.
func Median(samples []int64) (*int64, registry.ValueState) {
	if len(samples) < MinSample {
		return nil, registry.StateInsufficientSample
	}
	s := append([]int64(nil), samples...)
	slices.Sort(s)
	n := len(s)
	var m int64
	if n%2 == 1 {
		m = s[n/2]
	} else {
		// чётная выборка → среднее двух центральных. Overflow-safe форма (s отсортирован по возр.,
		// разность ≥ 0): не суммируем два больших int64 напрямую. Целочисленно (целые ₸).
		lo, hi := s[n/2-1], s[n/2]
		m = lo + (hi-lo)/2
	}
	return &m, registry.StateOK
}
