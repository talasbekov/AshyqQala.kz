---
baseline_commit: 5cc6ad99ffc5c882db5708324027d871bbebf5ba
---

# Story 3.1: Автоматическая геопривязка (batch-Nominatim)

Status: in-progress

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

> **Тип истории:** Epic 3, первая история (карта/геопривязка). **КЛЮЧЕВОЙ ВЫВОД РАЗВЕДКИ: 3.1 — на 100%
> МЕХАНИЗМ, 0% ≥70%-вердикта.** Ни один из 3 AC не содержит порога/Go-No-Go/≥70% — это чистый пайплайн +
> канонические таблицы. Порог ≥70% живёт ВНЕ 3.1 (FR-6/SM-2/OQ-4/**Story 3.3**/stage0-runbook), весь за
> токеном. Значит 3.1 **полностью токен-независима** по прецеденту 2.0/4.1/4.3/5.1/6.4/0.7 (строим механизм на
> синтетике/scraped, живой вердикт честно дескоупим). **Бонус-разблокировка:** 3.1 вводит `geo_objects.length_km`
> — ровно ту колонку, которую УЖЕ ждут построенные движки флага «цена/км» (Story 4.3) и медианы района (Story
> 6.4); их швы (`RecalcBenchmarks`→пустой снапшот; `PricePerKMSamples`→`(nil,false)`) начнут отдавать реальные
> значения БЕЗ переписывания. [Source: epics.md:1222–1240; data-model:46–49, 85; 4-3-…md:18; 6-4-…md:21; агенты-разведка A/B/C]

## Story

As a **гражданин**,
I want **чтобы объекты закупок автоматически появлялись на карте (точка/полилиния) с привязкой к району по КАТО**,
so that **я нахожу контракт по адресу своей улицы, а негеокодированные объекты остаются честно видимы в
списке/агрегатах района, а не исчезают**.

## Acceptance Criteria

**AC1 — Канонические таблицы `districts`/`geo_objects` + эфемерный batch-Nominatim → `geocode_status=auto`+confidence, кэш в `geo_objects`**
**Given** таблицы `districts` (KATO-полигоны Астаны) и `geo_objects` (кураторская: `geom` POINT|LINESTRING SRID 4326,
`district_id` FK, `geocode_status`, `confidence`, `length_km`, `contract_id` FK, `geocoded_by`, `geocoded_at`)
**When** эфемерный batch-контейнер Nominatim прогоняет адреса/КАТО объектов
**Then** для распознанного адреса создаётся кандидат `geocode_status=auto` с `confidence`; результат **кэшируется в
`geo_objects`** (Nominatim после batch не дёргается, AR-6); контейнер гасится (AR-22).
[Source: epics.md:1230–1232; FR-4 prd.md:116–120; data-model:46–49; architecture.md:462–474 (AR-22), 401 (AR-6), 517–518]

**AC2 — Район по КАТО присваивается ДАЖЕ без точки (негеокодированные учитываются в агрегатах/медиане района)**
**Given** КАТО объекта (`contracts.kato_code`)
**When** считается геопривязка
**Then** `district_id` присваивается по КАТО (членство точки-в-полигоне ИЛИ префикс-КАТО как в 6.3) **даже без
геокодированной точки**; негеокодированный объект остаётся честно видим (`geocode_status=unmatched`, `geom=NULL` —
**НИКОГДА 0,0/центр города**) и учитывается в агрегатах/медиане района (Story 6.3/6.4).
[Source: epics.md:1234–1236; FR-4 prd.md:120; UX EXPERIENCE.md:255 «без точки на карте»; 0-7-…md:134 (never-0,0)]

**AC3 — Managed-геокодер задокументирован как fallback (одно-проходный, рантайм-зависимость = ноль)**
**Given** managed-геокодер как fallback к self-host Nominatim
**When** self-host ops тяжёл (osmium-extract/импорт 30–90 мин, диск 8–20 ГБ)
**Then** **задокументирован одно-проходный путь**: batch-геокодинг managed-провайдером → координаты в БД
(`geo_objects`) → в рантайме внешней зависимости ноль. Это же — практический путь демонстрации на синтетике
(публичный Nominatim, politeness 1100 мс, механизм 0.7).
[Source: epics.md:1238–1240; architecture.md:468–474 (AR-22 managed-fallback); 0-7-…md:49 (PoliteDelayDefault)]

## Tasks / Subtasks

