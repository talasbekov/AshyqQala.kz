-- +goose Up
-- Связь контракт↔организация (Story 5.2): реализует отложенный TODO 0002_projection.sql:23-27 — теперь
-- таблица organizations существует (миграция 0012), dangling FK исключён. Колонки NULL; НАПОЛНЕНИЕ
-- (живая перелинковка supplier_bin/customer_bin → organizations.id) — Story 2.2 (живой импорт). До наполнения
-- агрегаты подрядчика честно «профиль неполный» (FR-13 это санкционирует). Токен-готово: 5.2 проводит
-- запросы по supplier_org_id; когда 2.2 проставит id — список контрактов наполнится без изменения API/UI.
ALTER TABLE contracts
    ADD COLUMN customer_org_id BIGINT REFERENCES organizations(id),
    ADD COLUMN supplier_org_id BIGINT REFERENCES organizations(id);

-- Индекс под агрегаты карточки подрядчика (контракты по supplier_org_id).
CREATE INDEX contracts_supplier_org_idx ON contracts (supplier_org_id);

-- +goose Down
DROP INDEX IF EXISTS contracts_supplier_org_idx;
ALTER TABLE contracts
    DROP COLUMN IF EXISTS supplier_org_id,
    DROP COLUMN IF EXISTS customer_org_id;
