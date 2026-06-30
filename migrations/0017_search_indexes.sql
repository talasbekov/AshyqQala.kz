-- +goose Up
-- Поиск по БИН и наименованию (FR-16, Story 6.2). pg_trgm + GIN-trgm индексы под нечёткий substring-поиск
-- наименований организаций и предметов контрактов. ВАЖНО (ловушка истории): фактический номер 0017 —
-- 0005 занята methodology_params; epics.md/architecture называют «0005_search_indexes» по УСТАРЕВШЕЙ плановой
-- нумерации, следуем фактической последовательности (последняя была 0016_notifications_outbox).
--
-- pg_trgm НЕ был включён (0001_extensions ставит только postgis). Включаем здесь. Идемпотентно
-- (CREATE IF NOT EXISTS) — образ postgis/postgis несёт contrib pg_trgm, но явное объявление делает
-- требование схемы переносимым и самодокументируемым (паттерн postgis в 0001).
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- GIN-trgm индексы ускоряют регистронезависимый ILIKE '%term%' (substring) при терме ≥3 символов
-- (короче — seq-scan; хендлер требует мин. длину 3 для имени, см. search.go). Имена по конвенции
-- <table>_<col>_trgm_idx (architecture.md:518). Двуязычные колонки индексируются раздельно (RU/KZ).
--
-- VERIFY (ICU / case-fold казахских букв, AC2): ILIKE регистронезависим ПО LC_CTYPE коллации БД. Образ
-- postgis/postgis:16-3.4 инициализирует кластер с UTF-8 локалью (en_US.utf8) → ctype Unicode-aware и
-- складывает кириллицу/казахские буквы (`lower('Қарағанды')`='қарағанды', `'Қарағанды' ILIKE 'қарағанды'`=t).
-- Это подтверждено integration-тестом store/search_integration_test.go (case-fold `Қарағанды`↔`қарағанды`).
-- Если целевой кластер развёрнут под локалью C (только ASCII-fold) — индекс/запрос надо строить через
-- `lower(col COLLATE "und-x-icu")` (ICU, локаль-независимый Unicode-фолд). Пока деплой на UTF-8 локали —
-- доп. ICU-коллация не нужна; trgm-индекс на сырой колонке корректен.
CREATE INDEX IF NOT EXISTS organizations_name_ru_trgm_idx ON organizations USING gin (name_ru gin_trgm_ops);
CREATE INDEX IF NOT EXISTS organizations_name_kk_trgm_idx ON organizations USING gin (name_kk gin_trgm_ops);
CREATE INDEX IF NOT EXISTS contracts_subject_ru_trgm_idx  ON contracts    USING gin (subject_ru gin_trgm_ops);
CREATE INDEX IF NOT EXISTS contracts_subject_kk_trgm_idx  ON contracts    USING gin (subject_kk gin_trgm_ops);

-- +goose Down
DROP INDEX IF EXISTS organizations_name_ru_trgm_idx;
DROP INDEX IF EXISTS organizations_name_kk_trgm_idx;
DROP INDEX IF EXISTS contracts_subject_ru_trgm_idx;
DROP INDEX IF EXISTS contracts_subject_kk_trgm_idx;
-- Расширение pg_trgm — намеренный no-op при откате (как postgis в 0001): contrib-расширение могло уже
-- использоваться/провижиться образом; DROP EXTENSION при откате одной миграции снёс бы общий ресурс.
-- Обратимость схемы обеспечивается на уровне индексов (выше).
SELECT 1;
