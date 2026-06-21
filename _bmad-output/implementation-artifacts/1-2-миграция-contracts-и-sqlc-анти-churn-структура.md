---
baseline_commit: 796bd548f2f033d33b2de3c6870b7e4a100eeca6
---

# Story 1.2: Миграция contracts и sqlc (анти-churn структура)

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a **разработчик платформы AshyqQala.kz**,
I want **таблицу `contracts` (goose-миграция) и типобезопасный доступ через sqlc с анти-churn раскладкой**,
so that **карточка контракта (Story 1.3/1.7) читает реальную строку из БД без ручных SQL-склеек**.

> **Тип истории:** вторая история Epic 1. Реализует **T3+T4** walking-skeleton-последовательности S-0
> (на синтетике, НЕ ждёт токен). Зависит от Story 1.1 (скелет `server/` + `compose db`, **done**).
> Готовит почву для Story 1.3 (`cmd/api` читает `GetContractByID`) и 1.7 (сквозная карточка).
> **Скоуп строго: ОДНА таблица `contracts` (минимум колонок, БЕЗ flags/geo) + sqlc + seed.**

## Acceptance Criteria

**AC1 — goose-миграции применяются и откатываются чисто**
**Given** каталог `migrations/`
**When** применены goose-миграции `0001_extensions` (PostGIS) и `0002` таблицы `contracts` (минимум колонок, БЕЗ flags/geo; natural `goszakup_contract_id` UNIQUE; двуязычные `subject_kk`/`subject_ru`; up/down)
**Then** миграция применяется (`goose up`) и **откатывается чисто** (`goose down`).

**AC2 — sqlc генерит типобезопасный доступ, анти-churn**
**Given** `sqlc.yaml` с per-domain `.sql` (`contracts.sql`) и раздельными пакетами
**When** `make gen-sqlc`
**Then** сгенерирован `GetContractByID`, `go build ./...` зелёный, и codegen **НЕ складывает все запросы в один `.gen.go`** (анти-churn: per-file вывод).

**AC3 — seed вставляет узнаваемую строку**
**Given** seed
**When** применён
**Then** вставлена строка-контракт с **узнаваемым значением** (для проверки сквозного байта в 1.3/1.7).

## Tasks / Subtasks

- [x] **Task 1 — goose-тулинг + миграция `0001_extensions` (PostGIS) (AC1)**
  - [x] Запинить goose в `Makefile` (`GOOSE_VERSION`, напр. `v3.24.x`); добавить таргеты `migrate-up`/`migrate-down`/`migrate-status` через `go run github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION) -dir migrations postgres "$$DATABASE_URL" up|down|status` (по образцу запиненного `gen-sqlc`)
  - [x] `DATABASE_URL` берётся из окружения/`.env` (см. `deploy/.env.example`: `postgres://ashyqqala:...@127.0.0.1:5432/ashyqqala?sslmode=disable`); по умолчанию — локальный compose `db`
  - [x] `migrations/0001_extensions.sql` (goose-формат `-- +goose Up` / `-- +goose Down`): up = `CREATE EXTENSION IF NOT EXISTS postgis;`, down = `DROP EXTENSION IF EXISTS postgis;`
- [x] **Task 2 — Миграция `0002` проекции `contracts` (AC1)**
  - [x] `migrations/0002_projection.sql` (goose up/down) — таблица `contracts`, **минимум колонок**, конвенции архитектуры (см. Dev Notes «DDL contracts»): internal `id` (bigint identity), natural `goszakup_contract_id` TEXT **NOT NULL UNIQUE**, `subject_ru`/`subject_kk` (двуязычные), `amount` NUMERIC, `sign_date`/`plan_start`/`plan_end` DATE, `status`, `direction` (CHECK road/water/other), `kato_code`, `source_url`, `is_deleted` BOOL DEFAULT false, `imported_at`/`updated_at` TIMESTAMPTZ DEFAULT now()
  - [x] **БЕЗ flags/geo** и **БЕЗ FK** на ещё не существующие `organizations`/`lots` (см. Dev Notes «Решение по FK» — отложенные колонки добавляются миграцией позже, когда появятся те таблицы)
  - [x] down = `DROP TABLE contracts;` — проверить `goose up && goose down && goose up` на compose `db`
