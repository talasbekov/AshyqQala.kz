---
baseline_commit: 796bd548f2f033d33b2de3c6870b7e4a100eeca6
---

# Story 1.3: cmd/api — контракт-эндпоинт, wire-форматы, OpenAPI, URL-схема

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a **гражданин**,
I want **получить данные контракта по стабильному URL**,
so that **карточка отображает реальные поля с честными состояниями**.

> **Тип истории:** третья история Epic 1. Реализует **T5** walking-skeleton (`cmd/api` chi+pgx,
> `/healthz`, `/metrics`, эндпоинт контракта). Зависит от 1.2 (таблица `contracts` + `GetContractByID`,
> **done**) и 1.1 (скелет + compose db). Готовит T7+T8 (web-карточка фетчит этот эндпоинт, Story 1.7).
> **Идёт на синтетике (seed `DEMO-0001`), НЕ ждёт токен.**

## Acceptance Criteria

**AC1 — `cmd/api` (chi v5 + pgx) отдаёт `/healthz` и `/metrics`**
**Given** `cmd/api` на chi v5 + pgx
**When** сервис запущен
**Then** отвечают `/healthz` и `/metrics` (slog + `/metrics`).

**AC2 — `GET /api/contracts/{goszakup_id}` отдаёт честный wire-формат**
**Given** `GET /api/contracts/{goszakup_id}`
**When** запрошен существующий id
**Then** ответ — JSON **snake_case**: деньги `amount_tng` **СТРОКОЙ**, даты ISO8601 `Z` (даты-без-времени — `YYYY-MM-DD`), отсутствующее поле → `{value:null, state:"no_data"}` (**НИКОГДА «0»/пустая строка**). Несуществующий id → `404` с error-кодом `NOT_FOUND`.

**AC3 — OpenAPI-арбитр генерит TS-типы и валидирует ответ**
**Given** рукописный OpenAPI YAML (арбитр)
**When** `make gen-web`
**Then** `openapi-typescript` генерит TS-типы (web), а `kin-openapi` валидирует реальный ответ хендлера в Go-тесте (НЕ генерация из Go-хендлеров).

**AC4 — URL-схема как общий контракт**
**Given** URL-схема как общий контракт
**When** зафиксирован публичный id карточки
**Then** внешний id = **natural `goszakup_id`** (для `geo_object`/`risk_flag` — UUIDv7; здесь не нужны); **суррогатный `bigint` в URL запрещён**; схема используется картой/поиском/OG одинаково.

## Tasks / Subtasks

- [x] **Task 0 — Примирить колонку денег с wire-конвенцией (РЕШЕНИЕ, см. Dev Notes) (AC2)**
  - [x] Архитектура (557/669): деньги = `bigint` целые тенге, суффикс `_tng`, на проводе СТРОКОЙ; «db-тег == json == OpenAPI дословно». В 1.2 колонка названа `amount NUMERIC(18,2)` — расхождение
  - [x] **Рекомендация (1.2 ещё не закоммичена):** поправить `migrations/0002_projection.sql` — `amount NUMERIC(18,2)` → **`amount_tng BIGINT`**; синхронно обновить `internal/store/queries/contracts.sql`, `fixtures/seed/contracts.sql` (целые тенге), интеграционный тест, перегенерить sqlc (`make gen-sqlc`). **Альтернатива (если 1.2 уже закоммичена):** аддитивная миграция `0003` (rename+retype). Зафиксировать выбор владельца
  - [x] Обновить File List Story 1.2 при правке её файлов (честность артефактов)
- [x] **Task 1 — `cmd/api`: chi v5 + pgxpool, `/healthz`, `/metrics` (AC1)**
  - [x] Добавить зависимости: `github.com/go-chi/chi/v5`; metrics — минимальный `/metrics` (prometheus client ИЛИ простой хендлер; slog уже в stdlib `log/slog`)
  - [x] `cmd/api/main.go`: читать `DATABASE_URL` (config), поднять `pgxpool.Pool`, chi-роутер; `GET /healthz` (200 + проверка `pool.Ping`); `GET /metrics`; slog структурный (ключи snake_case)
  - [x] Логика хендлеров — в `internal/httpapi` (роутер, DTO, error-маппинг); `cmd/api/main.go` тонкий (wire-up). НЕ держит `GOSZAKUP_TOKEN` (изоляция: токен только у importer)
