---
baseline_commit: 796bd548f2f033d33b2de3c6870b7e4a100eeca6
---

# Story 1.1: Монорепо-скелет и compose db

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a **разработчик платформы AshyqQala.kz**,
I want **собираемый монорепо-скелет (целевое дерево пустыми пакетами) и поднятую Postgres 16 + PostGIS через Docker Compose**,
so that **все последующие истории строятся на работающем фундаменте, а структура задаётся один раз и анти-churn**.

> **Тип истории:** первая история Epic 1 (каркас). Реализует **T1+T2** walking-skeleton-последовательности
> S-0 + CI-гейты (S-0b). **Идёт на синтетике, НЕ ждёт `GOSZAKUP_TOKEN`** (явно: Epic 1 не зависит от
> живого ows_v2 и гейта №0). Разблокирует и весь Epic 1, и downstream трека «Парсер-мост» (Story 0.6
> сидит поверх этого скелета). **Скоуп строго ограничен скелетом** — наполнение пакетов кодом идёт в
> 1.2 (миграция `contracts`+sqlc), 1.3 (`cmd/api`), 1.4 (registry/render), 1.5 (токены), 1.6 (i18n),
> 1.7 (сквозная карточка, DoD=T8).

## Acceptance Criteria

**AC1 — Скелет монорепо собирается**
**Given** пустой репозиторий (сейчас в нём только `stage0-audit/`, `docs/`, планировочные артефакты)
**When** создан монорепо-скелет (`server/` с `go mod init ashyqqala/server`, `web/`, `deploy/`, `migrations/`, `registry/`, `fixtures/`, корневой `Makefile`) и целевое дерево заполнено **пустыми собираемыми пакетами** (`.gitkeep` / минимальные `package`-файлы)
**Then** `go build ./...` (в `server/`) и `npm run build` (в `web/`) **зелёные** на скелете.

**AC2 — Postgres + PostGIS поднимается через Compose**
**Given** `deploy/docker-compose.yml`
**When** `docker compose up db`
**Then** контейнер образа `postgis/postgis` (Postgres 16) поднимается и `postgis_version()` отвечает.

**AC3 — CI с path-фильтрами блокирует merge на красном**
**Given** CI (GitHub Actions) с path-фильтрами (`server/**`, `web/**`, `fixtures/**`)
**When** открыт PR
**Then** гейты `vet`/`gofmt` (server) и `tsc`/`eslint`/`prettier` (web) проходят; **красный = блок merge**.

## Tasks / Subtasks

- [x] **Task 1 — Go-модуль `server/` + целевое дерево пакетов (AC1)**
  - [x] `server/`: `go mod init ashyqqala/server` (Go **1.25**). `go.mod` живёт **в `server/`**, НЕ в корне репо
  - [x] Создать `server/internal/` пакеты целевого дерева **пустыми, но собираемыми** (объявление `package <name>` или `.gitkeep`): `config`, `goszakup`, `ingest/decode`, `normalize`, `flags`, `median`, `benchmark`, `geo`, `store/{gen,projection,curation}` (+ `store/geo.go` заглушкой), `httpapi`, `render`, `og`, `bot`, `outbox`, `metrics`, `clock`, `lang`
  - [x] `server/cmd/{api,importer,stage0}/main.go`: `cmd/api` — минимальный `func main(){}` (хендлеры — Story 1.3), `cmd/importer` и `cmd/stage0` — **пустые `func main(){}`** (наполняются позже; B-1+)
  - [x] `server/tools/{scrape,osm,tiles}/` → **`.gitkeep` без `.go`** (преждевременно до B-1/B-2/B-3; `tools/scrape` наполняется в Story 0.6, `osm`/`tiles` — к S-0.5)
  - [x] `server/sqlc.yaml` — каркас (пустой/минимальный; реальные запросы в 1.2). `go build ./...` зелёный
