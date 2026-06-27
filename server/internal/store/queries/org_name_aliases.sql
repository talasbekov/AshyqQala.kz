-- name: UpsertAlias :exec
-- Запись решения нормализатора. ИДЕМПОТЕНТНО по (raw_name, source). Ре-импорт ПЕРЕ-выводит ТОЛЬКО авто-строки;
-- ручные разрешения оператора (manual/conflict) НЕ затираются (WHERE-гейт) — кураторская правка переживает
-- перезапись проекции (AR-4/AR-10). Авто может эскалировать в conflict при новых данных; conflict/manual «заморожены»
-- для оператора. WHERE false → ON CONFLICT DO NOTHING (без ошибки).
INSERT INTO org_name_aliases (organization_id, raw_name, source, resolve_status)
VALUES ($1, $2, $3, $4)
ON CONFLICT (raw_name, source) DO UPDATE SET
    organization_id = EXCLUDED.organization_id,
    resolve_status  = EXCLUDED.resolve_status,
    updated_at      = now()
WHERE org_name_aliases.resolve_status = 'auto';

-- name: ResolveAliasManually :exec
-- Оператор разрешает запись очереди: проставляет organization_id И статус 'manual' (берёт строку под
-- кураторское владение). Статус 'manual' (не 'auto') гарантирует, что UpsertAlias-гейт `WHERE status='auto'`
-- НЕ перезатрёт правку при ре-импорте — даже если оператор поправил бывшую auto-строку. Имитация Directus;
-- полная S-0-приёмка «правка переживает ре-импорт» — Story 2.6.
UPDATE org_name_aliases
SET organization_id = $3, resolve_status = 'manual', updated_at = now()
WHERE raw_name = $1 AND source = $2;

-- name: GetAlias :one
-- Запись псевдонима по (raw_name, source) — для проверки статуса резолва / «профиль уточняется».
SELECT id, organization_id, raw_name, source, resolve_status, detected_at, updated_at
FROM org_name_aliases
WHERE raw_name = $1 AND source = $2;

-- name: ListAliasesByStatus :many
-- Очередь оператору: псевдонимы заданного статуса (manual/conflict — «требует проверки»). Порядок стабилен.
SELECT id, organization_id, raw_name, source, resolve_status, detected_at, updated_at
FROM org_name_aliases
WHERE resolve_status = $1
ORDER BY id;

-- name: ListResolvedAliases :many
-- Разрешённые псевдонимы (organization_id проставлен) + БИН организации. Движок может использовать как
-- дополнительные известные написания (обратная связь резолва). Порядок стабилен.
SELECT a.id, a.organization_id, a.raw_name, a.source, a.resolve_status, o.bin
FROM org_name_aliases a
JOIN organizations o ON o.id = a.organization_id
ORDER BY a.id;