- [x] **Task 2 — Хендлер `GET /api/contracts/{goszakup_id}` + честный wire-формат (AC2)**
  - [x] `internal/httpapi`: хендлер вызывает `gen.GetContractByID`, мапит в DTO. **Wire:** snake_case; `amount_tng` — СТРОКА целых тенге; `sign_date`/`plan_start`/`plan_end` — `YYYY-MM-DD`; `imported_at`/`updated_at` — ISO8601 `Z`
  - [x] **Честный конверт:** отсутствующее (NULL) поле → `{"value":null,"state":"no_data"}`; присутствующее → `{"value":<val>,"state":"ok"}`. В S-0 нужны только `ok`/`no_data` (полный `value_state`-enum + registry — Story 1.4). Реализовать как переиспользуемый generic-конверт в `httpapi` (не хардкодить per-field)
  - [x] Несуществующий id → `404` + тело с error-кодом `NOT_FOUND` (см. Task 5)
- [x] **Task 3 — Рукописный OpenAPI YAML (арбитр) (AC3)**
  - [x] `docs/api-contracts/openapi.yaml`: описать `GET /healthz`, `GET /api/contracts/{goszakup_id}` (path param = `goszakup_id`, не bigint), схему `Contract` с конвертом `{value,state}`, `amount_tng` string, даты, error-ответы. Honest-state enum — минимальный (`ok`,`no_data`) с пометкой «расширяется из registry в 1.4»
  - [x] Это **арбитр**, не генерация из Go-хендлеров; имена полей дословно == json-теги == db-теги
- [x] **Task 4 — gen-web (TS-типы) + kin-openapi валидация в Go-тесте (AC3)**
  - [x] `make gen-web` (таргет уже есть, guard снимется при наличии `openapi.yaml`) → `openapi-typescript` пишет `web/src/shared/api/schema.gen.ts`; `npm run build` зелёный с генерёнными типами
  - [x] Go-тест: поднять хендлер (httptest), получить реальный ответ на `DEMO-0001`, провалидировать его против `openapi.yaml` через `github.com/getkin/kin-openapi` (`openapi3filter`). Тест за `//go:build integration` если нужен pgxpool к БД, либо с мок-DBTX (предпочтительно мок — не требует БД в CI)
- [x] **Task 5 — Error-каталог + URL-схема (AC2, AC4)**
  - [x] `internal/apierr/codes.go`: закрытый UPPER_SNAKE каталог (`NOT_FOUND`, `VALIDATION_FAILED`, `INTERNAL`; `INVALID_CURSOR`/`RATE_LIMITED` — задел для list/Epic later); зеркало в OpenAPI responses
  - [x] Зафиксировать URL-схему (в OpenAPI + краткая заметка в `docs/api-contracts/`): публичный id = natural `goszakup_id`; суррогатный bigint в URL запрещён; для `geo_object`/`risk_flag` — UUIDv7 (здесь не используются)
- [x] **Task 6 — Тесты и верификация**
  - [x] `go build ./...`, `go vet`, `gofmt`, `golangci-lint` зелёные; новые зависимости (`chi`, `kin-openapi`) в go.mod/go.sum
  - [x] Юнит: wire-маппинг (NULL→`no_data`, present→`ok`, `amount_tng` строка, форматы дат); kin-openapi-валидация ответа; `404 NOT_FOUND`
  - [x] Интеграция (compose db + seed): `curl /api/contracts/DEMO-0001` → корректный JSON; `/healthz` 200; `/metrics` отвечает
  - [x] Обновить File List, Change Log, Completion Notes

### Review Findings (2026-06-21, code-review)