- [x] **Task 2 — Корневой `Makefile` + registry/fixtures/migrations/docs скелет (AC1)**
  - [x] Корневой `Makefile` с целями: `gen-sqlc`, `gen-web` (openapi-typescript), `gen-tokens`, `test`, `lint`; **`gen` = сумма gen-целей**. Версии генераторов **ЗАПИНЕНЫ** (sqlc `1.31.x`, openapi-typescript, golangci-lint — конкретные версии в Makefile/CI). В S-0 `gen` реально гоняет только `gen-sqlc` + `gen-web` (registry спит, токены — 1.5)
  - [x] `registry/{values,rules,methodology}/` — `.gitkeep` (в S-0 **спит**: флагов/значений нет до 1.4)
  - [x] `fixtures/` — `.gitkeep` (+ задел под `fixtures.go` с `//go:embed`; синтетика наполняется позже)
  - [x] `migrations/` — `.gitkeep` (goose-миграции `0001_extensions`/`0002` — Story 1.2)
  - [x] `docs/api-contracts/` — `.gitkeep` (OpenAPI-арбитр — Story 1.3; снапшоты ows_v2 — B-1)
  - [x] Корневые `.editorconfig`; обновить `.gitignore` (добавить `node_modules/`, `web/dist/`, `**/dist/`)
- [x] **Task 3 — `web/` скелет Vite + React + TS (AC1)**
  - [x] `web/`: `package.json`, `tsconfig.json` (alias `@fixtures → ../fixtures`), `vite.config.ts`, `index.html`, `src/{main.tsx,App.tsx,router.tsx}`; скелет `src/shared/{ui,api,i18n,tokens,map,state}` и `src/features/{map,contract,contractor,district,search,flag,share}` (пустые index-файлы)
  - [x] Стек по addendum: **React + Vite + TS** (MapLibre/Router/Query подключаются в 1.7; здесь только сборка). `npm run build` зелёный на скелете
  - [x] ⚠ Задел на будущее: при первом `new Map()` (MapLibre, 1.8) — `useRef`-guard под React StrictMode (двойной mount). В 1.1 карты нет — только пометка
- [x] **Task 4 — `deploy/docker-compose.yml` + `compose up db` (AC2)**
  - [x] `deploy/docker-compose.yml` (ГОРЯЧИЙ): `postgres` (образ `postgis/postgis`, **Postgres 16 + PostGIS**), задел под `api` и `caddy(+web+pmtiles)`
  - [x] `deploy/docker-compose.cold.yml` (profiles): `importer` · `directus` · `bot(polling)` · `nominatim(geocode)` — каркас профилей (не запускаются в S-0)
  - [x] `deploy/{Caddyfile,.env.example}`, задел `deploy/directus/`
  - [x] Проверка AC2: `docker compose up db` → контейнер поднят, `SELECT postgis_version();` отвечает
- [x] **Task 5 — CI path-фильтры (GitHub Actions) (AC3)**
  - [x] `.github/workflows/ci-server.yml` (path `server/**` ИЛИ `fixtures/**`): `go vet`, `gofmt -l` (diff = fail), `golangci-lint`, `go test ./...` (каркас; golden/property/integration/go-list-страж добавляются последующими историями)
  - [x] `.github/workflows/ci-web.yml` (path `web/**` ИЛИ `fixtures/**`): `tsc --noEmit`, `eslint`, `prettier --check` (vitest/playwright-инфра — S-0b/1.7)
  - [x] Задел `ci-registry.yml` и `nightly-ows.yml` (пустые/минимальные; наполняются 1.4 и B-1)
  - [x] Гейты обязательны для merge: **красный = блок** (branch protection — заметка владельцу, если настраивается в GitHub UI)
- [x] **Task 6 — Финализация и сквозная проверка**
  - [x] `go build ./...` (server) + `npm run build` (web) + `docker compose up db` (postgis_version) + локальный прогон lint/vet/tsc — все зелёные
  - [x] Обновить File List, Change Log, Completion Notes; зафиксировать, что наполнение пакетов — за последующими историями

### Review Findings (2026-06-21, code-review)

Состязательное ревью (Blind Hunter + Edge Case Hunter + Acceptance Auditor). Acceptance Auditor: **нарушений AC нет** (AC1–AC3 удовлетворены и проверены). Находки — про CI-надёжность и гигиену:

**Patch (рекомендованы к исправлению):**

