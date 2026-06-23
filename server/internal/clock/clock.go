// Package clock — Clock-интерфейс (Real|Fixed) для детерминизма (Story 1.10, AR-13). Ядро берёт время
// ТОЛЬКО через Clock-аргумент, не через time.Now напрямую → go-list-граница «без реального времени».
package clock

import "time"

// Clock — источник времени для чистого ядра (Story 1.10, AR-13). Ядро берёт «сейчас» ТОЛЬКО через
// Clock-аргумент, НИКОГДА через time.Now напрямую → детерминизм тестов/golden и go-list-граница
// (чистые пакеты не импортируют "time"). Этот пакет — единственное легальное место прямого "time".
type Clock interface {
	Now() time.Time
}

// Real — продовые часы.
type Real struct{}

func (Real) Now() time.Time { return time.Now() }

// Fixed — детерминированные часы для тестов/golden: всегда возвращают T.
type Fixed struct{ T time.Time }

func (f Fixed) Now() time.Time { return f.T }
