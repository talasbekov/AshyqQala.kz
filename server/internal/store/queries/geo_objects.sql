-- name: UpsertGeoObject :exec
-- Идемпотентный канон-UPSERT геопривязки по contract_id (Story 3.1, AC1/Task 3). geom строится из WKT
-- (geom_wkt=NULL ⇒ geom NULL — честный unmatched, НИКОГДА 0,0/центр). length_km ВСЕГДА ВЫВОДИТСЯ из geom
-- (НЕ принимается параметром — целый класс багов «забыли пересчитать» структурно невозможен): LINESTRING →
-- NULLIF(ST_Length(geom::geography)/1000, 0) — вырожденная линия (совпадающие вершины) БЕЗ длины = NULL,
-- честно, не «длина=0» (ревью 3.2); POINT/NULL → NULL. Гейт на DO UPDATE (AR-4:
-- курация переживает ре-геокод, Story 3.2): batch перезаписывает ТОЛЬКО auto|unmatched — manual (ручная
-- разметка), verified (подтверждённая точка) и wrong_reported (спорная, ждёт куратора) неприкосновенны.
INSERT INTO geo_objects (
    contract_id, district_id, geom, address_text, length_km, geocode_status, confidence, geocoded_by
) VALUES (
    sqlc.arg('contract_id'),
    sqlc.narg('district_id'),
    ST_GeomFromText(sqlc.narg('geom_wkt')::text, 4326),
    sqlc.narg('address_text'),
    CASE WHEN GeometryType(ST_GeomFromText(sqlc.narg('geom_wkt')::text, 4326)) = 'LINESTRING'
         THEN NULLIF(ST_Length(ST_GeomFromText(sqlc.narg('geom_wkt')::text, 4326)::geography) / 1000.0, 0)
         ELSE NULL END,
    sqlc.arg('geocode_status'),
    sqlc.narg('confidence'),
    sqlc.arg('geocoded_by')
)
ON CONFLICT (contract_id) WHERE contract_id IS NOT NULL DO UPDATE SET
    district_id    = EXCLUDED.district_id,
    geom           = EXCLUDED.geom,
    address_text   = EXCLUDED.address_text,
    length_km      = EXCLUDED.length_km,
    geocode_status = EXCLUDED.geocode_status,
    confidence     = EXCLUDED.confidence,
    geocoded_by    = EXCLUDED.geocoded_by,
    geocoded_at    = now()
WHERE geo_objects.geocode_status IN ('auto', 'unmatched');

-- name: FindDistrictIDByPoint :one
-- Геометрическое членство (AC2, «при наличии точки»): район, чей полигон СОДЕРЖИТ геокодированную точку.
-- pgx.ErrNoRows у вызывающего = честное «нет геометрически совпавшего района» (НЕ ошибка).
SELECT id FROM districts
WHERE geom IS NOT NULL AND ST_Contains(geom, ST_GeomFromText(sqlc.arg('geom_wkt')::text, 4326))
ORDER BY id
LIMIT 1;