- [x] [Review][Patch] HIGH: `golangci-lint-action@v6` несовместим с golangci-lint v2.5.0 → bump до `@v7` [.github/workflows/ci-server.yml] — иначе lint-гейт падает всегда. Проверено: бинарь golangci-lint v2.5.0 даёт `0 issues` на скелете → проблема только в версии action, не в коде/конфиге.
- [x] [Review][Patch] HIGH: setup-go `cache-dependency-path: server/go.sum` указывает на несуществующий файл (зависимостей нет) → `cache: false` для server-джоба [.github/workflows/ci-server.yml]
- [x] [Review][Patch] MED: `make gen`/`gen-sqlc`/`gen-web` падают в S-0 (нет `docs/api-contracts/openapi.yaml`; `sqlc.yaml` `sql: []`) → сделать таргеты S-0-safe (гард на отсутствие входов) [Makefile]
- [x] [Review][Patch] LOW: `tsconfig.node.json` осиротевший — `vite.config.ts` не типизируется ничем → добавить проверку в `typecheck` [web/package.json]
- [x] [Review][Patch] LOW: eslint регистрирует только browser-globals; node-конфиги (`vite.config.ts`, `eslint.config.js`) без node-globals → добавить node-блок [web/eslint.config.js]
- [x] [Review][Patch] LOW: публикуемый порт БД биндится на `0.0.0.0` с дефолт-паролем → bind `127.0.0.1` по умолчанию [deploy/docker-compose.yml]
- [x] [Review][Patch] LOW: `cold.yml` `bot` ссылается на бинарь `/bot`, но `cmd/bot` нет (это Epic 7) → смягчить/закомментировать [deploy/docker-compose.cold.yml]
- [x] [Review][Patch] LOW: `ci-registry.yml` триггерится на `server/**`/`web/**` (ложно-зелёный «registry guards» на каждом PR) → сузить до `registry/**` в S-0 [.github/workflows/ci-registry.yml]

**Defer (предсуществующее, вне скоупа 1.1):**

- [x] [Review][Defer] Скомпилированный бинарь `stage0-audit/stage0-audit` отслеживается git [stage0-audit/stage0-audit] — deferred, pre-existing; вычистить в рамках Story 0.2 (`git rm --cached` + `.gitignore`)

**Dismissed (шум / ложные / по дизайну):** golangci `.golangci.yml` не нужен (дефолты → 0 issues, проверено); Go 1.25 доступен (CI `go-version-file`); `@fixtures` пуст — латентно до прихода фикстур; `store/geo.go` как стаб — разрешено спекой; `docs/ops` — скоуп Story 0.1; compose `api`/`importer` Dockerfile отсутствует — по дизайну за профилем `app` (`up db` изолирован); `nightly-ows if:false` — намеренный S-0-каркас. **Напоминание-действие:** `web/package-lock.json` существует и НЕ игнорируется — убедиться, что он коммитится (нужен `npm ci` в CI).

## Dev Notes

### Контекст истории (фундамент всего)

