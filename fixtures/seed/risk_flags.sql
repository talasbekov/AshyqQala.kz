-- Синтетический seed risk_flags для карточки (Story 5.1; реальный compute Epic 4 наполнит это при живых
-- данных). Идемпотентен (UPSERT по partial-unique (flag_type, contract_id) WHERE contract_id IS NOT NULL,
-- как RaiseContractFlag). evidence — форма соответствующих flags.*Evidence-структур (FR-23, пересчитываемость).
-- contract_id по natural id. На карточке: DEMO-0001 → single_participant raised; DEMO-0002 → price_per_km raised.
-- methodology_version = v1.0 (совпадает с registry/values/methodology_params.v1.yaml — инвариант пересчёта).
INSERT INTO risk_flags (flag_type, subject_type, contract_id, evidence, is_active, methodology_version)
SELECT 'single_participant', 'contract', c.id,
       '{"participant_count":1,"procurement_method":"Из одного источника","exclude_methods":[],"enabled":true,"methodology_version":"v1.0"}'::jsonb,
       TRUE, 'v1.0'
FROM contracts c
WHERE c.goszakup_contract_id = 'DEMO-0001'
ON CONFLICT (flag_type, contract_id) WHERE contract_id IS NOT NULL DO UPDATE SET
    evidence            = EXCLUDED.evidence,
    is_active           = TRUE,
    cleared_at          = NULL,
    methodology_version = EXCLUDED.methodology_version;

INSERT INTO risk_flags (flag_type, subject_type, contract_id, evidence, is_active, methodology_version)
SELECT 'price_per_km', 'contract', c.id,
       '{"price_per_km":71000000,"median":38400000,"sample_size":9,"deviation_factor":1.5,"comparability_key":"road|710000000","methodology_version":"v1.0"}'::jsonb,
       TRUE, 'v1.0'
FROM contracts c
WHERE c.goszakup_contract_id = 'DEMO-0002'
ON CONFLICT (flag_type, contract_id) WHERE contract_id IS NOT NULL DO UPDATE SET
    evidence            = EXCLUDED.evidence,
    is_active           = TRUE,
    cleared_at          = NULL,
    methodology_version = EXCLUDED.methodology_version;

-- Story 5.2: контракторский флаг МОНОПОЛИЯ (FR-21, subject_type='contractor', organization_id) для 222… —
-- демонстрирует raised агрегированный флаг на карточке подрядчика. Идемпотентно (UPSERT по partial-unique
-- (flag_type, organization_id) WHERE organization_id IS NOT NULL, как RaiseContractorFlag). evidence — форма
-- MonopolyEvidence (FR-23, пересчитываемость): доля БИН по сумме ₸ в группе (КАТО×направление).
INSERT INTO risk_flags (flag_type, subject_type, organization_id, evidence, is_active, methodology_version)
SELECT 'monopoly', 'contractor', o.id,
       '{"supplier_total_tng":311000000,"group_total_tng":480000000,"share":0.648,"comparability_key":"road|710000000","min_group_contracts":5,"group_contracts":6,"methodology_version":"v1.0"}'::jsonb,
       TRUE, 'v1.0'
FROM organizations o
WHERE o.bin = '222222222222'
ON CONFLICT (flag_type, organization_id) WHERE organization_id IS NOT NULL DO UPDATE SET
    evidence            = EXCLUDED.evidence,
    is_active           = TRUE,
    cleared_at          = NULL,
    methodology_version = EXCLUDED.methodology_version;