Состязательное ревью (Blind + Edge + Acceptance). **Acceptance Auditor: нарушений AC нет** (AC1–AC4 + scope-дисциплина соблюдены; kin-openapi реально энфорсит additionalProperties/required, суррогат `id` не утекает, токен не читается). Находки — честность/покрытие/харднинг:

**Patch:**

- [x] [Review][Patch] MED: `imported_at`/`updated_at` читаются без `.Valid` → при NULL фабрикуют `"0001-01-01T00:00:00Z"` (нарушение гардрейла честности; сейчас недостижимо — NOT NULL, но латентно) + ломают единый конверт `{value,state}` → завернуть в `Field[string]` как остальные поля [server/internal/httpapi/contracts.go, docs/api-contracts/openapi.yaml, contracts_test.go]
- [x] [Review][Patch] MED: нет теста рендера контракта со ВСЕМИ NULL-полями → регрессия no_data-конверта пройдёт CI молча [server/internal/httpapi/contracts_test.go]
- [x] [Review][Patch] MED: нет `ReadTimeout`/`WriteTimeout`/`IdleTimeout` у `http.Server` + нет per-request таймаута на запрос к БД → slowloris/зависший запрос держит соединение [server/cmd/api/main.go]
- [x] [Review][Patch] LOW: `pgxpool.New` ленив → недоступная БД не ловится на старте (стартует «здоровым») → startup `pool.Ping` (fail-fast) [server/cmd/api/main.go]
- [x] [Review][Patch] LOW: `/healthz` пишет тело без `Content-Type` (полагается на sniffing, расходится с OpenAPI `text/plain`) [server/cmd/api/main.go]

**Defer:**

- [x] [Review][Defer] `/metrics` на публичном listener без гейтинга (promhttp отдаёт runtime-внутренности) → ограничить через Caddy (path-restrict) в Story 1.7 или отдельный admin-listener [server/cmd/api/main.go] — deferred (Caddy — 1.7)
- [x] [Review][Defer] `amount_tng` = целые тенге (решение владельца) → импортёр (Epic 2) должен валидировать/округлять дробные суммы goszakup, если такие встретятся; добавить guard на overflow `>9.2e18` — deferred (Epic 2)

**Dismissed:** `/metrics` не в OpenAPI (Task 3 scope — спец-комплаенс); URL-схема в OpenAPI-параметре (AC4 ok); `format: date-time` валидируется (kin-openapi v0.140 регистрирует валидатор — проверено Edge против исходников); comma-ok в тесте (паники нет, читается из nil-map); enum `[ok,no_data]` заперт (1.4 расширит — намеренная граница); `kin-openapi` в `require` (Go не умеет test-only require); относительный путь к yaml в тесте (тест бежит из каталога пакета); правка `0002` in-place вместо `0003` (greenfield, не закоммичено — задокументировано в Change Log 1.2); `_ = json.Encode` swallow (стандартный Go-паттерн); нейминг `StringField`↔`Field[T]` (косметика).

## Dev Notes

### Контекст истории (T5)