- [x] **Task 0 — РЕШЕНИЯ ВЛАДЕЛЬЦУ (см. «Открытые вопросы»)** *(AC1–AC3)* — подтверждено «ok delai» (rec-defaults)
  - [x] Q1–Q5 rec-defaults подтверждены (геокодим DEMO-контракты; managed-демо; districts OSM+KATO-null; синтетик road-LINESTRING; contract_id nullable).
- [x] **Task 1 — Миграция `0020_geo_objects.sql` (goose Up/Down): `districts` + `geo_objects`** *(AC1)* ✅ применена+верифицирована (55432)
  - [x] `districts`: `id` bigint identity, `kato_code TEXT` (nullable до 0.1), `name_ru`/`name_kk`, `geom geometry(Polygon,4326)`. [seed: 1 демо-район Есиль в geo_objects.sql; полные 5 районов OSM = остаток]
  - [x] `geo_objects`: `public_id UUID`, `contract_id`/`district_id` FK, `geom geometry(Geometry,4326)` POINT|LINESTRING, **`length_km`**, `geocode_status auto|manual|unmatched`, `confidence`, `geocoded_by`. GIST + honesty-CHECK `(geom IS NULL) = (geocode_status='unmatched')` (**доказано: unmatched-с-точкой и auto-без-точки REJECTED**). [Source: data-model:49; architecture.md:517–518, 527]
  - [x] **Кураторская граница (AR-4):** гранты curator write / importer select (образец 0015).
  - [x] **НЕ трогать** `interim_geo_lots` (0004) — не тронут (карта 0.8 до токена).
- [x] **⚡ РАЗБЛОКИРОВКА МЕДИАНЫ 6.4 через length_km (headline, Q4)** — запрос `PricePerKMSamplesByDirection` + наполнен шов `PricePerKMSamples` → медиана района зажглась из `not_comparable` в 42 000 000 ₸/км (road, N=6) на синтетике; verified SQL→API→браузер; integration-тест red-страж→lit; seed. Флаг 4.3 (price_benchmarks) — отдельный путь, остаток.
- [ ] **Task 2 — Канонический geocode-пайплайн: переиспользовать `internal/geo` + канон-caller/store** *(AC1)*
  - [ ] **НЕ изобретать геокодер:** `internal/geo/nominatim.go` (`Geocoder`, `NewNominatim`, `AstanaViewbox`, NaN/Inf-гард, politeness) — БЕЗ build-tag, переиспользуем как есть (док прямо: «Epic 3 Story 3.1»). Добавить: парс `importance`→`confidence` (сейчас `nominatimResult` читает только lat/lon); 429/Retry-After backoff (долг 0.7 deferred-work:129); нормализация адреса перед геокодингом (долг 0.7 deferred-work:128). [Source: nominatim.go:27–33, 61–64; 0-7-…md:214]
  - [ ] Канон-store: `store/projection|curation` upsert в `geo_objects` (по образцу `projection/geo_lots.go` `UpsertGeoLot ON CONFLICT`, но канон-ключ + geometry). sqlc с `overrides` для PostGIS `geometry`; `ST_GeomFromText`/`ST_AsGeoJSON(geom)::jsonb`; wire `[lon,lat]` GeoJSON RFC7946 (AR-19). [Source: architecture.md:302–304, 561–562; projection/geo_lots.go]
  - [ ] Каноническая batch-команда **ВНЕ hot-path** (страж `TestHotPathDoesNotImportScrape` — `cmd/api`/`importer` не тянут geocode) ИЛИ импортёр-пост-хук (паттерн 2.4 `RunPostImport`). Рекоменд.: отдельная `cmd/`-команда (не под `//go:build scrape` — это уже канон, не интерим). [Source: arch/boundaries_test.go:160; 2-4-…md (RunPostImport)]
- [ ] **Task 3 — КАТО→район + length_km для дорог** *(AC1/AC2)*
  - [ ] `district_id` по КАТО: членство `ST_Contains(district.geom, geo.geom)` при наличии точки, ИЛИ префикс-КАТО (как 6.3 `kato_code LIKE 'X%'`) без точки. Негеокодированный → `district_id` всё равно проставлен (AC2). [Source: epics.md:1234; 6-3-…md (префикс-членство)]
  - [ ] `length_km` для LINESTRING = `ST_Length(geom::geography)/1000`; для POINT = NULL (честно). **Это зажигает 4.3/6.4** — `price_benchmarks`/`PricePerKMSamples` начнут считать реальные значения при наличии length_km. [Source: data-model:85; 4-3-…md:18; 6-4-…md:154]
