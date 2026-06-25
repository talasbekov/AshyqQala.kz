-- +goose Up
-- risk_flags — ПРОИЗВОДНЫЕ автофлаги риска (Story 4.2 заводит таблицу + наполняет флаг «единственный участник»;
-- флаги 4.3–4.5 переиспользуют через flag_type). Производное от ЧИСТЫХ функций `internal/flags`; пересчёт
-- идемпотентен; снятие = `is_active=false` + `cleared_at` (строка НЕ удаляется; при ре-активации строка
-- перезаписывается — полный append-only аудит циклов raise/clear отдельной задачей). `evidence`(jsonb) +
-- `methodology_version` → пересчитываемость третьим лицом (FR-23, §4.1). Выход НЕ публикуется (Epic 5 — сервер-gate).
CREATE TABLE risk_flags (
    id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    flag_type           TEXT NOT NULL,                       -- напр. single_participant (разделяет флаги 4.2–4.5)
    subject_type        TEXT NOT NULL,                       -- contract | contractor
    contract_id         BIGINT,                              -- subject_type=contract
    organization_id     BIGINT,                              -- subject_type=contractor
    severity            TEXT,                                -- presentation (Epic 5); backend 4.2 не задаёт (NULL)
    evidence            JSONB NOT NULL,                      -- ВСЕ входы расчёта (пересчитываемость)
    is_active           BOOLEAN NOT NULL DEFAULT TRUE,
    detected_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    cleared_at          TIMESTAMPTZ,                          -- заполняется при авто-снятии (is_active=false)
    methodology_version TEXT NOT NULL,
    CONSTRAINT risk_flags_subject_chk CHECK (subject_type IN ('contract', 'contractor')),
    -- Честность: subject-ссылка соответствует subject_type (contract-флаг → contract_id; contractor → org_id).
    CONSTRAINT risk_flags_subject_ref_chk CHECK (
        (subject_type = 'contract'   AND contract_id IS NOT NULL AND organization_id IS NULL)
     OR (subject_type = 'contractor' AND organization_id IS NOT NULL AND contract_id IS NULL)
    ),
    -- Активный ⟺ не снят (cleared_at NULL); снятый ⟺ cleared_at заполнен.
    CONSTRAINT risk_flags_active_cleared_chk CHECK (is_active = (cleared_at IS NULL)),
    -- Честность: id субъекта положителен (FK не ставим — AR-4: проекции перестраиваются, FK хрупок; CHECK ловит 0/отриц./мусор).
    CONSTRAINT risk_flags_positive_ref_chk CHECK (
        (contract_id IS NULL OR contract_id > 0) AND (organization_id IS NULL OR organization_id > 0)
    )
);

-- Идемпотентность: один флаг данного типа на субъект (повтор пересчёта → UPSERT, не дубль). Частичные
-- unique-индексы по типу субъекта (contract-флаги 4.2/4.3; contractor-флаги 4.4).
CREATE UNIQUE INDEX risk_flags_contract_uniq ON risk_flags (flag_type, contract_id) WHERE contract_id IS NOT NULL;
CREATE UNIQUE INDEX risk_flags_org_uniq ON risk_flags (flag_type, organization_id) WHERE organization_id IS NOT NULL;
-- Чтение активных флагов контракта.
CREATE INDEX risk_flags_active_idx ON risk_flags (contract_id) WHERE is_active;

-- +goose Down
DROP TABLE IF EXISTS risk_flags;
