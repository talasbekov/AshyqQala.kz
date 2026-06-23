-- name: UpsertLot :exec
-- Идемпотентный UPSERT лота по natural goszakup_lot_id (импортёр перестраивает проекцию; повтор не плодит дубли).
INSERT INTO lots (
    goszakup_lot_id, announcement_id, title_ru, title_kk, amount, quantity, unit, kato_code
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
ON CONFLICT (goszakup_lot_id) DO UPDATE SET
    announcement_id = EXCLUDED.announcement_id,
    title_ru        = EXCLUDED.title_ru,
    title_kk        = EXCLUDED.title_kk,
    amount          = EXCLUDED.amount,
    quantity        = EXCLUDED.quantity,
    unit            = EXCLUDED.unit,
    kato_code       = EXCLUDED.kato_code,
    updated_at      = now();

-- name: GetLotByID :one
-- Лот по публичному natural id; удалённые скрыты.
SELECT
    id,
    goszakup_lot_id,
    announcement_id,
    title_ru,
    title_kk,
    amount,
    quantity,
    unit,
    kato_code,
    is_deleted,
    imported_at,
    updated_at
FROM lots
WHERE goszakup_lot_id = $1
  AND NOT is_deleted;