- [ ] **Task 4 — Эфемерный geocode-профиль (AR-22) + managed-fallback док (AC3)** *(AC3)*
  - [ ] compose-профиль `geocode` (по образцу профиля `app`): self-host Nominatim (osmium-extract Астаны bbox → `mediagis/nominatim` урезанный IMPORT_STYLE → batch → `geo_objects` → контейнер гасится). Scaffold + документация (реальный self-host импорт = при живом объёме/токене). [Source: architecture.md:462–474; epics.md:263–265]
  - [ ] `docs/ops/geocoding.md` (нов.): AC3 managed-fallback одно-проходный путь (координаты в БД, рантайм-зависимость ноль) + self-host рецепт + **демо-путь на синтетике = managed/публичный Nominatim** (politeness 1100 мс, механизм 0.7). Честная пометка: реальное ≥70%-покрытие Астаны — на токене (3.3), здесь coverage = ФАКТ-число, не гейт.
- [ ] **Task 5 — Шов миграции `interim_geo_lots` → `geo_objects` + честный дескоуп-док** *(AC1)*
  - [ ] Задокументировать/скриптовать шов: interim (lot-keyed) → canonical (contract-keyed) при появлении lots↔contracts join (токен/Epic 2). Interim остаётся до токена. [Source: 0004:9; 0-7/0-8 обещание]
  - [ ] Дескоуп-раздел: живой ≥70%-вердикт (FR-6/OQ-4/SM-2 → 3.3/runbook, токен); реальные length_km-значения (Epic 2); Directus-курация `manual`/verification (3.2). НЕ выдумывать.
- [ ] **Task 6 — `store/geo.go` bbox-чтение (pgx-raw) + тесты** *(AC1/AC2)*
  - [ ] `store/geo.go` (сейчас пустой стаб «для 3.x») — pgx-raw динамический bbox-запрос `geo_objects` (AR-3), тот же golden-контракт. [Source: architecture.md:302–304, 727–729; store/geo.go]
  - [ ] Тесты (паттерн 0.7/0.8): httptest-стаб Nominatim (matched/empty/HTTP-err/bad-JSON/NaN-Inf); honesty-гард (unmatched→geom NULL, не 0,0; `auto ⇒ coords`); integration `//go:build integration` (postgis, миграции 0001–0020, skip без DATABASE_URL): geometry round-trip POINT/LINESTRING, КАТО→район, length_km для LINESTRING; OpenAPI-golden если новый DTO; coverage-как-факт + систематические-пропуски. [Source: nominatim_test.go; map_test.go; 0-7/0-8 testing]

## Dev Notes

### Контекст истории (мех vs вердикт — несущая граница)

3.1 открывает Epic 3 (карта/гео). Разведка 3 агентов дала жёсткий вывод: **у 3.1 НЕТ ≥70%-AC — это чистый
механизм** (канон-таблицы + пайплайн). Порог ≥70% = FR-6/SM-2/OQ-4/**Story 3.3**/stage0-runbook, всё на живых
данных Астаны (токен). Поэтому 3.1 строится **полностью токен-независимо** на синтетике/scraped (прецедент
2.0/4.1/4.3/5.1/6.4/0.7), а живой вердикт честно дескоупится. [Source: агент-A; epics.md:1222–1240; prd.md:128–133]

### ⚡ Разблокировка 4.3 + 6.4 (главная ценность 3.1)

`geo_objects.length_km` — ровно та колонка, которую УЖЕ ждут построенные токен-независимые движки:
- **Флаг «цена/км» (Story 4.3, done):** без `length_km` флаг честно НЕ строится (AC3 4.3, не баг). [4-3-…md:18,129]
- **Медиана района (Story 6.4, done):** `PricePerKMSamples→(nil,false)=not_comparable`; при появлении length_km → `JOIN+деление → (samples,true)` БЕЗ слома DTO/хендлера/тестов. [6-4-…md:154]
- **Демонстрация на синтетике (Q4):** засеять 1–2 синтетических road-`geo_objects` (LINESTRING + length_km) для DEMO-контрактов → флаг цена/км + медиана района **реально зажигаются** на синтетике = доказательство токен-независимой сборки. Это буквально цель «строим сейчас, под реалии — при токене».

