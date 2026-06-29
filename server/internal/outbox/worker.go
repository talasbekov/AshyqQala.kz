package outbox

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/clock"
	"ashyqqala/server/internal/store/gen"
)

// backoff-параметры видимости-таймаута (O-4). Простая ВОСПРОИЗВОДИМАЯ формула — линейный base*attempts с
// потолком (этос «простые воспроизводимые формулы»; Вопрос №3 — рекомендация принята), НЕ экспонента.
const (
	backoffBase = 30 * time.Second
	backoffMax  = 30 * time.Minute
)

// backoff — ЧИСТАЯ детерминированная функция задержки видимости после неудачной доставки (O-4): линейно
// растёт с числом попыток, ограничена потолком. Без time.Now → полностью тестируема (тот же attempts →
// та же задержка). attempts — НОВЫЙ счётчик попыток (после инкремента), ≥1.
func backoff(attempts int) time.Duration {
	const maxFactor = int(backoffMax / backoffBase) // потолок/база (=60): дальше растить незачем
	switch {
	case attempts < 1:
		attempts = 1
	case attempts > maxFactor:
		// Кламп ВЕРХНЕГО края ДО умножения: иначе backoffBase*attempts переполнил бы int64 при огромном
		// attempts → отрицательная задержка → available_at в прошлом → плотный ретрай-цикл.
		attempts = maxFactor
	}
	d := backoffBase * time.Duration(attempts)
	if d > backoffMax {
		return backoffMax
	}
	return d
}

// ProcessBatch — один проход воркера доставки (O-2). В ОДНОЙ транзакции: PollUnsent (FOR UPDATE SKIP LOCKED —
// два конкурентных воркера НЕ двоят одну строку) видимых неотправленных строк → для каждой Dispatcher.Send;
// успех → MarkSent($now); ошибка Send → BumpAttempt (+1 attempts, available_at = $now + backoff(attempts) —
// строка скрыта до истечения backoff). «Сейчас» берётся ТОЛЬКО из clk (O-4 детерминизм — НЕ now() в SQL для
// границы видимости). Возвращает число УСПЕШНО отправленных за проход. Ошибка Send одной строки НЕ валит
// батч (растим attempts и продолжаем); ошибка БД/commit — валит (откат всего прохода).
func ProcessBatch(ctx context.Context, pool *pgxpool.Pool, d Dispatcher, clk clock.Clock, limit int) (int, error) {
	if pool == nil {
		return 0, fmt.Errorf("outbox ProcessBatch: pool не задан")
	}
	if d == nil {
		return 0, fmt.Errorf("outbox ProcessBatch: dispatcher не задан")
	}
	if clk == nil {
		return 0, fmt.Errorf("outbox ProcessBatch: clock не задан")
	}
	if limit <= 0 || limit > math.MaxInt32 {
		// LIMIT 0 = вечный idle-проход; LIMIT <0 = ошибка Postgres; > MaxInt32 → int32-каст ниже оборачивается
		// в отрицательный/мусорный LIMIT (тихо не тот размер батча). Фейл-фаст до tx.
		return 0, fmt.Errorf("outbox ProcessBatch: limit должен быть в (0, %d] (получено %d)", math.MaxInt32, limit)
	}
	now := clk.Now()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("outbox ProcessBatch: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback после успешного commit — штатный no-op

	q := gen.New(tx)
	rows, err := q.PollUnsent(ctx, gen.PollUnsentParams{
		AvailableAt: pgtype.Timestamptz{Time: now, Valid: true},
		Limit:       int32(limit),
	})
	if err != nil {
		return 0, fmt.Errorf("outbox ProcessBatch: poll: %w", err)
	}

	sent := 0
	for _, r := range rows {
		e := Event{
			EventID:    EventID(r.EventID.Bytes),
			Type:       r.Type,
			OccurredAt: r.OccurredAt.Time,
			SubjectRef: r.SubjectRef,
			Payload:    json.RawMessage(r.Payload),
			V:          int(r.V),
		}
		if serr := d.Send(ctx, e); serr != nil {
			// Доставка не удалась → растим attempts и сдвигаем видимость на backoff (O-4 через clk).
			next := now.Add(backoff(int(r.Attempts) + 1))
			if berr := q.BumpAttempt(ctx, gen.BumpAttemptParams{
				ID:          r.ID,
				AvailableAt: pgtype.Timestamptz{Time: next, Valid: true},
			}); berr != nil {
				// rollback (defer) отменит ВСЕ MarkSent/BumpAttempt прохода → честно 0 персистнуто.
				return 0, fmt.Errorf("outbox ProcessBatch: bump attempt id=%d: %w", r.ID, berr)
			}
			continue
		}
		if merr := q.MarkSent(ctx, gen.MarkSentParams{
			ID:     r.ID,
			SentAt: pgtype.Timestamptz{Time: now, Valid: true},
		}); merr != nil {
			// rollback (defer) отменит ВСЕ MarkSent/BumpAttempt прохода → честно 0 персистнуто.
			return 0, fmt.Errorf("outbox ProcessBatch: mark sent id=%d: %w", r.ID, merr)
		}
		sent++
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("outbox ProcessBatch: commit: %w", err)
	}
	return sent, nil
}
