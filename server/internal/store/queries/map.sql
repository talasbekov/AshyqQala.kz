-- name: ListLotsWithGeo :many
-- ⏳ ИНТЕРИМ (Story 0.8, трек «Парсер-мост»): лоты Астаны с интерим-гео для ранней карты.
-- LEFT JOIN — лот БЕЗ строки в interim_geo_lots тоже попадает в выборку (g.* = NULL → честный
-- geocode_pending у потребителя); matched (auto + координата) → точка; unmatched → без точки (НЕ 0,0).
-- Удалённые скрыты; порядок стабилен. Канонический geo_objects/кластеры/bbox — Epic 3 (3.1/3.4).
SELECT
    l.goszakup_lot_id,
    l.title_ru,
    l.title_kk,
    l.amount,
    g.lat,
    g.lon,
    g.geocode_status
FROM lots l
LEFT JOIN interim_geo_lots g ON g.goszakup_lot_id = l.goszakup_lot_id
WHERE NOT l.is_deleted
ORDER BY l.id;