### ⚠️ Что УЖЕ есть — переиспользовать, НЕ изобретать

| Что | Где (существует) | Что делает 3.1 |
|---|---|---|
| Геокодер Nominatim | `internal/geo/nominatim.go` (БЕЗ build-tag, `Geocoder`, AstanaViewbox, NaN/Inf-гард, politeness) | переиспользует как есть + `importance`→confidence + 429-backoff + address-norm |
| PostGIS | `0001_extensions.sql` (включён) | `geometry(POINT\|LINESTRING,4326)` сразу, без новой extension-работы |
| Upsert-паттерн гео | `projection/geo_lots.go` (`UpsertGeoLot ON CONFLICT`) | зеркалит для канон `geo_objects` (+ geometry, contract-ключ) |
| Honest-states | `registry` `geocode_pending/geocode_failed/ok`; `container_state not_geocoded` (AR-17); never-0,0 в 4 слоях | наследует; canonical `unmatched→geom NULL` |
| КАТО-префикс членство | 6.3 `kato_code LIKE 'X%'` | район по КАТО без точки (AC2) |
| Пост-импорт хук | 2.4 `RunPostImport` | опция: geocode как хук (или отдельная cmd вне hot-path) |

> **Ловушка (агент-B/C):** `geo_objects` вешается на **contract_id** (FK к контракту), а интерим `interim_geo_lots`
> — на **lot** (goszakup_lot_id). На синтетике контракты = DEMO-seed (есть `kato_code=710000000`), scraped-лоты
> контрактов НЕ имеют (ждут токен). Значит на синтетике 3.1 геокодит **DEMO-контракты** в `geo_objects`; interim→
> canonical = документированный шов (исполняется при lots↔contracts join, токен/Epic 2). НЕ смешивать.

### Файлы и куда писать

- **Создаём (NEW):** `migrations/0020_geo_objects.sql` (districts+geo_objects+seed+гранты); канон geocode-caller/store (`internal/store/{curation|projection}/geo_objects.go` + `queries/geo_objects.sql` + sqlc-gen); канон batch-`cmd/geocode` (вне hot-path); `store/geo.go` наполнить (pgx-raw bbox); `docs/ops/geocoding.md` (AC3 managed-fallback + self-host + дескоуп); тесты (unit+integration); (Q4) синтетические road-geo_objects фикстуры в seed.
- **Меняем (UPDATE):** `internal/geo/nominatim.go` (+importance→confidence, +429-backoff, +address-norm); `deploy/docker-compose.yml` (+профиль `geocode`); sqlc `overrides` для geometry (если ещё нет); OpenAPI (если новый bbox-DTO).
- **НЕ трогать:** `interim_geo_lots` (0004), `interim-geocode` команду (0.7 — до токена), `/api/lots` (0.8 map), интерим-путь; готовые движки 4.3/6.4 (они САМИ зажгутся от length_km — не переписывать).
- **НЕ создавать (чужие истории):** серверный `geo_coverage`-гейт + ≥70%-фолбэк (3.3); Directus-курация `manual`/verification `auto/verified/wrong_reported` AR-28 (3.2); карта маркеры/кластеры/линии/превью/список-фолбэк/a11y (3.4/3.5/3.6/3.8).

### Соблюдение архитектуры / гардрейлы

- **AR-4 curated⊥projection:** `geo_objects`/`districts` — CURATED; импортёр read-only, пишут геокодер+Directus; гранты (0015-паттерн). [architecture.md:313,416]
- **AR-19 wire:** GeoJSON RFC7946, порядок `[lon,lat]` (НЕ `[lat,lon]`), `ST_AsGeoJSON(geom)::jsonb`, SRID на проводе не пишем; `geo_object` публичный id = **UUIDv7**. [epics.md:250–254; architecture.md:561–562,527]
- **AR-22 геокодер:** эфемерный batch (не 24/7), managed-fallback документирован. [epics.md:263–265]
- **Честность (несущая):** unmatched → `geom NULL`, `geocode_status=unmatched` — **НИКОГДА 0,0/центр** (гард в 4 слоях: DB CHECK → geocoder NaN/Inf → mapper → API). Coverage = ФАКТ-число, НЕ гейт (0.7); реальный ≥70%-вердикт = 3.3 (токен). НЕ выдумывать точки ради «зелёной карты». [0-7-…md:134,255; architecture.md:569–571]
- **Нейтральность:** маркеры нейтрально-синие (`--color-primary`), алый запрещён; CI taboo-страж. [0-8-…md:220]
- **Миграция = 0020** (последняя 0019; НЕ номер из старых story-заметок). geo_objects.length_km — новая колонка (нигде в миграциях нет). [агент-C; migrations/]

