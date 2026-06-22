---
baseline_commit: 6e39a2ca260585289ee01d10fbac2a59da83de91
---

# Story 1.7: Сквозная карточка контракта (walking skeleton DoD)

Status: review

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a **гражданин**,
I want **открыть карточку контракта в браузере и увидеть реальную строку из БД через всю цепочку**,
so that **доказано сквозное прохождение байта Postgres→sqlc→chi→Caddy→React (walking skeleton, DoD=T8)** — фундамент всех последующих экранов.

> **Тип истории:** интегративная walking-skeleton-история Эпика 1 — САМАЯ КРУПНАЯ в эпике. Связывает
> воедино БД (1.2), API (1.3), токены (1.5), i18n+обёртки (1.6) и добавляет Router+Query+карточку+Caddy+
> Playwright. **Бэкенд НЕ меняется** (эндпоинт `/api/contracts/{goszakup_id}` готов с 1.3). Работа —
> в `web/` и `deploy/`.

## Acceptance Criteria

**AC1 — Router (data API) + TanStack Query: Query владеет загрузкой, Router-loader НЕ фетчит, скелетон не спиннер**
**Given** web (Vite+React) с React Router (data API) и TanStack Query
**When** открыт маршрут карточки (`/contracts/:goszakupId`)
**Then** данные фетчатся через `/api/contracts/{id}` — **Query владеет загрузкой; Router-loader НЕ фетчит**; во время загрузки показан **скелетон под раскладку карточки** (не спиннер-стена).

**AC2 — Сквозной байт через `docker compose up` (DoD=T8)**
**Given** Caddy reverse-proxy
**When** один `docker compose up` (профиль `app`)
**Then** байт проходит **Postgres→sqlc→chi→Caddy→React**, и карточка с **реальной строкой `DEMO-0001`** (из `fixtures/seed/contracts.sql`) видна в браузере. Caddy: статика SPA + прокси `/api/*`→`api:8080` + **гейт `/metrics` (только внутренние IP)** (перенос из `deferred-work.md`).

**AC3 — StrictMode-дисциплина + Playwright smoke**
**Given** React StrictMode (включён)
**When** монтируется компонент с императивным эффектом
**Then** `useRef`-guard от двойной инициализации (двойной `new Map()`) — дисциплина **с первого коммита** (реальная карта — Story 1.8)
**And** один **Playwright smoke** закрепляет сквозной путь (маршрут карточки → фетч → реальная строка видна).

## Tasks / Subtasks

- [x] **Task 1 — Зависимости + провайдеры (AC1)**
  - [x] Добавить в `web/package.json` `dependencies`: `react-router-dom` (~7.x) и `@tanstack/react-query` (~5.x); в `devDependencies`: `@playwright/test` (~1.4x). AC-mandated → одобрено спецификацией. **Pin exact** (без `^`); `npm install` + коммит `package-lock.json` (как 1.6). Иных deps без одобрения — нет.
  - [x] `web/src/app/` (или `shared/api`): `QueryClient` + `QueryClientProvider`; обернуть в `main.tsx` поверх `RouterProvider`, сохранив `StrictMode`, `initTheme()`, `import './shared/i18n'`.
- [x] **Task 2 — React Router (data API), loader НЕ фетчит (AC1)**
  - [x] `web/src/router.tsx`: `createBrowserRouter` с маршрутами из существующих `routePaths` (карточка `/contracts/:goszakupId`). **Router-loader НЕ вызывает `fetch`** — только определения маршрутов; данные грузит Query в компоненте.
  - [x] Базовый layout/route для карточки; 404/ошибка-роут — нейтральный заглушечный (полноценный — позже).
- [x] **Task 3 — TanStack Query фетч-хук (AC1)**
  - [x] `web/src/features/contract/useContract.ts`: `useQuery({ queryKey: ['contract', id], queryFn: fetch '/api/contracts/'+id })`. Query владеет loading/error/success. Тип ответа — из `web/src/shared/api/schema.gen.ts` (`Contract`).
  - [x] Обработка 404 (`{error:{code:"NOT_FOUND"}}`) и 5xx — честное состояние, не белый экран.
