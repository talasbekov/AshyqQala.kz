-- name: UpsertGeoLot :exec
-- Идемпотентный UPSERT гео-результата лота по goszakup_lot_id (повтор batch-Nominatim не плодит дубли).
-- unmatched → lat/lon NULL (честность: «без точки на карте», НЕ 0,0). ⏳ интерим (Story 0.7).
INSERT INTO interim_geo_lots (
    goszakup_lot_id, lat, lon, kato_code, geocode_status, confidence, address_text
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
ON CONFLICT (goszakup_lot_id) DO UPDATE SET
    lat            = EXCLUDED.lat,
    lon            = EXCLUDED.lon,
    kato_code      = EXCLUDED.kato_code,
    geocode_status = EXCLUDED.geocode_status,
    confidence     = EXCLUDED.confidence,
    address_text   = EXCLUDED.address_text,
    geocoded_at    = now();

-- name: GetGeoLotByLotID :one
-- Гео-результат по natural goszakup_lot_id (для тестов/проверки).
SELECT
    id, goszakup_lot_id, lat, lon, kato_code, geocode_status, confidence, address_text, geocoded_at
FROM interim_geo_lots
WHERE goszakup_lot_id = $1;
