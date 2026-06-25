-- name: InsertMethodologyParam :exec
-- Append-only вставка строки версии порога. UPDATE/DELETE запрещены триггером (иммутабельность B-4):
-- правка порога = НОВАЯ версия. Источник ручной правки — registry/values/methodology_params.vN.yaml.
INSERT INTO methodology_params (version, key, value, description)
VALUES ($1, $2, $3, $4);

-- name: GetMethodologyParamsByVersion :many
-- Все пороги заданной версии (для evidence/пересчёта); порядок по ключу стабилен.
SELECT id, version, key, value, description, effective_from
FROM methodology_params
WHERE version = $1
ORDER BY key;
