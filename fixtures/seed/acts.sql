-- Синтетический seed актов приёмки (Story 5.1, FR-11; данные, НЕ схема). Идемпотентен (ON CONFLICT по
-- natural goszakup_act_id). contract_id берётся по natural goszakup_contract_id (internal id — GENERATED).
-- signer_info — СЛУЖЕБНАЯ информация должностного лица (§6.2.4), не приватные данные. Реальные акты приходят
-- из /v2/acts при появлении токена ows_v2 (декод-граница, имена полей — VERIFY через -probe). DEMO-0002 — с актом.
INSERT INTO acts (contract_id, goszakup_act_id, act_date, signer_info, source_url)
SELECT c.id,
       'ACT-DEMO-0002',
       DATE '2026-11-30',
       'Руководитель отдела государственных закупок акимата г. Астана',
       'https://goszakup.gov.kz/ru/act/ACT-DEMO-0002'
FROM contracts c
WHERE c.goszakup_contract_id = 'DEMO-0002'
ON CONFLICT (goszakup_act_id) DO UPDATE SET
    act_date    = EXCLUDED.act_date,
    signer_info = EXCLUDED.signer_info,
    source_url  = EXCLUDED.source_url,
    updated_at  = now();
