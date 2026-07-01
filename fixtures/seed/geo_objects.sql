-- Синтетический гео-seed (Story 3.1) — НЕ goose-миграция. Идемпотентен (ON CONFLICT / WHERE NOT EXISTS).
-- Цель: продемонстрировать ТОКЕН-НЕЗАВИСИМУЮ разблокировку — `geo_objects.length_km` (миграция 0020) даёт
-- цену/км = amount_tng / length_km → медиана района ₸/км (Story 6.4/FR-18) ЗАЖИГАЕТСЯ на синтетике из
-- честного «not_comparable» в реальное значение (шов PricePerKMSamplesByDirection). При получении токена эти
-- синтетические строки замещаются живыми geo_objects БЕЗ изменения кода (та же схема/шов).

-- 1) Районы Астаны (KATO-полигоны). kato_code NULL до Story 0.1 (коды из токена) — семантика 6.3 «имена без
--    кодов». Полигоны синтетические (bbox), реальные границы — OSM/токен. Для медианы не нужны, но закрывают
--    AC2 (район по КАТО) и страницу района.
INSERT INTO districts (kato_code, name_ru, name_kk, geom)
SELECT '710000000', 'Есиль', 'Есіл',
       ST_GeomFromText('POLYGON((71.30 51.05,71.55 51.05,71.55 51.20,71.30 51.20,71.30 51.05))', 4326)
WHERE NOT EXISTS (SELECT 1 FROM districts WHERE name_ru = 'Есиль');

-- 2) Синтетические дорожные контракты Астаны (kato 710000000, direction=road) с известной ценой/км.
--    ≥ MinSample(5) в группе direction=road × КАТО-префикс '710000000%' → медиана считается (ok, не insufficient).
INSERT INTO contracts (goszakup_contract_id, subject_ru, subject_kk, amount_tng, sign_date, status, direction, kato_code, source_url)
VALUES
 ('DEMO-GEO-01','Синтетика 3.1: дорога (₸/км демо)','Синтетика 3.1: жол (₸/км демо)',240000000,'2026-05-01','active','road','710000000','https://goszakup.gov.kz/ru/contract/DEMO-GEO-01'),
 ('DEMO-GEO-02','Синтетика 3.1: дорога (₸/км демо)','Синтетика 3.1: жол (₸/км демо)',200000000,'2026-05-02','active','road','710000000','https://goszakup.gov.kz/ru/contract/DEMO-GEO-02'),
 ('DEMO-GEO-03','Синтетика 3.1: дорога (₸/км демо)','Синтетика 3.1: жол (₸/км демо)',180000000,'2026-05-03','active','road','710000000','https://goszakup.gov.kz/ru/contract/DEMO-GEO-03'),
 ('DEMO-GEO-04','Синтетика 3.1: дорога (₸/км демо)','Синтетика 3.1: жол (₸/км демо)',300000000,'2026-05-04','active','road','710000000','https://goszakup.gov.kz/ru/contract/DEMO-GEO-04'),
 ('DEMO-GEO-05','Синтетика 3.1: дорога (₸/км демо)','Синтетика 3.1: жол (₸/км демо)',150000000,'2026-05-05','active','road','710000000','https://goszakup.gov.kz/ru/contract/DEMO-GEO-05'),
 ('DEMO-GEO-06','Синтетика 3.1: дорога (₸/км демо)','Синтетика 3.1: жол (₸/км демо)',220000000,'2026-05-06','active','road','710000000','https://goszakup.gov.kz/ru/contract/DEMO-GEO-06')
ON CONFLICT (goszakup_contract_id) DO UPDATE SET
  amount_tng = EXCLUDED.amount_tng, sign_date = EXCLUDED.sign_date, direction = EXCLUDED.direction, kato_code = EXCLUDED.kato_code;

-- 3) Канонические geo_objects (LINESTRING дороги + length_km) для этих контрактов. geocode_status=auto →
--    geom ОБЯЗАН быть не-NULL (CHECK 0020). district_id — район Есиль. length_km подобран так, что
--    ₸/км = amount/length_km образует выборку с медианой ~42 млн ₸/км.
INSERT INTO geo_objects (contract_id, district_id, geom, address_text, length_km, geocode_status, confidence, geocoded_by)
SELECT c.id, d.id,
       ST_GeomFromText('LINESTRING(71.40 51.10, 71.45 51.14)', 4326),
       'Астана, ' || c.goszakup_contract_id, v.len_km, 'auto', 0.6, 'synthetic-seed'
FROM (VALUES
  ('DEMO-GEO-01', 5.0), ('DEMO-GEO-02', 5.0), ('DEMO-GEO-03', 6.0),
  ('DEMO-GEO-04', 5.0), ('DEMO-GEO-05', 6.0), ('DEMO-GEO-06', 5.0)
) AS v(gid, len_km)
JOIN contracts c ON c.goszakup_contract_id = v.gid
LEFT JOIN districts d ON d.name_ru = 'Есиль'
WHERE NOT EXISTS (SELECT 1 FROM geo_objects g WHERE g.contract_id = c.id);
