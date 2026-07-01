-- +goose Up
-- Story 3.1 (Epic 3): КАНОНИЧЕСКАЯ геопривязка. Вводит две КУРАТОРСКИЕ таблицы (AR-4: пишут геокодер-batch +
-- Directus 3.2; импортёр только читает) — НЕ путать с интеримом `interim_geo_lots` (0004, lot-keyed doubles,
-- удаляется при токене). PostGIS geometry SRID 4326 (extension включена в 0001). Токен-независима: механизм +
-- схема строятся на синтетике; живой ≥70%-вердикт (FR-6/3.3) и реальные length_km-значения — дескоуп на токен.

-- districts — районы Астаны (KATO-полигоны). kato_code NULLABLE: коды подтверждаются Story 0.1 (токен,
-- /search/getKato) — прецедент registry/values/astana_districts.json «имена без кодов» (Story 6.3). Полигоны —
-- синтетические bbox сейчас; реальные границы OSM/токен. Привязка geo_objects + агрегаты района (6.3/6.4).
CREATE TABLE districts (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kato_code   TEXT,                          -- NULL до Story 0.1 (см. выше)
    name_ru     TEXT NOT NULL,
    name_kk     TEXT NOT NULL,
    geom        geometry(Polygon, 4326),       -- полигон района (SRID 4326, WGS84)
    imported_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX districts_geom_gist ON districts USING GIST (geom);
CREATE UNIQUE INDEX districts_kato_uniq ON districts (kato_code) WHERE kato_code IS NOT NULL;

-- geo_objects — точка/полилиния объекта закупки (data-model D, UC-10). Вешается на CONTRACT (не lot — интерим
-- на lot). `geom` POINT|LINESTRING; `length_km` для дорог → цена/км (флаг 4.3 + медиана района 6.4 УЖЕ построены
-- и ждут именно эту колонку). Публичный id = UUID (AR-19: сущности без natural-id; UUIDv7 app-side — цель, здесь
-- gen_random_uuid()=v4 как токен-независимый плейсхолдер). geocode_status auto|manual|unmatched (manual пишет 3.2).
CREATE TABLE geo_objects (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,  -- внутр.
    public_id      UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,   -- публичный (AR-19)
    contract_id    BIGINT REFERENCES contracts(id),                  -- nullable (синтетик scraped без контракта)
    district_id    BIGINT REFERENCES districts(id),                  -- район по КАТО (проставляется даже без точки)
    geom           geometry(Geometry, 4326),                         -- POINT|LINESTRING; NULL ⟺ unmatched
    address_text   TEXT,                                             -- реальная строка запроса к геокодеру
    length_km      DOUBLE PRECISION,                                 -- для LINESTRING (дороги); NULL для POINT/нет длины
    geocode_status TEXT NOT NULL,                                    -- auto | manual | unmatched
    confidence     DOUBLE PRECISION,                                 -- Nominatim importance (NULL если нет)
    geocoded_by    TEXT,                                             -- провенанс: 'nominatim'|'directus'|...
    geocoded_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Честность в БД (зеркало 0004): статус из закрытого набора; geom NULL ТОГДА И ТОЛЬКО ТОГДА когда unmatched
    -- (auto/manual ОБЯЗАНЫ нести точку — never-фейк-координата; unmatched НЕ несёт точку — never 0,0/центр города).
    CONSTRAINT geo_objects_status_chk CHECK (geocode_status IN ('auto', 'manual', 'unmatched')),
    CONSTRAINT geo_objects_geom_null_chk CHECK ((geom IS NULL) = (geocode_status = 'unmatched'))
);
CREATE INDEX geo_objects_geom_gist ON geo_objects USING GIST (geom);
CREATE INDEX geo_objects_contract_idx ON geo_objects (contract_id);
CREATE INDEX geo_objects_district_idx ON geo_objects (district_id);

-- Гранты (AR-4): geo_objects/districts — КУРАТОРСКИЕ. curator (Directus 3.2) ПИШЕТ; importer только ЧИТАЕТ
-- (обратно проекционным таблицам 0015, где importer пишет). geocode-batch подключается ролью curator/owner.
GRANT SELECT, INSERT, UPDATE, DELETE ON districts, geo_objects TO app_curator;
GRANT SELECT ON districts, geo_objects TO app_importer;

-- +goose Down
REVOKE ALL ON districts, geo_objects FROM app_curator, app_importer;
DROP TABLE IF EXISTS geo_objects;
DROP TABLE IF EXISTS districts;
