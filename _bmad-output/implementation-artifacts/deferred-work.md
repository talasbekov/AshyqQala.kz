# Deferred Work

## Deferred from: code review of 1-7-сквозная-карточка-контракта-walking-skeleton-dod (2026-06-22)

- **`caddy` зависит от `api` только по старту, не по health** (`deploy/docker-compose.yml`) — `caddy.depends_on: [api]` (короткая форма = `service_started`), а у `api` нет `healthcheck` (образ distroless — нет shell/curl для стандартной проверки). Первые запросы `/api/*` сразу после `docker compose --profile app up` могут получить 502 от `reverse_proxy api:8080`, пока chi поднимается. Для ручной DoD-проверки — транзиентный флай. Захардить позже: добавить Go-based healthcheck в api (вызов своего бинаря) + `caddy.depends_on.api.condition: service_healthy`, ИЛИ `lb_try_duration`/retry в Caddyfile reverse_proxy.
- **`migrate` тянет goose из сети на каждом холодном старте** (`deploy/docker-compose.yml`) — `go run github.com/pressly/goose/v3/cmd/goose@v3.27.1` скачивает модуль при каждом запуске one-shot `migrate`, без ретрая. Хрупко к сетевым/IPv6 сбоям среды (см. memory: Docker Hub/IPv6 ретраи). Захардить: предсобрать goose в builder-слое образа / вендорить / добавить `restart`-политику с бэкоффом для one-shot миграции.

## Deferred from: code review of 1-1-монорепо-скелет-и-compose-db (2026-06-21)

- **`/metrics` на публичном listener без гейтинга** (Story 1.3) — `cmd/api` отдаёт `/metrics` (promhttp: go-runtime-внутренности, позже SM-C1/SM-C2) на том же публичном listener, что и `/api/*`. В **Story 1.7** (Caddy reverse-proxy) ограничить путь `/metrics` (allow только внутренние IP) ИЛИ вынести на отдельный admin-listener.
- **`amount_tng` — целые тенге (решение владельца, Story 1.3)** — в **Epic 2** (живой импорт) импортёр обязан: (а) валидировать/округлять дробные суммы goszakup, если такие встречаются; (б) guard на overflow BIGINT (`>9.2e18`). Сейчас S-0/синтетика — целые тенге.
- **sqlc-регенерация зависит от распознавания goose `-- +goose Down`** (Story 1.2) — `server/sqlc.yaml` читает `../migrations` целиком; sqlc v1.31 корректно игнорирует Down-секции (проверено: генерация чистая), но это version-хрупкое неявное допущение. В **Story 1.4** добавить CI-страж «generated==regenerated» (уже в плане `ci-registry.yml`), чтобы дрейф sqlc/миграций ловился автоматически.
- **Скомпилированный бинарь `stage0-audit/stage0-audit` отслеживается git** — предсуществующая проблема (не вызвана Story 1.1). Бинарь ~9.6 МБ закоммичен в ранних коммитах; меняется при каждой сборке `stage0-audit`. Вычистить в рамках Story 0.2: `git rm --cached stage0-audit/stage0-audit` + добавить правило в `.gitignore` (например `stage0-audit/stage0-audit`).

## Deferred from: code review of 1-6-i18n-каркас-kz-дефолт-и-обёртки-инкапсуляторы (2026-06-22)

- **i18n init-порядок безопасен только на in-memory ресурсах.** Сейчас `i18n.init({resources})` синхронен → `App` рендерится после готовности, `<Suspense>` не нужен. При добавлении async-backend (`i18next-http-backend`/lazy-namespaces) `useTranslation` начнёт suspend'иться, а `<Suspense>`-границы нет → пустой рендер/ошибка. Зафиксировать инвариант или добавить `<Suspense>` при переходе на async-загрузку локалей.
- **i18n-cross тест однонаправленный.** Проверяет «локаль покрывает каждый honest-state», но не ловит ЛИШНИЕ ключи и даёт `TypeError` (не чистый assert) при отсутствии объекта `value_state`/`flag_state` в локали. При росте локалей — сделать двунаправленную сверку (равенство множеств) + мягкий fail.
- **DOM-render тесты обёрток** (`<LangValue>`/`<DataState>`/`<Icon>`) — требуют RTL+jsdom (новые devDeps). Отложено на Playwright-резерв (`ci-web.yml`) ИЛИ добавить по одобрению владельца. Сейчас покрыто чистой логикой + tsc.

## Deferred from: code review of 1-5-дизайн-токены-и-две-темы (2026-06-22)

- **Codegen `tokens-codegen.mjs` — устойчивость к НЕштатному `tokens.json`.** Сейчас вход авторский (не внешний), триггеров нет. При росте/внешнем источнике добавить: ошибку (не молчаливую потерю) для узла с `$value` + дочерними; экранирование `}`/`;`/переводов строки в значениях; детекцию дублей CSS-var-имён из разных путей; экранирование кавычек в TS-union; явную сортировку ключей (сейчас порядок numeric-ключей делегирован V8 — вывод детерминирован, но порядок source≠output для integer-ключей может удивить редактора).
- **Фронт-тулинг латентные дыры.** `lint:css --allow-empty-input` вернёт зелёный, если glob промахнётся мимо будущего фичевого CSS (страж молча перестанет защищать) — добавить проверку, что glob матчит ожидаемое, когда появится фичевый CSS. `scripts/` не входит в tsconfig → `tokens-codegen.d.mts` может разойтись с `.mjs` без ошибки компиляции; при росте codegen — перенести на TS или добавить тест соответствия типов.

## Deferred from: code review of 1-4-registry-двухосевой-enum-честных-состояний-render-каркас (2026-06-22)

- **Матчер нейтральности однонаправленный** (`server/internal/registry/neutrality.go`) — `FindTaboo` проверяет лишь ОТСУТСТВИЕ taboo-строк, но не ПРИСУТСТВИЕ обязательной нейтральной рамки в выводе. Архитектура (doc-нейтральность) предполагает двунаправленный матчер. Сейчас рамка `frame.signal` структурно гарантирована в `render.Render`, поэтому риск низкий. Активировать в **Epic 4**, когда появятся per-flag шаблоны прозы (текст уже не сводится к одной рамке).
