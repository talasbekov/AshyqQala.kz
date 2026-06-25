-- +goose Up
-- Проекция: acts — акт приёмки/договор по контракту (FR-11, Story 5.1). Привязка к contracts (internal id).
-- signer_info — СЛУЖЕБНАЯ информация должностного лица (§6.2.4: ФИО/должность подписанта как публичная
-- служебная инфо, НЕ приватные данные). Источник — /v2/acts (токен ows_v2, Epic 2); на синтетике — seed.
-- Отсутствующие значения = NULL (честное «нет данных»), без фейковых дефолтов. Карточка показывает акт за
-- честным состоянием: нет акта → блок не выдумывается. Декод-граница (имена полей /v2/acts) — VERIFY (-probe).
CREATE TABLE acts (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,    -- внутр. id
    contract_id     BIGINT NOT NULL REFERENCES contracts(id),           -- FK на контракт (целевая таблица существует)
    goszakup_act_id TEXT NOT NULL UNIQUE,                               -- natural id акта (идемпотентный UPSERT-ключ)
    act_date        DATE,                                              -- дата акта (NULL = нет данных)
    signer_info     TEXT,                                              -- служебная информация подписанта (ФИО/должность)
    source_url      TEXT,                                              -- ссылка на первоисточник акта
    imported_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Чтение карточки идёт по contract_id (последний акт контракта).
CREATE INDEX acts_contract_idx ON acts (contract_id);

-- +goose Down
DROP TABLE IF EXISTS acts;
