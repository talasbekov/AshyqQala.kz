-- +goose Up
-- flag_disputes — статус ВЕРИФИКАЦИИ автофлага (AR-28, Story 4.6): измеримость SM-C1 (доля некорректных флагов).
-- Один диспут на флаг (UNIQUE risk_flag_id). risk_flag_id — soft-ref (БЕЗ FK, AR-4: risk_flags перестраивается);
-- денормализованный субъект (flag_type/subject_type/subject_id) для стабильной ссылки даже при перестройке проекции.
-- Статусы (AR-28): raised → disputed → confirmed (флаг подтверждён валидным) | withdrawn (флаг отозван как ложный).
-- resolved_at заполнен ⟺ финальный статус (confirmed|withdrawn) — честный инвариант.
CREATE TABLE flag_disputes (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    risk_flag_id  BIGINT NOT NULL,            -- soft-ref на risk_flags.id (без FK — AR-4)
    flag_type     TEXT NOT NULL,              -- денорм: single_participant|price_per_km|monopoly|rnu
    subject_type  TEXT NOT NULL,              -- денорм: contract|contractor
    subject_id    BIGINT NOT NULL,            -- денорм: contract_id|organization_id
    status        TEXT NOT NULL,              -- raised|disputed|confirmed|withdrawn (AR-28)
    note          TEXT,
    source_url    TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at   TIMESTAMPTZ,                -- заполнен ⟺ confirmed|withdrawn
    CONSTRAINT flag_disputes_status_chk CHECK (status IN ('raised', 'disputed', 'confirmed', 'withdrawn')),
    -- денорм-субъект ограничен доменом (опечатка/неизвестный тип молча исказили бы агрегации по flag_type).
    CONSTRAINT flag_disputes_flag_type_chk CHECK (flag_type IN ('single_participant', 'price_per_km', 'monopoly', 'rnu')),
    CONSTRAINT flag_disputes_subject_type_chk CHECK (subject_type IN ('contract', 'contractor')),
    CONSTRAINT flag_disputes_refs_positive_chk CHECK (risk_flag_id > 0 AND subject_id > 0),
    -- resolved ⟺ финальный статус ⟺ resolved_at заполнен (иначе скрытая рассинхронизация измеримости).
    CONSTRAINT flag_disputes_resolved_chk CHECK ((status IN ('confirmed', 'withdrawn')) = (resolved_at IS NOT NULL)),
    UNIQUE (risk_flag_id)
);
CREATE INDEX flag_disputes_status_idx ON flag_disputes (status);

-- +goose Down
DROP TABLE IF EXISTS flag_disputes;
