-- +goose Up
-- price_benchmarks — ПРОИЗВОДНЫЙ кэш медиан групп сопоставимости (Story 4.1, AC3). Одна истина — чистая
-- benchmark.GroupMedian; таблица перестраивается пересчётом «в сторону → атомарный swap» (читатель видит
-- снапшот целиком: старый ИЛИ новый, не полупересчёт — MVCC в одной транзакции DELETE+INSERT).
-- median_price_per_km NULL = ЧЕСТНОЕ «недостаточно сопоставимых данных» (sample_size < min_sample) или
-- «нет сопоставимого значения» — НИКОГДА 0/выдуманное. Цена — целые ₸/км (Q3; bigint, без плавающей точки).
-- methodology_version фиксирует версию методики снапшота (пересчитываемость: тот же вход+версия → тот же кэш).
CREATE TABLE price_benchmarks (
    id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    comparability_key   TEXT NOT NULL UNIQUE,           -- benchmark.Group.Key() (direction × kato)
    median_price_per_km BIGINT,                          -- NULL = insufficient/not_comparable (честно)
    sample_size         INTEGER NOT NULL,                -- размер сопоставимой выборки в окне (фиксируется)
    methodology_version TEXT NOT NULL,                   -- версия методики снапшота
    computed_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Честность: размер выборки не может быть отрицательным (ловит баг/усечение в продьюсере перед записью).
    CONSTRAINT price_benchmarks_sample_size_chk CHECK (sample_size >= 0)
);

-- +goose Down
DROP TABLE IF EXISTS price_benchmarks;
