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
    updated_at,
    customer_org_id,
    supplier_org_id
FROM contracts
WHERE goszakup_contract_id = $1
  AND NOT is_deleted;

-- name: ListContracts :many
-- Фасетная фильтрация контрактов (FR-15, Story 6.1). Условные фасеты: narg IS NULL ⇒ фасет не задан
-- (комбинации работают совместно — AND между типами; OR внутри направлений через ANY). Публичный
-- goszakup_contract_id (не суррогат); удалённые скрыты. has_active_flag — «есть сигнал, требующий
-- проверки» (нейтрально, AC6): EXISTS активного contract-флага. Порядок детерминирован
-- (sign_date DESC NULLS LAST, goszakup_contract_id) + keyset-курсор. Окно медианы тут НЕ при чём (AC4):
-- signed_from/to — это фасет поиска по дате подписания, а не скользящее окно 24 мес benchmark-движка.
SELECT
    c.goszakup_contract_id,
    c.subject_ru,
    c.subject_kk,
    c.amount_tng,
    c.sign_date,
    c.status,
    c.direction,
    c.kato_code,
    EXISTS (SELECT 1 FROM risk_flags rf WHERE rf.contract_id = c.id AND rf.is_active) AS has_active_flag
FROM contracts c
WHERE NOT c.is_deleted
  AND (sqlc.narg('directions')::text[] IS NULL OR c.direction = ANY(sqlc.narg('directions')::text[]))
  AND (sqlc.narg('signed_from')::date IS NULL OR c.sign_date >= sqlc.narg('signed_from')::date)
  AND (sqlc.narg('signed_to')::date IS NULL OR c.sign_date <= sqlc.narg('signed_to')::date)
  AND (sqlc.narg('amount_min')::bigint IS NULL OR c.amount_tng >= sqlc.narg('amount_min')::bigint)
  AND (sqlc.narg('amount_max')::bigint IS NULL OR c.amount_tng <= sqlc.narg('amount_max')::bigint)
  AND (sqlc.narg('supplier_org_id')::bigint IS NULL OR c.supplier_org_id = sqlc.narg('supplier_org_id')::bigint)
  AND (NOT sqlc.arg('has_flag_only')::bool OR EXISTS (
        SELECT 1 FROM risk_flags rf WHERE rf.contract_id = c.id AND rf.is_active))
  -- keyset-курсор (sign_date DESC NULLS LAST, goszakup_contract_id ASC). Нет курсора (cursor_gid IS NULL)
  -- ⇒ первая страница. NULL-хвост обработан явно: при курсоре в хвосте берём только NULL-строки с бОльшим gid.
  AND (
    sqlc.narg('cursor_gid')::text IS NULL
    OR (CASE WHEN sqlc.narg('cursor_sd')::date IS NULL
          THEN (c.sign_date IS NULL AND c.goszakup_contract_id > sqlc.narg('cursor_gid')::text)
          ELSE (c.sign_date < sqlc.narg('cursor_sd')::date
                OR c.sign_date IS NULL
                OR (c.sign_date = sqlc.narg('cursor_sd')::date AND c.goszakup_contract_id > sqlc.narg('cursor_gid')::text))
        END)
  )
ORDER BY c.sign_date DESC NULLS LAST, c.goszakup_contract_id
LIMIT sqlc.arg('lim')::int;

-- name: ResolveSupplierOrgID :one
-- Резолв фасета «подрядчик» (Story 6.1): канонический БИН → внутренний supplier org id (bigint). Нормализация
-- БИН — в Go (normalize.CanonicalBIN) ДО вызова; сюда приходит уже-канонический bin. Не найдено → pgx.ErrNoRows
-- (хендлер трактует как пустой список честно, не 500 — несуществующий подрядчик ≠ ошибка сервера).
SELECT id FROM organizations WHERE bin = $1;