### Тестирование / проверка готовности

- **Детерминизм без живой сети:** httptest-стаб Nominatim (0.7-паттерн) — matched/empty(→ok=false, не err)/HTTP-err/bad-JSON/NaN-Inf-range. Honesty-гард: unmatched→geom NULL (не 0,0); `auto ⇒ coords`.
- **Integration (`//go:build integration`, postgis, миграции 0001–0020, skip без DATABASE_URL):** geometry round-trip POINT/LINESTRING; КАТО→район (членство + префикс); length_km для LINESTRING (`ST_Length::geography/1000`); **сквозная разблокировка 4.3/6.4 на синтетике** (road-geo_object с length_km → price_benchmarks/median отдают значение, не insufficient). Прогон через раздельные clean/seed-группы (долг закрыт `c7dfdbb` — geo_objects тесты в нужную группу).
- **Гейты:** go build/vet/gofmt/test -count=1; check-core/check-registry; gen-sqlc идемпотентен (docker); boundary `TestHotPathDoesNotImportScrape` зелёный (канон geocode-cmd вне hot-path); web (если новый bbox-DTO) typecheck/vitest.
- **DoD:** AC1–AC3 закрыты; length_km зажигает 4.3/6.4 на синтетике (демо); честный дескоуп (≥70%-вердикт→3.3, real-length_km-значения→Epic2, Directus→3.2); interim не тронут; never-0,0.

### Previous Story Intelligence (0.7/0.8 — прямые предшественники)

- **0.7 (done) — механизм, который 3.1 продвигает:** `internal/geo` (порт из stage0, БЕЗ build-tag, «переиспользуется Epic 3 Story 3.1»); интерим `interim_geo_lots` (doubles, lot-keyed, `auto`/`unmatched`, never-0,0, confidence=NULL); coverage=ФАКТ не гейт; 5 review-патчей (NaN/Inf-гард; систематич-пропуски; ctx; -max+operational-exit; DB-CHECK honesty). **3 долга 0.7 явно на 3.1:** address-нормализация, 429/Retry-After backoff, (атомарность — опц.). [0-7-…md; deferred-work.md:128–130]
- **0.8 (done) — карта-контракт, honest-state который 3.1 чтит:** `/api/lots` LEFT JOIN interim; `MapLot` DTO; wire matched→`ok`/[lon,lat], unmatched→`geocode_failed`/no-coord, no-row→`geocode_pending`; «auto ⇒ coords» инвариант; empty→honest plaque «не выдумывать ради демо». [0-8-…md:163–170,249]
- **Гардрейл честности (0.7/0.8/6.3):** негеокодированное → честное состояние/список, район по КАТО без точки; никогда фейк-координата. Наследовать во всех AC. [memory: guards-must-prove-red, data-source-interim-parser-first]

### Project Structure Notes

- Конфликтов нет: миграция 0020 (следующая); канон geocode вне hot-path (boundary-страж); `store/geo.go` — существующий стаб под это; docs в `docs/ops/`. geo_objects — новая CURATED-таблица (0015-гранты паттерн).
- **Токен-независима полностью** (в отличие от 0-1/0-3/2-1). Первая история Epic 3 → `epic-3: in-progress` (проставлено). После 3.1: 3.2 (Directus manual) / 3.3 (coverage-гейт) / 3.4 (карта UI) — часть тоже токен-независимы на синтетике.
- **Epic-3-go-live кластер (память AI-4):** length_km здесь оживит флаг цена/км (4.3) + медиану (6.4) + снимет отложенное решение «город»-базы (6.4) — при живых числах (токен). На синтетике демонстрируем механику.

### Внешние знания / web-research

Nominatim self-host (`mediagis/nominatim`, osmium-extract, IMPORT_STYLE) + managed-провайдеры + PostGIS
`geometry(POINT|LINESTRING,4326)`/`ST_AsGeoJSON`/`ST_Length::geography`/GIST — зафиксированы в architecture (AR-22/AR-3)
и data-model; НЕ до-исследуются из памяти модели (версии postgis/sqlc-overrides — из репо-конвенций). UUIDv7 —
AR-19 (внутренняя конвенция). При реализации подтвердить точный sqlc `overrides`-синтаксис для geometry на пинованной
sqlc 1.31 (docker gen-sqlc). [Source: architecture.md:462–474; data-model:21,49]

