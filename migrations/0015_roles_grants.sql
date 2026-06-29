-- +goose Up
-- Story 2.6 (AC1, скорректировано code-review 2026-06-29): роли + гранты ПРОЕКЦИОННЫЕ ⊥ КУРАТОРСКИЕ (AR-4).
-- ВЕРНАЯ модель границы (табличные гранты):
--   • app_curator (Directus) НЕ пишет ПРОЕКЦИОННЫЕ таблицы (только SELECT) — это и есть чистая табличная граница;
--   • app_importer пишет проекционные read-модели И пишет org_name_aliases (importer САМ материализует
--     AUTO-псевдонимы через orgnorm.Apply) — поэтому табличного запрета importer на курат. таблицу НЕТ.
-- Выживание КУРИРУЕМЫХ строк (manual/conflict) обеспечивает SQL-ГЕЙТ `UpsertAlias ... WHERE resolve_status='auto'`
-- (row-level), а НЕ табличный grant (Story 2.3). Реальное подключение под ролями (LOGIN/пароль/DSN + GRANT
-- membership + полная классификация остальных таблиц) — ops/Story 2.2; сейчас app коннектится owner-ролью
-- (owner обходит GRANT) → миграция аддитивна и инертна для рантайма, корректность держит страж границы.
-- Роли NOLOGIN, idempotent (кластер-глобальны → переживают re-CREATE базы при том же кластере).

-- +goose StatementBegin
DO $$ BEGIN
    CREATE ROLE app_importer NOLOGIN;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
DO $$ BEGIN
    CREATE ROLE app_curator NOLOGIN;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;
-- +goose StatementEnd

-- Проекционные таблицы: importer пишет (включая rnu_entries — читает ListRNUInputs и наполняет /v2/rnu),
-- curator только читает. Identity-колонки не требуют отдельного GRANT на sequence (GENERATED AS IDENTITY).
GRANT SELECT, INSERT, UPDATE, DELETE ON contracts, lots, organizations, price_benchmarks, risk_flags, rnu_entries TO app_importer;
GRANT SELECT ON contracts, lots, organizations, price_benchmarks, risk_flags, rnu_entries TO app_curator;

-- org_name_aliases (гибрид): пишут ОБА — importer (AUTO-псевдонимы), curator (manual-разрешения). Курируемые
-- строки переживают ре-импорт через SQL-гейт `WHERE resolve_status='auto'`, не через табличный запрет.
GRANT SELECT, INSERT, UPDATE ON org_name_aliases TO app_importer;
GRANT SELECT, INSERT, UPDATE, DELETE ON org_name_aliases TO app_curator;

-- +goose Down
REVOKE ALL ON contracts, lots, organizations, price_benchmarks, risk_flags, rnu_entries, org_name_aliases FROM app_importer, app_curator;
-- DROP ROLE робастно: роли кластер-глобальны; если ops уже навесил зависимости/гранты в другой БД — не падаем.
-- +goose StatementBegin
DO $$ BEGIN
    DROP ROLE IF EXISTS app_importer;
    DROP ROLE IF EXISTS app_curator;
EXCEPTION WHEN dependent_objects_still_exist OR OTHERS THEN
    RAISE NOTICE 'roles app_importer/app_curator not dropped (зависимости в кластере) — пропуск';
END $$;
-- +goose StatementEnd
