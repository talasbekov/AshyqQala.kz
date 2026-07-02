# Канонический геокодинг (Story 3.1) и ручная курация (Story 3.2)

> **Статус:** КАНОН (не интерим — в отличие от `interim-scrape-bridge.md`). Реализовано в
> `server/cmd/geocode` (batch-команда) + `server/internal/geo` (клиент Nominatim) +
> `server/internal/store/curation/geo_objects.go` (запись + кураторские методы). Пишет в канонические
> таблицы `districts`/`geo_objects` (миграции `0020` + `0021` + `0022_geo_verification.sql`).
> Ручная разметка/верификация — Directus (cold-профиль `directus`), см. секцию
> «Ручная курация через Directus» ниже.

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
- ~~**Directus-курация `manual`/verification** (`auto`/`verified`/`wrong_reported`, AR-28) — Story 3.2.~~
  **РЕАЛИЗОВАНО Story 3.2** (миграция `0022`, секция «Ручная курация через Directus» ниже). Гейт
  batch-перезаписи расширен: `WHERE geo_objects.geocode_status IN ('auto','unmatched')` — курация
  (`manual`/`verified`/`wrong_reported`) неприкосновенна для batch'а.
- **Карта (маркеры/кластеры/линии/превью/список-фолбэк/a11y)** — Story 3.4/3.5/3.6/3.8. `cmd/geocode`
  только наполняет `geo_objects`; чтение для UI — отдельные истории.
- **Реальные полигоны районов Астаны (OSM)** — сейчас в `districts` только синтетический bbox
  («Есиль», сид `fixtures/seed/geo_objects.sql`); остальные 4 района и подтверждённые КАТО-коды
  ждут Story 0.1 (`/search/getKato`, токен).

## Ручная курация через Directus (Story 3.2)

Полуручная разметка FR-5: оператор подтверждает/корректирует точку или полилинию в **Directus** —
внутренней админке поверх той же Postgres. Directus видит ТОЛЬКО кураторские коллекции
(`geo_objects`, `districts` — read-only справочник); проекционные таблицы в админку не заводятся
(граница AR-4; страж `internal/store/projection/grants_integration_test.go`).

### Поднять / погасить (только на время курирования — AR-21)

```bash
# db должен быть жив (docker-compose.yml). POSTGRES_PORT — как у вашего db (локально часто 55432).
POSTGRES_PORT=55432 docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.cold.yml \
  --profile directus up -d directus

# Первый бут создаёт системные таблицы directus_* в той же БД (вне goose — НЕ дрейф миграций).
# Затем один раз настроить коллекции/очередь (идемпотентно):
deploy/directus/bootstrap.sh

# Погасить после сессии курирования:
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.cold.yml --profile directus stop directus
```

Доступ: `http://127.0.0.1:8055` (порт ТОЛЬКО loopback, НЕ за Caddy — решение D4). На VPS —
SSH-туннель: `ssh -L 8055:127.0.0.1:8055 <vps>`. Логин/пароль — `DIRECTUS_ADMIN_EMAIL`/
`DIRECTUS_ADMIN_PASSWORD` (`deploy/.env`; дефолты dev-only).

### Очередь и чек-лист оператора

Очередь — закладка **«Очередь геопривязки»** коллекции `geo_objects` (пресет: `geocode_status` ∈
`unmatched`, `wrong_reported`); закладка **«Верификация»** — `auto`-кандидаты, сомнительные первыми
(confidence по возрастанию). Правила (зеркало enum 0022 и переходов `store/curation/geo_objects.go`):

| Действие оператора | Переход | Как в Directus |
|---|---|---|
| Подтвердить верную авто-точку | `auto` → `verified` | сменить статус (геометрию НЕ трогать) |
| Скорректировать/нарисовать точку или линию дороги | `auto`/`manual`/`unmatched`/`wrong_reported` → `manual` | править `geom` (POINT — объект; LINESTRING — дорога) + статус `manual` + `geocoded_by='directus'`. Перерисовать `verified` напрямую нельзя — сначала пометить `wrong_reported` (двухшаговый след SM-C2) |
| Пометить «точка не там» | `auto`/`manual`/`verified` → `wrong_reported` | сменить статус; спорная геометрия ОСТАЁТСЯ (факт, не подмена) |
| Снять неверную точку | `wrong_reported` → `unmatched` | очистить `geom` + статус `unmatched` — объект честно «без точки на карте» |

**Запрет фабрикации (§7.4 PRD):** не ставить точку «примерно/на глаз» — честный `unmatched` лучше
выдуманной координаты. БД сама отвергнет ложь: `unmatched` с геометрией и `auto/manual/verified/
wrong_reported` без геометрии невозможны (CHECK `geo_objects_geom_null_chk`).

**Провенанс:** при ручной правке ставить `geocoded_by='directus'` (свободный TEXT — БД не заставит,
это дисциплина чек-листа; Go-методы курации проставляют сами). `geocoded_at` обновлять не нужно.

**Не создавать новые строки** для контрактов, у которых гео-строка уже есть — частичный уникальный
индекс `geo_objects_contract_uniq` (0021) отвергнет дубль `contract_id`. Работайте с существующими
строками очереди.

**`length_km` не заполнять руками:** для LINESTRING длина выводится триггером `0022`
(`ST_Length(geom::geography)/1000`) при рисовании/перерисовке линии — ручные дороги автоматически
попадают в цену/км (флаг 4.3) и медиану района (6.4). Только `POINT`/`LINESTRING`: другие типы
(MULTILINESTRING/POLYGON) БД отвергнет (CHECK `geo_objects_geom_type_chk`) — дорогу из нескольких
сегментов рисовать одной линией.

**Не запускать batch-прогон `cmd/geocode` во время сессии курации:** batch легально перезаписывает
`auto`-строки — между просмотром авто-точки и кликом «verified» геометрия могла смениться, и
подтверждённой оказалась бы точка, которую человек не видел. Сначала прогон, потом курация (оба —
холодные разовые процессы, AR-21/22, одновременно им работать незачем).

### Что даёт верификация (AR-28 → SM-C2)

`verified` терминален для batch: подтверждённые контракты перестают пере-геокодироваться каждым
прогоном (и жечь rate-limit публичного Nominatim). `wrong_reported` делает SM-C2 («доля ошибочной
геопривязки») вычислимым: `SELECT count(*) FILTER (WHERE geocode_status='wrong_reported') … FROM
geo_objects`. Экспорт в `/metrics` — при появлении metrics-провода (NFR-4). Автопровод канала
«точка не там» (`error_reports.kind='geo_wrong_point'`, Story 5.4) в `wrong_reported` — Epic 3,
позже; до него пометку ставит куратор вручную по содержимому `error_reports`.

### «Публикация» (решение D6)

Публикация = смена статуса строки: публичные читатели (`store/geo.go` bbox для карты 3.4, медиана
района 6.4) читают `geo_objects` напрямую — ручная правка видна сразу. Пересчёт `price_benchmarks`
(флаг цена/км 4.3) происходит на следующем `RecalcBenchmarks` (конвейер AR-9), немедленный пересчёт
кураторской правкой НЕ триггерится.

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
