// Command api — публичный read-only HTTP-сервис (chi + pgx). Токен goszakup НЕ видит.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/config"
	"ashyqqala/server/internal/httpapi"
	"ashyqqala/server/internal/methodology"
	"ashyqqala/server/internal/metrics"
	"ashyqqala/server/internal/store/gen"
	"ashyqqala/server/internal/store/projection"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		log.Error("config_error", "error", "DATABASE_URL пуст")
		os.Exit(1)
	}

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Error("db_connect_failed", "error", err.Error())
		os.Exit(1)
	}
	defer pool.Close()

	// fail-fast: pgxpool.New ленив (не дозванивается), поэтому пингуем явно на старте
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := pool.Ping(pingCtx); err != nil {
		pingCancel()
		log.Error("db_ping_failed", "error", err.Error())
		os.Exit(1)
	}
	pingCancel()

	// Story 4.6 (D2): seed methodology_params на старте + fail-fast при дрейфе YAML↔DB (несущий инвариант
	// пересчитываемости FR-23 — пороги в evidence воспроизводимы по публичному YAML == DB-реестру).
	params, err := methodology.Load(cfg.RegistryRoot)
	if err != nil {
		log.Error("methodology_load_failed", "error", err.Error())
		os.Exit(1)
	}
	kvs := methodology.Flatten(params)
	seedCtx, seedCancel := context.WithTimeout(context.Background(), 10*time.Second)
	seeded, err := projection.SeedMethodologyParams(seedCtx, pool, params.MethodologyVersion, kvs)
	if err != nil {
		seedCancel()
		log.Error("methodology_seed_failed", "error", err.Error())
		os.Exit(1)
	}
	if err := projection.VerifyMethodologyParams(seedCtx, pool, params.MethodologyVersion, kvs); err != nil {
		seedCancel()
		log.Error("methodology_drift", "error", err.Error())
		os.Exit(1)
	}
	seedCancel()
	log.Info("methodology_seed", "version", params.MethodologyVersion, "seeded", seeded)

	// Story 4.6 (D1): SM-C1 — DB-derived из flag_disputes (restart-safe/race-free), экспонируется через /metrics.
	metrics.RegisterSMC1(projection.NewFlagDisputeStore(pool).SMC1Counts, log)

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("db unavailable"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	r.Handle("/metrics", metrics.Handler())

	h := httpapi.ContractsHandler{Store: gen.New(pool), Log: log}
	r.Get("/api/contracts/{goszakup_id}", h.Get)

	// Story 5.3: пороги методики (FR-23) — единый источник для экрана методики (формула/пороги ВСЕГДА).
	methH := httpapi.MethodologyHandler{Params: params}
	r.Get("/api/methodology", methH.Get)

	// ⏳ ИНТЕРИМ (Story 0.8, трек «Парсер-мост»): ранняя карта лотов Астаны. Читает interim_geo_lots
	// через store (НЕ tools/scrape — изоляция); замещается живым импортом при swap scrape→ows.
	mapH := httpapi.MapLotsHandler{Store: gen.New(pool), Log: log}
	r.Get("/api/lots", mapH.List)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Info("api_listening", "addr", cfg.Addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("server_error", "error", err.Error())
		os.Exit(1)
	}
}
