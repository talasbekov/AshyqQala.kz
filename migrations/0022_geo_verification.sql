-- +goose Up
-- Story 3.2 (Epic 3): статусы ВЕРИФИКАЦИИ гео (AR-28 → SM-C2) в ЗАКРЫТОМ enum geocode_status — одна ось
-- (решение D1 истории; происхождение точки хранит geocoded_by):
--   verified       — куратор ПОДТВЕРДИЛ авто-точку. Терминально для batch: ListContractsForGeocode
--                    пере-выбирает только auto → подтверждённые контракты перестают жечь rate-limit
--                    Nominatim каждый прогон (закрывает деферред ревью 3.1 «нет терминального статуса»).
--   wrong_reported — точка помечена «не там» (AR-28, расширение FR-28): несёт СПОРНУЮ геометрию до решения
--                    куратора (перерисовать → manual; снять → unmatched с geom=NULL). Дефектная точка
--                    остаётся видимой как факт, не подменяется выдуманной.
-- Honesty-CHECK geo_objects_geom_null_chk (0020) НЕ меняется: (geom IS NULL) = (status = 'unmatched') —
-- verified/wrong_reported ОБЯЗАНЫ нести геометрию; unmatched — никогда (never-фейк-координата).
-- ALTER под коротким ACCESS EXCLUSIVE (зеркало 0021): таблица мала (S-0 синтетика), горячего трафика нет.
ALTER TABLE geo_objects DROP CONSTRAINT geo_objects_status_chk;
ALTER TABLE geo_objects ADD CONSTRAINT geo_objects_status_chk
    CHECK (geocode_status IN ('auto', 'manual', 'unmatched', 'verified', 'wrong_reported'));

-- Контракт типа геометрии (AC1: «POINT — объект; LINESTRING — дорога», ревью 3.2): map-интерфейс Directus
-- не ограничивает тип рисуемой фигуры — без CHECK куратор мог бы сохранить MULTILINESTRING/POLYGON, строка
-- стала бы manual, а length_km молча NULL («дорога» навсегда выпала бы из цены/км 4.3 и медианы 6.4).
-- Честная ошибка на сохранении лучше тихой потери длины.
ALTER TABLE geo_objects ADD CONSTRAINT geo_objects_geom_type_chk
    CHECK (geom IS NULL OR GeometryType(geom) IN ('POINT', 'LINESTRING'));

-- length_km для ПРЯМОЙ записи Directus (решение D2). SQL-вывод длины в UpsertGeoObject покрывает только
-- batch/Go-путь; Directus пишет в таблицу мимо Go — без триггера ручная LINESTRING осталась бы с
-- length_km=NULL и выпала бы из цены/км (флаг 4.3) и медианы района (6.4). Правила:
--   * не-LINESTRING (POINT/geom NULL)   -> length_km := NULL (у точки нет длины — честно, не «0»);
--   * INSERT LINESTRING без length_km   -> вывести NULLIF(ST_Length(geom::geography)/1000, 0) —
--                                          вырожденная линия (совпадающие вершины) без длины = NULL, не «0»;
--   * INSERT LINESTRING с length_km     -> уважать явное значение (seed задаёт СВОЙ length при одинаковой
--                                          геометрии — GENERATED COLUMN поэтому не подходит, урок 3.1);
--   * UPDATE с ИЗМЕНЁННЫМ geom          -> пересчитать (длина следует за перерисованной линией);
--   * UPDATE без изменения geom         -> length_km не трогать (явные правки уважаются).
-- +goose StatementBegin
CREATE FUNCTION geo_objects_derive_length_km() RETURNS trigger AS $$
BEGIN
    IF NEW.geom IS NULL OR GeometryType(NEW.geom) <> 'LINESTRING' THEN
        NEW.length_km := NULL;
    ELSIF (TG_OP = 'INSERT' AND NEW.length_km IS NULL)
       OR (TG_OP = 'UPDATE' AND NEW.geom IS DISTINCT FROM OLD.geom) THEN
        NEW.length_km := NULLIF(ST_Length(NEW.geom::geography) / 1000.0, 0);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER geo_objects_length_km_trg
    BEFORE INSERT OR UPDATE ON geo_objects
    FOR EACH ROW EXECUTE FUNCTION geo_objects_derive_length_km();

-- +goose Down
-- Down best-effort (dev-инструмент): узкий CHECK не вернётся, если verified/wrong_reported-строки уже есть.
DROP TRIGGER geo_objects_length_km_trg ON geo_objects;
DROP FUNCTION geo_objects_derive_length_km();
ALTER TABLE geo_objects DROP CONSTRAINT geo_objects_geom_type_chk;
ALTER TABLE geo_objects DROP CONSTRAINT geo_objects_status_chk;
ALTER TABLE geo_objects ADD CONSTRAINT geo_objects_status_chk
    CHECK (geocode_status IN ('auto', 'manual', 'unmatched'));
