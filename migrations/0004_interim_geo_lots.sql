-- +goose Up
-- ⏳ ИНТЕРИМ (трек «Парсер-мост», Story 0.7): batch-Nominatim-координаты scraped-лотов для ранней карты (0.8).
-- НЕ канонический geo_objects — его строит Epic 3 / Story 3.1 (полная PostGIS geometry POINT|LINESTRING,
-- district FK, кураторская правка Directus, length_km). Здесь МИНИМУМ:
--   • точка как lat/lon DOUBLE (без PostGIS-geometry — проще для sqlc; интерим не нужна полная геометрия);
--   • связь по СТАБИЛЬНОМУ goszakup_lot_id (переживает ре-импорт; БЕЗ hard FK к проекции lots — AR-4:
--     кураторские ⊥ проекционные, проекция перестраивается импортёром, FK к ней хрупок);
--   • lat/lon NULL для негеокодированного (честность: «без точки», НЕ 0,0).
-- УДАЛЯЕТСЯ/ЗАМЕЩАЕТСЯ при получении токена (как интерим-команды трека); данные мигрируют в geo_objects (Epic 3).
CREATE TABLE interim_geo_lots (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    goszakup_lot_id TEXT NOT NULL UNIQUE,           -- логическая связь с lots.goszakup_lot_id (стабильный natural id)
    lat             DOUBLE PRECISION,               -- NULL для unmatched (НЕ 0,0)
    lon             DOUBLE PRECISION,               -- NULL для unmatched
    kato_code       TEXT,
    geocode_status  TEXT NOT NULL,                  -- auto | unmatched
    confidence      DOUBLE PRECISION,               -- NULL в интериме (importance не парсится; см. Story 0.7 Q6)
    address_text    TEXT,                           -- РЕАЛЬНАЯ строка, поданная геокодеру (воспроизводимость)
    geocoded_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Гардрейл честности В БД (не только в коде): домен статуса + «обе координаты или ни одной»
    -- (нет половинчатой точки; unmatched ⇒ обе NULL).
    CONSTRAINT interim_geo_lots_status_chk CHECK (geocode_status IN ('auto', 'unmatched')),
    CONSTRAINT interim_geo_lots_latlon_chk CHECK ((lat IS NULL) = (lon IS NULL))
);

-- +goose Down
DROP TABLE IF EXISTS interim_geo_lots;