-- name: FindDistrictIDByKATOPrefix :one
-- КАТО-членство БЕЗ точки (AC2, «префикс-КАТО как в 6.3», зеркало district.Catalog.NameByKATO): район,
-- чей код — префикс КАТО-кода контракта. Самое длинное совпадение первым (иерархическая вложенность).
-- kato_code ЭКРАНИРУЕТСЯ явно (ESCAPE '\'): в отличие от district.go/district.ValidKATO (регэксп
-- цифры-онли на JSON-реестре), СТОЛБЕЦ districts.kato_code (эта таблица, 0020) НЕ имеет CHECK-ограничения
-- на формат — код-ревью нашло, что % или _ в сохранённом значении иначе действовали бы как LIKE-wildcard.
SELECT id FROM districts
WHERE kato_code IS NOT NULL
  AND sqlc.arg('contract_kato')::text LIKE (replace(replace(replace(kato_code, '\', '\\'), '%', '\%'), '_', '\_') || '%') ESCAPE '\'
ORDER BY length(kato_code) DESC, id
LIMIT 1;

-- name: ListContractsForGeocode :many
-- Контракты — кандидаты batch-геокодинга: без geo_object ИЛИ с существующим auto (переген допустим).
-- Курация 3.2 (manual/verified/wrong_reported) и unmatched НИКОГДА не выбираются повторно (AR-4: курация
-- не трогается батчем; verified терминален — подтверждённые не жгут rate-limit Nominatim). Удалённые исключены.
-- БЕЗ SQL LIMIT (зеркало ListLots/0.7): «-max» — срез на стороне Go (LIMIT 0 в Postgres = ноль строк, НЕ
-- «без лимита» — этот footgun обходим на уровне вызывающего, не здесь).
SELECT c.id, c.goszakup_contract_id, c.subject_ru, c.kato_code
FROM contracts c
LEFT JOIN geo_objects g ON g.contract_id = c.id
WHERE NOT c.is_deleted
  AND (g.id IS NULL OR g.geocode_status = 'auto')
ORDER BY c.id;

-- name: GetGeoObjectByContractID :one
-- Гео-результат по contract_id (для тестов/проверки, аналог GetGeoLotByLotID). geom → GeoJSON явным
-- ::jsonb-алиасом (AR-19: наружу всегда GeoJSON [lon,lat]; sqlc НЕ трогает raw geometry напрямую).
SELECT
    id, public_id, contract_id, district_id, address_text, length_km,
    geocode_status, confidence, geocoded_by, geocoded_at,
    ST_AsGeoJSON(geom)::jsonb AS geom_geojson
FROM geo_objects
WHERE contract_id = sqlc.arg('contract_id');

-- name: ListGeoObjectsByStatus :many
-- Очередь куратора (Story 3.2, зеркало ListAliasesByStatus/2.3): unmatched/wrong_reported — требуют ручной
-- работы; auto — кандидаты на верификацию (сомнительные первыми: confidence ASC NULLS FIRST). Go-потребители:
-- тесты («имитация Directus») и будущий провод error_reports→wrong_reported; сам Directus читает таблицу
-- напрямую (пресет-фильтр коллекции, решение D3).
SELECT
    id, public_id, contract_id, district_id, address_text, length_km,
    geocode_status, confidence, geocoded_by, geocoded_at,
    ST_AsGeoJSON(geom)::jsonb AS geom_geojson
FROM geo_objects
WHERE geocode_status = sqlc.arg('geocode_status')
ORDER BY confidence ASC NULLS FIRST, id;

-- name: ResolveGeoObjectManually :execrows
-- Куратор СТАВИТ/КОРРЕКТИРУЕТ геометрию (POINT|LINESTRING) → строка становится manual (AC1). Допустимо из
-- любого статуса, КРОМЕ verified (D1: перерисовка ПОДТВЕРЖДЁННОЙ точки — сначала пометка wrong_reported,
-- двухшаговый след для SM-C2 — ревью 3.2 выровняло по списку переходов D1). length_km здесь НЕ задаётся —
-- выводится триггером 0022 при изменении geom (LINESTRING → длина, POINT → NULL). confidence обнуляется
-- честно: Nominatim importance не применим к человеческой разметке. :execrows — вызывающий видит «0 строк»
-- (id не найден ИЛИ запрещённый verified→manual) без домысла.
UPDATE geo_objects SET
    geom           = ST_GeomFromText(sqlc.arg('geom_wkt')::text, 4326),
    geocode_status = 'manual',
    confidence     = NULL,
    geocoded_by    = sqlc.arg('geocoded_by'),
    geocoded_at    = now()
WHERE id = sqlc.arg('id') AND geocode_status <> 'verified';

-- name: MarkGeoObjectVerified :execrows
-- Куратор ПОДТВЕРЖДАЕТ авто-точку (AC3, AR-28): ТОЛЬКО auto→verified. Точка/провенанс/confidence НЕ
-- меняются (геометрия та же, ставил её nominatim — verified фиксирует лишь факт проверки человеком).
-- Запрещённые переходы (manual/unmatched/wrong_reported → verified) дают 0 строк, не тихую запись.
UPDATE geo_objects SET geocode_status = 'verified'
WHERE id = sqlc.arg('id') AND geocode_status = 'auto';

-- name: MarkGeoObjectWrongReported :execrows
-- Пометка «точка не там» (AC3, AR-28 — расширение FR-28; питает SM-C2): auto|manual|verified →
-- wrong_reported. Спорная геометрия СОХРАНЯЕТСЯ (факт, не подмена) до решения куратора: перерисовать
-- (ResolveGeoObjectManually) или снять (ClearGeoObjectToUnmatched). unmatched помечать нечем — 0 строк.
UPDATE geo_objects SET geocode_status = 'wrong_reported'
WHERE id = sqlc.arg('id') AND geocode_status IN ('auto', 'manual', 'verified');

-- name: ClearGeoObjectToUnmatched :execrows
-- Куратор СНИМАЕТ неверную точку (wrong_reported → unmatched): geom=NULL — объект честно «без точки на
-- карте» (AC2), length_km обнулит триггер 0022 (geom NULL ⇒ длины нет). ТОЛЬКО из wrong_reported: снятие
-- подтверждённых/ручных точек — через wrong_reported (двухшаговый след для SM-C2), не мимо него.
UPDATE geo_objects SET
    geom           = NULL,
    geocode_status = 'unmatched',
    confidence     = NULL,
    geocoded_by    = sqlc.arg('geocoded_by'),
    geocoded_at    = now()
WHERE id = sqlc.arg('id') AND geocode_status = 'wrong_reported';
