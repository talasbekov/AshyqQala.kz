// Command importer — единственный держатель GOSZAKUP_TOKEN; пишет в проекции (failure domain ⊥ cmd/api).
//
// Story 2.4: проводит ПОСТИМПОРТНЫЙ КОНВЕЙЕР (pipeline.RunPostImport) в зафиксированном порядке
// нормализация (FR-2) → хук пересчёта (benchmark swap → flags) → атомарная публикация снапшота.
// Шов реален и наблюдаем в бинаре (анти-фиктивность): пересчёт benchmark + флага РНУ выполняется над уже
// материализованной проекцией / `rnu_entries` (токен-независимо).
//
// ДЕСКОУП (честно, Story 2.2): живой инкрементальный импорт `/v2/journal` (Source→decode→UPSERT проекций) и
// живые провайдеры флагов «единственный участник»/«цена за км»/«монополия» (суммы/участники из ows_v2) ещё
// НЕ подключены — здесь их провайдеры пусты (нет входов → пересчёт этих флагов no-op, существующие НЕ
// трогаются), а нормализация-над-источником вынесена в 2.2. Порядок и одиночность джоба уже зашиты.
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/clock"
	"ashyqqala/server/internal/config"
	"ashyqqala/server/internal/ingest/pipeline"
	"ashyqqala/server/internal/methodology"
	"ashyqqala/server/internal/store/projection"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		log.Error("config_error", "error", "DATABASE_URL пуст")
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("db_connect_failed", "error", err.Error())
		os.Exit(1)
	}
	defer pool.Close()

	// fail-fast: pgxpool.New ленив — пингуем явно (как cmd/api).
	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	if err := pool.Ping(pingCtx); err != nil {
		pingCancel()
		log.Error("db_ping_failed", "error", err.Error())
		os.Exit(1)
	}
	pingCancel()

	// methodology_params (flags.Params) — источник порогов + канонической methodology_version для пересчёта.
	params, err := methodology.Load(cfg.RegistryRoot)
	if err != nil {
		log.Error("methodology_load_failed", "error", err.Error())
		os.Exit(1)
	}

	benchStore := projection.NewBenchmarkStore(pool, params)
	riskStore := projection.NewRiskFlagStore(pool, params)

	// Один «сейчас» на весь проход пересчёта (дата-зависимый флаг РНУ не пересечёт границу суток в длинном
	// батче — закрытие долга deferred-work.md:218).
	frozen := pipeline.FreezeClock(clock.Real{})

	// Композит флагов = ОДИН recalc.FlagRecalculator из четырёх флагов Epic 4, пересчитываемых ПОСЛЕ benchmark
	// (порядок recalc.Run). АСИММЕТРИЯ пустых входов: benchmark РЕПУБЛИКУЕТ пустой снапшот честно (нет length_km
	// → нет вычислимых медиан; ReplaceSnapshot заменяет любой набор пустым), а флаги single/price/monopoly с
	// ПУСТЫМИ провайдерами — no-op (applyFlagOutcomes ранний выход на 0 исходов → существующие флаги НЕ
	// трогаются): честный «нет живых входов до Story 2.2». РНУ — живой токен-независимый провайдер из `rnu_entries`.
	emptySingle := func(context.Context) ([]projection.SingleParticipantInput, error) { return nil, nil }
	emptyPrice := func(context.Context) ([]projection.PricePerKMInput, error) { return nil, nil }
	emptyMonopoly := func(context.Context) ([]projection.MonopolyInput, error) { return nil, nil }
	flagsRecalc := pipeline.CompositeFlags{
		projection.SingleParticipantRecalculator{Store: riskStore, Provider: emptySingle},
		projection.PricePerKMRecalculator{Store: riskStore, Bench: benchStore, Provider: emptyPrice},
		projection.MonopolyRecalculator{Store: riskStore, Provider: emptyMonopoly},
		projection.RNURecalculator{Store: riskStore, Clock: frozen, Provider: riskStore.ListRNUInputs},
	}
	hook := pipeline.RecalcRunHook{Bench: benchStore, Flags: flagsRecalc}

	// Нормализатор не задан: живой импорт+нормализация над источником — Story 2.2 (нормализация-движок есть в
	// 2.3, но наполнение источника токен-зависимо). Конвейер всё равно фиксирует порядок и публикует снапшот.
	log.Info("post_import_start", "normalizer", "deferred(Story 2.2)", "hook", "benchmark+flags(rnu live)")

	runCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := pipeline.RunPostImport(runCtx, nil, hook); err != nil {
		log.Error("post_import_failed", "error", err.Error())
		os.Exit(1)
	}
	log.Info("post_import_done", "methodology_version", params.MethodologyVersion)
}
