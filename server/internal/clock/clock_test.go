package clock_test

import (
	"testing"
	"time"

	"ashyqqala/server/internal/clock"
)

func TestFixed_Deterministic(t *testing.T) {
	ts := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	c := clock.Fixed{T: ts}
	if !c.Now().Equal(ts) {
		t.Fatalf("Fixed.Now() = %v, ожидалось %v", c.Now(), ts)
	}
	if !c.Now().Equal(c.Now()) {
		t.Fatal("Fixed.Now() недетерминирован")
	}
}

func TestReal_NonZero(t *testing.T) {
	if (clock.Real{}).Now().IsZero() {
		t.Fatal("Real.Now() вернул нулевое время")
	}
}

// Clock — интерфейс; обе реализации ему удовлетворяют (компайл-тайм проверка).
var _ clock.Clock = clock.Real{}
var _ clock.Clock = clock.Fixed{}
