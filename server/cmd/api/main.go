// Command api — публичный HTTP-сервис (chi + pgx): read-эндпоинты + один write-канал «сообщить об ошибке»
// (FR-28, Story 5.4). Токен goszakup НЕ видит.
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
	"ashyqqala/server/internal/lexicon"
	"ashyqqala/server/internal/methodology"
	"ashyqqala/server/internal/metrics"
	"ashyqqala/server/internal/og"
	"ashyqqala/server/internal/registry"
	"ashyqqala/server/internal/render"
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

	// Story 5.5: единый источник прозы (registry) + рендерер для OG-поверхности («текст только через render»).
	// Честный fail-fast на рассинхроне registry↔Go-консты (как методика выше).
	reg, err := registry.Load(cfg.RegistryRoot)
	if err != nil {
		log.Error("registry_load_failed", "error", err.Error())
		os.Exit(1)
	}
	renderer := render.Renderer{Reg: reg}

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

	h := httpapi.ContractsHandler{Store: gen.New(pool), Log: log, Version: params.MethodologyVersion}
	r.Get("/api/contracts/{goszakup_id}", h.Get)

	// Story 5.2 (FR-13/FR-14): карточка подрядчика по натуральному БИН. Идентичность + агрегаты (по supplier_org_id,
	// наполнение — Story 2.2) + агрегированные флаги (честная реконструкция) + метки РНУ (авто-снятие по end_date).
	// Лексикон (Story 2.3) — для детекта «профиль уточняется» по совпадению имени с conflict/manual-псевдонимом.
	lex, err := lexicon.Load(cfg.RegistryRoot)
	if err != nil {
		log.Error("lexicon_load_failed", "error", err.Error())
		os.Exit(1)
	}
	ch := httpapi.ContractorsHandler{Store: gen.New(pool), Log: log, Lexicon: lex}
	r.Get("/api/contractors/{bin}", ch.Get)

	// Story 5.5 (FR-27/UX-DR36): серверный OG-рендер карточки из ТОЙ ЖЕ проекции (h.Projection) — OG-`<meta>`-теги
	// + версионированный по methodology_version URL картинки (cache-bust, AR-20). PNG-картинку добавит T5.
	// PUBLIC_BASE_URL (если задан) → абсолютные og:url/og:image; иначе относительные (dev). Caddy маршрутизирует
	// /og/* и/или соц-краулеров на api (deploy/Caddyfile).
	ogH := og.MetaHandler{Contracts: h, Renderer: renderer, Version: params.MethodologyVersion, BaseURL: os.Getenv("PUBLIC_BASE_URL"), Log: log}
	r.Get("/og/contracts/{goszakup_id}", ogH.ServeHTTP)
	r.Get("/og/contracts/{goszakup_id}/image.png", ogH.Image) // Story 5.5 T5: серверный PNG-превью

	// Story 5.3: пороги методики (FR-23) — единый источник для экрана методики (формула/пороги ВСЕГДА).
	methH := httpapi.MethodologyHandler{Params: params}
	r.Get("/api/methodology", methH.Get)

	// Story 5.6 (AR-29/SM-5): экспорт evidence raised-флага для цитирования третьим лицом/СМИ — канонический
	// JSON + печатный нейтральный текст. Та же проекция (GetContractByID → ListContractFlags); рамка из render.
	evH := httpapi.EvidenceExportHandler{Store: gen.New(pool), Renderer: renderer, Log: log}
	r.Get("/api/contracts/{goszakup_id}/flags/{flag_type}/evidence.json", evH.GetJSON)
	r.Get("/api/contracts/{goszakup_id}/flags/{flag_type}/evidence.txt", evH.GetText)

	// ⏳ ИНТЕРИМ (Story 0.8, трек «Парсер-мост»): ранняя карта лотов Астаны. Читает interim_geo_lots
	// через store (НЕ tools/scrape — изоляция); замещается живым импортом при swap scrape→ows.
	mapH := httpapi.MapLotsHandler{Store: gen.New(pool), Log: log}
	r.Get("/api/lots", mapH.List)

	// Story 5.4 (FR-28): публичный безаккаунтный канал «сообщить об ошибке» — ПЕРВЫЙ write-эндпоинт.
	// Защита: honeypot + лимит тела + in-memory rate-limit (без новых зависимостей). Токен не нужен.
	erH := httpapi.NewErrorReportsHandler(gen.New(pool), log)
	r.Post("/api/error-reports", erH.Create)

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
