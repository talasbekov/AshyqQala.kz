-- name: DistrictAggregates :one
-- Агрегаты района по КАТО-ПРЕФИКСУ (FR-17, Story 6.3). Район = contracts.kato_code НЕЗАВИСИМО от
-- геопривязки: гео-джойна нет → негеопривязанные объекты ВКЛЮЧЕНЫ в агрегат (AC1). $1 (kato_prefix) —
-- LIKE-паттерн вида '710%', собирается в Go из валидированного цифрами КАТО (LIKE-метасимволов нет).
-- amount_known_count: сколько контрактов с непустой суммой → хендлер честно отдаёт no_data (НЕ «0 ₸»),
-- если контракты есть, а сумм нет (честность над домыслом, epic-4-retro). 0 контрактов = честный ноль
-- района (container_state no_contracts), не «нет данных».
SELECT
    count(*)::bigint              AS contract_count,
    coalesce(sum(amount_tng), 0)::bigint AS total_amount_tng,
    count(amount_tng)::bigint     AS amount_known_count
FROM contracts
WHERE kato_code LIKE sqlc.arg('kato_prefix')::text
  AND NOT is_deleted;

-- name: DistrictActiveFlagsByType :many
-- Активные флаги контрактов района по типу (FR-17 «число активных флагов»; разбивка для блока
-- «Сигналы района»). JOIN risk_flags→contracts по КАТО-префиксу; ТОЛЬКО is_active (инвариант
-- is_active = (cleared_at IS NULL), снятые не считаются). Порядок по flag_type — стабильность wire.
SELECT rf.flag_type, count(*)::bigint AS n
FROM risk_flags rf
JOIN contracts c ON c.id = rf.contract_id
WHERE rf.is_active
  AND c.kato_code LIKE sqlc.arg('kato_prefix')::text
  AND NOT c.is_deleted
GROUP BY rf.flag_type
ORDER BY rf.flag_type;

-- name: ListContractsByDistrict :many
-- Список объектов района (FR-17 AC1), bounded (LIMIT). Публичный goszakup_contract_id (→ карточка
-- контракта); удалённые скрыты. has_active_flag — нейтральный признак «есть сигнал» (как ListContracts).
-- Порядок детерминирован (sign_date DESC NULLS LAST, goszakup_contract_id). Агрегаты считаются отдельными
-- запросами по ВСЕМ объектам — этот список лишь bounded-срез для отображения.
SELECT
    c.goszakup_contract_id,
    c.subject_ru,
    c.subject_kk,
    c.amount_tng,
    c.kato_code,
    c.direction,
    EXISTS (SELECT 1 FROM risk_flags rf WHERE rf.contract_id = c.id AND rf.is_active) AS has_active_flag
FROM contracts c
WHERE c.kato_code LIKE sqlc.arg('kato_prefix')::text
  AND NOT c.is_deleted
ORDER BY c.sign_date DESC NULLS LAST, c.goszakup_contract_id
LIMIT sqlc.arg('lim')::int;

-- name: PricePerKMSamplesByDirection :many
-- ₸/км-выборка группы сопоставимости (направление × КАТО-префикс) для медианы района/города (FR-18).
-- ШОВ Story 6.4 НАПОЛНЕН Story 3.1: цена/км = amount_tng / geo_objects.length_km для дорог с известной
-- длиной (LINESTRING, 3.1). Целочисленная (bigint) — детерминизм без float (ядро benchmark.Sample.PricePerKM).
-- Только контракты с суммой И геообъектом length_km > 0. sign_date → unix (скользящее окно в GroupMedian).
SELECT
    (c.amount_tng / g.length_km)::bigint                 AS price_per_km,
    coalesce(extract(epoch FROM c.sign_date), 0)::bigint AS sign_date_unix
FROM contracts c
JOIN geo_objects g ON g.contract_id = c.id
WHERE c.direction = sqlc.arg('direction')::text
  AND c.kato_code LIKE sqlc.arg('kato_prefix')::text
  AND NOT c.is_deleted
  AND c.amount_tng IS NOT NULL
  AND g.length_km IS NOT NULL
  AND g.length_km > 0;
