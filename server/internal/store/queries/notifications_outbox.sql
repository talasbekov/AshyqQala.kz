-- name: EnqueueEvent :exec
-- Запись конверта события в outbox В ТРАНЗАКЦИИ ВЫЗЫВАЮЩЕГО (атомарно с бизнес-объектом — O-1: откат tx ⇒
-- нет ни бизнес-строки, ни события). Дедуп (O-3): event_id UNIQUE + ON CONFLICT DO NOTHING — повторная
-- вставка того же события no-op (не двоит у получателя). attempts/available_at — дефолты схемы (0 / now()).
INSERT INTO notifications_outbox (event_id, type, occurred_at, subject_ref, payload, v)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (event_id) DO NOTHING;

-- name: PollUnsent :many
-- Воркер берёт ЖИВЫЕ неотправленные ВИДИМЫЕ строки: sent_at IS NULL AND dead_at IS NULL AND available_at <= $1.
-- dead_at IS NULL — dead-letter строки (Story 7.1, долг 2.7) выпадают из поллинга (не ре-поллятся вечно).
-- $1 («сейчас») — момент из clock.Clock (O-4: граница видимости детерминирована инъекцией Clock, НЕ now() в SQL).
-- FOR UPDATE SKIP LOCKED — два конкурентных воркера НЕ двоят одну строку (берут непересекающиеся наборы, O-2).
-- Порядок (available_at, id) — детерминизм/справедливость FIFO. $2 — размер батча.
SELECT id, event_id, type, occurred_at, subject_ref, payload, v, sent_at, attempts, available_at, created_at
FROM notifications_outbox
WHERE sent_at IS NULL AND dead_at IS NULL AND available_at <= $1
ORDER BY available_at, id
FOR UPDATE SKIP LOCKED
LIMIT $2;

-- name: MarkSent :exec
-- Успешная доставка (O-2): проставить sent_at (момент из clock.Clock). Строка больше не поллится
-- (выпадает из частичного индекса notifications_outbox_unsent_idx).
UPDATE notifications_outbox SET sent_at = $2 WHERE id = $1;

-- name: BumpAttempt :exec
-- Неудачная доставка (O-2/O-4): +1 к attempts и сдвиг видимости available_at = $2 (= $now + backoff(attempts),
-- вычислено в Go из clock.Clock — детерминизм, НЕ now() в SQL). Строка вновь станет видимой после available_at.
UPDATE notifications_outbox SET attempts = attempts + 1, available_at = $2 WHERE id = $1;

-- name: MarkDead :exec
-- Dead-letter (Story 7.1, закрывает долг 2.7): poison-строка достигла потолка attempts (Telegram надолго
-- недоступен) → пометить dead_at=$2 (момент из clock.Clock) и dead_reason=$3 (последняя transient-ошибка).
-- Строка выпадает из частичного индекса/поллинга (PollUnsent: dead_at IS NULL) — не ре-поллится вечно.
UPDATE notifications_outbox SET dead_at = $2, dead_reason = $3 WHERE id = $1;

-- name: CountOutbox :one
-- Число строк в outbox (для тестов/диагностики дедупа O-3).
SELECT count(*) AS n FROM notifications_outbox;

-- name: GetOutboxByEventID :one
-- Строка по event_id (для тестов: проверка sent_at/attempts/available_at/dead_at после доставки/ретрая/dead-letter).
SELECT id, event_id, type, occurred_at, subject_ref, payload, v, sent_at, attempts, available_at, created_at, dead_at, dead_reason
FROM notifications_outbox
WHERE event_id = $1;
