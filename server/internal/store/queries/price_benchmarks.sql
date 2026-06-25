-- name: DeleteAllPriceBenchmarks :exec
-- Очистка кэша перед публикацией нового снапшота. В ОДНОЙ транзакции с InsertPriceBenchmark = атомарный
-- swap (читатель видит старый ИЛИ новый снапшот целиком — MVCC; полупересчёт невидим).
DELETE FROM price_benchmarks;

-- name: InsertPriceBenchmark :exec
-- Вставка строки кэша медиан (часть пересчёта «в сторону»). median_price_per_km NULL = honest insufficient.
INSERT INTO price_benchmarks (comparability_key, median_price_per_km, sample_size, methodology_version)
VALUES ($1, $2, $3, $4);

-- name: GetPriceBenchmark :one
-- Медиана группы по ключу сопоставимости. Отсутствие строки → читатель отдаёт not_comparable (нечего сравнивать).
SELECT id, comparability_key, median_price_per_km, sample_size, methodology_version, computed_at
FROM price_benchmarks
WHERE comparability_key = $1;

-- name: ListPriceBenchmarks :many
-- Весь снапшот кэша; порядок по ключу стабилен (детерминизм чтения).
SELECT id, comparability_key, median_price_per_km, sample_size, methodology_version, computed_at
FROM price_benchmarks
ORDER BY comparability_key;

-- name: CountPriceBenchmarks :one
SELECT count(*) AS n FROM price_benchmarks;
