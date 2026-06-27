-- name: UpsertOrganization :exec
-- Идемпотентный UPSERT организации по natural bin (импортёр перестраивает проекцию; повтор не плодит дубли).
-- Роли OR-ятся (одна орг бывает и заказчиком, и подрядчиком в разных контрактах). Наименования/КАТО —
-- COALESCE (новый NULL НЕ затирает известное значение: честность над «last-write-wins» для отсутствующих).
INSERT INTO organizations (
    bin, name_ru, name_kk, reg_kato, is_customer, is_supplier, source_url
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
ON CONFLICT (bin) DO UPDATE SET
    name_ru      = COALESCE(EXCLUDED.name_ru, organizations.name_ru),
    name_kk      = COALESCE(EXCLUDED.name_kk, organizations.name_kk),
    reg_kato     = COALESCE(EXCLUDED.reg_kato, organizations.reg_kato),
    is_customer  = organizations.is_customer OR EXCLUDED.is_customer,
    is_supplier  = organizations.is_supplier OR EXCLUDED.is_supplier,
    source_url   = COALESCE(EXCLUDED.source_url, organizations.source_url),
    updated_at   = now();

-- name: GetOrganizationByBIN :one
-- Организация по natural bin (удалённые скрыты).
SELECT
    id, bin, name_ru, name_kk, reg_kato, is_customer, is_supplier,
    first_seen_at, source_url, is_deleted, imported_at, updated_at
FROM organizations
WHERE bin = $1
  AND NOT is_deleted;

-- name: ListOrganizations :many
-- Все неудалённые организации (движок нормализации строит из них кандидатов для матча). Порядок стабилен.
SELECT
    id, bin, name_ru, name_kk, reg_kato, is_customer, is_supplier,
    first_seen_at, source_url, is_deleted, imported_at, updated_at
FROM organizations
WHERE NOT is_deleted
ORDER BY id;