После 1.2 (таблица `contracts` + `GetContractByID` через sqlc/pgx) эта история поднимает HTTP-слой:
`cmd/api` (chi+pgx), служебные эндпоинты, и **первый публичный эндпоинт** карточки с честным
wire-форматом. Это T5; за ней T6 (seed — уже есть), T7 (web фетчит), T8 (Caddy proxy, сквозной байт —
DoD, Story 1.7).
[Source: _bmad-output/planning-artifacts/architecture.md#Walking-Skeleton (строка 359)]
[Source: _bmad-output/planning-artifacts/epics.md#Story-1.3 (строки 874–896)]

### ⚠️ РЕШЕНИЕ: колонка денег (`amount` → `amount_tng`)

Архитектура (несущая wire-конвенция): **деньги — `bigint` целые тенге, суффикс `_tng`, на проводе
СТРОКОЙ** (`"amount_tng":"1234567890"`); анти-паттерн — `money float/number > 2^53`. И: «JSON
snake_case; json-тег == OpenAPI-имя == db-тег **дословно**».
[Source: architecture.md строки 524, 557, 669]

В 1.2 колонка названа `amount NUMERIC(18,2)` (data-model не задавал тип). Чтобы db==wire дословно и
без float-денег, **колонку нужно привести к `amount_tng BIGINT`** (целые тенге):
- **Рекомендуемый путь (1.2 НЕ закоммичена):** поправить `0002_projection.sql` прямо (rename+retype),
  обновить query/seed/integration-test, `make gen-sqlc`. Минимум churn, чисто.
- **Альтернатива (если закоммичена):** аддитивная миграция `0003_*` (`ALTER ... RENAME`, смена типа).
- Если goszakup-суммы имеют тиын (копейки) — округление/хранение решает владелец; архитектура выбрала
  **целые тенге** осознанно (простота/пересчитываемость). Зафиксировать решение в Completion Notes.

> Это единственное реальное расхождение с конвенцией, найденное при планировании; примирить здесь,
> т.к. 1.3 — история wire-форматов.

### Wire-форматы (несущие конвенции — следовать дословно)

- **snake_case на проводе**; acronyms lower (`bin/id/kato/url`); `json`-тег == OpenAPI == `db`. [Source: architecture.md строки 516, 524]
- **Деньги:** `amount_tng` СТРОКА целых тенге (см. решение выше). [Source: 557, 669]
- **Даты/время:** `timestamptz` UTC → ISO8601 с `Z` (`imported_at`/`updated_at`); тип `date` без времени → `YYYY-MM-DD` (`sign_date`/`plan_*`). [Source: 559]
- **Честный конверт (tri-state):** `{value, state}`; `value:null` ⇒ честное состояние (`no_data`), не пустая строка/0. Enum `value_state` (полный: `ok·no_data·insufficient_sample·not_comparable·stale·…`) — **из registry, Story 1.4**; в 1.3 достаточно `ok`/`no_data`. [Source: 547, 564, 569–570]
- **Enum/булевы:** lower_snake строки, не int. [Source: 569]
- **Pagination:** keyset (`?limit&cursor`) — для list-эндпоинтов (Epic 6/поиск), НЕ для единичной карточки; здесь не реализуем, но не закрывать путь. [Source: 572]

### URL-схема и error-каталог

- **Публичный id = natural `goszakup_id`** (стабилен между реимпортами); **суррогатный bigint в URL — анти-паттерн**. Сущности без natural-id (`geo_object`,`risk_flag`) — UUIDv7 (в 1.3 не участвуют). Схема едина для карты/поиска/OG. [Source: architecture.md строки 526–527, 667]
- **Error-коды** — закрытый UPPER_SNAKE каталог в `internal/apierr/codes.go` (`NOT_FOUND`, `VALIDATION_FAILED`, `INVALID_CURSOR`, `RATE_LIMITED`, `INTERNAL`) + зеркало в OpenAPI responses. В 1.3 реально нужны `NOT_FOUND`/`INTERNAL`; остальные — задел. [Source: 554–555]

### OpenAPI как исполняемый контракт

- **Рукописный OpenAPI YAML — арбитр** (НЕ генерится из Go-хендлеров). `openapi-typescript` → TS-типы фронта (web их не пишет руками); `kin-openapi` (`github.com/getkin/kin-openapi`) валидирует ответ Go-хендлера в тесте. [Source: architecture.md строки 325–326, 424–425]
- **Граница со Story 1.4:** генерация OpenAPI-enum честных состояний ИЗ registry + 5-й перекрёстный тест на смыкание (`value_state` РАВНЫ в OpenAPI ∩ i18n ∩ tokens ∩ glossary) — это **1.4** (registry в S-0 спит). В 1.3 OpenAPI рукописный с минимальным enum; не дублировать 1.4. [Source: 320–328, 643–646]

### Что переиспользовать (НЕ изобретать)

- **`internal/store/gen.GetContractByID`** (1.2) — хендлер вызывает его, не пишет SQL. `gen.New(DBTX)`; в проде — `pgxpool.Pool` (реализует DBTX). [Source: 1-2-…md, store/gen]
- Скелет 1.1: пакеты `internal/{httpapi,config,metrics,clock}` (doc.go-стабы) — наполнить. `cmd/api/main.go` (пустой) — наполнить тонким wire-up. `Makefile gen-web` (guard на `openapi.yaml`) — активируется Task 3.
- `deploy/docker-compose.yml` `db` + seed `DEMO-0001` — для интеграционной проверки. `DATABASE_URL` — из `.env`/Makefile.
- **pgxpool** уже доступен (pgx v5.10.0 из 1.2).

### Соблюдение архитектуры (guardrails)

- **Изоляция токена:** `cmd/api` `GOSZAKUP_TOKEN` НЕ читает и не видит (только importer). [Source: architecture.md строки 414, 771–773]
- **Тонкий `cmd/api`, логика в `internal/httpapi`** (DTO snake, error-маппинг, конверт); чистые домены (flags/median) сюда не тянуть. [Source: 723–731]
- **Честность над домыслом:** NULL → `no_data`, никогда `0`/`""`. Это AC2 и гардрейл проекта.
- **`/metrics`:** slog + `/metrics`; SM-C1/SM-C2 — доменные хуки (появятся с флагами, Epic 4); в 1.3 — базовый `/metrics`. [Source: 478, 734]

### Тестирование / проверка готовности

- Юнит (без БД): wire-маппинг (NULL→no_data; amount_tng строка; форматы дат), kin-openapi-валидация ответа (мок-DBTX, чтобы CI не требовал БД), 404 NOT_FOUND. Это идёт в `go test ./...` (CI).
- Интеграция (compose db + seed, тег `integration`): живой `curl /api/contracts/DEMO-0001`, `/healthz`, `/metrics`.
- **DoD:** AC1–AC4; CI зелёный (vet/gofmt/golangci-lint/build/test); `make gen-web` + `npm run build` зелёные; kin-openapi-валидация проходит; `amount_tng` строка, NULL→no_data доказаны тестом.
- ⚠ Замечание из review 1.2: `ci-server` теперь кэширует Go-модули (go.sum есть); новые deps (chi, kin-openapi) попадут в go.sum.

### Previous Story Intelligence (1.1, 1.2 — done)

- 1.2: `contracts` + `GetContractByID` (pgx/v5), seed `DEMO-0001` (`amount=123456789.00` — станет целым тенге после Task 0). sqlc через **Docker** (`go run sqlc` несовместим — replace-директивы). Миграции goose; `0001` down — no-op (postgis от образа).
- 1.1: скелет, `ci-server.yml` (vet/gofmt/golangci-lint v7/build/test), `ci-web.yml` (tsc/eslint/prettier/build). `make gen-web` guard'ится на `openapi.yaml`. `web/package-lock.json` коммитить.
- Defer-долг: страж «generated==regenerated» (Story 1.4) — учтёт и OpenAPI↔TS дрейф.

### Project Structure Notes

- Наполняются существующие пакеты: `internal/httpapi` (хендлеры/DTO/конверт), `internal/apierr` (НОВЫЙ — `codes.go`; в дереве 1.1 не было `apierr`, создать в `server/internal/apierr/`), `internal/config`, `internal/metrics`, `cmd/api/main.go`. `docs/api-contracts/openapi.yaml` (новый). `web/src/shared/api/schema.gen.ts` (генерёный).
- `internal/apierr` отсутствует в дереве 1.1 — создать (архитектура его предполагает: `internal/apierr/codes.go`). Это не отклонение, а доуплотнение скелета.
- Деньги-решение (Task 0) может затронуть файлы 1.2 — обновить её File List честно.

### Web research

Версии запинить на момент реализации: `chi/v5` (последняя стабильная v5.x), `kin-openapi` (последняя стабильная), `openapi-typescript` (уже `7.9.1` в Makefile). Не брать API из памяти — сверять с актуальной докой chi/kin-openapi (особенно `openapi3filter` для response-валидации).

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-1.3 (строки 874–896)]
- [Source: _bmad-output/planning-artifacts/architecture.md — wire/JSON (516–524, 557, 559, 564, 569–572); URL/id (526–527, 667–669); OpenAPI/kin (325–326, 424–425, 643–646); error-коды (554–555); httpapi/cmd (723–734); изоляция токена (414, 771–773); /metrics (478, 734)]
- [Source: docs/AshyqQala_MVP_data_model_and_flags_v1.md — contracts (стр.43); honest states (value_state)]
- [Source: _bmad-output/implementation-artifacts/1-2-…md (done) — contracts/GetContractByID/seed/sqlc-Docker]
- [Source: CLAUDE.md — гардрейлы честности и нейтральности; планируемый стек chi/pgx]

## Dev Agent Record

### Agent Model Used

claude-opus-4-8[1m] (Claude Opus 4.8, 1M context) — dev-story workflow.

### Debug Log References

- **build/lint:** `go build/vet ./...` OK; `gofmt` чист; **golangci-lint v2.5.0 → 0 issues** (исправлен errcheck `defer resp.Body.Close()` в тесте). `go test ./...` (без тега, DB-free) → `internal/httpapi` PASS (kin-openapi + wire + 404).
- **gen-web:** `make gen-web` → `openapi-typescript 7.9.1` → `web/src/shared/api/schema.gen.ts`; web `tsc/eslint/prettier/build` зелёные.
- **живая интеграция** (compose db :55432 + seed): `cmd/api` (:8099) → `/healthz` 200 `ok`; `/api/contracts/DEMO-0001` → `amount_tng:{value:"123456789",state:"ok"}` (СТРОКА), даты `YYYY-MM-DD`, конверт на всех полях; `/api/contracts/NOPE` → 404 `{error:{code:NOT_FOUND}}`; `/metrics` 200 (`go_goroutines`,`go_info`). Teardown чистый.

### Completion Notes List

- **AC1 ✅ (live):** `cmd/api` (chi v5 + pgxpool); `/healthz` пингует pool (200/503); `/metrics` через prometheus client (go-метрики; доменные SM-C1/SM-C2 — Epic 4). slog JSON.
- **AC2 ✅ (live+unit):** `GET /api/contracts/{goszakup_id}` — snake_case; `amount_tng` СТРОКА целых тенге; даты-без-времени `YYYY-MM-DD`, timestamptz ISO8601 `Z`; честный конверт `{value,state}` (present→ok, NULL→no_data — проверено unit'ом); несуществующий → 404 `NOT_FOUND`. Конверт — переиспользуемый `Field[T]` (generic), не per-field.
- **AC3 ✅:** рукописный `docs/api-contracts/openapi.yaml` (арбитр) → `openapi-typescript` (TS-типы web) + `kin-openapi` (`openapi3.Schema.VisitJSON`) валидирует реальный ответ хендлера в Go-тесте (мок-store, без БД → в CI). НЕ генерация из хендлеров.
- **AC4 ✅:** публичный id = natural `goszakup_id` (path-параметр `{goszakup_id}`); суррогатный bigint в URL не используется (внутр. `id` на провод не отдаётся); зафиксировано в OpenAPI.
- **Task 0 (деньги):** примирено — `amount NUMERIC(18,2)` (1.2) → **`amount_tng BIGINT`** (целые тенге) по wire-конвенции (architecture 557/669). Т.к. 1.2 не закоммичена — поправлен `0002` напрямую + query/seed/integration-test + перегенерён sqlc (`AmountTng pgtype.Int8`). **Решение владельца по умолчанию:** целые тенге (тиын/копейки не храним — архитектурная простота); при необходимости тиын — отдельное решение позже.
- **Зависимости:** `go-chi/chi/v5 v5.3.0`, `prometheus/client_golang v1.23.2`, `getkin/kin-openapi v0.140.0` (+ indirect). go.sum обновлён.
- **Граница со Story 1.4:** `value_state` минимальный (`ok`/`no_data`); полный enum + registry/render + 5-й перекрёстный тест (OpenAPI∩i18n∩tokens∩glossary) — это 1.4.
- **Не закоммичено** (dev-story по умолчанию). `internal/apierr` создан (в скелете 1.1 его не было — доуплотнение по архитектуре).

### File List

**Новые:**
- `server/internal/config/config.go` — Config (DATABASE_URL, Addr); токен НЕ читает
- `server/internal/apierr/codes.go` — закрытый error-каталог + `Write` (новый пакет)
- `server/internal/metrics/metrics.go` — `/metrics` (promhttp)
- `server/internal/httpapi/field.go` — generic `Field[T]` конверт + from-pgtype хелперы
- `server/internal/httpapi/contracts.go` — `ContractDTO`, маппинг, `ContractsHandler`, `ContractStore`
- `server/internal/httpapi/contracts_test.go` — unit: kin-openapi-валидация, wire, 404 (без БД)
- `docs/api-contracts/openapi.yaml` — рукописный арбитр (Contract/StringField/Error)
- `web/src/shared/api/schema.gen.ts` — **сгенерировано** openapi-typescript (DO NOT EDIT)

**Изменённые:**
- `server/cmd/api/main.go` — стаб → wire-up (chi+pgxpool, /healthz, /metrics, эндпоинт)
- `server/go.mod` / `server/go.sum` — + chi v5.3.0, prometheus client_golang v1.23.2, kin-openapi v0.140.0
- `migrations/0002_projection.sql`, `server/internal/store/queries/contracts.sql`, `fixtures/seed/contracts.sql`, `server/internal/store/contracts_integration_test.go`, `server/internal/store/gen/{models.go,contracts.sql.go}` — **Task 0** (`amount`→`amount_tng BIGINT`; sqlc перегенерён). _Файлы Story 1.2 — см. примечание в её Change Log._
- `_bmad-output/implementation-artifacts/{1-3-…md, sprint-status.yaml}`

## Change Log

| Дата | Изменение |
|---|---|
| 2026-06-21 | code-review (Blind+Edge+Acceptance): нарушений AC нет. 5 patch **исправлены и проверены** (часть — вживую): `imported_at`/`updated_at` завёрнуты в `Field[string]` (устранена латентная фабрикация `0001-01-01` + единый конверт; OpenAPI+gen-web обновлены), добавлен тест «все поля NULL», `http.Server` Read/Write/Idle-таймауты + per-request таймаут к БД, startup `pool.Ping` (fail-fast), `/healthz` Content-Type. 2 defer (`/metrics`-гейтинг → Caddy/1.7; `amount_tng` дробные/overflow → импортёр Epic 2) в `deferred-work.md`. Статус → done. |
| 2026-06-21 | dev-story: реализован T5. `cmd/api` (chi v5 + pgxpool, `/healthz`, `/metrics`), `internal/{httpapi,apierr,config,metrics}`, эндпоинт `GET /api/contracts/{goszakup_id}` с честным конвертом `{value,state}`, рукописный OpenAPI-арбитр + `kin-openapi` Go-тест + `openapi-typescript` TS-типы. **AC1–AC4 проверены вживую** (curl: healthz/контракт/404/metrics; `amount_tng` СТРОКА; kin-openapi+web build зелёные; golangci-lint 0 issues). **Task 0:** деньги `amount`→`amount_tng BIGINT` (wire-конвенция; правлен `0002` + sqlc-регенерация). Граница с 1.4 (полный value_state/registry/render) проведена. Статус → review. |
| 2026-06-21 | Создан context engine для Story 1.3 (T5: cmd/api + wire-форматы + OpenAPI + URL-схема). Анализ: epics 1.3, architecture (wire/JSON-конвенции, URL/id, OpenAPI-арбитр+kin-openapi, error-коды, httpapi/cmd, изоляция токена), переиспользование 1.1/1.2. 4 AC, 7 задач. **Зафиксировано решение по деньгам** (`amount NUMERIC` 1.2 → `amount_tng BIGINT` по wire-конвенции 557/669). Граница со Story 1.4 (registry/render/полный honest-states enum) проведена явно. Статус → ready-for-dev. |
