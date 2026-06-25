-- name: RaiseContractFlag :exec
-- Идемпотентно ставит/обновляет АКТИВНЫЙ contract-флаг (повтор не плодит дубли — UPSERT по (flag_type,
-- contract_id)). Снятый ранее флаг ре-активируется (is_active=true, cleared_at=NULL); detected_at сохраняется
-- (первое обнаружение). evidence/methodology_version обновляются (актуальный снапшот входов).
INSERT INTO risk_flags (flag_type, subject_type, contract_id, severity, evidence, is_active, methodology_version)
VALUES ($1, 'contract', $2, $3, $4, TRUE, $5)
ON CONFLICT (flag_type, contract_id) WHERE contract_id IS NOT NULL
DO UPDATE SET
    severity            = EXCLUDED.severity,
    evidence            = EXCLUDED.evidence,
    is_active           = TRUE,
    cleared_at          = NULL,
    methodology_version = EXCLUDED.methodology_version;

-- name: ClearContractFlag :exec
-- Авто-снятие: гасит АКТИВНЫЙ contract-флаг (is_active=false + cleared_at). Идемпотентно (WHERE is_active —
-- повтор на уже снятом = no-op). История строки сохраняется (не DELETE).
UPDATE risk_flags
SET is_active = FALSE, cleared_at = now()
WHERE flag_type = $1 AND contract_id = $2 AND is_active;

-- name: GetContractFlag :one
-- Флаг данного типа по контракту (активный или снятый) — для чтения/тестов. Не найдено → pgx.ErrNoRows.
SELECT id, flag_type, subject_type, contract_id, organization_id, severity, evidence, is_active, detected_at, cleared_at, methodology_version
FROM risk_flags
WHERE flag_type = $1 AND contract_id = $2;

-- name: CountActiveContractFlags :one
-- Число активных флагов данного типа (диагностика/тесты).
SELECT count(*) AS n FROM risk_flags WHERE flag_type = $1 AND is_active;

-- name: RaiseContractorFlag :exec
-- FR-21 (Story 4.4): идемпотентно ставит/обновляет АКТИВНЫЙ contractor-флаг (повтор не плодит дубли — UPSERT
-- по (flag_type, organization_id)). Снятый ранее флаг ре-активируется (is_active=true, cleared_at=NULL);
-- detected_at сохраняется (первое обнаружение). evidence/methodology_version обновляются (актуальный снапшот).
INSERT INTO risk_flags (flag_type, subject_type, organization_id, severity, evidence, is_active, methodology_version)
VALUES ($1, 'contractor', $2, $3, $4, TRUE, $5)
ON CONFLICT (flag_type, organization_id) WHERE organization_id IS NOT NULL
DO UPDATE SET
    severity            = EXCLUDED.severity,
    evidence            = EXCLUDED.evidence,
    is_active           = TRUE,
    cleared_at          = NULL,
    methodology_version = EXCLUDED.methodology_version;

-- name: ClearContractorFlag :exec
-- Авто-снятие: гасит АКТИВНЫЙ contractor-флаг (is_active=false + cleared_at). Идемпотентно (WHERE is_active —
-- повтор на уже снятом = no-op). История строки сохраняется (не DELETE).
UPDATE risk_flags
SET is_active = FALSE, cleared_at = now()
WHERE flag_type = $1 AND organization_id = $2 AND is_active;

-- name: GetContractorFlag :one
-- Флаг данного типа по подрядчику (активный или снятый) — для чтения/тестов. Не найдено → pgx.ErrNoRows.
SELECT id, flag_type, subject_type, contract_id, organization_id, severity, evidence, is_active, detected_at, cleared_at, methodology_version
FROM risk_flags
WHERE flag_type = $1 AND organization_id = $2;
