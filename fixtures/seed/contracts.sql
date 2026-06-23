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

-- Story 1.9: ещё два контракта-истории (вертикальный демо-срез). Ручные флаги/evidence — на фронте
-- (web/src/features/flag/contractStories.ts), гео-независимо. DEMO-0003.plan_end = NULL → честное
-- «нет данных» (AC3). Координаты точек карты — Story 1.8 (astanaPoints.ts). Идемпотентно (ON CONFLICT).
INSERT INTO contracts (
    goszakup_contract_id, subject_ru, subject_kk, amount_tng,
    sign_date, plan_start, plan_end, status, direction, kato_code, source_url
) VALUES
(
    'DEMO-0002',
    'Строительство автомобильной дороги (демо-история: цена за км)',
    'Автомобиль жолын салу (демо-оқиға: шақырым құны)',
    240000000,
    '2026-02-10', '2026-03-01', '2026-11-30',
    'active', 'road', '710000000',
    'https://goszakup.gov.kz/ru/contract/DEMO-0002'
),
(
    'DEMO-0003',
    'Ремонт участка дороги (демо-история: мало сопоставимых данных)',
    'Жол учаскесін жөндеу (демо-оқиға: салыстырмалы дерек аз)',
    18000000,
    '2026-04-05', '2026-05-01', NULL,
    'active', 'road', '710000000',
    'https://goszakup.gov.kz/ru/contract/DEMO-0003'
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
