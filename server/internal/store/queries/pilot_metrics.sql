-- Метрики этапа 5 для pilot-one-pager (Story 5.6, AC-3). Честные count'ы по проекции; источник данных
-- (синтетика/интерим/живой ows) штампуется артефактом, не выдаётся за пилотный результат.

-- name: CountMarkedContracts :one
-- N — размеченные контракты (импортированы в проекцию, не помечены удалёнными).
SELECT count(*) FROM contracts WHERE NOT is_deleted;

-- name: CountActiveFlags :one
-- M — поднятые (активные) сигналы.
SELECT count(*) FROM risk_flags WHERE is_active;

-- name: CountRecomputableFlags :one
-- X — активные сигналы, ПЕРЕСЧИТЫВАЕМЫЕ третьим лицом по опубликованной методике: непустые methodology_version
-- и evidence (оба NOT NULL по схеме; вырожденные '' / '{}' не пересчитываемы). [Story 5.6 OQ#3 ✅ вариант (а)]
SELECT count(*) FROM risk_flags
WHERE is_active AND methodology_version <> '' AND evidence <> '{}'::jsonb;
