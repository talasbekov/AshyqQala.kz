-- +goose Up
-- rnu_entries — ПРОИЗВОДНАЯ проекция реестра недобросовестных участников (РНУ, FR-22, Story 4.5). Наполняется
-- живым импортом /v2/rnu (Epic 2); на синтетике — seed. Флаг 4.5 читает записи → risk_flags (contractor-субъект).
-- organization_id БЕЗ FK (AR-4: проекции перестраиваются, FK хрупок; CHECK ловит 0/отриц.). Авто-снятие флага по
-- end_date — в compute-слое (re-eval с clock), не в схеме (запись НЕ удаляется при истечении; end_date проставлен).
CREATE TABLE rnu_entries (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    organization_id BIGINT NOT NULL,            -- БИН-подрядчик (без FK — AR-4)
    goszakup_rnu_id TEXT,                        -- идентификатор записи в реестре goszakup (атрибуция государству)
    start_date      DATE NOT NULL,               -- дата внесения в реестр (запись активна при start_date ≤ today)
    end_date        DATE,                        -- дата исключения (NULL = открытая запись; ≤ today → авто-снятие)
    reason_ref      TEXT,                        -- ссылка/код основания записи
    source_url      TEXT,                        -- ссылка на реестр (первоисточник)
    CONSTRAINT rnu_entries_org_positive_chk CHECK (organization_id > 0),
    CONSTRAINT rnu_entries_dates_chk CHECK (end_date IS NULL OR end_date >= start_date)
);
CREATE INDEX rnu_entries_org_idx ON rnu_entries (organization_id);

-- +goose Down
DROP TABLE IF EXISTS rnu_entries;