## Открытые вопросы / решения владельцу

1. **Q1 источник геокодинга на синтетике.** Рекоменд.: геокодить **DEMO-контракты** (canonical, contract_id FK) в `geo_objects`; interim→canonical = документированный шов (при токене/Epic 2). НЕ смешивать lot-keyed interim с contract-keyed canonical. Подтвердить.
2. **Q2 managed-демо-путь vs self-host.** Рекоменд.: scaffold compose-профиль `geocode` + документировать self-host (AR-22), но фактический синтетик-геокодинг = **managed/публичный Nominatim** (politeness, механизм 0.7); реальный self-host импорт = при живом объёме (токен). AC3 = документирован путь. Подтвердить.
3. **Q3 districts-полигоны + КАТО-коды.** Рекоменд.: 5 районов Астаны полигонами из публичного OSM (не токен); `kato_code` **nullable до Story 0.1** (прецедент `astana_districts.json` names-без-кодов, 6.3); имена+geom сейчас, коды при токене. Подтвердить.
4. **Q4 демонстрация разблокировки 4.3/6.4.** Рекоменд.: засеять **1–2 синтетических road-`geo_objects` (LINESTRING+length_km)** → флаг цена/км + медиана района реально зажигаются на синтетике (доказательство токен-независимой сборки). Или оставить length_km пустым (4.3/6.4 честно-degrade до токена). **Рекоменд.: засеять — это буквально цель истории.** Подтвердить.
5. **Q5 `geo_objects.contract_id` nullable?** Рекоменд.: да (синтетик scraped-лоты контрактов не имеют; DEMO-контракты имеют). Подтвердить.

## Dev Agent Record

### Agent Model Used

claude-opus-4-8[1m] (Claude Opus 4.8, 1M context) — create-story (3 Explore-агента) + dev-story (частичный инкремент).

### Debug Log References

- **Решение владельца 2026-07-01: «делай всё токен-независимым»** → dev-story по rec-defaults (5 open-q подтверждены). **Развилка (объём 3.1 = фундамент Epic 3, ~2-3 истории): владелец выбрал «headline-инкремент сейчас + честный сплит остатка».** Baseline `5cc6ad9`.
- **Доставленный инкремент = миграция 0020 + разблокировка медианы 6.4 через length_km** (самое наглядное «строим сейчас → под реалии оживает»). Верифицирован СКВОЗНО: миграция → SQL → API → браузер.
- Миграция 0020 применена на 55432; honesty-CHECK'и доказаны вживую (unmatched-с-точкой(0,0)→REJECTED; auto-без-точки→REJECTED); `ST_Length(geography)/1000` даёт length_km.
- sqlc переварил PostGIS `geometry` (→ `interface{}`, запросы его не трогают); новый запрос `PricePerKMSamplesByDirection` (JOIN geo_objects, ₸/км целочисленно).
- Шов `districtStore.PricePerKMSamples` наполнен (был `(nil,false)` → теперь `(samples,true)`). Existing-тест `TestDistrictMedianSamples_SeamEmpty` был СТРАЖЕМ, спроектированным краснеть при появлении length_km → переписан на `..._LengthKmLit` (length_km есть → 6 выборок, ₸/км детерминирован, computable=true).

### Completion Notes List

