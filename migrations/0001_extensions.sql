-- +goose Up
-- PostGIS — геометрия/гео-проверки (используется с geo_objects в Epic 3). Идемпотентно:
-- образ postgis/postgis уже ставит расширение (+ зависимые topology/tiger/fuzzystrmatch),
-- но CREATE IF NOT EXISTS делает миграцию переносимой и явно документирует требование схемы.
CREATE EXTENSION IF NOT EXISTS postgis;

-- +goose Down
-- Намеренный no-op: postgis — фундаментальное расширение, провижится базовым образом
-- postgis/postgis вместе с зависимыми (topology/tiger) → DROP EXTENSION postgis невозможен без
-- CASCADE по этим зависимостям, а сносить не нами созданное при откате схемы недопустимо.
-- Обратимость схемы обеспечивается на уровне таблиц (0002).
SELECT 1;
