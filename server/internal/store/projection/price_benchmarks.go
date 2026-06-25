package projection

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/flags"
	"ashyqqala/server/internal/registry"
	"ashyqqala/server/internal/store/gen"
)

// BenchmarkStore — доступ к ПРОИЗВОДНОМУ кэшу price_benchmarks (Story 4.1, AC3). Производное от чистой
// benchmark.GroupMedian; перестраивается пересчётом «в сторону → атомарный swap». Держит загруженные
// methodology_params (flags.Params): версия снапшота берётся ТОЛЬКО отсюда (AC5б — версия в данных ==
// канонической). Нужен пул (а не gen.DBTX), т.к. swap идёт в ЯВНОЙ транзакции (DELETE+INSERT целиком).
type BenchmarkStore struct {
	pool   *pgxpool.Pool
	params flags.Params
}

// NewBenchmarkStore — конструктор. params — загруженные methodology_params (источник methodology_version).
func NewBenchmarkStore(pool *pgxpool.Pool, params flags.Params) *BenchmarkStore {
	return &BenchmarkStore{pool: pool, params: params}
}

// BenchmarkRow — результат пересчёта одной группы сопоставимости для записи в кэш. MedianPricePerKM == nil →
// honest insufficient/not_comparable (медиана НЕ показывается; в БД NULL, никогда 0/выдуманное).
type BenchmarkRow struct {
	ComparabilityKey string
	MedianPricePerKM *int64
	SampleSize       int
}

// ReplaceSnapshot АТОМАРНО публикует новый снапшот кэша (AC3): в ОДНОЙ транзакции очищает price_benchmarks
// и вставляет новый набор. Читатель (другая транзакция) видит снапшот ЦЕЛИКОМ — старый до COMMIT, новый
// после (MVCC); полупересчёт невидим. methodology_version во всех строках — канонический (из params).
// Детерминизм: тот же вход + та же версия → тот же набор строк.
func (s *BenchmarkStore) ReplaceSnapshot(ctx context.Context, rows []BenchmarkRow) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("price_benchmarks swap: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback после успешного commit — no-op

	q := gen.New(tx)
	if err := q.DeleteAllPriceBenchmarks(ctx); err != nil {
		return fmt.Errorf("price_benchmarks swap: clear: %w", err)
	}
	for _, r := range rows {
		med := pgtype.Int8{}
		if r.MedianPricePerKM != nil {
			med = pgtype.Int8{Int64: *r.MedianPricePerKM, Valid: true}
		}
		// sample_size — колонка INTEGER (int32). Реальные размеры групп (Astana-пилот) много меньше потолка
		// int32 (~2.1млрд); БД-CHECK (sample_size >= 0) ловит отрицательное усечение. Потолок учесть при
		// общенациональном масштабе (клампинг/валидация до cast).
		if err := q.InsertPriceBenchmark(ctx, gen.InsertPriceBenchmarkParams{
			ComparabilityKey:   r.ComparabilityKey,
			MedianPricePerKm:   med,
			SampleSize:         int32(r.SampleSize),
			MethodologyVersion: s.params.MethodologyVersion,
		}); err != nil {
			return fmt.Errorf("price_benchmarks swap: insert %q: %w", r.ComparabilityKey, err)
		}
	}
	return tx.Commit(ctx)
}

// Lookup — медиана группы по ключу с ЧЕСТНЫМ состоянием (AR-16):
//   - строки нет → (nil, not_comparable): нечего сравнивать (группа/ключ отсутствует);
//   - median NULL → (nil, insufficient_sample): выборка была < min_sample (медиана не показывается);
//   - иначе → (median, ok).
func (s *BenchmarkStore) Lookup(ctx context.Context, comparabilityKey string) (*int64, registry.ValueState, error) {
	row, err := gen.New(s.pool).GetPriceBenchmark(ctx, comparabilityKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, registry.StateNotComparable, nil
	}
	if err != nil {
		return nil, registry.StateError, fmt.Errorf("price_benchmarks lookup %q: %w", comparabilityKey, err)
	}
	if !row.MedianPricePerKm.Valid {
		return nil, registry.StateInsufficientSample, nil
	}
	v := row.MedianPricePerKm.Int64
	return &v, registry.StateOK, nil
}

// Count — число строк в текущем снапшоте (для тестов/диагностики).
func (s *BenchmarkStore) Count(ctx context.Context) (int64, error) {
	return gen.New(s.pool).CountPriceBenchmarks(ctx)
}

// MedianForKey — медиана группы для расчёта флага цены/км (Story 4.3): (median, sample_size). median == nil,
// если строки нет ИЛИ median NULL (insufficient/not_comparable) → флаг цены тогда НЕ строится (честно).
// sample_size берётся из строки (для evidence/проверки достаточности в чистом флаге).
func (s *BenchmarkStore) MedianForKey(ctx context.Context, comparabilityKey string) (*int64, int, error) {
	row, err := gen.New(s.pool).GetPriceBenchmark(ctx, comparabilityKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, fmt.Errorf("price_benchmarks median %q: %w", comparabilityKey, err)
	}
	if !row.MedianPricePerKm.Valid {
		return nil, int(row.SampleSize), nil
	}
	v := row.MedianPricePerKm.Int64
	return &v, int(row.SampleSize), nil
}

// RecalcBenchmarks реализует recalc.BenchmarkRecalculator — пересчёт + атомарная публикация кэша.
//
// ДЕСКОУП 4.1 (честно): цена/км требует geo_objects.length_km (Epic 3 / Story 3.1) — её ещё НЕТ, поэтому
// сопоставимых price/km-выборок не существует и публикуется ПУСТОЙ снапшот (любой ключ → not_comparable),
// а НЕ выдуманные медианы. Сбор выборок из contracts/lots + вызов чистой benchmark.GroupMedian подключается
// вместе с length_km. Атомарный swap (ReplaceSnapshot) и контракт порядка (recalc.Run) — уже реальны и
// покрыты тестами; здесь пустой набор честно отражает отсутствие вычислимых бенчмарков на этапе 4.1.
func (s *BenchmarkStore) RecalcBenchmarks(ctx context.Context) error {
	return s.ReplaceSnapshot(ctx, nil)
}
