# Канонический геокодинг (Story 3.1)

> **Статус:** КАНОН (не интерим — в отличие от `interim-scrape-bridge.md`). Реализовано в
> `server/cmd/geocode` (batch-команда) + `server/internal/geo` (клиент Nominatim) +
> `server/internal/store/curation/geo_objects.go` (запись). Пишет в канонические таблицы
> `districts`/`geo_objects` (миграция `0020_geo_objects.sql` + `0021_geo_objects_contract_uniq.sql`).

## Что это

Эфемерный batch-геокодер (AR-22: **не 24/7**, не часть горячего пути): на каждый прогон читает
контракты без геопривязки (или с устаревшей `auto`), геокодит адрес (`subject_ru`) через Nominatim,
присваивает район по КАТО и кэширует результат в `geo_objects`. После прогона контейнер/процесс
завершается — Nominatim после batch не дёргается повторно (AR-6).

Поток: `contracts` (нет `geo_object` ИЛИ `geocode_status=auto`) → `internal/geo.GeocodeMatch` →
`resolveDistrict` (КАТО) → `curation.GeoObjectStore.UpsertGeoObject` (идемпотентно по `contract_id`).

## AC3 — Self-host vs managed (единый однопроходный путь)

Решение владельца (Bratan, architecture.md) — **self-host Nominatim** как целевой путь; **managed
геокодер** — задокументированный fallback, если self-host ops слишком тяжёл для команды без DevOps.

### Self-host (целевой путь)

```bash
# 1) osmium-extract региона (Астана bbox) из полного OSM-дампа Казахстана/Азии — вне scope этого репо,
#    делается один раз офлайн/на build-машине (не в горячем пути).
# 2) Импорт в mediagis/nominatim (урезанный IMPORT_STYLE — только адреса, не полный POI-набор):
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.cold.yml --profile geocode up nominatim
#    Реальная стоимость (architecture.md): импорт 30–90 мин, диск 8–20 ГБ, RAM на импорте ≥8 ГБ,
#    ~2 чел-дня на первый зелёный импорт. Требует живой объём контрактов (токен) для окупаемости.

# 3) Прогон batch-геокода против self-host инстанса (без публичной паузы — свой Nominatim):
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.cold.yml --profile geocode up geocode
# ИЛИ локально:
DATABASE_URL=postgres://... NOMINATIM_URL=http://localhost:8080 go run ./cmd/geocode
```

### Managed / публичный Nominatim (fallback, практический путь MVP/демо)

Без `NOMINATIM_URL` команда обращается к публичному `nominatim.openstreetmap.org` (политика ≤1
запрос/сек, `geo.PoliteDelayDefault`=1100мс между запросами, `geo.AstanaViewbox` сужает поиск).
Это **однопроходный путь**: batch-геокодинг → координаты в `geo_objects` → в рантайме внешней
зависимости **ноль** (Nominatim не нужен между batch-прогонами).

```bash
DATABASE_URL=postgres://... go run ./cmd/geocode
# -max=N ограничивает прогон (вежливость к публичному сервису на большом объёме)
DATABASE_URL=postgres://... go run ./cmd/geocode -max=200
```

На синтетике/демо-объёме (несколько десятков-сотен контрактов) managed-путь достаточен и не требует
self-host ops-инвестиции. Managed-провайдер (коммерческий, если публичный Nominatim не подходит по
объёму/SLA) подключается тем же кодом — просто другой `NOMINATIM_URL` + `-max` под лимиты провайдера.

### Честная граница (не гейт)

Автопокрытие, которое печатает `cmd/geocode` (`stdout`, «автопокрытие=N%»), — **ФАКТ прогона**, НЕ
вердикт FR-6/OQ-4/SM-2. Живой ≥70%-Go/No-Go гейт считается на **реальных данных Астаны** после
получения `GOSZAKUP_TOKEN` — это Story 3.3 (`serverный гейт автопокрытия`) и
`docs/AshyqQala_stage0_data_audit_runbook_v1.md`. До токена цифра из `cmd/geocode` не подтверждает и
не опровергает готовность к запуску — она честно описывает только текущий синтетический/пилотный
прогон.

## Шов: `interim_geo_lots` → `geo_objects` (Task 5)

Интерим (`server/tools/scrape/cmd/interim-geocode`, Story 0.7, за `//go:build scrape`) пишет
lot-keyed результаты в `interim_geo_lots` (миграция `0004`) — временная таблица трека «Парсер-мост»,
существует **до получения токена**. Канон (`cmd/geocode`, эта команда) пишет **contract-keyed**
результаты в `geo_objects`. Это две РАЗНЫЕ таблицы с разными ключами — они не путаются и не
пересекаются автоматически.

