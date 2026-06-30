-- name: SearchOrganizations :many
-- Поиск организаций по БИН (точный) ИЛИ наименованию (нечёткий substring, pg_trgm GIN). FR-16, Story 6.2.
-- Ветвление БИН/имя — в Go (normalize.CanonicalBIN до вызова): валидный 12-зн БИН → @bin_exact задан →
-- ТОЛЬКО точный матч по UNIQUE bin (мусорный «БИН» канонизируется в "" и сюда приходит как NULL bin_exact →
-- ветка имени, без фантомной орг — FR-2/AC3). @q — исходный терм для ILIKE по имени (в БИН-ветке игнорируется
-- CASE-ом, но передаётся всегда: required-параметр). ILIKE регистронезависим по коллации БД (case-fold
-- казахских букв — integration-тест). Порядок ДЕТЕРМИНИРОВАН: релевантность (trgm similarity) DESC, затем
-- стабильный тай-брейк по bin (UNIQUE, NOT NULL). Удалённые скрыты.
SELECT id, bin, name_ru, name_kk, reg_kato, is_deleted
FROM organizations
WHERE NOT is_deleted
  AND CASE
        WHEN sqlc.narg('bin_exact')::text IS NOT NULL
          THEN bin = sqlc.narg('bin_exact')::text
        -- LIKE-метасимволы (%/_/\) в терме экранируются (default ESCAPE '\'), иначе «%%%»/«_» стали бы
        -- wildcard-ами и матчили бы всё (pattern-injection; code review 6.2). similarity ниже — на СЫРОМ q.
        ELSE (name_ru ILIKE '%' || replace(replace(replace(sqlc.arg('q')::text, '\', '\\'), '%', '\%'), '_', '\_') || '%'
              OR name_kk ILIKE '%' || replace(replace(replace(sqlc.arg('q')::text, '\', '\\'), '%', '\%'), '_', '\_') || '%')
      END
ORDER BY
  GREATEST(similarity(coalesce(name_ru, ''), sqlc.arg('q')::text),
           similarity(coalesce(name_kk, ''), sqlc.arg('q')::text)) DESC,
  bin
LIMIT sqlc.arg('lim')::int;

-- name: SearchContracts :many
-- Поиск контрактов по предмету (subject_ru/subject_kk, нечёткий substring, pg_trgm GIN). FR-16, Story 6.2.
-- Только ветка имени (БИН в предмете не ищем — это идентификатор организации). Колонки совпадают с
-- ListContracts (переиспользуем маппинг в хендлере). has_active_flag — «есть сигнал, требующий проверки»
-- (нейтрально): EXISTS активного contract-флага. Порядок ДЕТЕРМИНИРОВАН: релевантность DESC, затем
-- sign_date DESC NULLS LAST, затем goszakup_contract_id (UNIQUE тай-брейк, как 6.1). Удалённые скрыты.
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
  -- LIKE-метасимволы экранируются (см. SearchOrganizations); similarity ниже — на СЫРОМ q.
  AND (c.subject_ru ILIKE '%' || replace(replace(replace(sqlc.arg('q')::text, '\', '\\'), '%', '\%'), '_', '\_') || '%'
       OR c.subject_kk ILIKE '%' || replace(replace(replace(sqlc.arg('q')::text, '\', '\\'), '%', '\%'), '_', '\_') || '%')
ORDER BY
    GREATEST(similarity(coalesce(c.subject_ru, ''), sqlc.arg('q')::text),
             similarity(coalesce(c.subject_kk, ''), sqlc.arg('q')::text)) DESC,
    c.sign_date DESC NULLS LAST,
    c.goszakup_contract_id
LIMIT sqlc.arg('lim')::int;
