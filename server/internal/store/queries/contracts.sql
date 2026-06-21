-- name: GetContractByID :one
-- Читает контракт по публичному natural id (goszakup_contract_id); удалённые скрыты.
SELECT
    id,
    goszakup_contract_id,
    subject_ru,
    subject_kk,
    amount_tng,
    sign_date,
    plan_start,
    plan_end,
    status,
    direction,
    kato_code,
    source_url,
    is_deleted,
    imported_at,
    updated_at
FROM contracts
WHERE goszakup_contract_id = $1
  AND NOT is_deleted;
