-- name: ListRNUEntries :many
-- Все записи РНУ для пересчёта флага FR-22 (Story 4.5). Источник наполнения — живой /v2/rnu-импорт (Epic 2);
-- на синтетике — seed. Пересчёт читает ВСЕ записи: активные → raised, истёкшие/будущие → not_raised (clock).
SELECT id, organization_id, goszakup_rnu_id, start_date, end_date, reason_ref, source_url
FROM rnu_entries
ORDER BY organization_id, start_date, id; -- детерминированный порядок: агрегация per-org берёт последнюю активную (наибольший start_date)

-- name: InsertRNUEntry :exec
-- Вставка записи РНУ (seed-тесты / живой импорт Epic 2).
INSERT INTO rnu_entries (organization_id, goszakup_rnu_id, start_date, end_date, reason_ref, source_url)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListRNUByOrg :many
-- Записи РНУ конкретной организации (FR-14, карточка подрядчика, Story 5.2). Метка реконструируется на ЧТЕНИИ
-- из дат (end_date NULL/в будущем = активна; авто-снятие по end_date), а не из булева «активна». Порядок стабилен.
SELECT id, organization_id, goszakup_rnu_id, start_date, end_date, reason_ref, source_url
FROM rnu_entries
WHERE organization_id = $1
ORDER BY start_date DESC, id;
