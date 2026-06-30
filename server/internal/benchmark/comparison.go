package benchmark

import (
	"ashyqqala/server/internal/clock"
	"ashyqqala/server/internal/registry"
)

// Outcome — исход медианы ОДНОЙ группы сопоставимости (район ИЛИ город) для сравнения FR-18: медиана
// (nil = НЕ показывается — не 0/NaN), честное состояние (ось 1, registry) и размер сопоставимой выборки.
// Однозначность median↔state — несущий инвариант: Median != nil ⟺ State == ok ⟺ N ≥ median.MinSample
// (гарантируется ядром median.Median; property-тест AC3).
type Outcome struct {
	Median *int64
	State  registry.ValueState
	N      int
}

// Evaluate — ЧИСТЫЙ исход группы из её ₸/км-выборки (Story 6.4, FR-18). Два режима честности:
//   - computable=false → ₸/км СТРУКТУРНО невычислима (нет geo_objects.length_km — Epic 3/Story 3.1):
//     not_comparable (нечего сравнивать), БЕЗ обращения к выборке. Это текущее токен-независимое состояние
//     (прецедент 4.3: движок реален, живые числа ждут length_km). НЕ insufficient_sample — это не «мало
//     контрактов», а отсутствие измеримой базы ₸/км.
//   - computable=true → делегирует GroupMedian (скользящее окно × порог MinSample): ok с медианой при
//     ≥MinSample сопоставимых, иначе insufficient_sample (медиана не показывается). Подключается вместе с
//     length_km БЕЗ изменения этой формы (шов).
//
// Без store/IO/реального времени (AR-13/AR-27): «сейчас» — только через clock.Clock.
func Evaluate(samples []Sample, computable bool, windowMonths int, now clock.Clock) Outcome {
	if !computable {
		return Outcome{State: registry.StateNotComparable}
	}
	m, st, n := GroupMedian(samples, windowMonths, now)
	return Outcome{Median: m, State: st, N: n}
}

// Comparison — сравнение «район vs город» по ОДНОМУ направлению (FR-18). DeltaPercent валиден ТОЛЬКО при
// DeltaShown (обе медианы ok и городская > 0); иначе сравнивать нечего (дельты нет, НЕ 0).
type Comparison struct {
	District     Outcome
	City         Outcome
	DeltaPercent int64 // (район − город) × 100 / город; усечение к нулю; смысл только при DeltaShown
	DeltaShown   bool
}

// CompareGroups — ЧИСТОЕ сравнение исходов района и города (FR-18). Дельта целочисленная (детерминизм без
// float-дрейфа, как рациональное ×1.5 в 4.3): (район − город) × 100 / город. Показывается лишь когда ОБЕ
// медианы ok И городская > 0 — иначе база сравнения недостоверна (деление на ≤0 фабриковало бы %), дельты
// нет. Разность считается ПЕРВОЙ (она мала) → ×100 не переполняет int64 на реалистичных ₸/км.
func CompareGroups(district, city Outcome) Comparison {
	c := Comparison{District: district, City: city}
	if district.State == registry.StateOK && city.State == registry.StateOK &&
		district.Median != nil && city.Median != nil && *city.Median > 0 {
		c.DeltaPercent = (*district.Median - *city.Median) * 100 / *city.Median
		c.DeltaShown = true
	}
	return c
}
