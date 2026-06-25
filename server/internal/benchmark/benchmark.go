// Package benchmark — ЧИСТЫЙ движок медиан групп сопоставимости (Story 4.1, AC2). Питает кэш
// price_benchmarks и флаги риска 4.2–4.5 + медиану района (FR-18). Без store/IO/реального времени
// (AR-13/AR-27): опорное «сейчас» приходит ТОЛЬКО через clock.Clock (НЕ time.Now), выборки — аргументом,
// персист/atomic swap — в store-слое. Состояния берутся из registry (свои не плодим). MinSample — из ядра
// median (единый источник). Одни входы + та же methodology → один выход бит-в-бит (пересчитываемость).
package benchmark

import (
	"ashyqqala/server/internal/clock"
	"ashyqqala/server/internal/median"
	"ashyqqala/server/internal/registry"
)

// Group — группа сопоставимости (data-model §2): direction × kato. Скользящее окно (мес) применяется
// к выборке при расчёте, не входит в идентичность группы.
type Group struct {
	Direction string
	Kato      string
}

// Key — ДЕТЕРМИНИРОВАННЫЙ ключ сопоставимости (стабильная строка для price_benchmarks.comparability_key).
// Одни (direction, kato) → один ключ бит-в-бит; разные направления/kato → разные ключи.
func (g Group) Key() string {
	return "direction=" + g.Direction + "|kato=" + g.Kato
}

// Sample — единица выборки медианы цены за км: целая цена ₸/км (Q3: bigint, без плавающей точки →
// детерминизм бесплатен) и момент подписания в unix-секундах. Дата — int64 (НЕ time.Time): чистое ядро
// не импортирует пакет time; окно считается арифметикой на time.Time, полученном из clock, без его имени.
type Sample struct {
	PricePerKM   int64
	SignDateUnix int64
}

// GroupMedian — ЧИСТАЯ медиана группы за скользящее окно windowMonths относительно опорного «сейчас»
// (clock.Clock, НЕ time.Now). Отсекает выборки старше окна, считает median.Median (порог MinSample — из
// ядра), возвращает (медиана, честное состояние, размер сопоставимой выборки):
//   - размер < median.MinSample → (nil, insufficient_sample): медиана НЕ показывается (не 0/NaN);
//   - иначе → (median, ok).
//
// ПРЕДУСЛОВИЕ: windowMonths > 0 (контракт вызывающего; значение приходит из methodology_params, где
// validate() гарантирует > 0). При windowMonths <= 0 окно вырождается (cutoff == «сейчас»/в будущем) →
// почти всё отсекается → insufficient_sample. Ядро НЕ маскирует это своим guard'ом: баг конфигурации
// обязан ловиться на загрузке методики, а не прятаться здесь (честность над домыслом).
//
// Граница окна — КАЛЕНДАРНАЯ (AddDate(0,-windowMonths,0)): для опорных дат конца месяца действует
// нормализация Go (напр. 31-е минус месяцы может «съехать» на короткий месяц). Для кратных 12 окон (24 мес)
// и первых чисел дрейфа нет. Граничные n=0/1/2 → честное insufficient_sample, без паники. Не мутирует вход
// (median.Median копирует). Детерминирована: тот же вход + тот же clock → тот же результат.
func GroupMedian(samples []Sample, windowMonths int, now clock.Clock) (*int64, registry.ValueState, int) {
	// cutoff — нижняя граница окна в unix-секундах. now.Now() даёт time.Time; .AddDate/.Unix — методы
	// (пакет time НЕ импортируется) → go-list-граница чистоты соблюдена. Выборка на границе включается (>=).
	cutoff := now.Now().AddDate(0, -windowMonths, 0).Unix()
	prices := make([]int64, 0, len(samples))
	for _, s := range samples {
		if s.SignDateUnix >= cutoff {
			prices = append(prices, s.PricePerKM)
		}
	}
	v, st := median.Median(prices)
	return v, st, len(prices)
}
