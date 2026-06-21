-- +goose Up
-- Проекция: contracts — центральный объект карточки. МИНИМУМ колонок (S-0), БЕЗ flags/geo.
-- Конвенции: internal bigint identity + natural goszakup_contract_id UNIQUE; двуязычные subject_*.
-- Отсутствующие значения = NULL (честное «нет данных»), без фейковых дефолтов.
CREATE TABLE contracts (
    id                   BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,  -- внутр. id
    goszakup_contract_id TEXT NOT NULL UNIQUE,                             -- natural / публичный id
    subject_ru           TEXT,
    subject_kk           TEXT,
    amount_tng           BIGINT,                                          -- целые ₸; на проводе СТРОКОЙ (wire-конвенция: db==json==OpenAPI)
    sign_date            DATE,
    plan_start           DATE,
    plan_end             DATE,
    status               TEXT,
    direction            TEXT CHECK (direction IN ('road', 'water', 'other')),
    kato_code            TEXT,
    source_url           TEXT,
    is_deleted           BOOLEAN NOT NULL DEFAULT FALSE,
    imported_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- TODO (поздняя миграция, когда появятся organizations/lots — B-3 / Epic 2):
--   ALTER TABLE contracts
--     ADD COLUMN lot_id          BIGINT REFERENCES lots(id),
--     ADD COLUMN customer_org_id BIGINT REFERENCES organizations(id),
--     ADD COLUMN supplier_org_id BIGINT REFERENCES organizations(id);
-- FK сейчас НЕ добавляем — целевые таблицы ещё не существуют (dangling FK недопустим).

-- +goose Down
DROP TABLE IF EXISTS contracts;
