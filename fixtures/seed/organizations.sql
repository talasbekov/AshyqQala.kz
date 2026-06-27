-- Синтетический seed организаций для карточки подрядчика (Story 5.2; реальный наполнит живой импорт Epic 2).
-- Три демо-орг покрывают честные состояния профиля:
--   222… ТОО Астана Жол   — связаны контракты (supplier_org_id) → агрегаты видны; монополия raised; РНУ активна;
--   333… ТОО СуВодоканал  — БЕЗ связанных контрактов → «профиль неполный»; РНУ истекла (auto-clear);
--   444… ТОО Дала Строй   — manual-псевдоним → «профиль уточняется» (флаги не основание).
-- Идемпотентно (UPSERT по bin).
INSERT INTO organizations (bin, name_ru, name_kk, reg_kato, is_supplier) VALUES
    ('222222222222', 'ТОО Астана Жол',   'Астана Жол ЖШС',   '710000000', TRUE),
    ('333333333333', 'ТОО СуВодоканал',  'СуВодоканал ЖШС',  '710000000', TRUE),
    ('444444444444', 'ТОО Дала Строй',   'Дала Строй ЖШС',   '710000000', TRUE)
ON CONFLICT (bin) DO UPDATE SET
    name_ru     = EXCLUDED.name_ru,
    name_kk     = EXCLUDED.name_kk,
    reg_kato    = EXCLUDED.reg_kato,
    is_supplier = EXCLUDED.is_supplier;

-- Связь демо-контрактов с подрядчиком 222… (supplier_org_id) — демонстрирует список/агрегаты (FR-13).
-- Реальная перелинковка — Story 2.2 (живой импорт); здесь seed показывает наполненную ветку.
UPDATE contracts
SET supplier_org_id = (SELECT id FROM organizations WHERE bin = '222222222222')
WHERE goszakup_contract_id IN ('DEMO-0001', 'DEMO-0002');

-- 444…: CONFLICT-псевдоним с organization_id=NULL (так нормализатор 2.3 пишет неразрешённый conflict).
-- raw_name «ТОО Дала-Строй» канонически совпадает с именем орг «ТОО Дала Строй» (дефис→пробел) → карточка
-- детектит спорную идентичность ПО ИМЕНИ → «профиль уточняется» (review-фикс F1; conflict не привязан по FK).
INSERT INTO org_name_aliases (organization_id, raw_name, source, resolve_status)
VALUES (NULL, 'ТОО Дала-Строй', 'contract', 'conflict')
ON CONFLICT (raw_name, source) DO NOTHING;
