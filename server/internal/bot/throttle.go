package bot

import (
	"sync"
	"time"

	"ashyqqala/server/internal/clock"
)

// Throttle — вежливость к Telegram (AC1, architecture.md:440 «троттл outbox»): минимальный интервал между
// отправками. Детерминизм — через clock.Clock + инъектируемый Sleep (тесты подставляют запоминающий
// sleeper, прод — time.Sleep). Потокобезопасен (mutex): хотя bot-воркер один (architecture.md:733), DB-слой
// (`FOR UPDATE SKIP LOCKED`) допускает конкурентных воркеров — гонку по `last` исключаем.
type Throttle struct {
	Clock  clock.Clock           // источник «сейчас» (Real|Fixed); nil → clock.Real{}
	MinGap time.Duration         // минимальный интервал между сообщениями
	Sleep  func(d time.Duration) // ожидание (прод time.Sleep; тест — мок); nil → не ждать (только учёт)

	mu   sync.Mutex
	last time.Time // ЛОГИЧЕСКИЙ момент прошлой отправки (нулевой = первой ещё не было)
}

// Wait блокирует до истечения MinGap с прошлой отправки (если нужно), затем фиксирует ЛОГИЧЕСКИЙ момент
// текущей отправки `now + wait` — иначе слитое сном время засчиталось бы в следующий gap (недо-троттл ~2×).
// Первый вызов не ждёт. nil Clock → clock.Real{} (как Sleep уже nil-safe).
func (t *Throttle) Wait() {
	var clk clock.Clock = clock.Real{}
	if t.Clock != nil {
		clk = t.Clock
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	now := clk.Now()
	send := now
	if !t.last.IsZero() {
		if wait := t.MinGap - now.Sub(t.last); wait > 0 {
			if t.Sleep != nil {
				t.Sleep(wait)
			}
			send = now.Add(wait) // момент фактической отправки = после ожидания
		}
	}
	t.last = send
}
