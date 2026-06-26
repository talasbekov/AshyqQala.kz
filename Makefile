# AshyqQala.kz — корневой Makefile.
# Цели codegen/проверок; `gen` = сумма gen-целей. Версии генераторов ЗАПИНЕНЫ (ниже).
# В S-0 реально используются gen-sqlc + gen-web; registry спит (1.4), токены — 1.5.

# ---- запиненные версии инструментов ----
SQLC_VERSION            := 1.31.0
GOLANGCI_LINT_VERSION   := v2.5.0
OPENAPI_TS_VERSION      := 7.9.1
GOOSE_VERSION           := v3.27.1

SERVER_DIR := server
WEB_DIR    := web

# ---- БД / DATABASE_URL (переопределяется окружением; дефолт = локальный compose db) ----
POSTGRES_USER     ?= ashyqqala
POSTGRES_PASSWORD ?= ashyqqala
POSTGRES_DB       ?= ashyqqala
POSTGRES_PORT     ?= 5432
DATABASE_URL      ?= postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@127.0.0.1:$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=disable

.DEFAULT_GOAL := help

.PHONY: help
help: ## Список целей
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

# ---- codegen ----
.PHONY: gen gen-sqlc gen-web gen-tokens
gen: gen-sqlc gen-web gen-tokens ## Сумма gen-целей (sqlc + openapi-typescript + дизайн-токены)

gen-sqlc: ## sqlc generate (per-domain .sql → per-file .gen.go; анти-churn). sqlc через Docker (go run несовместим: replace-директивы)
	@if ls migrations/*.sql >/dev/null 2>&1; then \
		docker run --rm -u $$(id -u):$$(id -g) -v "$(CURDIR)":/src -w /src/$(SERVER_DIR) sqlc/sqlc:$(SQLC_VERSION) generate; \
	else \
		echo "gen-sqlc: пропуск — нет migrations/*.sql (S-0; наполняется в Story 1.2)"; \
	fi

gen-web: ## openapi-typescript: docs/api-contracts/openapi.yaml → web TS-типы (арбитр — Story 1.3)
	@if [ -f docs/api-contracts/openapi.yaml ]; then \
		npx -y openapi-typescript@$(OPENAPI_TS_VERSION) docs/api-contracts/openapi.yaml -o $(WEB_DIR)/src/shared/api/schema.gen.ts; \
	else \
		echo "gen-web: пропуск — нет docs/api-contracts/openapi.yaml (S-0; наполняется в Story 1.3)"; \
	fi

gen-tokens: ## tokens.json (DTCG) → tokens.css + tokens.ts (свой codegen) — Story 1.5
	cd $(WEB_DIR) && npm run gen-tokens

# ---- проверки ----
.PHONY: test lint build check-registry check-core
test: ## go test (server) — unit/property/golden/go-list; integration (testcontainers) — позже
	cd $(SERVER_DIR) && go test ./...

check-registry: ## Сторожа registry (Story 1.4/4.1): перекрёстный тест registry↔OpenAPI↔Go + doc-нейтральность + single-source methodology_params
	cd $(SERVER_DIR) && go test -count=1 ./internal/registry/... ./internal/render/... ./internal/methodology/...

check-core: ## Сторож ядра (Story 1.10): go-list-границы — median/flags/normalize/benchmark не импортируют store/httpapi/goszakup/время
	cd $(SERVER_DIR) && go test -count=1 ./internal/arch/...

lint: ## go vet + gofmt (server); tsc/eslint/prettier — в web-CI
	cd $(SERVER_DIR) && go vet ./... && { out=$$(gofmt -l .); [ -z "$$out" ] || { echo "gofmt: не отформатированы:"; echo "$$out"; exit 1; }; }

build: ## go build (server) + npm build (web)
	cd $(SERVER_DIR) && go build ./...
	cd $(WEB_DIR) && npm run build

# ---- инфра ----
.PHONY: db-up db-down
db-up: ## Поднять только Postgres+PostGIS (AC2)
	docker compose -f deploy/docker-compose.yml up -d db

db-down: ## Остановить compose-стек
	docker compose -f deploy/docker-compose.yml down

# ---- миграции (goose) и seed ----
.PHONY: migrate-up migrate-down migrate-status db-seed
migrate-up: ## goose: применить миграции (DATABASE_URL)
	go run github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION) -dir migrations postgres "$(DATABASE_URL)" up

migrate-down: ## goose: откатить одну миграцию
	go run github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION) -dir migrations postgres "$(DATABASE_URL)" down

migrate-status: ## goose: статус миграций
	go run github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION) -dir migrations postgres "$(DATABASE_URL)" status

db-seed: ## Применить seed (данные, не схема). Бьёт в COMPOSE-db (НЕ DATABASE_URL!); сначала migrate-up.
	# Порядок важен (FK): contracts → acts/risk_flags (Story 5.1). Все файлы идемпотентны (ON CONFLICT).
	for f in contracts acts risk_flags; do \
		docker compose -f deploy/docker-compose.yml exec -T db psql -v ON_ERROR_STOP=1 -U $(POSTGRES_USER) -d $(POSTGRES_DB) < fixtures/seed/$$f.sql; \
	done
