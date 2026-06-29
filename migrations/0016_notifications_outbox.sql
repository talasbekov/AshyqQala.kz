-- +goose Up
-- Story 2.7: notifications_outbox — транзакционный outbox (architecture.md#Communication Patterns 432-438, 577-579).
-- Конверт <event_id, type, occurred_at, subject_ref, payload, v>:
--   • O-1 — строка пишется В ТОЙ ЖЕ tx, что и бизнес-объект (Enqueue принимает tx вызывающего) → откат ⇒
--     нет ни бизнес-строки, ни события;
--   • O-3 — event_id UNIQUE = дедуп (повтор того же события не двоит у получателя; ON CONFLICT DO NOTHING);
--   • O-4 — attempts/available_at = ретрай с ДЕТЕРМИНИРОВАННОЙ видимостью: «сейчас» инъектится воркером из
--     clock.Clock (available_at = $now + backoff(attempts)), НЕ now() в SQL для границы видимости.
-- subject_ref = URN <entity>:<public_id> (natural goszakup id — стабилен между ре-импортами, НЕ внутренний
-- bigint; прецедент evidence_export 5.6). Форма АРХИТЕКТУРНАЯ (без subscription_id — fan-out по подпискам и
-- Telegram-доставка появятся в Epic 7, таблицы подписок ещё нет; data-model:64 описывает позднюю Epic-7-форму).
-- Владелец: importer ПИШЕТ (Enqueue), bot ЧИТАЕТ/доставляет (Epic 7). В S-0 реальных эмиттеров нет —
-- получатель = LoggingDispatcher (только логирует нейтральный конверт).
CREATE TABLE notifications_outbox (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id     UUID NOT NULL UNIQUE,                              -- дедуп-ключ (O-3); детерминируется эмиттером
    type         TEXT NOT NULL,                                     -- <aggregate>.<event>, lower-dot (напр. flag.raised)
    occurred_at  TIMESTAMPTZ NOT NULL,                              -- момент бизнес-события
    subject_ref  TEXT NOT NULL,                                     -- URN <entity>:<public_id> (публичный id)
    payload      JSONB NOT NULL DEFAULT '{}'::jsonb,                -- НЕЙТРАЛЬНЫЙ конверт (id/ref), НЕ проза флага
    v            INTEGER NOT NULL DEFAULT 1,                        -- версия payload
    sent_at      TIMESTAMPTZ,                                       -- NULL = ещё не доставлено; проставляет воркер (O-2)
    attempts     INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),  -- счётчик неудачных доставок (честность)
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),                -- видимость для поллинга (O-4 backoff сдвигает)
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Частичный индекс под поллинг воркера: только НЕотправленные, по порядку видимости (available_at).
CREATE INDEX notifications_outbox_unsent_idx ON notifications_outbox (available_at) WHERE sent_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS notifications_outbox;