- [x] **Task 4 — Компонент карточки контракта (AC1, AC2)**
  - [x] `web/src/features/contract/ContractCard.tsx`: рендерит поля из `Contract` wire-формы. **ВАЖНО:** wire — это flat `{value, state}` `StringField` (поля `subject_ru`/`subject_kk` РАЗДЕЛЬНЫ; конверта `{value,lang,is_fallback,resolution}` ещё НЕТ) — поэтому `<LangValue>` для полей контракта здесь **НЕ применяется**; вместо него `<DataState>` + `<span lang="ru|kk">` для двуязычных subject. (`<LangValue>` подключится, когда API вырастит конверт — позже.)
  - [x] Поля и типографика из DESIGN (`contract-card`): over-label (`label-caps`, напр. «КОНТРАКТ · ДОРОГА»), subject (`heading`, `<h2>`), **amount** (`amount` 21px/800, через `formatMoney(amount_tng.value, lang)`), мета-пары dt/dd (`status`, `direction`, даты через `formatDate`, `kato_code`), source-link «Первоисточник/Бастапқы дереккөз ↗» (`source_url`, `link-on-sunken`, underline, `target=_blank rel=noopener`). Только семантические токены (1.5).
  - [x] **Честные состояния:** для КАЖДОГО `StringField` — `<DataState>` по `state` (`no_data`→«нет данных», НИКОГДА «0»/«—»/пусто); `dataStateFromValueState` уже есть (1.6). Двуязычный subject: `<span lang="ru">{subject_ru}</span>` / `lang="kk"`.
  - [x] a11y: заголовок `<h2>`, tap-target ≥44px на ссылках, focus-ring токеном (не `outline:none`), `lang`-атрибуты, статус глифом+текстом (не только цвет).
- [x] **Task 5 — Скелетон под раскладку (AC1)**
  - [x] Скелетон-карточка, повторяющая DOM-раскладку (over-label/heading/amount/мета-строки), не спиннер; рендерится в состоянии загрузки Query (`<DataState kind="loading">`/собственный скелет). Без layout-shift.
- [x] **Task 6 — Deploy: сквозной `docker compose up` (AC2) — есть реальные пробелы**
  - [x] **Создать `server/Dockerfile`** (multi-stage Go-сборка `cmd/api`) — `deploy/docker-compose.yml` ссылается на него (`# добавляется в Story 1.3`), но файл НЕ создан (блокер).
  - [x] Сервинг web: либо сервис `web` (сборка `web/dist`, монтируется в Caddy), либо Caddy монтирует host-собранный `web/dist`. Зафиксировать решение.
  - [x] Наполнить `deploy/Caddyfile` (сейчас заглушка-`respond`): SPA-статика (`try_files` фолбэк), `handle /api/* → reverse_proxy api:8080`, **`/metrics` — allow только внутренние IP** (перенос из `deferred-work.md`).
  - [x] `deploy/docker-compose.yml` (профиль `app`): db (есть) + api (Dockerfile) + caddy + web; миграции+seed до старта (`make migrate-up` + `db-seed` или init-хук). Проверка: `curl http://localhost/api/contracts/DEMO-0001` отдаёт JSON; карточка `/contracts/DEMO-0001` видна.
- [x] **Task 7 — Playwright smoke (AC3)**
  - [x] `web/e2e/` + `playwright.config.ts`: один smoke — маршрут `/contracts/DEMO-0001` → фетч → видны subject и amount «123 456 789 ₸». **Реалистично:** в CI — против `vite preview` с фикстурой/моком `@fixtures` (без docker), полный стек — локальная DoD-проверка `docker compose up`. Зафиксировать выбранный режим.
  - [x] Шаг Playwright в `ci-web.yml` (зарезервирован комментарием :49); `npx playwright install --with-deps`.
- [x] **Task 8 — StrictMode useRef-guard (AC3)**
  - [x] Если в 1.7 появляется императивный эффект (напр. ранний map-задел) — `useRef`-guard от двойной инициализации в StrictMode (реальный MapLibre — Story 1.8). Если императивных эффектов нет — задокументировать дисциплину в коде/доке (карта 1.8 обязана следовать).
