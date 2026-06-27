-- +goose Up
-- Проекция: organizations — заказчики и подрядчики в одной таблице (БИН — ключ). Derived из снапшота
-- (импортёр перестраивает). Паттерн 0002_projection (contracts)/0003 (lots): internal bigint identity +
-- natural bin UNIQUE; двуязычные name_*. Отсутствующие значения = NULL (честное «нет данных»).
-- Колонки строго по data-model (organizations): bin, name_ru, name_kk, reg_kato, is_customer, is_supplier,
-- first_seen_at, source_url. БИН — TEXT (12 цифр, ведущие нули; точный матч), НЕ число.
CREATE TABLE organizations (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,  -- внутр. id
    bin           TEXT NOT NULL UNIQUE,                             -- natural / канонический id организации
    name_ru       TEXT,
    name_kk       TEXT,
    reg_kato      TEXT,
    is_customer   BOOLEAN NOT NULL DEFAULT FALSE,                   -- роль: выступала заказчиком
    is_supplier   BOOLEAN NOT NULL DEFAULT FALSE,                   -- роль: выступала подрядчиком
    first_seen_at TIMESTAMPTZ,                                      -- первое появление в снапшоте (NULL = нет данных)
    source_url    TEXT,
    is_deleted    BOOLEAN NOT NULL DEFAULT FALSE,
    imported_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- FK contracts.customer_org_id/supplier_org_id → organizations(id) и перелинковка контрактов — Story 2.2
-- (живая перелинковка при импорте; TODO 0002_projection.sql:23-27). Проекционная таблица ⊥ кураторские
-- (org_name_aliases/лексикон — миграция 0013, AR-4): импортёр перестраивает проекцию, кураторские — только читает.

-- +goose Down
DROP TABLE IF EXISTS organizations;
