---
baseline_commit: e31bd208e212dcbb5a4a9f7a4c795c8adf9dc369
---

# Story 3.2: Полуручная разметка через Directus (FR-5)

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

> **Рамка (создано 2026-07-02, baseline-намерение = `e31bd20`):** история **полностью
> токен-независима** (прецедент 2.0/4.1/6.4/3.1 «механизм на синтетике, живое честно дескоупим»).
> Схема и защитные гейты под ручную разметку **уже построены Story 3.1** — 3.2 добавляет
> «писателя»: Directus-контур, терминальные статусы верификации (AR-28) и кураторские методы.
> Цитата docs/ops/geocoding.md:105-108: «Directus просто начнёт писать эти строки,
> схема/гейт уже на месте».

## Story

As a оператор геопривязки,
I want подтверждать/корректировать точку или полилинию,
so that карта точна там, где автогеокодер ошибся.

[Source: epics.md:1242-1246; UJ-4 prd.md:62-63]

## Acceptance Criteria

**AC1 — Очередь → ручная геометрия → `manual`, правка переживает перезапись** [Source: epics.md:1250-1252]
**Given** Directus-очередь негеопривязанных объектов (curated)
**When** оператор подтверждает/корректирует геометрию (POINT|LINESTRING для дорог) и публикует
**Then** объект переходит в `geocode_status=manual` с сохранением геометрии; правка переживает ре-импорт (граница из 2.6).
*Уточнение к реальности кода:* живого импортёра нет (токен), и importer к `geo_objects` не пишет вовсе (гранты `0020:48-49` + AR-10) — «переживает ре-импорт» проверяется против ближайшего РЕАЛЬНОГО перезаписывателя: **повторного batch-прогона `cmd/geocode`** (адаптация приёмки AR-10 «два прогона + курация между», epics.md:215-217). Гейт `UpsertGeoObject ... WHERE geocode_status IS DISTINCT FROM 'manual'` уже защищает `manual`; расширить на новые статусы (Task 1).

