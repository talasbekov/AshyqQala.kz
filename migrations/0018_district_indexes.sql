-- +goose Up
-- Индекс под агрегаты района (FR-17, Story 6.3). Страница района группирует/фильтрует contracts по
-- КАТО-ПРЕФИКСУ (`kato_code LIKE '710%'`): район определяется КАТО независимо от геопривязки. Номер 0018 —
-- фактическая последовательность (последняя миграция 0017_search_indexes); планерочные номера из
-- epics.md/architecture не используем (та же ловушка, что снимали в 0017).
--
-- text_pattern_ops НУЖЕН для индекс-поддержки префиксного LIKE 'x%' на UTF-8 локали (en_US.utf8, см. 0017),
-- где обычный btree сравнивает по локали и не годен для анкоренного префикса; text_pattern_ops индексирует
-- побайтно. КАТО — цифры (валидируется в Go), регистр не при чём.
-- ОГОВОРКА (честность): запрос использует bind-параметр (`kato_code LIKE $1`, $1 = kato||'%'). Планировщик
-- переписывает LIKE в index-range scan только при ПРЕФИКСЕ-КОНСТАНТЕ на этапе плана — под generic-планом pgx
-- эта оптимизация может НЕ примениться (тогда seq-scan). Для пилотного объёма seq-scan терпим (NFR-1); индекс
-- ускоряет custom-план и не вредит. Гарантированный index-range на масштабе (literal-префикс / range
-- >=kato AND <kato++) — follow-up (deferred-work).
CREATE INDEX IF NOT EXISTS contracts_kato_code_idx ON contracts (kato_code text_pattern_ops);

-- +goose Down
DROP INDEX IF EXISTS contracts_kato_code_idx;
