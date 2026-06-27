-- name: ContractorAggregates :one
-- Агрегаты подрядчика по supplier_org_id (FR-13): число контрактов, сумма ₸ (bigint), distinct регионы (КАТО).
-- До наполнения связи (Story 2.2) вернёт нули/пусто → хендлер честно помечает «профиль неполный» (НЕ «0 контрактов»).
SELECT
    count(*)::bigint                                                                  AS contract_count,
    coalesce(sum(amount_tng), 0)::bigint                                              AS total_amount_tng,
    coalesce(array_agg(DISTINCT kato_code) FILTER (WHERE kato_code IS NOT NULL), '{}')::text[] AS regions
FROM contracts
WHERE supplier_org_id = $1
  AND NOT is_deleted;

-- name: ListContractsBySupplierOrg :many
-- Список контрактов подрядчика (FR-13, AC-1) по supplier_org_id. До наполнения связи (Story 2.2) — пусто
-- → карточка «профиль неполный». Публичный goszakup_contract_id (не суррогат); удалённые скрыты; порядок стабилен.
SELECT
    goszakup_contract_id,
    subject_ru,
    subject_kk,
    amount_tng,
    kato_code,
    direction
FROM contracts
WHERE supplier_org_id = $1
  AND NOT is_deleted
ORDER BY sign_date DESC NULLS LAST, goszakup_contract_id;

-- name: ListUnresolvedAliasNames :many
-- Сырые написания псевдонимов в очереди (manual/conflict) — для детекта «профиль уточняется» по СОВПАДЕНИЮ
-- ИМЕНИ (Story 5.2 review-фикс F1). conflict/неразрешённые-manual псевдонимы имеют organization_id=NULL (2.3 P1),
-- поэтому привязка к орг — НЕ по FK, а по канонизированному имени (нормализация в Go). Порядок стабилен.
SELECT raw_name
FROM org_name_aliases
WHERE resolve_status IN ('manual', 'conflict')
ORDER BY raw_name, id;

-- name: CountUnresolvedAliasesByOrg :one
-- Сколько у организации НЕ-auto псевдонимов (manual/conflict, привязанных оператором) — драйвер состояния
-- «профиль уточняется» (FR-13/Story 2.3): при наличии таких флаги НЕ основание для выводов о подрядчике.
SELECT count(*)::bigint AS n
FROM org_name_aliases
WHERE organization_id = $1
  AND resolve_status <> 'auto';