Epic 1 закладывает сквозной «позвонок» на **синтетике**, параллельно гейту №0 и треку «Парсер-мост»,
и **не ждёт токен**. Story 1.1 — это **T1+T2** инициализации: пустое **собираемое** дерево + поднятая
БД + CI-гейты. Дерево создаётся пустым и собираемым один раз, затем последующие истории наполняют
конкретные пакеты. Это снимает структурный риск (анти-churn): один раз правильная раскладка → меньше
переделок.
[Source: _bmad-output/planning-artifacts/architecture.md#Инициализация-Walking-Skeleton-S-0 (строки 354–372)]
[Source: _bmad-output/planning-artifacts/epics.md#Epic-1 (строки 825–844)]

### Целевое дерево — следовать ТОЧНО (анти-churn, НЕ изобретать раскладку)

Раскладка задана архитектурой — воспроизвести её, а не придумывать свою. Сжатая карта (полная —
architecture.md строки 685–755):

```
<repo root>
├── Makefile                 # gen-sqlc · gen-web · gen-tokens · test · lint (версии ЗАПИНЕНЫ; gen = сумма)
├── .github/workflows/       # ci-server.yml · ci-web.yml · ci-registry.yml · nightly-ows.yml
├── registry/{values,rules,methodology}/   # источник имён/значений (рантайм-чтение); в S-0 СПИТ
├── docs/api-contracts/{openapi.yaml, ows_v2/}   # OpenAPI-арбитр (1.3); снапшоты (B-1)
├── migrations/              # goose SQL (0001_extensions, 0002_projection … — Story 1.2)
├── fixtures/                # ОБЩИЕ; fixtures.go (//go:embed); синтетика
├── server/                  # go.mod ЗДЕСЬ — модуль ashyqqala/server
│   ├── go.mod · go.sum · sqlc.yaml
│   ├── cmd/{api,importer,stage0}/main.go   # в S-0: importer/stage0 — пустой func main(){}
│   ├── internal/{config,goszakup,ingest/decode,normalize,flags,median,benchmark,geo,
│   │             store/{gen,projection,curation,geo.go},httpapi,render,og,bot,outbox,
│   │             metrics,clock,lang}/
│   └── tools/{scrape,osm,tiles}/   # ВНЕ боевого бинаря; в S-0 .gitkeep без .go
├── web/                     # Vite+React+TS (alias @fixtures → ../fixtures)
└── deploy/                  # docker-compose.yml (HOT: postgres,api,caddy) · .cold.yml · Caddyfile · .env.example · directus/
```

[Source: architecture.md строки 685–755 (полное дерево); 757–767 (что реально наполняем в S-0)]

### Стек и запиненные версии (addendum + architecture)

- **Backend:** Go **1.25** + **chi v5**; **pgx v5** + **sqlc 1.31.x**; `slog` + `/metrics`.
- **БД:** **PostgreSQL 16 + PostGIS** (образ `postgis/postgis`), геометрия `geom`.
- **Frontend:** **React + Vite + TS** (MapLibre GL — позже, 1.8).
- **Инфра:** **Caddy**, **Docker Compose** на одном VPS; CI — **GitHub Actions**.
- **Миграции:** **goose** (SQL — источник схемы, sqlc читает оттуда).
- **Codegen (запинен):** `sqlc 1.31.x`, `openapi-typescript` (gen-web), golangci-lint — версии в `Makefile`/CI.
- **Админка:** Directus поверх Postgres (cold-профиль; не в S-0).

[Source: _bmad-output/planning-artifacts/prds/prd-AshyqQala.kz-2026-06-17/addendum.md (строки 7–11)]
[Source: architecture.md строка 300 (chi v5; pgx v5; sqlc 1.31.x; slog)]

### Что НЕ входит в Story 1.1 (границы скоупа — строго)

- **НЕТ** таблицы `contracts`/миграций/seed (Story 1.2 = T3+T4) — `migrations/` лишь `.gitkeep`.
- **НЕТ** `cmd/api`-хендлеров/`/healthz`/`/metrics`/OpenAPI (Story 1.3 = T5) — `cmd/api/main` пустой.
- **НЕТ** registry-значений/`render`/honest-states (Story 1.4) — `registry/` спит.
- **НЕТ** дизайн-токенов (1.5), i18n-обёрток (1.6), сквозной карточки/Router/Query/Caddy-proxy (1.7 = T7+T8, DoD).
- **НЕТ** `internal/goszakup`-клиента, `decode`, `outbox`, `bot`, нормализации, `tools/*`-кода (преждевременно
  до B-1/B-2/B-3 — `.gitkeep` без `.go`).
- **НЕ удалять и НЕ переносить** существующий `stage0-audit/` (отдельный модуль `ashyqqala/stage0-audit`).
  Его вливание в `cmd/stage0` поверх `internal/goszakup` — **поздняя** миграция (после B-1), НЕ эта история.

[Source: architecture.md строки 757–767 (S-0 наполняет ТОЛЬКО перечисленное); 370–372 (НЕ входит в S-0)]

### Соблюдение архитектуры (guardrails)

- **Анти-churn структурой.** Раскладка дерева и per-domain sqlc (`store/{gen,projection,curation}`,
  раздельные `.sql`) задаются сразу правильно, чтобы codegen НЕ складывал всё в один `.gen.go` (это
  проверяется в 1.2; здесь — заложить раздельные пакеты). [Source: epics.md строки 866–868]
- **Изоляция токена (структурой на будущее).** `cmd/importer` будет держать `GOSZAKUP_TOKEN` и писать;
  `cmd/api` токена **не видит**. В 1.1 это закладывается раскладкой (отдельные `cmd/*`), не кодом.
  [Source: architecture.md строки 414, 771–773]
- **Исполняемые границы (CI go-list-тест) — позже.** Границы вида «`goszakup` импортируется только из
  `ingest/decode`», «прод не импортирует `tools/scrape`» (это AC2 Story 0.6) реализуются go-list-тестом,
  когда появятся пакеты с кодом. В 1.1 — только каркас `ci-server.yml`, куда тест добавится. [Source: architecture.md строки 769–781]
- **Проекционные ⊥ кураторские.** `store/projection` (derived из снапшота) и `store/curation` (ручные
  правки) — раздельные пакеты с разными грантами. Заложить как отдельные каталоги уже в скелете.
  [Source: architecture.md строки 778–781]
- **Нейтральность/честность — структурой.** Вся проза через `render` (одна точка); честные состояния —
  закрытый enum. В 1.1 не активны (нет флагов), но пакеты `render`/`registry` присутствуют в дереве.
  [Source: architecture.md строки 590, 769–777]

### Существующее состояние репозитория

- Корень репо **чист от монорепо**: есть только `stage0-audit/`, `docs/`, `_bmad*`, `CLAUDE.md`,
  `LICENSE`, `README.md`, `.gitignore`. Конфликтов с целевым деревом нет — создаём с нуля.
- **`.gitignore` уже покрывает:** бинарники Go, `*.test`, `*.out`, `.env`, `.idea/`, `.vscode/`,
  `_bmad/`, `.claude/`, `.agents/`. **Добавить в 1.1:** `node_modules/`, `web/dist/`, `**/dist/`.
- `stage0-audit/` — отдельный Go-модуль (`ashyqqala/stage0-audit`, только stdlib). Останется как есть;
  новый модуль — `ashyqqala/server`. Два модуля сосуществуют (без go.work в S-0; `go.work` уже в `.gitignore`).

### Тестирование / проверка готовности

- **DoD Story 1.1 — операционно-сборочный:** (а) `go build ./...` в `server/` зелёный на пустом дереве;
  (б) `npm run build` в `web/` зелёный; (в) `docker compose up db` → `postgis_version()` отвечает;
  (г) CI-гейты `vet`/`gofmt`/`tsc`/`eslint`/`prettier` зелёные на PR, красный блокирует merge.
- Юнит-тестов логики здесь нет (нет логики — скелет). Полноценный `go test` (+golden +property
  +integration testcontainers) и Playwright smoke приходят со Story 1.2/1.3/1.7.
- Сквозной DoD walking skeleton (байт Postgres→sqlc→chi→Caddy→React, T8) — это **Story 1.7**, НЕ 1.1.

[Source: architecture.md строки 354–365 (S-0/S-0b/DoD=T8); 815–817 (тесты)]

### Project Structure Notes

- 1.1 создаёт всю целевую раскладку один раз → конфликтов с последующими историями нет (они наполняют
  существующие пустые пакеты). Это устраняет риск переделок структуры.
- Единственная «странность»: дерево заведомо шире, чем нужно в S-0 (многие пакеты пустые). Это намеренно
  (architecture.md: «дерево создаётся пустым и собираемым»), не over-engineering.
- `tools/*` пустые (`.gitkeep` без `.go`) — это предусловие, которое **разблокирует Story 0.6** (туда
  ляжет `tools/scrape`).

### Web research

Не требуется: версии стека уже зафиксированы (addendum + architecture). Конкретные patch-версии
генераторов dev фиксирует в `Makefile`/CI на момент реализации (запиненные мажор/минор: Go 1.25, chi v5,
pgx v5, sqlc 1.31.x, Postgres 16). Не брать версии «из памяти» там, где addendum/architecture молчат —
выбрать актуальную стабильную и записать явно.

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-1.1 (строки 834–852); Epic 1 контекст 825–973]
- [Source: _bmad-output/planning-artifacts/architecture.md#Целевое-дерево (строки 685–755)]
- [Source: architecture.md#S-0-минимальный-каркас (757–767); #Инициализация-Walking-Skeleton (354–372)]
- [Source: architecture.md#Architectural-Boundaries (769–783); изоляция токена (414, 771–773)]
- [Source: _bmad-output/planning-artifacts/prds/prd-AshyqQala.kz-2026-06-17/addendum.md (стек, строки 7–11)]
- [Source: .gitignore (текущее покрытие); CLAUDE.md (планируемый стек, два guardrail)]

## Dev Agent Record

### Agent Model Used

claude-opus-4-8[1m] (Claude Opus 4.8, 1M context) — dev-story workflow.

### Debug Log References

- **server:** `go build ./...` → OK; `go vet ./...` → OK; `gofmt -l .` → пусто (Go 1.26 тулчейн, модуль `go 1.25`).
- **web:** `npm install` → 182 пакета; `tsc --noEmit` / `eslint .` / `prettier --check .` / `vite build` (26 модулей, dist 143 КБ) → все OK. `.prettierignore` исключает `dist`/`node_modules`.
- **deploy:** `docker compose config` (hot и hot+cold) → OK. Хостовые порты 5432/5433 заняты чужими проектами (`accessportal-db-1`, `vaps-db-1`) → публикуемый порт сделан настраиваемым (`POSTGRES_PORT`, дефолт 5432). `POSTGRES_PORT=55432 up -d db` → контейнер `healthy`; `SELECT postgis_version();` → `3.4 USE_GEOS=1 USE_PROJ=1 USE_STATS=1`; затем `down -v` (контейнер+том удалены, чужие контейнеры не тронуты).
- **CI:** все 4 workflow-YAML (`ci-server`/`ci-web`/`ci-registry`/`nightly-ows`) парсятся; `package-lock.json` создан (нужен `npm ci`).

### Completion Notes List

- **AC1 ✅ (проверено):** `go build ./...` (server) и `npm run build` (web) зелёные на пустом собираемом дереве.
- **AC2 ✅ (проверено реально):** `docker compose up db` → PostGIS поднят, `postgis_version()` отвечает. ⚠ На этом хосте 5432/5433 заняты чужими проектами — публикуемый порт вынесен в `POSTGRES_PORT` (дефолт 5432; тест шёл на 55432).
- **AC3 — частично проверяемо локально:** созданы 4 path-фильтрованных workflow; **базовые гейты `vet`/`gofmt` (server) и `tsc`/`eslint`/`prettier` (web) проверены зелёными локально**. `golangci-lint` зашит в `ci-server.yml`, но **локально не запускался** — впервые отработает на PR (для пустого дерева дефолтные линтеры должны пройти). **«Красный = блок merge» требует настройки branch protection в GitHub UI — действие владельца** (нельзя задать файлом в репо).
- **Дисциплина скоупа соблюдена:** дерево пустое, но собираемое; наполнение пакетов — за 1.2–1.7. `stage0-audit/` не тронут (отдельный модуль `ashyqqala/stage0-audit`). `tools/*` пустые (`.gitkeep` без `.go`) — это разблокирует Story 0.6.
- **Два Go-модуля сосуществуют** (`ashyqqala/server`, `ashyqqala/stage0-audit`); `go.work` не создавался (в `.gitignore`, согласно S-0). `go build ./...` запускается внутри `server/`.
- **Не закоммичено** (по умолчанию dev-story не коммитит): изменения в рабочем дереве на ветке `story/0-1-stage0-access`. `web/node_modules/` игнорируется; `web/package-lock.json` — коммитится.

### File List

**Корень / инфра (новые):**
- `Makefile` — цели gen-sqlc/gen-web/gen-tokens/test/lint/build/db-up; `gen` = сумма; версии запинены
- `.editorconfig`
- `.github/workflows/{ci-server.yml, ci-web.yml, ci-registry.yml, nightly-ows.yml}` — path-фильтрованный CI
- `registry/{values,rules,methodology}/.gitkeep`, `fixtures/.gitkeep`, `migrations/.gitkeep`
- `docs/api-contracts/.gitkeep`, `docs/api-contracts/ows_v2/.gitkeep`

**`server/` (новые) — модуль `ashyqqala/server`, Go 1.25:**
- `server/go.mod`, `server/sqlc.yaml`
- `server/cmd/{api,importer,stage0}/main.go` (api — inert main; importer/stage0 — пустые)
- `server/internal/{config,goszakup,ingest/decode,normalize,flags,median,benchmark,geo,httpapi,render,og,bot,outbox,metrics,clock,lang}/doc.go`
- `server/internal/store/{gen,projection,curation}/doc.go` + `server/internal/store/geo.go`
- `server/tools/{scrape,osm,tiles}/.gitkeep`

**`web/` (новые) — Vite + React + TS:**
- `web/{package.json, package-lock.json, tsconfig.json, tsconfig.node.json, vite.config.ts, index.html}`
- `web/{eslint.config.js, .prettierrc.json, .prettierignore}`
- `web/src/{main.tsx, App.tsx, router.tsx}`
- `web/src/shared/{ui,api,i18n,tokens,map,state}/.gitkeep`
- `web/src/features/{map,contract,contractor,district,search,flag,share}/.gitkeep`

**`deploy/` (новые):**
- `deploy/{docker-compose.yml, docker-compose.cold.yml, Caddyfile, .env.example}`, `deploy/directus/.gitkeep`

**Изменённые:**
- `.gitignore` — добавлены `node_modules/`, `**/node_modules/`, `web/dist/`, `**/dist/`, `*.tsbuildinfo`
- `_bmad-output/implementation-artifacts/1-1-монорепо-скелет-и-compose-db.md` — frontmatter `baseline_commit`, чекбоксы, Dev Agent Record, File List, Change Log, Status
- `_bmad-output/implementation-artifacts/sprint-status.yaml` — `epic-1` → in-progress; `1-1` backlog → ready-for-dev → in-progress → review

## Change Log

| Дата | Изменение |
|---|---|
| 2026-06-21 | Создан context engine для Story 1.1 (фундамент Epic 1). Анализ: epics 1.1 + кросс-стори 1.2–1.7, architecture (целевое дерево 685–755, S-0 T1→T8 354–372, границы 769–781), addendum (стек/версии), фактическое состояние репо (монорепо нет, есть только stage0-audit) и `.gitignore`. 3 AC (скелет-build / compose db PostGIS / CI path-фильтры), 6 задач, точное целевое дерево + запиненные версии + строгие границы скоупа (что НЕ входит). Статус → ready-for-dev. |
| 2026-06-21 | code-review (Blind+Edge+Acceptance): нарушений AC нет. 8 patch-находок (CI-надёжность/гигиена) **исправлены и проверены**: golangci-lint-action `@v6→@v7` (v2.5.0 даёт 0 issues), setup-go `cache: false` (нет go.sum), `make gen` S-0-safe (гарды), `vite.config.ts` типизируется (+`@types/node`), eslint node-globals, БД-порт bind `127.0.0.1`, `cold.yml` bot без висячего `/bot`, `ci-registry` path сужен. 1 defer (tracked-бинарь `stage0-audit/stage0-audit` → `deferred-work.md`, Story 0.2). Статус → done. |
| 2026-06-21 | dev-story: реализован монорепо-скелет. `server/` (модуль `ashyqqala/server`, целевое дерево пустыми собираемыми пакетами, `cmd/{api,importer,stage0}`, `tools/*` .gitkeep), корневой `Makefile`, `web/` (Vite+React+TS), `deploy/` (compose hot+cold, Caddyfile, .env.example), `.github/workflows/` (4 path-фильтрованных CI), registry/fixtures/migrations/docs скелет, `.editorconfig`, `.gitignore`+Node. **AC1 ✅** (go build + npm build зелёные), **AC2 ✅** (compose up db → postgis_version 3.4; порт вынесен в `POSTGRES_PORT` — на хосте 5432/5433 заняты), **AC3** базовые гейты vet/gofmt/tsc/eslint/prettier зелёные локально (golangci-lint — впервые на CI; branch protection — действие владельца). Статус → review. |