- **✅ ДОСТАВЛЕНО + ВЕРИФИЦИРОВАНО (headline-инкремент):**
  - **Миграция `0020_geo_objects.sql`** (AC1 таблицы): канон `districts` (KATO-полигоны, kato_code nullable-до-0.1) + `geo_objects` (PostGIS `geometry(POINT|LINESTRING,4326)`, `length_km`, UUID public_id, contract_id/district_id FK, GIST, curated-гранты AR-4, honesty-CHECK'и: geom NULL ⟺ unmatched → **never-фейк-координата в БД**). Применена, geometry roundtrip + CHECK'и проверены.
  - **РАЗБЛОКИРОВКА МЕДИАНЫ РАЙОНА (6.4/FR-18) через `length_km`** — главная ценность: запрос `PricePerKMSamplesByDirection` (JOIN geo_objects.length_km, ₸/км=amount/length) + наполнен шов `PricePerKMSamples` → **медиана зажглась из `not_comparable` в реальные 42 000 000 ₸/км** (road, N=6) на синтетике; water честно `insufficient`. Проверено: SQL (медиана 42M) → API (`/api/districts/710000000` road=ok/42M) → **браузер (полосы район/город, «0% к городу», «6 контрактов»)**.
  - **Seed** `fixtures/seed/geo_objects.sql` (район Есиль + 6 синтетических road-контрактов + geo_objects LINESTRING+length_km) + порядок seed (Makefile + compose) обновлён.
  - **Тест** `districts_median_integration_test.go` переписан red-страж → lit (integration PASS на 55432).
  - **Гейты зелёные:** gofmt/vet/build; unit `go test ./...`; check-core (boundary — geo не в hot-path); check-registry; gen-sqlc идемпотентен.
- **⏳ ОСТАТОК 3.1 (честный сплит — следующим заходом, НЕ сделано):**
  - **Task 2 (live geocode-команда):** переиспользовать `internal/geo` (доказано reusable, 0.7) в канон-caller/cmd (upsert geo_objects) + importance→confidence + 429-backoff + address-norm. Сейчас geo_objects наполняются seed'ом, не живым геокодом.
  - **Task 3 (КАТО→район авто в пайплайне):** `length_km` из `ST_Length` (в seed) — механику вычисления в пайплайне; авто-district_id членством/префиксом при геокоде.
  - **Task 4 (эфемерный geocode-профиль AR-22 + `docs/ops/geocoding.md` AC3 managed-fallback).**
  - **Task 5 (interim→canonical шов + дескоуп-док).**
  - **Task 6 (`store/geo.go` pgx-raw bbox + доп. тесты).**
  - **Флаг цена/км (4.3):** читает `price_benchmarks` (кэш, `RecalcBenchmarks`→пустой) — отдельный путь, НЕ разблокирован этим инкрементом (район-медиана 6.4 читает `PricePerKMSamples` напрямую — вот его и зажёг). Разблокировка 4.3 = врезка length_km в `RecalcBenchmarks` (следующим заходом).
- **Дескоуп на токен (не меняется):** живой ≥70%-вердикт (FR-6/3.3), реальные length_km-значения (Epic 2), Directus-курация (3.2).

### File List

**Код/схема (новые):**
- `migrations/0020_geo_objects.sql` — канон districts + geo_objects (PostGIS, length_km, UUID, GIST, curated-гранты, honesty-CHECK).
- `fixtures/seed/geo_objects.sql` — синтетик: район + 6 road-контрактов + geo_objects (LINESTRING+length_km) для демо разблокировки медианы.

**Код (изменены):**
- `server/internal/store/queries/districts.sql` — `+PricePerKMSamplesByDirection` (JOIN geo_objects.length_km → ₸/км).
- `server/internal/store/gen/{districts.sql.go,querier.go,models.go}` — sqlc-регенерат (District/GeoObject-модели, новый запрос).
- `server/internal/httpapi/districts.go` — шов `PricePerKMSamples` наполнен (nil,false → samples,true).
- `server/internal/httpapi/districts_median_integration_test.go` — red-страж → lit-тест (length_km зажёг ₸/км).
- `Makefile` — db-seed loop `+geo_objects`.
- `deploy/docker-compose.yml` — seed-сервис `+geo_objects.sql`.

**Трекинг:**
- story `3-1-…batch-nominatim.md` — frontmatter baseline, чекбоксы Task 0/1, Dev Agent Record, File List, Change Log, Status.
- `sprint-status.yaml` — `ready-for-dev → in-progress`.

## Change Log

| Дата | Изменение |
|---|---|
| 2026-07-01 | dev-story (частичный инкремент по выбору владельца — 3.1 = фундамент Epic 3 ~2-3 истории, «headline сейчас + сплит»). `baseline=5cc6ad9`. **Доставлено+верифицировано:** миграция `0020` (канон districts+geo_objects, PostGIS geometry POINT|LINESTRING 4326, length_km, UUID, GIST, curated-гранты AR-4, honesty-CHECK never-фейк-координата — доказано REJECT'ами); **РАЗБЛОКИРОВКА медианы района 6.4/FR-18: `PricePerKMSamplesByDirection` (JOIN length_km) + наполнен шов → медиана зажглась `not_comparable`→42 000 000 ₸/км (road,N=6) на синтетике, verified SQL→API→БРАУЗЕР** (полосы район/город); seed `geo_objects.sql` (район+6 road-контрактов+geo LINESTRING); integration-тест red-страж→lit (PASS); Makefile/compose seed +geo_objects. Гейты зелёные (gofmt/vet/build/unit/check-core/check-registry/gen-sqlc). **Остаток (честный сплит, Status=in-progress):** live geocode-команда через internal/geo (Task 2), КАТО→район авто (Task 3), эфемерный geocode-профиль+docs/ops/geocoding.md (Task 4), interim→canonical шов (Task 5), store/geo.go bbox (Task 6), флаг 4.3 (RecalcBenchmarks). Дескоуп на токен: живой ≥70%-вердикт (3.3), реальные length_km-значения (Epic2), Directus (3.2). |
| 2026-07-01 | Создан context engine (create-story, 3 параллельных Explore-агента). **Вывод: 3.1 = 100% механизм, 0% ≥70%-вердикта → полностью токен-независима** (прецедент 2.0/4.1/6.4/0.7). Baseline `5cc6ad9`. Рамка: канон `districts`+`geo_objects` (миграция **0020**, PostGIS geometry POINT|LINESTRING 4326, UUIDv7 AR-19, curated-границы AR-4, **length_km**) + переиспользование `internal/geo` (не изобретать; +importance/backoff/address-norm) + КАТО→район даже без точки (AC2) + эфемерный geocode-профиль AR-22 + managed-fallback док (AC3) + `store/geo.go` bbox + interim→canonical шов. **Главная ценность: length_km зажигает отложенные 4.3 (флаг цена/км) + 6.4 (медиана района) БЕЗ переписывания** (Q4: синтетические road-LINESTRING демонстрируют). Дескоуп на токен: живой ≥70%-вердикт (3.3/runbook), реальные length_km-значения (Epic2), Directus-курация (3.2). 5 open-q. Статус → ready-for-dev. |

## References

- [Source: epics.md:1217–1240 — Epic 3 + Story 3.1 (3 AC); 1242–1376 — границы 3.2–3.8; 193–290 — AR-3/4/6/16/17/19/21/22/28; 488–491 — FR-4/5/6 split; 585 — Epic 3 standalone/Epic 2]
- [Source: prd.md:113–133 — FR-4/5/6 (гейт ≥70% вне 3.1); 378–380 SM-2; 389 SM-C2; 395 risk; 407 OQ-4]
- [Source: docs/AshyqQala_MVP_data_model_and_flags_v1.md:21, 28 (districts), 46–49 (geo_objects), 56 (price_benchmarks), 85 (length_km→цена/км)]
- [Source: architecture.md:38–41, 302–304 (sqlc geometry overrides + store/geo.go pgx-raw), 401/416 (AR-6/грант), 462–474 (AR-22 геокодер эфемерный+managed), 517–518 (GIST/SRID), 527 (UUIDv7), 561–562 (GeoJSON [lon,lat]), 569–571 (null-честность), 727–729 (internal/geo, store/geo.go), 885–894 (container_state, AR-28 verification)]
- [Source: migrations/0001_extensions.sql (postgis on), 0004_interim_geo_lots.sql:2–24 (интерим, НЕ канон), 0018_district_indexes.sql:15 (kato-индекс на contracts, НЕ districts-таблица); последняя 0019 → 0020]
- [Source: server/internal/geo/nominatim.go:19–33, 61–64, 102–118 (Geocoder, viewbox, NaN/Inf-гард); tools/scrape/cmd/interim-geocode/main.go (0.7 batch-паттерн); store/projection/geo_lots.go (upsert); httpapi/map.go (0.8 honest MapLot); store/geo.go (пустой стаб под 3.x)]
- [Source: 0-7-…md:24,49,134,154,201,210–215,253–265 (механизм 0.7, долги→3.1); 0-8-…md:163–170,249,270 (карта honest-state); 4-3-…md:18,129 (флаг цена/км ждёт length_km); 6-4-…md:21,154 (медиана ждёт length_km); deferred-work.md:128–130 (долги 0.7→3.1), 293 (KATO codes null-до-0.1)]
- [Source: CLAUDE.md — гардрейлы честности/нейтральности; memory: guards-must-prove-red, data-source-interim-parser-first, sprint-sequencing (Epic-3-go-live кластер)]