- [x] **Task 3 — sqlc-конфиг + `GetContractByID` + анти-churn (AC2)**
  - [x] Переписать `server/sqlc.yaml` (сейчас `sql: []`) на реальный конфиг v2: `schema: ../migrations` (источник схемы = goose-файлы), `queries:` per-domain каталог (напр. `internal/store/queries/`), движок `postgresql`, gen.go в `internal/store/gen` (package `gen`, **НЕ править**); включить per-query-file вывод (анти-churn) и `emit_*` по необходимости
  - [x] `internal/store/queries/contracts.sql`: запрос `-- name: GetContractByID :one` по `goszakup_contract_id` (публичный id) — `SELECT ... FROM contracts WHERE goszakup_contract_id = $1 AND NOT is_deleted`
  - [x] `make gen-sqlc` (guard на `migrations/*.sql` уже снят 1.1) → сгенерирован `GetContractByID`; `go build ./...` зелёный; **проверить, что вывод per-file** (`contracts.sql.go`, не единый `.gen.go`)
  - [x] Роль-разделение `store/projection` (чтение проекции) vs `store/curation` (кураторские) — заложено 1.1; здесь `contracts` обслуживается **projection**-ролью (тонкая обёртка над `gen` или прямое использование — не плодить лишнего в S-0)
- [x] **Task 4 — seed узнаваемой строки (AC3)**
  - [x] Механизм seed: отдельный `migrations/`-seed (goose `-no-versioning` / отдельный каталог) ИЛИ `make db-seed` с `seed.sql`; НЕ смешивать seed со схемными миграциями (seed — данные, не схема)
  - [x] Вставить контракт с **узнаваемым** `goszakup_contract_id` (напр. `DEMO-0001`), осмысленными `subject_ru`/`subject_kk`, `amount`, `sign_date`, `direction='road'` — чтобы в 1.3/1.7 было видно глазами
- [x] **Task 5 — Верификация и финализация**
  - [x] Сквозной прогон на compose `db`: `make migrate-up` → таблица есть; `make gen-sqlc` → `GetContractByID` собирается (`go build ./...`); `make db-seed` → строка видна (`SELECT * FROM contracts`); `make migrate-down` чисто откатывает
  - [x] `go vet`/`gofmt` зелёные; обновить File List, Change Log, Completion Notes; зафиксировать отложенные FK-колонки как явный TODO следующих миграций

### Review Findings (2026-06-21, code-review)

Состязательное ревью (Blind + Edge + Acceptance). **Acceptance Auditor: нарушений AC нет** (AC1–AC3 pass; scope-дисциплина — минимум колонок, отложенные FK, анти-churn, NULL=нет данных — соблюдена). Находки — надёжность Makefile/CI и покрытие теста:

**Patch:**

- [x] [Review][Patch] MED: `db-seed` без `ON_ERROR_STOP=1` → `psql` через stdin возвращает 0 даже при ошибке INSERT (молчаливый «успех», напр. если таблицы нет) [Makefile]
- [x] [Review][Patch] MED: `ci-server.yml` `cache: false` устарел — в 1.2 появился `go.sum` (pgx) → вернуть Go-кэш [.github/workflows/ci-server.yml]
- [x] [Review][Patch] LOW: seed `ON CONFLICT` обновляет только `subject_ru` → «идемпотентность» неполная (повторный прогон не сводит amount/direction/…) [fixtures/seed/contracts.sql]
- [x] [Review][Patch] LOW: интеграционный тест не читает `Amount` (NUMERIC→pgtype.Numeric — самый рискованный scan-путь) [server/internal/store/contracts_integration_test.go]
- [x] [Review][Patch] LOW: `db-seed` использует `compose exec` (игнорирует `DATABASE_URL`) → задокументировать, что таргет бьёт в compose-db [Makefile]
- [x] [Review][Patch] LOW: `make lint` (`test -z "$(gofmt -l .)"`) не показывает, какие файлы не отформатированы → печатать список [Makefile]

**Defer:**

- [x] [Review][Defer] sqlc парсит `../migrations` целиком; корректность регенерации зависит от распознавания `-- +goose Down` как границы (на v1.31 работает — проверено) → добавить страж «generated==regenerated» в Story 1.4 (уже в плане `ci-registry`) [server/sqlc.yaml] — deferred

**Dismissed:** goose `go run` из корня работает (pkg@ver вне модуля — проверено Edge); `0001` down no-op — намеренно, принято Auditor'ом; `gen-sqlc` guard-путь — теоретически хрупок; `gen-tokens` без guard — не в `gen`-агрегате (1.5); query тянет `is_deleted` — безвреден (генерат); `go.sum`/`doc.go` нет в диффе — полнота диффа, не дефект; `projection`-роль не подключена — в рамках допущенного спекой; Docker-sqlc вместо go run — документированное отклонение.