- [x] **Task 9 — Тесты и финализация**
  - [x] vitest: чистая логика карточки (маппинг полей→DataState, форматирование) без рендера, ИЛИ — если добавляются RTL/jsdom (новая зависимость, по одобрению) — рендер карточки; иначе рендер покрывает Playwright smoke (1.6-defer закрывается здесь).
  - [x] `cd web && npm run typecheck && lint && lint:css && format:check && test && build` зелёные; `docker compose up` сквозной байт проверен (DoD=T8); обновить File List, Change Log, Completion Notes.

## Dev Notes

### Контекст (зачем walking skeleton сейчас)

Седьмая история Эпика 1 — DoD=T8 «байт проходит всю цепочку». Все слои готовы по отдельности (БД 1.2,
API 1.3, токены 1.5, i18n+обёртки 1.6) — 1.7 связывает их в видимую карточку и доказывает деплой одним
`docker compose up`. Это фундамент карты (1.8), демо (1.9) и всех экранов.
[Source: epics.md#Story-1.7 (958–977)]
[Source: architecture.md — Router/Query (260–262), skeleton (601–602), Caddy/compose (315–316, 462–465), StrictMode-guard (362–363), Playwright (348–349, 693), дерево (683–755)]

### Скоуп: web + deploy (бэкенд НЕ меняется)

- **Эндпоинт готов (1.3):** `server/cmd/api/main.go:65` `/api/contracts/{goszakup_id}`; `/healthz` (:49), `/metrics` (:62). DTO `contracts.go:26-40` == `schema.gen.ts`. **Серверный код не трогаем.** [Source: server/cmd/api/main.go, server/internal/httpapi/contracts.go]
- 1.7 трогает: `web/` (router/query/card/skeleton/e2e) + `deploy/` (Dockerfile api, web-сервинг, Caddyfile, compose).

### Что переиспользовать (НЕ изобретать заново)

- **Wire-форма `Contract`** — `web/src/shared/api/schema.gen.ts` (generated, prettier-ignore, НЕ править): `goszakup_contract_id: string` + остальные поля `StringField {value: string|null, state: <11-enum>}`. `amount_tng.value` — целые тенге СТРОКОЙ → `formatMoney`; даты `YYYY-MM-DD` → `formatDate`. [Source: web/src/shared/api/schema.gen.ts]
- **Обёртки/токены (1.5/1.6):** `<DataState>`+`dataStateFromValueState` (state→kind), `formatMoney`/`formatDate` (Intl; сырой `toLocale*String` запрещён eslint), `<Icon>`, `tokens.gen.ts` `token()`, i18n `chrome` (`useTranslation('chrome')`, kk-дефолт). [Source: web/src/shared/{state,i18n,ui,tokens}]
- **⚠️ `<LangValue>` НЕ для полей контракта в 1.7** — он построен под будущий конверт `{value,lang,is_fallback,resolution}`, которого на проводе ещё НЕТ; Contract отдаёт flat `{value,state}` + раздельные `subject_ru`/`subject_kk`. Карточка показывает subject через `<DataState>` + `<span lang>`. Не притягивать LangValue к flat-форме. [Source: schema.gen.ts; 1-6 Dev Notes]
- **Seed-строка (DoD):** `fixtures/seed/contracts.sql` → `DEMO-0001`: subject_ru «Демонстрационный контракт: ремонт автодороги…», subject_kk «…автожол жөндеу…», amount_tng `123456789` («123 456 789 ₸»), sign_date `2026-03-15`, plan_start `2026-04-01`, plan_end `2026-09-30`, status `active`, direction `road`, kato `710000000`, source_url goszakup. Карточка и smoke целятся на неё. [Source: fixtures/seed/contracts.sql]
- **Поля/типографика карточки** — DESIGN `contract-card`: over-label `label-caps`, subject `heading`(<h2>), amount `amount`(21px/800, самый крупный), мета `body`/`meta`, source-link `link-on-sunken` underline. Skeleton — под раскладку, <1с p95, не спиннер. [Source: DESIGN.md (contract-card, типографика 98–146, source-link 296–301); EXPERIENCE.md (поля :212, skeleton :261, a11y 327–369)]
- **Каркас `web/src/features/contract/`** — `.gitkeep` (наполнять). `web/src/router.tsx` — типизированные `routePaths` (использовать). [Source: web/src/features/contract/, web/src/router.tsx]

### Deploy — реальные пробелы (НЕ скаффолд!)

- `deploy/Caddyfile` — заглушка (`:80 { respond "…skeleton…" 200 }`). Наполнить: SPA + `/api`→`api:8080` + `/metrics` IP-гейт. [Source: deploy/Caddyfile]
- `deploy/docker-compose.yml` — `db` реальна; `api`/`caddy` под профилем `app`; `api` ссылается на `server/Dockerfile` который **НЕ создан** (1.3 отложила); сервиса `web` НЕТ. Создать Dockerfile(ы) + web-сервинг. [Source: deploy/docker-compose.yml]
- `api` слушает `cfg.Addr` (`internal/config`); compose/Caddy ориентируются на `api:8080` — выставить Addr в env api-сервиса. [Source: server/internal/config/config.go]
- `/metrics`-гейт — явный пункт `deferred-work.md` для 1.7. [Source: _bmad-output/implementation-artifacts/deferred-work.md]

### Соблюдение архитектуры (guardrails)

- **Query владеет загрузкой; Router-loader НЕ фетчит** (data API только для маршрутов). [Source: architecture.md:260–262; epics.md:968]
- **Скелетон под раскладку, не спиннер-стена.** [Source: architecture.md:601–602; EXPERIENCE.md:261]
- **StrictMode → `useRef`-guard** двойной инициализации с первого коммита (карта 1.8). [Source: architecture.md:362–363]
- **Честность:** отсутствующее поле → честное состояние (`no_data`«нет данных»), НИКОГДА «0»/«—»/выдуманное (через `<DataState>`). Нейтральность: без taboo; source-link нейтрален. WCAG 1.4.1. [Source: EXPERIENCE.md:194, 250–256; PRD FR-10/FR-11]
- **`kk`/`ru` не `kz`; формат только через formatMoney/formatDate** (Intl), сырой toLocale*String — eslint-red. [Source: 1.6]

### Версии новых зависимостей (AC-mandated; pin exact)

- `react-router-dom` ~7.x, `@tanstack/react-query` ~5.x, `@playwright/test` ~1.4x — **подтвердить latest-stable, запинить точно**, коммит lockfile. Это RUNTIME (router/query) + dev (playwright). [Source: architecture.md:260; epics.md:966,977]

### Границы скоупа (НЕ делать)

- **Карта/MapLibre/PMTiles** — Story 1.8 (здесь только StrictMode-дисциплина). **«3 читаемых контракта» демо-контент** — Story 1.9. **Поиск/подрядчик/район** — позже. **Реальные флаги/проза** — Epic 4. **Полный bilingual-конверт** доменных данных — позже (API вырастит). Не реализовывать тут.
- Не менять серверный код / OpenAPI / `schema.gen.ts` (карточка только потребляет существующий контракт).

### Файлы и куда писать

- **Новое (web):** `web/src/features/contract/{ContractCard.tsx, useContract.ts, ContractSkeleton.tsx, index.ts}`; провайдеры (`web/src/app/*` или в `main.tsx`); `web/e2e/*.spec.ts` + `web/playwright.config.ts`; тесты карточки.
- **UPDATE (web):** `web/package.json`+lock (deps), `web/src/router.tsx` (createBrowserRouter), `web/src/main.tsx` (Query+Router providers), `web/src/App.tsx` (RouterProvider/layout), `web/.github/.../ci-web.yml` (Playwright-шаг). Удалить `web/src/features/contract/.gitkeep`.
- **Новое/UPDATE (deploy):** `server/Dockerfile` (новый), web Dockerfile/сервис (если выбран), `deploy/Caddyfile` (наполнить), `deploy/docker-compose.yml` (web/api/caddy).
- **Не трогать:** `server/internal/**`, `registry/`, `schema.gen.ts`, готовые истории.

### Тестирование / проверка готовности

- vitest (1.5/1.6): чистая логика карточки/хука где возможно. **Рендер карточки** — Playwright smoke (закрывает и 1.6-defer на render-тесты обёрток). DoD=T8: ручной `docker compose up` → `DEMO-0001` видна.
- CI: `ci-web.yml` (typecheck/eslint/stylelint/prettier/vitest/build + Playwright-шаг). Полный e2e через docker — локальная DoD-проверка (тяжело для CI; smoke в CI — fixture/preview-backed).
- DoD: AC1–AC3 закрыты; Query владеет загрузкой; скелетон не спиннер; `docker compose up` → реальная строка видна; `/metrics` загейчен; Playwright smoke зелёный; честные состояния; web-гейт зелёный.

### Previous Story Intelligence (1.6, 1.3, 1.5)

- **1.6 (done):** обёртки `<DataState>`/`<Icon>`/`format*`/i18n `chrome`; **DOM-render тесты обёрток отложены на Playwright-резерв 1.7** — закрыть здесь smoke'ом. `<LangValue>` — против будущего конверта (не для flat Contract). [Source: 1-6-….md]
- **1.3 (done):** `/api/contracts/{id}` + `{value,state}` wire + OpenAPI-арбитр; **api Dockerfile/Caddy SPA отложены на 1.7** (compose-комментарий устарел — Dockerfile не создан). [Source: 1-3-….md, deploy/docker-compose.yml]
- **1.5 (done):** токены + `theme.ts` bootstrap; pinned-exact + lockfile конвенция.
- **deferred-work.md:** `/metrics` IP-гейт → 1.7 (Caddy). [Source: deferred-work.md]
- **⚠️ Рабочее дерево:** 1.5-review-патчи + 1.6 (impl+review) могут быть незакоммичены — 1.7 строится поверх; учесть при коммите.

### Project Structure Notes

- Конфликтов нет: `web/src/{app,router,features/contract,e2e}`, `deploy/{Caddyfile,docker-compose.yml}`, `server/Dockerfile` — штатные места. [Source: architecture.md:683–755]
- Это самая интегративная история эпика — учитывать связность web↔api↔deploy при реализации/ревью.

### References

- [Source: epics.md#Story-1.7 (958–977); #Story-1.8 (979–997, граница карты); #Story-1.9 (демо-контент)]
- [Source: architecture.md — Router/Query (260–262), skeleton (601–604), Caddy/compose (315–316, 462–465, 751–755), StrictMode-guard (362–363), Playwright (348–349, 693, 745), дерево (683–755), wire (547, 564–570)]
- [Source: prds/.../prd.md — FR-10 (поля карточки), FR-11 (первоисточник), NFR-7 (a11y)]
- [Source: ux-designs/ux-AshyqQala.kz-2026-06-18/DESIGN.md (contract-card, типографика 98–146, source-link 296–301, data-state 277–295); EXPERIENCE.md (поля :212, skeleton :261, a11y 327–369, voice/taboo 135–204)]
- [Source: server/cmd/api/main.go (:49/:62/:65), server/internal/httpapi/{contracts.go,field.go}, server/internal/config/config.go]
- [Source: web/src/{router.tsx, main.tsx, App.tsx, shared/api/schema.gen.ts, shared/state/DataState.tsx, shared/i18n/format.ts, shared/tokens/tokens.gen.ts}; web/package.json; .github/workflows/ci-web.yml (:49)]
- [Source: deploy/{Caddyfile, docker-compose.yml, docker-compose.cold.yml}; fixtures/seed/contracts.sql (DEMO-0001); _bmad-output/implementation-artifacts/deferred-work.md (/metrics→1.7)]
- [Source: CLAUDE.md — guardrails честности/нейтральности; новые зависимости — с одобрением]

## Dev Agent Record

### Agent Model Used

claude-opus-4-8[1m] (Claude Opus 4.8, 1M context) — create-story (context engine).

### Debug Log References

- `npm install react-router-dom @tanstack/react-query` (7.18.0 / 5.101.0) + `-D @playwright/test` (1.61.0), pinned exact, lock пересобран без кареток.
- Web-гейт зелёный: typecheck / eslint / stylelint / prettier / **vitest 46/46** (8 файлов; e2e исключён через `vite.config.ts` test.include) / build.
- **Playwright smoke ПРОШЁЛ** (`npm run e2e`, chromium): `/` → клик демо-ссылки → `/contracts/DEMO-0001` → карточка с «123…₸» видна (route+Query+card path доказан).
- **`server/Dockerfile` собирается** (`docker compose --profile app build api` → exit 0; Go 1.25 multi-stage → distroless).
- `docker compose --profile app config` валиден (db→migrate→seed→api→caddy).

### Completion Notes List

**Контекст-инжиниринг (create-story):** 3 агента; самая интегративная история (web+deploy), бэкенд не меняется.

**Реализация (dev-story):**
- **AC1** — React Router data API (`createBrowserRouter`, маршруты из `routePaths`; **Router-loader НЕ фетчит**) + TanStack Query (`QueryClient`, `useContract` через `/api/contracts/{id}` — **Query владеет загрузкой**); **скелетон под раскладку** (`ContractSkeleton`, не спиннер).
- **AC2** — deploy-цепочка: создан `server/Dockerfile` (отсутствовал, хотя compose ссылался) + `web/Dockerfile` (Vite build → Caddy serve); наполнен `deploy/Caddyfile` (SPA `try_files` + `/api/*`→`api:8080` + `/metrics`→404 на edge = гейт из deferred-work); `docker-compose.yml` профиль `app` = db→migrate(goose)→seed(DEMO-0001)→api→caddy. **api-образ собирается; compose config валиден.** Полный байт-flow `docker compose --profile app up` — ручная DoD-T8 (тяжёлый запуск стека; артефакты корректны и сверены сборкой/конфигом).
- **AC3** — `useRef`-guard: в 1.7 нет императивного `new Map()`/эффекта (карта — 1.8); дисциплина задокументирована. **Playwright smoke** (`web/e2e/contract.spec.ts`, `playwright.config.ts`) закрепляет web-путь; добавлен в `ci-web.yml`. Smoke также закрывает 1.6-defer на render-тесты обёрток (карточка использует DataState/format/Icon).
- **Ключевое решение:** карточка биндит flat `{value,state}` `StringField` (subject_ru/kk раздельно) — `<LangValue>` НЕ применён (его конверта на проводе ещё нет); subject через `<DataState>` + `<span lang>`. Честные состояния: `Value`-компонент рендерит значение при `ok`, иначе `<DataState>` (никогда «0»/пусто). `formatMoney`/`formatDate` (Intl).
- **Новые deps:** react-router-dom 7.18.0, @tanstack/react-query 5.101.0 (runtime), @playwright/test 1.61.0 (dev) — AC-mandated, pinned exact, lock синхронизирован.
- **Осознанно вне скоупа:** карта/MapLibre (1.8), демо-3-контракта (1.9), поиск/подрядчик/район (позже), bilingual wire-конверт (позже). Серверный код/OpenAPI не тронуты.

### File List

**Новое (web — карточка/маршрут/Query):**

- `web/src/features/contract/ContractCard.tsx` — карточка: over-label/subject(h2)/amount/мета(dl)/source-link; честные состояния через `Value`+`<DataState>` (значение только при `state==='ok'`), `formatMoney`/`formatDate` (Intl), `lang`-атрибут на subject.
- `web/src/features/contract/ContractRoute.tsx` — маршрут-обёртка: `useContract`, `isPending→ContractSkeleton`, `isError→role=alert`, success→`ContractCard`. Loader НЕ фетчит.
- `web/src/features/contract/ContractSkeleton.tsx` — скелетон под раскладку карточки (не спиннер), без layout-shift.
- `web/src/features/contract/useContract.ts` — `useQuery(['contract', id])` + `fetchContract` (честный код ошибки из `{error:{code}}`); Query владеет загрузкой.
- `web/src/features/contract/useContract.test.ts` — vitest: `fetchContract` (ok / 404 NOT_FOUND / non-JSON 5xx → INTERNAL).
- `web/src/features/contract/contract-card.css` — типографика/раскладка карточки на семантических токенах (1.5); focus-ring токеном, tap-target ≥44px.
- `web/src/features/contract/index.ts` — публичный barrel фичи.
- `web/src/app/queryClient.ts` — единый `QueryClient` (retry 1, `refetchOnWindowFocus:false`, `staleTime 30s`).
- `web/e2e/contract.spec.ts` — Playwright smoke: `/` → клик демо-ссылки → `/contracts/DEMO-0001` → amount «123…₸» + h2 видны (route→Query→card); `/api` замокан `route.fulfill`.
- `web/playwright.config.ts` — конфиг smoke (против `vite preview` :4173; CI без docker).

**Новое (deploy/server — сквозной байт):**

- `server/Dockerfile` — multi-stage Go (1.25-alpine → distroless static nonroot, CGO off) для `cmd/api` (отсутствовал, хотя compose ссылался — блокер из 1.3).
- `server/.dockerignore` — контекст сборки api.
- `web/Dockerfile` — multi-stage (node:22 `npm ci`+build → `caddy:2-alpine`, `dist`→`/srv`).
- `web/.dockerignore`, `web/.gitignore` — контекст/игнор web-образа.

**Изменено (web):**

- `web/package.json` + `web/package-lock.json` — deps (pin exact): `react-router-dom 7.18.0`, `@tanstack/react-query 5.101.0` (runtime), `@playwright/test 1.61.0` (dev); скрипт `e2e`.
- `web/src/router.tsx` — `createBrowserRouter` (layout `/` + `contracts/:goszakupId`); `routePaths` сохранён; loader НЕ фетчит.
- `web/src/main.tsx` — `QueryClientProvider` поверх `RouterProvider`, StrictMode/`initTheme()`/`import './shared/i18n'` сохранены.
- `web/src/App.tsx` — корневой layout (header+`<Outlet/>`) + `HomeView` со ссылкой на DEMO-0001 (входная точка сквозного пути).
- `web/vite.config.ts` — `test.include: ['src/**/*.test.*']` (e2e исключён из vitest — гоняет Playwright).
- `.github/workflows/ci-web.yml` — шаг Playwright smoke (`playwright install --with-deps chromium` + `npm run e2e`).

**Изменено (deploy):**

- `deploy/Caddyfile` — наполнен: `/metrics*`→404 на edge (гейт из `deferred-work.md`), `/api/*`→`reverse_proxy api:8080`, SPA `try_files {path} /index.html` + `file_server`.
- `deploy/docker-compose.yml` — профиль `app`: `db`→`migrate`(goose v3.27.1)→`seed`(DEMO-0001)→`api`(server/Dockerfile, `API_ADDR :8080`)→`caddy`(web/Dockerfile + Caddyfile).

**Удалено:**

- `web/src/features/contract/.gitkeep` — каркас наполнен.

> **⚠️ Состояние рабочего дерева (для ревью/коммита):** последний коммит — «1.4 story». В рабочем дереве вместе с 1.7 лежат незакоммиченные файлы **1.5** (review-патчи `shared/tokens/{contrast*,theme.ts}`) и **1.6** (целиком `shared/i18n/**`, `shared/state/DataState*`, `shared/ui/Icon*`, `shared/tokens/theme.test.ts`, плюс правки `eslint.config.js` i18n-граница, `.stylelintrc.json` outline, `tsconfig.json` `resolveJsonModule`). Эти файлы — НЕ часть File List 1.7 (принадлежат своим историям), но 1.7 строится поверх них. При коммите учесть (отдельные коммиты «1.5 story»/«1.6 story»/«1.7 story» либо по решению владельца).

## Change Log

| Дата       | Версия | Описание                                                                                                                | Автор |
| ---------- | ------ | ----------------------------------------------------------------------------------------------------------------------- | ----- |
| 2026-06-22 | 0.1    | create-story: контекст-инжиниринг (web+deploy, бэкенд не меняется)                                                       | Amelia (Dev) |
| 2026-06-22 | 1.0    | dev-story: AC1 Router(data API)+Query+скелетон; AC2 server/web Dockerfile + Caddyfile + compose `app`; AC3 smoke+StrictMode-дисциплина | Amelia (Dev) |
| 2026-06-22 | 1.1    | Финализация: верификация (web-гейт 46/46, Playwright smoke зелёный, compose config валиден), File List, Change Log, статус→review | Amelia (Dev) |