**AC2 — Некорректный адрес → честное «без точки на карте», объект остаётся доступен** [Source: epics.md:1254-1256; prd.md:126]
**Given** некорректный адрес
**When** объект нельзя геопривязать
**Then** он помечается «без точки на карте», но остаётся доступен в поиске/списке/карточке.
*Уточнение:* «без точки на карте» = `geocode_status='unmatched'`, `geom IS NULL` (маппинг data_model:49; публичное состояние `{data-state-ungeocoded}` EXPERIENCE.md:255). Доступность в поиске/списке/карточке архитектурно уже гарантирована (поиск 6.2/фильтры 6.1/карточка 5.1 работают по `contracts`, join'а на geo нет) — закрепляется тестом-стражем против будущего регресса. Публичная МЕТКА на карте/карточке — Story 3.4/3.6 (фронта в 3.2 нет).

**AC3 — Статус верификации фиксируется на `geo_objects` (питает SM-C2)** [Source: epics.md:1258-1260; AR-28 epics.md:286-289]
**Given** статус верификации гео (`auto/verified/wrong_reported`, AR-28)
**When** оператор размечает
**Then** статус фиксируется на `geo_objects` (питает SM-C2).
*Уточнение:* в схеме 3.1 значений `verified`/`wrong_reported` НЕТ нигде (CHECK `0020:39` = `auto|manual|unmatched`) — 3.2 расширяет закрытый enum (решение D1). «Питает SM-C2» = доля ошибочной геопривязки становится вычислимой SQL'ом по статусу; экспорт в `/metrics` — при появлении metrics-провода (NFR-4), НЕ в 3.2.

## Tasks / Subtasks

- [x] **Task 0 — Рамка и дефолты (решения D1–D6)** (AC: все)
  - [x] Прочитать «Открытые вопросы / решения владельцу» ниже. Если владелец не отклонил дефолты — следовать им без переспроса (прецедент 7-1). *(Владелец запустил dev сразу после доклада с D1–D6 — дефолты приняты, следовал им.)*
- [x] **Task 1 — Миграция `0022`: enum верификации + гейты + length_km-триггер** (AC: 1, 3)
  - [x] `migrations/0022_geo_verification.sql`: расширить `geo_objects_status_chk` до `('auto','manual','unmatched','verified','wrong_reported')` (D1). Honesty-CHECK `(geom IS NULL)=(geocode_status='unmatched')` НЕ трогать: `verified`/`wrong_reported` обязаны нести геометрию; снятие точки = переход в `unmatched` c `geom=NULL`.
  - [x] Триггер length_km (D2): BEFORE INSERT OR UPDATE — для LINESTRING вычислять `ST_Length(geom::geography)/1000.0`, когда `length_km IS NULL` (INSERT) или geom изменился (UPDATE); явно заданное значение при неизменном geom уважать. *(Плюс: не-LINESTRING/geom NULL ⇒ length_km := NULL — у точки нет длины.)*
  - [x] Синхронно правка `queries/geo_objects.sql`: гейт `UpsertGeoObject` → `WHERE geocode_status IN ('auto','unmatched')`. `ListContractsForGeocode` НЕ менялся (verified терминален = deferred-work.md:321 закрыт).
  - [x] `make gen-sqlc` (docker) — идемпотентность доказана md5-суммой двух прогонов.
  - [x] Стражи, доказанно красные (RED-прогон против схемы 0021 ДО применения 0022 — все падали по правильным причинам; после `migrate-up` — зелёные): REJECT bogus-статус (negative-control), verified/wrong_reported без geom, триггер-кейсы (вывод/уважение явного/пересчёт при перерисовке/NULL при снятии).
  - [x] ⚠ Goose-ловушка 3.1 соблюдена (prose-комментарии 0022 без буквальных директив; migrate-up прошёл на двух БД).
- [x] **Task 2 — Кураторские методы (зеркало `org_aliases.go`) + sqlc** (AC: 1, 3)
  - [x] 5 запросов + 5 методов `GeoObjectStore`: `ListGeoObjectsByStatus`, `ResolveGeoObjectManually` (`:execrows`, confidence→NULL честно), `MarkGeoObjectVerified` (только auto→verified), `MarkGeoObjectWrongReported` (auto|manual|verified→), `ClearGeoObjectToUnmatched` (только wrong_reported→).
  - [x] Методы = «имитация Directus» в тестах + шов для будущего провода error_reports→wrong_reported (Epic 3, не здесь).
  - [x] Integration-тесты переходов, вкл. запрещённые (manual→verified=0 строк; unmatched→wrong_reported=0; manual→unmatched напрямую=0; несуществующий id=0).
- [x] **Task 3 — Приёмка AR-10 (адаптация): «прогон → курация → прогон»** (AC: 1)
  - [x] `TestAcceptance_AR10_BatchCurationBatch` (через реальный `geocodeContracts`+store+фейк-геокодер): verified/wrong_reported/manual пережили повторный прогон ЦЕЛИКОМ (geom/status/провенанс), некурированный auto честно пере-геокодирован.
  - [x] `TestVerified_TerminalForBatch`: verified уходит из `ListContractsForGeocode` + прямой UPSERT его не трогает — deferred-work.md:321 ЗАКРЫТ.
  - [x] ⚠ Makefile-ловушка снята РАЗМЕЩЕНИЕМ (адаптация): тесты живут в `cmd/geocode` (переиспользован харнесс mustConn/insertContract/fakeGeocoder — main_test.go без build-тега компилируется и под integration) — `INTEG_CLEAN_PKGS` менять НЕ пришлось; обе группы прогнаны зелёными на СВЕЖЕЙ БД 0001-0022 (clean) + свежей засеянной (seed).
- [x] **Task 4 — Directus-контур: compose + коллекции + ops-док** (AC: 1)
  - [x] `deploy/docker-compose.cold.yml`: сервис наполнен — `directus/directus:11.17` (актуальный минор, сверен с docs/Docker Hub), SECRET/ADMIN_*/DB_* из env c dev-дефолтами (паттерн POSTGRES_*), `PUBLIC_URL`, `WEBSOCKETS_ENABLED=false`, `TELEMETRY=false`, порт `127.0.0.1:8055`, `depends_on: db (healthy)`. Caddyfile НЕ тронут.
  - [x] `.env.example`: DIRECTUS_SECRET/ADMIN_EMAIL/ADMIN_PASSWORD (плейсхолдеры-комментарии).
  - [x] Коллекции: `deploy/directus/bootstrap.sh` (идемпотентен, доказано повторным прогоном) — только кураторские geo_objects (map-интерфейс geom, dropdown 5 статусов, read-only id/public_id/contract_id/district_id/geocoded_at/length_km/confidence) + districts (read-only) + закладки «Очередь геопривязки» (unmatched/wrong_reported) и «Верификация (auto)» (confidence ASC). **Адаптация против плана:** вместо `schema snapshot/apply` — API-bootstrap-скрипт (snapshot несёт schema-часть таблиц, а владелец схемы — goose; скрипт применяет ТОЛЬКО метаданные админки; отклонение обосновано в README). `geocoded_by` default в UI не ставится (default применяется на INSERT, а куратор ПРАВИТ существующие строки) — провенанс держит чек-лист + Go-методы.
  - [x] `docs/ops/geocoding.md`: секция «Ручная курация через Directus (Story 3.2)» — up/down профиля, SSH-туннель, таблица переходов, запрет фабрикации §7.4, length_km руками не заполнять, не создавать дубли строк (0021-индекс), D6 «публикация».
  - [x] Верификация ЖИВЬЁМ (headless через Directus REST API — тот же путь записи, что UI; скринов нет, PG-истина в Debug Log): очередь видна из Directus; куратор нарисовал LINESTRING на unmatched DEMO-GEO-07 → `manual`+`geocoded_by='directus'`+**триггер вывел length_km=3.06** (в PATCH длина не передавалась); auto DEMO-GEO-06 → `verified` (провенанс/явный length 5.0 сохранены); состояние возвращено к seed. Гранты: `TestRoleGrants_GeoCurationBoundary` добавлен (curator пишет geo_objects/districts; importer — 42501) — зелёный.
- [x] **Task 5 — Seed очереди + страж доступности (AC2)** (AC: 2)
  - [x] `fixtures/seed/geo_objects.sql`: DEMO-GEO-07 (мусорный адрес, unmatched, geom NULL, район по КАТО) — идемпотентно; 6 auto-строк и медиана 42M не тронуты (проверено re-seed'ом на dev-БД).
  - [x] `TestUnmatchedContract_VisibleInSearchAndCard_Integration` (SEED-группа, httpapi): unmatched-контракт находится SearchContracts и открывается GetContractByID — страж против будущего join-регресса.
- [x] **Task 6 — Полные гейты + DoD** (AC: все)
  - [x] `make build` (go+web) ✓; `make lint` ✓; unit `go test -count=1 ./...` ✓ (0 регрессий); `check-core` ✓; `check-registry` ✓; `gen-sqlc` идемпотентен (md5×2) ✓; `test-integration-clean` НА СВЕЖЕЙ БД 0001-0022 ✓; `test-integration-seed` на свежей засеянной ✓; `docker compose config` hot + cold(geocode+directus) ✓. Directus погашен после сессии (AR-21), dev-db оставлена как была.
  - [x] Story-файл: File List, Change Log, Completion Notes; sprint-status → review.

### Review Findings

Code review 2026-07-02 (3 слоя: Blind Hunter / Edge Case Hunter / Acceptance Auditor; дифф = working tree против baseline `e31bd20`). Триаж: 1 decision / 11 patch / 2 defer / 7 dismiss.

- [x] [Review][Decision→Defer] Directus-путь записи не ограничен стейт-машиной переходов на уровне БД — оператор в dropdown может выставить `manual`/`verified`/`wrong_reported` → `auto` (следующий batch легально перезапишет кураторскую геометрию через гейт `IN ('auto','unmatched')`); двинуть geom у `verified`-строки без смены статуса; при прямой правке geom остаётся чужой Nominatim-`confidence`. **Решение владельца 2026-07-02: принять дисциплину чек-листа (как D5 спеки), hardening (транзишн-триггер ИЛИ Directus-permissions) → deferred-work.md, сделать до живого пилота бандлом с role-DSN (deferred:269).**
- [x] [Review][Patch] `ResolveGeoObjectManually` допускает `verified→manual` в обход двухшагового следа D1 (verified сначала → wrong_reported) — добавить статус-гард + negative-тест + ops-таблица [server/internal/store/queries/geo_objects.sql:94]
- [x] [Review][Patch] bootstrap.sh: логин-JSON собирается сырой интерполяцией `$EMAIL`/`$PASSWORD` — кавычка/бэкслеш в пароле ломает JSON; пароль виден в argv (`ps`) — собрать через `jq -n --arg` + `--data @-` [deploy/directus/bootstrap.sh:22-23]
- [x] [Review][Patch] bootstrap.sh: при недоступном Directus `set -e` убивает скрипт на присваивании TOKEN ДО дружелюбной ошибки строки 24 — обработать сбой curl явно [deploy/directus/bootstrap.sh:22-24]
- [x] [Review][Patch] bootstrap.sh: `api()` использует `curl -s` без `-f` — транзиентный сбой GET /presets даёт `found=""`/error-JSON → безусловный POST → дубль закладки; плюс в телах пресетов нет явных `"user":null,"role":null` (глобальность недоказуема) [deploy/directus/bootstrap.sh:26-34,75-88]
- [x] [Review][Patch] bootstrap.sh: нет проверки наличия `curl`/`jq` (`command -v`) — падение с сырым «command not found» [deploy/directus/bootstrap.sh:14]
- [x] [Review][Patch] Триггер 0022: вырожденная LINESTRING (совпадающие вершины / EMPTY) даёт `length_km=0` вместо честного NULL — `NULLIF(..., 0)` [migrations/0022_geo_verification.sql:33]
- [x] [Review][Patch] Тип геометрии не ограничен: MULTILINESTRING/POLYGON проходят в `manual`, длина молча NULL — CHECK `GeometryType(geom) IN ('POINT','LINESTRING')` (контракт AC1) + negative-тест [migrations/0022_geo_verification.sql]
- [x] [Review][Patch] Страж AC2 покрывает поиск+карточку, но не «список» (`ListContracts`) из трёх заявленных поверхностей — добавить проверку листинга [server/internal/httpapi/geo_unmatched_visibility_integration_test.go]
- [x] [Review][Patch] Нет теста «нетронутый unmatched НЕ пере-выбирается вторым прогоном» (клейм CLI-сообщения и комментария запроса) — добавить assertion [server/cmd/geocode/verification_integration_test.go]
- [x] [Review][Patch] TOCTOU batch↔курация: между просмотром auto-точки и `verified` batch может заменить геометрию — строка в ops-чек-лист «не запускать batch-прогон во время сессии курации» [docs/ops/geocoding.md]
- [x] [Review][Patch] deferred-work.md:321 (терминальный verified) закрыт кодом, но не помечен ✅ РЕШЕНО по конвенции репо [_bmad-output/implementation-artifacts/deferred-work.md:321]
- [x] [Review][Defer] Fail-fast секретов Directus в compose (`${VAR:?}` вместо тихих dev-дефолтов `dev-only-secret-change-me`/`change-me`) [deploy/docker-compose.cold.yml] — deferred, dev-дефолты санкционированы Task 4 (паттерн POSTGRES_*); ужесточение = ops-решение пилота
- [x] [Review][Defer] Аудит-след верификации (кто/когда поставил verified/wrong_reported; `ClearGeoObjectToUnmatched` перезаписывает geocoded_by виновной точки) — атрибуция SM-C2 [server/internal/store/queries/geo_objects.sql] — deferred, связано с Epic-3-проводом error_reports→wrong_reported

Dismissed (7, для следа): перезапись unmatched батчем (опровергнуто: `ListContractsForGeocode` выбирает только auto, CLI-сообщение верно); правка length_km при неизменном geom (санкционировано D2 — «явное значение уважать», seed жив); grants-тест «мусорит в БД» (опровергнуто: `underRole` = tx+rollback); down-миграция 0022 «оставляет промежуточное состояние» (goose оборачивает в tx + best-effort документирован); повторный wrong_reported=0 строк (no-op семантика, провод = Epic 3); Directus owner-DSN может регистрировать проекционные коллекции (D5-санкционировано + deferred:269); story-файл в File List отсутствует в патче (артефакт сборки ревью-диффа — спека исключена намеренно).

## Dev Notes

### Контекст: 3.2 = «писатель» для уже готовой схемы

Story 3.1 (`e31bd20` + `36f93d2`) построила канон: `districts`+`geo_objects` (миграция `0020`),
идемпотентный `UpsertGeoObject` c гейтом защиты `manual`, КАТО→район, batch `cmd/geocode`.
`docs/ops/geocoding.md:105-108` прямо фиксирует: Directus-курация = Story 3.2, «схема/гейт уже
на месте». 3.2 НЕ строит геокодер и НЕ трогает карту — она даёт оператору инструмент (Directus)
и вводит статусы верификации.

### ⚡ Несущая ценность (закрываемые долги)

1. **deferred-work.md:321 (из ревью 3.1):** `auto`-строки пере-геокодируются каждый прогон
   навсегда — нет терминального `verified`. 3.2 добавляет его в закрытый enum → повторные
   прогоны перестают бить по rate-limited Nominatim по подтверждённым контрактам.
2. **AR-28 / SM-C2:** статус `wrong_reported` делает «долю ошибочной геопривязки» вычислимой —
   фундамент контр-метрики юр-щита. Канал `error_reports(kind=geo_wrong_point)` УЖЕ пишется
   (Story 5.4) — авто-провод в `wrong_reported` остаётся Epic 3 (deferred-work.md:69), здесь
   только ручная пометка.
3. **Ручные LINESTRING дорог несут `length_km`** (через триггер D2) → флаг цена/км (4.3) и
   медиана района (6.4) работают и для вручную размеченных дорог (открытый пункт 3-1.md:208 закрыт).

### ⚠️ Что УЖЕ есть — переиспользовать, НЕ изобретать

- `migrations/0020_geo_objects.sql` — enum CHECK (`:39`), honesty-CHECK (`:40`), гранты
  `app_curator`/`app_importer` (`:48-49`). `0015_roles_grants.sql` — роли (NOLOGIN, идемпотентные).
- `queries/geo_objects.sql` — `UpsertGeoObject` (гейт `:31`, SQL-вывод length_km `:15-17`),
  `ListContractsForGeocode` (`:53-63`, только `auto` пере-выбирается), ESCAPE-LIKE паттерн.
- `store/curation/org_aliases.go` — КАНОНИЧЕСКИЙ прецедент очереди/резолва (2.3):
  `ListAliasesByStatus` + `ResolveAliasManually` («имитация Directus»). Зеркалить, не изобретать.
- `store/curation/geo_objects.go` — тонкая обёртка над sqlc (`gen.DBTX` = пул или tx); методы добавлять сюда.
- `cmd/geocode/idempotency_integration_test.go` — паттерн integration-приёмки с мок-геокодером.
- `store/projection/grants_integration_test.go` — паттерн проверки грантовой границы.
- `deploy/docker-compose.cold.yml:15-18` — скелет сервиса `directus` (профиль уже объявлен);
  `deploy/directus/` — пустой (`.gitkeep`), ЖДЁТ snapshot/README.
- Seed: `fixtures/seed/geo_objects.sql` — район «Есиль» (bbox, kato `710000000`) + 6 auto-LINESTRING
  (length 5.0/6.0 подобраны под медиану ~42M ₸/км — НЕ ломать).
- **НЕ создавать (чужие истории):** карта-читатель bbox (3.4 — `store/geo.go.ListGeoObjectsInBBox`
  уже ждёт), гейт покрытия ≥70% (3.3), авто-провод error_reports→wrong_reported (Epic 3, deferred:69),
  реальные районы/КАТО-полигоны (0.1 — в `districts` только синтетический Есиль), role-DSN разводка
  LOGIN/паролей (deferred:269 — ops/2.2), `/metrics`-экспорт SM-C2 (NFR-4-провод), фронт-метка
  «без точки на карте» (3.4/3.6).

### Файлы и куда писать

| Что | Где |
|---|---|
| Миграция enum+триггер | `migrations/0022_geo_verification.sql` (следующий номер после 0021) |
| SQL-запросы курации | `server/internal/store/queries/geo_objects.sql` (+ `make gen-sqlc`) |
| Кураторские методы | `server/internal/store/curation/geo_objects.go` (+тесты рядом) |
| Приёмка AR-10 | `server/cmd/geocode/` (расширить idempotency-тест) и/или curation-пакет |
| Compose/env | `deploy/docker-compose.cold.yml`, `deploy/.env.example` |
| Схема коллекций | `deploy/directus/snapshot.yaml` + `deploy/directus/README.md` |
| Ops-док | `docs/ops/geocoding.md` (расширить секцией курации — когезия, прецедент 0.9) |
| Seed | `fixtures/seed/geo_objects.sql` (идемпотентно, существующие строки не трогать) |
| Integration-группы | `Makefile:60-61` (добавить curation-пакет в CLEAN) |

### Соблюдение архитектуры / гардрейлы

- **AR-4/AR-10** (epics.md:197-199, 215-217): проекционные ⊥ кураторские; в `geo_objects` пишут
  только геокодер и Directus; правка переживает перезапись. Приёмка «два прогона + курация между».
- **AR-21/AR-24** (epics.md:260-262, 269-271): Directus — холодный профиль, «поднимать на время
  курирования»; ноль лишних 24/7-сервисов; секреты — env вне репо; Directus без write к проекционным.
- **AR-28** (epics.md:286-289): статус верификации на `geo_objects` питает SM-C2.
- **Честность (§7.4 prd.md:328-330, CHECK 0020:37-40):** never-фейк-координата — auto/manual/verified/
  wrong_reported несут геометрию, unmatched — никогда; платформа не «достраивает» точки. Оператору
  это правило — в чек-лист (ops-док).
- **Нейтральность:** операторский контур не производит публичной прозы — taboo-лексикон не задет;
  `check-registry` остаётся зелёным без новых строк.
- **AR-3-решение владельца (3-1.md:86):** явные `ST_*`-касты; sqlc `overrides` для geometry НЕ заводить.
- **Directus = внутренняя админка** (architecture.md:772): НЕ за Caddy, порт только на loopback,
  доступ SSH-туннелем; экспозиция при пилоте — отдельное ops-решение.

### Тестирование / проверка готовности

- Гейты репо: `make build lint test` (`-count=1`), `check-core` (go-list границы),
  `check-registry`, `gen-sqlc` идемпотентен (docker), `test-integration-clean` / `test-integration-seed`.
- Integration-энв: БД compose на `POSTGRES_PORT=55432` (5432 занят ЧУЖИМ Postgres — memory
  local-dev-docker-env); postgis initdb двухфазный — ждать стабильного `pg_isready`.
- Урок 3.1: **оба реальных бага 3.1 поймали только integration-тесты на реальном Postgres**
  (ON CONFLICT partial-index предикат; изоляция от persistent-seed) — юнитов недостаточно,
  REJECT-стражи гонять на реальной БД.
- Стражи обязаны доказать, что краснеют (memory guards-must-prove-red): negative-control для
  каждого нового CHECK/гейта.
- Playwright/web НЕ задет (фронта в 3.2 нет) — ci-web должен остаться зелёным без изменений.

### Previous Story Intelligence (3.1 — прямой предшественник)

- Транспортная ошибка ≠ `unmatched`: инфраструктурный сбой НЕ пишет строку (retryable, main.go:62-66);
  `unmatched` = «геокодер ответил, но не нашёл». НЕ размывать эту семантику новыми статусами.
- Goose-parser ловушка: буквальный текст директивы `+goose NO TRANSACTION` в prose-комментарии
  миграции ломает `migrate-up` (0021:13-15) — в 0022 не воспроизводить.
- `geocoded_by` — свободный TEXT без CHECK (конвенция `'nominatim'|'directus'|'synthetic-seed'`),
  провенанс держится дисциплиной: в Directus — default-значение поля, в Go-методах — параметр.
- `confidence` честно NULL (`*float64`), не выдуманный 0.0 — при верификации НЕ трогать.
- `districts` заселён на 1/5 (только Есиль): `FindDistrictIDByPoint` для остальных вернёт
  ErrNoRows → district_id NULL честно. НЕ баг 3.2.
- Роли инертны в рантайме (app коннектится owner-ролью, 0015:9-10) — граница доказывается
  grants-тестами, а не рантаймом. Для Directus дополнительная страховка D5: только кураторские коллекции.
- Дубли `contract_id` (deferred-work.md:324): Directus — первый писатель «мимо `UpsertGeoObject`»,
  но частичный уникальный индекс `geo_objects_contract_uniq` (0021) отвергнет дубль на уровне БД —
  честная ошибка в UI. В чек-лист оператора: редактировать СУЩЕСТВУЮЩИЕ строки очереди, не создавать новые.

### Project Structure Notes

- Go-модуль `ashyqqala/server` в `server/`; sqlc через docker (`make gen-sqlc`, Makefile:31-36);
  миграции goose `NNNN_*.sql` в `migrations/` (репо-корень).
- `deploy/`: `docker-compose.yml` (hot: db/migrate/seed/api/caddy, профиль `app`),
  `docker-compose.cold.yml` (профили `directus`/`geocode`/importer/bot), `Caddyfile` (НЕ трогать),
  `directus/` (конфиг-каталог, сейчас пустой).
- Языки/стиль: комментарии и доки — русский; текущий стиль соседних файлов соблюдать.

### Внешние знания / web-research (проверить при реализации, не доверять слепо)

- Directus 11 (образ `directus/directus`): миноры на Docker Hub наблюдаются ≥11.12 (июль 2026);
  «пин на минор» (architecture.md:306) — зафиксировать актуальный и свериться с changelog
  (в районе 11.16 упоминались breaking changes). Источники: hub.docker.com/r/directus/directus,
  directus.io/docs/releases/changelog.
- Каркас env для bootstrap: `SECRET` (обязателен), `ADMIN_EMAIL`/`ADMIN_PASSWORD` (первичный админ),
  `DB_CLIENT=pg`, `DB_HOST/DB_PORT/DB_DATABASE/DB_USER/DB_PASSWORD`. Directus поверх СУЩЕСТВУЮЩЕЙ
  Postgres НЕ активирует таблицы сам — коллекции подключаются явно (это и есть желаемая граница D5).
- PostGIS `geometry` поддерживается map-интерфейсом (рисование POINT/LINESTRING в UI). Схема
  коллекций: `directus schema snapshot` / `schema apply`. Точные имена команд/интерфейсов —
  по docs.directus.io (MCP Context7 доступен из сессии).

## Открытые вопросы / решения владельцу (дефолты — дев следует им, если не отклонены)

- **D1 (ось статусов):** одна ось — расширить закрытый enum `geocode_status` до
  `{auto, manual, unmatched, verified, wrong_reported}`. Обоснование: deferred-work.md:321 прямо
  планирует «терминальный статус в закрытый enum»; data_model:49 второго поля не знает; происхождение
  хранит `geocoded_by`. Переходы: `auto→verified` (подтвердил), `auto|manual|verified→wrong_reported`
  («точка не там»), `wrong_reported→manual|unmatched` (перерисовал/снял), `unmatched→manual` (поставил).
  Планинг-рассинхрон (AC1 говорит `manual`, AC3 — `verified`) разрешается так: ПОДТВЕРЖДЕНИЕ
  авто-точки = `verified`, КОРРЕКЦИЯ/рисование = `manual`. Альтернатива (отдельная колонка
  `verification_status`) — чище оси, но дороже каскад и против deferred:321.
- **D2 (length_km при прямой записи Directus):** BEFORE-триггер в 0022 (вычислять для LINESTRING
  при NULL/изменённом geom; явное значение при неизменном geom уважать — seed жив). Альтернативы:
  GENERATED COLUMN (отвергнут — 3-1.md:208), Directus Flow-хук (хрупко), «писать только через Go»
  (обесценивает Directus-поверх-БД).
- **D3 (очередь):** пресет/фильтр коллекции `geo_objects` в Directus (ноль новой схемы) +
  Go-метод `ListGeoObjectsByStatus` для тестов/будущего провода. Альтернатива: SQL VIEW.
- **D4 (экспозиция):** `127.0.0.1:8055` + SSH-туннель; НЕ за Caddy (внутренняя админка,
  architecture.md:772; AR-21 «ноль лишних 24/7»). Прод-экспозиция — отдельное ops-решение при пилоте.
- **D5 (подключение к БД):** в dev/S-0 Directus ходит owner-DSN (как всё приложение, 0015:9-10);
  граница держится (а) только кураторскими коллекциями в Directus, (б) grants-тестами
  (`app_curator` пишет / `app_importer` читает geo-таблицы). Полная role-DSN разводка
  (LOGIN/пароли/membership) — остаётся deferred-work.md:269 (ops/2.2), в 3.2 НЕ раздувается.
- **D6 («публикует»):** публикация = смена статуса строки (compute-on-read: карта 3.4 / медиана 6.4
  увидят сразу); пересчёт `price_benchmarks`/флага 4.3 — на следующем `RecalcBenchmarks`
  (AR-9-конвейер), немедленный пересчёт НЕ триггерится; зафиксировать в ops-доке.

## Dev Agent Record

### Agent Model Used

Claude Fable 5 (claude-fable-5), 2026-07-02.

### Debug Log References

- **RED→GREEN доказан по-настоящему:** новые integration-тесты прогнаны ДО применения 0022 (схема 0021 на dev-БД) — падали по правильным причинам (`geo_objects_status_chk` отвергал verified/wrong_reported; триггера нет → length не пересчитывался); после `migrate-up` — все зелёные.
- **Пойман дефект собственного теста на RED-прогоне:** `insertContract` (харнесс 3.1) хардкодит одинаковый `subject_ru` → у всех контрактов приёмки AR-10 был ОДИН addr()-ключ, и C матчился «за компанию». Фикс: уникализация `subject_ru` per-контракт в тесте.
- **Живая Directus-верификация (PG-истина):** PATCH id=77 (LINESTRING, без length) → `manual/directus/length_km=3.0599` (триггер), confidence NULL; PATCH id=24 → `verified`, провенанс `synthetic-seed` и явный length 5.0 сохранены; откат к seed-состоянию — триггер обнулил length при geom=NULL.
- **Env-ловушки, пойманные живьём:** (1) compose без `POSTGRES_PORT=55432` попытался пересоздать db на занятом 5432 (чужой Postgres) — всегда передавать override; (2) `ADMIN_EMAIL=admin@ashyqqala.local` не прошёл email-валидацию Directus (FAILED_VALIDATION) → `.local`-домены нельзя, дефолт заменён на `admin@example.com`; (3) упавший первый bootstrap оставил `directus_*`-таблицы БЕЗ админа (повторный бут пропускает createAdmin) → лечение: DROP всех `directus_*` + чистый ре-бут (зафиксировано в README).
- Финальные гейты: clean-группа на СВЕЖЕЙ БД (throwaway postgis, 0001-0022), seed-группа на свежей засеянной; обе зелёные. `gen-sqlc` идемпотентен (md5 двух прогонов идентичны).

### Completion Notes List

- **AC1 MET:** очередь (закладка Directus поверх geo_objects) → оператор рисует POINT|LINESTRING → `manual` с сохранением геометрии; «переживает ре-импорт» доказано адаптацией AR-10 «прогон→курация→прогон» (`TestAcceptance_AR10_BatchCurationBatch`) + живым Directus-путём записи. Гейт batch-перезаписи расширен: `IN ('auto','unmatched')`.
- **AC2 MET:** «без точки на карте» = `unmatched` (geom NULL, honesty-CHECK); seed DEMO-GEO-07 даёт непустую очередь; доступность в поиске/карточке закреплена стражем `TestUnmatchedContract_VisibleInSearchAndCard_Integration`. Публичная МЕТКА на карте/карточке — 3.4/3.6 (фронта в 3.2 нет, по плану).
- **AC3 MET:** статусы верификации `verified`/`wrong_reported` в закрытом enum (D1, одна ось; миграция 0022) фиксируются на geo_objects; SM-C2 вычислим SQL'ом; `/metrics`-экспорт — при NFR-4-проводе (дескоуп, честно).
- **Закрыт долг deferred-work.md:321:** `verified` терминален — подтверждённые контракты не пере-геокодируются (не жгут rate-limit Nominatim).
- **Закрыт открытый пункт 3-1.md:208 (length_km при прямой записи):** BEFORE-триггер 0022 — ручные LINESTRING автоматически несут длину → цена/км (4.3) и медиана (6.4) работают для ручных дорог; явный seed-length уважается (GENERATED COLUMN не нужен).
- **Инфра:** Directus 11.17 (пин на минор) как cold-профиль, только loopback (D4), только кураторские коллекции (D5); bootstrap.sh идемпотентен; ops-док с чек-листом оператора и таблицей переходов.
- **Адаптации против плана истории (обоснованные):** (1) integration-тесты курации живут в `cmd/geocode` (переиспользован харнесс) — Makefile-группы не менялись; (2) вместо `schema snapshot` — API-bootstrap-скрипт (schema принадлежит goose); (3) `geocoded_by` default в Directus-UI не ставится (default работает только на INSERT, куратор правит существующие строки) — провенанс держат чек-лист и Go-методы.
- **Дескоуп (честно, чужие истории):** карта-читатель (3.4), гейт покрытия (3.3), авто-провод error_reports→wrong_reported (Epic 3), реальные районы/КАТО (0.1), role-DSN разводка (deferred:269 / ops 2.2), /metrics SM-C2 (NFR-4).

### File List

- migrations/0022_geo_verification.sql (новый)
- server/internal/store/queries/geo_objects.sql (изменён: гейт UPSERT + 5 запросов курации)
- server/internal/store/gen/geo_objects.sql.go (перегенерирован)
- server/internal/store/gen/querier.go (перегенерирован)
- server/internal/store/curation/geo_objects.go (изменён: +5 кураторских методов, коммент границы)
- server/cmd/geocode/main.go (изменён: сообщение «нет контрактов» упоминает курацию)
- server/cmd/geocode/verification_integration_test.go (новый: стражи 0022 + переходы + приёмка AR-10)
- server/internal/store/projection/grants_integration_test.go (изменён: +TestRoleGrants_GeoCurationBoundary)
- server/internal/httpapi/geo_unmatched_visibility_integration_test.go (новый: страж AC2)
- deploy/docker-compose.cold.yml (изменён: сервис directus наполнен)
- deploy/.env.example (изменён: DIRECTUS_*)
- deploy/directus/bootstrap.sh (новый, исполняемый)
- deploy/directus/README.md (новый)
- fixtures/seed/geo_objects.sql (изменён: DEMO-GEO-07 unmatched + коммент триггера)
- docs/ops/geocoding.md (изменён: секция «Ручная курация через Directus» + фикс устаревшей ссылки на гейт)
- _bmad-output/implementation-artifacts/3-2-полуручная-разметка-через-directus-fr-5.md (story)
- _bmad-output/implementation-artifacts/sprint-status.yaml
- _bmad-output/implementation-artifacts/deferred-work.md (изменён ревью: :321 ✅ РЕШЕНО + 3 новых defer-записи 3.2)

## Change Log

| Дата | Описание |
|---|---|
| 2026-07-02 | create-story: контекст собран 3 параллельными Explore-агентами (планинг/архитектура/код) + сверка Makefile/grants/миграций. Story = «писатель» для готовой схемы 3.1; дефолты D1–D6 сформулированы; токен-независима полностью. Status: ready-for-dev. |
| 2026-07-02 | dev-story (baseline `e31bd20`, дефолты D1–D6 приняты владельцем запуском dev). Доставлено: миграция `0022` (enum +verified/wrong_reported, честность geom⟺unmatched цела, BEFORE-триггер length_km для прямой записи Directus); гейт `UpsertGeoObject`→`IN('auto','unmatched')`; 5 кураторских запросов/методов (`:execrows`, запрещённые переходы=0 строк); стражи RED→GREEN (прогнаны до/после 0022); приёмка AR-10 «прогон→курация→прогон» через реальный geocodeContracts; grants-страж гео-границы; Directus 11.17 cold-профиль (loopback, только кураторские коллекции) + идемпотентный `bootstrap.sh` + README + ops-секция курации; seed DEMO-GEO-07 (непустая очередь); страж AC2 (unmatched виден в поиске/карточке). **Живая верификация через Directus REST (тот же путь, что UI):** LINESTRING→manual с триггер-выведенным length_km=3.06; auto→verified с сохранением провенанса. Закрыты долги deferred:321 (терминальный verified) и 3-1:208 (length_km сырого писателя). Все гейты зелёные (unit/lint/check-core/check-registry/gen-sqlc-md5/clean+seed integration на СВЕЖЕЙ БД/compose config/web build). Адаптации: тесты в cmd/geocode (Makefile не тронут), bootstrap.sh вместо schema snapshot (schema принадлежит goose). Status → review. |
| 2026-07-02 | code-review (3 слоя Blind/Edge/Acceptance по диффу против `e31bd20`): вердикты AC1 MET / AC2 PARTIAL→MET (страж расширен) / AC3 MET; триаж 1 decision / 11 patch / 2 defer / 7 dismiss. Decision владельца: БД-гард переходов на Directus-пути ОТЛОЖЕН до живого пилота (дисциплина чек-листа, hardening → deferred-work, бандл с role-DSN). **11 патчей применены+верифицированы:** P1 статус-гард `ResolveGeoObjectManually` (verified→manual запрещён, двухшаговый след D1) + negative-тест; P2–P5 закалка bootstrap.sh (jq --arg логин + пароль вне argv; честная ошибка при недоступном Directus; `curl -f` в api() + валидация found против дублей закладок; явные user/role=null; гард curl/jq); P6 NULLIF в триггере 0022 и Upsert (вырожденная LINESTRING → NULL, не «0 км») + тест; P7 CHECK `geo_objects_geom_type_chk` (только POINT|LINESTRING, AC1) + negative-тест; P8 страж AC2 расширен третьей поверхностью (ListContracts); P9 нетронутый unmatched в приёмке AR-10 (контракт E — не пере-выбирается); P10 ops-чек-лист: запрет одновременного batch+курации (TOCTOU) + правка таблицы переходов; P11 deferred:321 помечен ✅ РЕШЕНО. Гейты после патчей зелёные: build/vet/gofmt/unit `-count=1`; lint; check-core; check-registry; gen-sqlc идемпотентен (md5×2); integration CLEAN+SEED на СВЕЖИХ БД 0001-0022 (throwaway postgis). Dev-БД (55432) синхронизирована дельтой правленой 0022 (CHECK+NULLIF), seed цел, погашена обратно. Status → done. |

## References

- epics.md:1242-1260 (Story 3.2), :82-84 (FR-5), :578-586 (Epic 3), :197-199 (AR-4), :215-217 (AR-10), :260-271 (AR-21/24), :286-289 (AR-28), :1187-1189 (граница 2.6), :1230-1236 (Story 3.1)
- prd.md:122-127 (FR-5), :62-63 (UJ-4), :113-114 (§5.2), :328-330 (§7.4 честность), :337 (Directus-интеграция); addendum.md:10
- architecture.md:306-313 (Directus:11 контур), :466-467 (cold-профили), :752-754 (compose/directus/), :771-773 (внутренняя админка), :893 (auto/verified/wrong_reported), :100 (контур секретов)
- docs/AshyqQala_MVP_data_model_and_flags_v1.md:49 (geo_objects)
- ux-designs/.../EXPERIENCE.md:38-39, :80, :255 ({data-state-ungeocoded}), :493-497 (операторский поток вне Key Flows)
- Код: migrations/0015, 0020 (:37-49), 0021 (:13-16); server/internal/store/queries/geo_objects.sql; server/internal/store/curation/{geo_objects,org_aliases}.go; server/cmd/geocode/main.go (:62-66, :96-160); server/internal/store/geo.go; server/internal/store/projection/grants_integration_test.go; deploy/docker-compose.cold.yml (:15-18); deploy/Caddyfile; docs/ops/geocoding.md (:105-113); Makefile (:31-36, :60-69); fixtures/seed/geo_objects.sql
- deferred-work.md:69-70 (error_reports→wrong_reported, гранты), :269 (role-DSN ops), :321 (терминальный verified), :324 (0021 дубли), :326 (district_id для 3.4)
- 3-1-автоматическая-геопривязка-batch-nominatim.md:86 (AR-3 решение), :98, :103, :208 (length_km открытый пункт)
- Внешние: hub.docker.com/r/directus/directus (миноры 11.x), directus.io/docs (env/schema snapshot/map-интерфейс)