**Почему нет прямого миграционного скрипта interim→canonical сейчас:** перенос строки
`interim_geo_lots` (ключ `goszakup_lot_id`) в `geo_objects` (ключ `contract_id`) требует join
`lots ↔ contracts`, которого **не существует в схеме** — миграция `0002_projection.sql` явно
откладывает `contracts.lot_id` до появления живого импорта (комментарий-TODO в файле: «когда
появятся organizations/lots — B-3/Epic 2»). Писать перенос против несуществующего join означало бы
выдумывать сопоставление лотов контрактам — прямое нарушение принципа честности. Скрипт появится
вместе с этим join (Epic 2, токен), не раньше.

**Триггер закрытия интерима** (критерий удаления, зеркало `interim-scrape-bridge.md`): получен
`GOSZAKUP_TOKEN` → живой импорт (Story 2.2) даёт `contracts.lot_id` → тогда:

1. Одноразовый перенос `interim_geo_lots` → `geo_objects` по join `lots.goszakup_lot_id =
   interim_geo_lots.goszakup_lot_id`, `lots.id → contracts.lot_id → contracts.id`
   (`geocode_status`/`confidence`/`address_text` копируются как есть; `geocoded_by='interim-nominatim'`
   как провенанс-пометка для отличия от нового canon-прогона).
2. `interim_geo_lots` (0004) и `server/tools/scrape/cmd/interim-geocode` удаляются (как и весь трек
   «Парсер-мост» — см. `interim-scrape-bridge.md`).
3. `cmd/geocode` (эта команда) становится единственным источником геопривязки; `internal/geo`
   (Nominatim-клиент) не меняется — он уже переиспользуется обоими путями без модификации.

До этого момента интерим и канон работают **параллельно и независимо**: `/api/lots` (Story 0.8,
ранняя карта) читает `interim_geo_lots`; будущая карта на канонических данных (Story 3.4) будет
читать `geo_objects` через `store/geo.go` (Task 6).

## Дескоуп (что НЕ входит в Story 3.1)

Честно не входит в эту историю — не выдумывать, не имитировать:

- **Живой ≥70%-вердикт** (FR-6/OQ-4/SM-2) — Story 3.3, на реальных данных Астаны (токен).
- **Реальные `length_km`-значения для существующих дорожных контрактов** — появятся при живом объёме
  (Epic 2/токен); формула (`ST_Length(geom::geography)/1000` для `LINESTRING`) уже реализована в
  `UpsertGeoObject` и протестирована на синтетике (`cmd/geocode/idempotency_integration_test.go`).
- **Directus-курация `manual`/verification** (`auto`/`verified`/`wrong_reported`, AR-28) — Story 3.2.
  `geo_objects.geocode_status='manual'` уже честно защищён от перезаписи batch'ем (см. `UpsertGeoObject`
  SQL: `WHERE geo_objects.geocode_status IS DISTINCT FROM 'manual'`) — Directus просто начнёт писать
  эти строки, схема/гейт уже на месте.
- **Карта (маркеры/кластеры/линии/превью/список-фолбэк/a11y)** — Story 3.4/3.5/3.6/3.8. `cmd/geocode`
  только наполняет `geo_objects`; чтение для UI — отдельные истории.
- **Реальные полигоны районов Астаны (OSM)** — сейчас в `districts` только синтетический bbox
  («Есиль», сид `fixtures/seed/geo_objects.sql`); остальные 4 района и подтверждённые КАТО-коды
  ждут Story 0.1 (`/search/getKato`, токен).

## Гейты / проверка готовности

```bash
make build                       # go build (включая cmd/geocode) + npm build
make test                        # go test ./... (unit, включая internal/geo + cmd/geocode фейки)
make test-integration-clean      # geometry round-trip, идемпотентность, honesty-CHECK'и, length_km-вывод
                                  # (cmd/geocode/... — в CLEAN-группе, самодостаточен, БД без seed ок)
make check-core                  # go-list-границы (median/flags/normalize/benchmark не задеты)
```

`internal/arch.TestHotPathDoesNotImportScrape` не касается `cmd/geocode` — это отдельный, не «горячий»
бинарь, физически вне `tools/scrape` (не импортирует парсер и не входит в защищаемый список
`cmd/api`/`cmd/importer`).
