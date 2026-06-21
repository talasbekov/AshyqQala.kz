-- Синтетический seed для S-0 (НЕ goose-миграция — данные, не схема). Идемпотентен (ON CONFLICT).
-- Узнаваемая строка для сквозной проверки байта Postgres→sqlc→chi→Caddy→React (Story 1.3/1.7).
INSERT INTO contracts (
    goszakup_contract_id, subject_ru, subject_kk, amount_tng,
    sign_date, plan_start, plan_end, status, direction, kato_code, source_url
) VALUES (
    'DEMO-0001',
    'Демонстрационный контракт: ремонт автодороги (синтетика S-0)',
    'Демонстрациялық келісімшарт: автожол жөндеу (синтетика S-0)',
    123456789,
    '2026-03-15', '2026-04-01', '2026-09-30',
    'active', 'road', '710000000',
    'https://goszakup.gov.kz/ru/contract/DEMO-0001'
)
ON CONFLICT (goszakup_contract_id) DO UPDATE SET
    subject_ru = EXCLUDED.subject_ru,
    subject_kk = EXCLUDED.subject_kk,
    amount_tng = EXCLUDED.amount_tng,
    sign_date  = EXCLUDED.sign_date,
    plan_start = EXCLUDED.plan_start,
    plan_end   = EXCLUDED.plan_end,
    status     = EXCLUDED.status,
    direction  = EXCLUDED.direction,
    kato_code  = EXCLUDED.kato_code,
    source_url = EXCLUDED.source_url,
    updated_at = now();
