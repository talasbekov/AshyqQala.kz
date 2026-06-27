-- +goose Up
-- Кураторская: org_name_aliases — нормализация наименований (UC-09/FR-2): варианты написания → канонический
-- БИН. Граница AR-4: КУРАТОРСКАЯ (импортёр перестраивает проекцию `organizations`, а очередь/разрешения
-- нормализации трогает только для авто-строк; ручные разрешения оператора переживают ре-импорт — см. UpsertAlias).
-- Колонки строго по data-model (org_name_aliases): organization_id(FK, NULL пока не разрешён), raw_name,
-- source, resolve_status(auto/manual/conflict).
CREATE TABLE org_name_aliases (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    organization_id BIGINT REFERENCES organizations(id),                  -- NULL пока БИН не разрешён (manual/conflict)
    raw_name        TEXT NOT NULL,                                        -- сырое вариативное написание из источника
    source          TEXT NOT NULL DEFAULT '',                            -- откуда (contract/trd-buy/…); '' = неизвестно
    resolve_status  TEXT NOT NULL CHECK (resolve_status IN ('auto', 'manual', 'conflict')),
    detected_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (raw_name, source)                                            -- идемпотентность: один вариант из источника = одна строка
);

-- Индекс для очереди оператору (manual/conflict — «требует проверки»).
CREATE INDEX org_name_aliases_status_idx ON org_name_aliases (resolve_status);

-- +goose Down
DROP TABLE IF EXISTS org_name_aliases;
