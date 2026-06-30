-- +goose Up
-- Story 7.1: dead-letter для notifications_outbox (закрывает долг 2.7, deferred-work:275). poison-строка
-- (вечно падающий TRANSIENT Send — Telegram надолго недоступен) при достижении потолка attempts помечается
-- dead_at/dead_reason и ВЫПАДАЕТ из поллинга: не ре-поллится вечно, attempts не растёт бесконечно (int32).
-- Постоянный отказ chat (заблокирован/удалён) обрабатывается РАНЬШЕ — деактивацией подписки (Story 7.2-шов),
-- НЕ dead-letter. Граница visibility/ретрая прежняя (O-4, Clock); это лишь верхний предел повторов (O-2).
ALTER TABLE notifications_outbox
    ADD COLUMN dead_at     TIMESTAMPTZ,  -- NULL = живая строка; проставляется при достижении max-attempts
    ADD COLUMN dead_reason TEXT;         -- честная причина квартина (последняя transient-ошибка; диагностика)

-- Пересобрать частичный индекс поллинга: видимы только ЖИВЫЕ неотправленные (sent_at IS NULL AND dead_at IS NULL).
DROP INDEX IF EXISTS notifications_outbox_unsent_idx;
CREATE INDEX notifications_outbox_unsent_idx ON notifications_outbox (available_at)
    WHERE sent_at IS NULL AND dead_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS notifications_outbox_unsent_idx;
CREATE INDEX notifications_outbox_unsent_idx ON notifications_outbox (available_at) WHERE sent_at IS NULL;
ALTER TABLE notifications_outbox DROP COLUMN IF EXISTS dead_reason, DROP COLUMN IF EXISTS dead_at;
