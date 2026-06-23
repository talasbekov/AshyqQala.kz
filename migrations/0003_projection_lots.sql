-- +goose Up
-- Проекция: lots — derived из снапшота (импортёр перестраивает). МИНИМУМ колонок (S-0), БЕЗ flags/geo.
-- Паттерн 0002_projection (contracts): internal bigint identity + natural goszakup_lot_id UNIQUE;
-- двуязычные title_*. Отсутствующие значения = NULL (честное «нет данных»), без фейковых дефолтов.
CREATE TABLE lots (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,  -- внутр. id
    goszakup_lot_id TEXT NOT NULL UNIQUE,                             -- natural / публичный id лота
    announcement_id BIGINT,                                           -- связь с объявлением (FK позже — таблицы нет)
    title_ru        TEXT,
    title_kk        TEXT,
    amount          BIGINT,                                           -- целые ₸ (как amount_tng в contracts)
    quantity        BIGINT,
    unit            TEXT,
    kato_code       TEXT,
    is_deleted      BOOLEAN NOT NULL DEFAULT FALSE,
    imported_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- FK announcement_id → trd_buy(id) и связь с organizations/contracts — поздняя миграция (целевых
-- таблиц ещё нет; dangling FK недопустим). Проекционная таблица ⊥ кураторские (AR-4).

-- +goose Down
DROP TABLE IF EXISTS lots;