## Dev Notes

### Контекст истории (T3+T4 walking skeleton)

После 1.1 (пустое собираемое дерево + поднятая PostGIS) эта история кладёт **первую реальную
таблицу** и типобезопасный доступ к ней. Это T3 (миграция `contracts`) + T4 (sqlc +
`GetContractByID`, `go build` зелёный) последовательности S-0; следом 1.3 (T5: `cmd/api` отдаёт
карточку) и 1.7 (T7+T8: сквозной байт Postgres→sqlc→chi→Caddy→React, DoD).
[Source: _bmad-output/planning-artifacts/architecture.md#Инициализация-Walking-Skeleton (строки 356–362)]
[Source: _bmad-output/planning-artifacts/epics.md#Story-1.2 (строки 854–872)]

### Что переиспользовать из 1.1 (НЕ создавать заново)

- `migrations/` уже существует (`.gitkeep`) — класть goose-файлы сюда.
- `server/sqlc.yaml` существует с `sql: []` — **переписать**, не создавать новый.
- `server/internal/store/{gen,projection,curation}/` уже есть (doc.go-стабы) — sqlc выводит в `gen`;
  `projection`/`curation` — роль-разделение (architecture 728, 778–781).
- `Makefile` `gen-sqlc` уже запинен (`SQLC_VERSION v1.31.0`) и **снимает guard**, как только в
  `migrations/*.sql` появятся файлы (см. 1.1 review-fix) — то есть после Task 2 `make gen-sqlc` заработает.
- `deploy/docker-compose.yml` `db` (PostGIS 16, bind `127.0.0.1:${POSTGRES_PORT:-5432}`) — целевая БД
  для прогонов миграций; `deploy/.env.example` — форма `DATABASE_URL`.
[Source: 1-1-монорепо-скелет-и-compose-db.md (File List, done); architecture.md строки 720–729]

### DDL `contracts` — минимум колонок (конвенции архитектуры)

Конвенции (architecture 517–527): **internal bigint identity** + **natural `goszakup_*_id` UNIQUE**;
двуязычные поля суффикс `_kk`/`_ru`; goose `NNNN_*.sql`; публичный id карточки = natural
`goszakup_contract_id` (стабилен между реимпортами). Минимальный набор для читаемой карточки
(полная форма — модель данных стр.43, но flags/geo/FK отложены):

```
id                    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY  -- внутр.
goszakup_contract_id  TEXT NOT NULL UNIQUE                              -- natural / публичный id
subject_ru            TEXT                                             -- двуязычный предмет
subject_kk            TEXT
amount                NUMERIC(18,2)                                    -- ₸; в API (1.3) — СТРОКОЙ
sign_date             DATE
plan_start            DATE
plan_end              DATE
status                TEXT
direction             TEXT CHECK (direction IN ('road','water','other'))
kato_code             TEXT
source_url            TEXT
is_deleted            BOOLEAN NOT NULL DEFAULT FALSE
imported_at           TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at            TIMESTAMPTZ NOT NULL DEFAULT now()
```

- **NULL — честное «нет данных».** Не ставить `0`/`''` дефолты для отсутствующих значений (гардрейл
  честности; в 1.3 wire-формат отдаёт `{value:null, state:"no_data"}`, НИКОГДА «0»). [Source: epics.md строка 888]
- `amount` — `NUMERIC`, не float (деньги). Рендер строкой — задача 1.3, не схемы.

### Решение по FK (важно — задокументировать, не выдумывать)

Полная `contracts` (модель данных) имеет FK `lot_id`→`lots`, `customer_org_id`/`supplier_org_id`→
`organizations`. **Эти таблицы в S-0 отложены** (`organizations`/нормализация блокированы B-3;
`lots` — отдельная проекция). Поэтому в миграции 1.2 FK-колонки **НЕ добавляются** (иначе dangling
FK на несуществующие таблицы). Миграции аддитивны: FK-колонки + констрейнты добавит **поздняя
миграция** (Epic 2 / когда появятся `organizations` и `lots`). Карточке walking-skeleton (1.7) на
синтетике имена заказчика/подрядчика не нужны — достаточно self-полей `contracts`. Зафиксировать
это TODO в коде миграции и в Completion Notes. [Source: architecture.md строка 371 (organizations блокированы B-3); модель данных стр.43]

> **Также вне 1.2:** проекция **`lots`** (нужна Story 0.6) здесь НЕ создаётся — 1.2 это только
> `contracts`. `lots`-миграция — отдельная (в составе 0.6 или 2.x).

### sqlc — анти-churn (architecture + AC2)

- **sqlc 1.31.x** (запинено в Makefile). Схема для sqlc = **те же goose-файлы** (`schema: ../migrations`)
  — единый источник истины, не отдельный DDL. [Source: architecture.md строки 395, 395–398]
- **Анти-churn:** per-domain `.sql` (`contracts.sql`, позже `lots.sql`…) → sqlc по умолчанию emit'ит
  `<file>.sql.go` на файл → запросы НЕ слипаются в один гигантский `.gen.go` (это и проверяет AC2).
  Вывод — `internal/store/gen` (package `gen`, «не править руками»). [Source: epics.md строка 868; architecture.md строка 728]
- **PostGIS overrides не нужны в 1.2** (в `contracts` нет `geometry`; `geom`/overrides появятся с
  `geo_objects` в Epic 3, `internal/store/geo.go` — pgx-raw). [Source: architecture.md строки 302–304, 393–394]
- Движок `postgresql`; `sql_package: pgx/v5` (целевой драйвер pgx v5). [Source: architecture.md строка 300]

### Соблюдение архитектуры (guardrails)

- **Проекционные ⊥ кураторские.** `contracts` — ПРОЕКЦИОННАЯ таблица (derived из снапшота импорта),
  обслуживается `store/projection`-ролью; `store/curation` (ручные правки, иные гранты) её не пишет.
  Гранты на уровне БД — позже (Directus/импортёр, Epic 2/3); в 1.2 — раскладка пакетов. [Source: architecture.md 778–781]
- **Идемпотентность — позже.** Дедуп-ключ `(source, goszakup_contract_id, snapshot_id)` UPSERT
  `ON CONFLICT DO UPDATE` — это живой импорт (Epic 2). В 1.2 достаточно UNIQUE на `goszakup_contract_id`
  (что и делает будущий UPSERT возможным). [Source: architecture.md строки 580–582]
- **Миграции — единственный источник схемы.** Не править схему вне goose; sqlc читает оттуда же. up/down
  обязателен (откат чист). [Source: architecture.md строка 395]
- **Честность.** Отсутствующие поля — NULL (см. выше), не фейковые дефолты.

### Тестирование / проверка готовности

- Целевой data-слой тест (Epic 1+): `go test -tags=integration` через **testcontainers postgis/postgis**
  (architecture 368, 689–691). Для 1.2 минимально: прогон миграций up/down на **compose `db`** вручную
  + `go build ./...` после `gen-sqlc`. Интеграционный тест `GetContractByID` через testcontainers — желателен,
  но если testcontainers-инфра ещё не заведена, достаточно ручного прогона + сборки (отметить честно).
- **DoD 1.2:** `goose up`/`down` чисто; `make gen-sqlc` → `GetContractByID` + `go build` зелёный + per-file
  вывод (анти-churn); seed-строка видна; FK-отсрочка задокументирована.
[Source: architecture.md строки 366–368, 815–817]

### Previous Story Intelligence (1.1, done)

- 1.1 создал собираемый скелет; `go build ./...` (server) и `npm run build` (web) зелёные; `compose up db`
  → `postgis_version()` 3.4 (проверено). Порт БД bind `127.0.0.1:${POSTGRES_PORT:-5432}` (на хосте 5432/5433
  заняты чужими проектами — использовать `POSTGRES_PORT` при конфликте).
- Code-review 1.1 (8 fix): `make gen-sqlc` теперь **guard'ится на `migrations/*.sql`** — как только Task 2
  создаст `0001/0002`, `gen-sqlc` перестанет пропускаться и реально запустит sqlc. Учесть: sqlc.yaml должен
  быть валиден к этому моменту (сейчас `sql: []`).
- `web/package-lock.json` нужно коммитить (вне этой истории).
[Source: 1-1-…compose-db.md (Dev Agent Record, Review Findings)]

### Project Structure Notes

- Новые файлы кладутся в существующие каталоги 1.1: `migrations/0001_extensions.sql`,
  `migrations/0002_projection.sql`, `internal/store/queries/contracts.sql`, сгенерированное в
  `internal/store/gen/`. Конфликтов нет — наполняем скелет.
- `Makefile` дополняется таргетами `migrate-*` и `db-seed` (рядом с `db-up`); `GOOSE_VERSION` запинить.
- Seed — отдельно от схемных миграций (данные ≠ схема), чтобы `goose down` не тащил демо-данные в логику отката.

### Web research

Не требуется: версии запинены (sqlc 1.31.x; goose — запинить конкретную v3.x в Makefile на момент
реализации). goose-формат миграций (`-- +goose Up/Down`, `StatementBegin/End` для функций) —
из официальной доки goose; не выдумывать синтаксис. PostGIS extension — `CREATE EXTENSION postgis`.

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-1.2 (строки 854–872); Epic 1 контекст 825–973]
- [Source: _bmad-output/planning-artifacts/architecture.md#Walking-Skeleton (356–362); миграции/sqlc (393–399); конвенции (517–527); store-роли (720–729, 778–781); идемпотентность (580–582)]
- [Source: docs/AshyqQala_MVP_data_model_and_flags_v1.md (contracts ⭐ стр.43; organizations стр.34; lots стр.42)]
- [Source: _bmad-output/implementation-artifacts/1-1-монорепо-скелет-и-compose-db.md (done; скелет, sqlc.yaml, store-пакеты, gen-sqlc guard)]
- [Source: CLAUDE.md — планируемый стек (pgx+sqlc, Postgres 16+PostGIS); гардрейл честности (NULL = нет данных)]

## Dev Agent Record

### Agent Model Used

claude-opus-4-8[1m] (Claude Opus 4.8, 1M context) — dev-story workflow.

### Debug Log References

- **миграции (вживую на compose PostGIS, порт 55432):** `goose up` → `0001`+`0002` → version 2; `goose down`×2 → чисто; `status` → оба Pending; повторный `up` → version 2 (реаппликабельно).
- **sqlc:** `make gen-sqlc` через Docker `sqlc/sqlc:1.31.0` → `store/gen/{contracts.sql.go, db.go, models.go, querier.go}` (per-file = анти-churn). `go build ./...` / `go vet` / `gofmt` зелёные.
- **seed + интеграция:** `make db-seed` → `INSERT 0 1`; `go test -tags=integration ./internal/store/...` (с `DATABASE_URL`) → **PASS** (`GetContractByID('DEMO-0001')` вернул seed: `subject_ru`, `direction=road`).
- **`go test ./...` без тега** → проходит без БД (интеграционный исключён build-тегом → CI не требует БД). `go mod tidy` стабилен.

### Completion Notes List

- **AC1 ✅ (проверено вживую):** goose `0001_extensions` + `0002_projection` (contracts) применяются и **откатываются чисто**, цикл up/down/up реаппликабелен на реальной PostGIS.
  - **Найдено и исправлено:** `0001` down `DROP EXTENSION postgis` падал («other objects depend on it») — образ `postgis/postgis` сам провижит `postgis`+зависимые (`topology/tiger/fuzzystrmatch`). Решение: `0001` up — идемпотентный `CREATE IF NOT EXISTS` (переносимость); **down — намеренный no-op** (фундаментальное расширение, провижится образом; не сносим не нами созданное). Обратимость схемы — на уровне таблиц (`0002`).
- **AC2 ✅ (проверено):** sqlc генерит **per-file** (`contracts.sql.go`, не один `.gen.go` — анти-churn); `GetContractByID` сгенерирован; `go build` зелёный.
  - **Найдено и исправлено:** Makefile `gen-sqlc` использовал `go run sqlc@ver` — не работает (у sqlc replace-директивы в go.mod). Переведено на **официальный Docker-образ `sqlc/sqlc:1.31.0`** (под текущим uid). `SQLC_VERSION` → docker-тег `1.31.0`.
  - Добавлен **pgx v5.10.0** (первая реальная зависимость `server`); создан `server/go.sum`.
- **AC3 ✅ (проверено):** seed `DEMO-0001` (узнаваемая строка) + интеграционный тест подтверждают сквозной байт Postgres→sqlc(pgx)→Go.
- **Скоуп:** только `contracts`; **FK-колонки отложены** (organizations/lots — B-3; TODO в `0002`); проекция `lots` (для 0.6) здесь НЕ создаётся.
- **CI-замечание:** теперь есть `server/go.sum` → в `ci-server.yml` можно вернуть кэш Go (в 1.1 review стоял `cache: false` из-за отсутствия go.sum). Оставил как есть (оптимизация, не блок).
- **Не закоммичено** (dev-story по умолчанию не коммитит): рабочее дерево, ветка `story/0-1-stage0-access`. Сгенерированный `store/gen/*` — коммитится (build зависит).

### File List

**Новые:**
- `migrations/0001_extensions.sql` — goose: PostGIS (up идемпотентно; down no-op, см. Completion Notes)
- `migrations/0002_projection.sql` — goose: таблица `contracts` (минимум колонок; up/down)
- `server/internal/store/queries/contracts.sql` — sqlc-запрос `GetContractByID`
- `server/internal/store/gen/{contracts.sql.go, db.go, models.go, querier.go}` — **сгенерировано sqlc** (DO NOT EDIT)
- `server/internal/store/contracts_integration_test.go` — интеграционный тест (`//go:build integration`)
- `fixtures/seed/contracts.sql` — синтетический seed (`DEMO-0001`, идемпотентен)
- `server/go.sum` — **новый** (первые зависимости)

**Изменённые:**
- `server/sqlc.yaml` — `sql: []` → реальный конфиг (schema=`../migrations`, queries, out=`store/gen`, pgx/v5, анти-churn)
- `server/go.mod` — + `github.com/jackc/pgx/v5 v5.10.0` (+ indirect); `go 1.25` → `1.25.0`
- `Makefile` — `SQLC_VERSION v1.31.0`→`1.31.0` (docker-тег); `gen-sqlc` → Docker-образ sqlc; +`GOOSE_VERSION`, БД-переменные/`DATABASE_URL`, таргеты `migrate-up/down/status`, `db-seed`
- `_bmad-output/implementation-artifacts/1-2-…md` — чекбоксы, Dev Agent Record, File List, Change Log, Status
- `_bmad-output/implementation-artifacts/sprint-status.yaml` — `1-2` backlog→ready-for-dev→in-progress→review

## Change Log

| Дата | Изменение |
|---|---|
| 2026-06-21 | **Правка из Story 1.3 (Task 0, wire-конвенция):** колонка `amount NUMERIC(18,2)` → `amount_tng BIGINT` (целые тенге). Затронуты файлы 1.2: `0002_projection.sql`, `queries/contracts.sql`, `fixtures/seed/contracts.sql`, `contracts_integration_test.go`, перегенерён `store/gen/*`. Сделано в рамках 1.3 (1.2 не была закоммичена → правка `0002` напрямую вместо аддитивной `0003`). |
| 2026-06-21 | code-review (Blind+Edge+Acceptance): нарушений AC нет. 6 patch (надёжность/покрытие) **исправлены и проверены вживую**: `db-seed` `ON_ERROR_STOP=1` (провал без таблицы подтверждён), `ci-server` Go-кэш возвращён (есть go.sum), seed `ON CONFLICT` обновляет все колонки, интеграционный тест проверяет `amount` (PASS), `db-seed` помечен как compose-only, `make lint` печатает неотформатированные. 1 defer (sqlc/goose-Down regen-страж → Story 1.4, в `deferred-work.md`). Статус → done. |
| 2026-06-21 | dev-story: реализованы T3+T4. goose `0001_extensions`/`0002_projection` (contracts, минимум колонок), sqlc-конфиг + `GetContractByID` (per-file, анти-churn), seed `DEMO-0001`, интеграционный тест. **AC1/AC2/AC3 проверены вживую** на compose PostGIS (up/down/up чисто; gen per-file; seed+тест PASS). Исправлено по ходу: `0001` down → no-op (образ провижит postgis+зависимые); `gen-sqlc` → Docker sqlc (go run несовместим). Добавлен pgx v5.10.0 + go.sum. Скоуп: только contracts; FK отложены (TODO). Статус → review. |
| 2026-06-21 | Создан context engine для Story 1.2 (T3+T4: миграция `contracts` + sqlc). Анализ: epics 1.2, architecture (walking-skeleton, миграции/sqlc, конвенции, store-роли, идемпотентность), модель данных (`contracts`⭐), переиспользование скелета 1.1 (migrations/, sqlc.yaml, store/gen, gen-sqlc guard). 3 AC (goose up/down / sqlc анти-churn / seed), 5 задач, минимальный DDL `contracts` + **зафиксировано решение по отложенным FK** (organizations/lots — B-3, FK добавит поздняя миграция). Статус → ready-for-dev. |
