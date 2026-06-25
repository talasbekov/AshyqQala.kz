-- +goose Up
-- methodology_params — append-only ИММУТАБЕЛЬНЫЙ version-реестр порогов методики (Story 4.1, AC1; B-4).
-- Ручная правка порогов — ТОЛЬКО в registry/values/methodology_params.vN.yaml (источник истины, рантайм-
-- чтение). Эта таблица фиксирует версии в БД для evidence/пересчёта. ИММУТАБЕЛЬНОСТЬ (B-4): правка порога =
-- НОВАЯ строка/версия, история НЕ переписывается → триггер ниже запрещает UPDATE/DELETE (append-only).
-- Триггер (а не REVOKE) выбран намеренно (Q5): переносимо и явно видно в миграции.
CREATE TABLE methodology_params (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    version        TEXT NOT NULL,                       -- methodology_version (формат ^v\d+\.\d+$)
    key            TEXT NOT NULL,                       -- логический ключ порога (напр. median.min_sample)
    value          TEXT NOT NULL,                       -- значение СТРОКОЙ (пороги разных типов; как на проводе)
    description    TEXT,
    effective_from TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (version, key)                               -- один порог на версию (история по версиям)
);

-- DDL-иммутабельность (B-4): append-only. UPDATE/DELETE строки → исключение (история версий неизменна).
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION methodology_params_immutable() RETURNS trigger
    LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'methodology_params иммутабельна (append-only): % запрещён; правка порога = новая версия', TG_OP;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER methodology_params_no_update
    BEFORE UPDATE ON methodology_params
    FOR EACH ROW EXECUTE FUNCTION methodology_params_immutable();

CREATE TRIGGER methodology_params_no_delete
    BEFORE DELETE ON methodology_params
    FOR EACH ROW EXECUTE FUNCTION methodology_params_immutable();

-- TRUNCATE обходит row-level триггеры → отдельный statement-level страж (иначе append-only обходится одним
-- TRUNCATE, стирая всю историю версий вопреки B-4).
CREATE TRIGGER methodology_params_no_truncate
    BEFORE TRUNCATE ON methodology_params
    FOR EACH STATEMENT EXECUTE FUNCTION methodology_params_immutable();

-- +goose Down
-- DROP TABLE снимает свои триггеры; затем свободно дропаем функцию.
DROP TABLE IF EXISTS methodology_params;
DROP FUNCTION IF EXISTS methodology_params_immutable();
