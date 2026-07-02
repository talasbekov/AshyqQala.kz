---
baseline_commit: 5cc6ad99ffc5c882db5708324027d871bbebf5ba
---

# Story 3.1: Автоматическая геопривязка (batch-Nominatim)

Status: done

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
- [x] **Task 2 — Канонический geocode-пайплайн: переиспользовать `internal/geo` + канон-caller/store** *(AC1)*
  - [x] **НЕ изобретал геокодер:** `internal/geo/nominatim.go` переиспользован как есть (старый `Geocode` — тонкая обёртка над новым `GeocodeMatch`, поведение/сигнатура неизменны, все существующие тесты 0.7 зелёные без правок). Добавлено: парс `importance`→`Match.Confidence`; 429/Retry-After backoff (до 2 повторов, инжектируемый `sleep` для тестов, капается 30с) закрывает долг 0.7 deferred-work:129; `geo.NormalizeAddress` (схлопывание пробелов) закрывает долг 0.7 deferred-work:128.
  - [x] Канон-store `store/curation/geo_objects.go` (`GeoObjectStore`, зеркало `AliasStore`/`GeoLotStore`) — CURATED-путь (AR-4). sqlc БЕЗ `overrides` для `geometry` (сознательное отклонение от AC1-заметки — фрагильно/непроверено в этой sqlc 1.31; вместо этого везде явные `ST_GeomFromText(text)`/`ST_AsGeoJSON(geom)::jsonb`, ровно паттерн architecture.md «ST_* с явным ::type+алиас»). Миграция `0021` добавляет частичный уникальный индекс `geo_objects(contract_id) WHERE contract_id IS NOT NULL` (нужен для идемпотентного `ON CONFLICT`; НЕ было в 0020). `geocode_status='manual'` НИКОГДА не перезаписывается batch'ем — WHERE-гейт на `DO UPDATE`, доказано integration-тестом.
  - [x] `server/cmd/geocode/main.go` — отдельная канон-команда ВНЕ hot-path (без build-tag, физически вне `tools/scrape`; `TestHotPathDoesNotImportScrape` касается только `cmd/api`/`cmd/importer` — не ограничивает новый бинарь, но и не нарушен). Мирроит скелет `interim-geocode` (ping-first, `signal.NotifyContext`, honest coverage, систематические пропуски), БЕЗ интерим-гейта/warning (канон, не временное отклонение §6.1/§6.4).
- [x] **Task 3 — КАТО→район + length_km для дорог** *(AC1/AC2)*
  - [x] `resolveDistrict` в `cmd/geocode/main.go`: `ST_Contains` при наличии точки (запрос `FindDistrictIDByPoint`) с фолбэком на КАТО-префикс (`FindDistrictIDByKATOPrefix`, зеркало 6.3/`district.Catalog.NameByKATO` — самое длинное совпадение первым); без точки — сразу префикс. Вызывается для КАЖДОГО контракта независимо от `matched` → негеокодированный получает `district_id` (AC2), доказано unit+integration тестами. Инфраструктурные ошибки (не `pgx.ErrNoRows`) прокидываются, не маскируются под «нет района».
  - [x] `length_km` **выводится в SQL** (`UpsertGeoObject`: `CASE WHEN GeometryType(geom)='LINESTRING' THEN ST_Length(geom::geography)/1000.0 ELSE NULL END`), НЕ принимается параметром — целый класс багов «забыли пересчитать» структурно невозможен для этого пути записи. Доказано integration-тестом (сверка с независимым psql-расчётом) + honest-NULL для POINT. **Ограничение (см. Dev Agent Record):** вывод действует только через `UpsertGeoObject`; сырые SQL-писатели (seed, будущая Directus 3.2) продолжают задавать `length_km` явно — не GENERATED COLUMN (сознательно, не трогать уже-верифицированные 6.4-числа сида).
- [x] **Task 4 — Эфемерный geocode-профиль (AR-22) + managed-fallback док (AC3)** *(AC3)*
  - [x] `deploy/docker-compose.cold.yml`: сервис `geocode` (профиль `geocode`, рядом с уже существовавшим `nominatim`). `server/Dockerfile` расширен вторым бинарём (`/geocode`, `entrypoint:`-override — голый `command:` склеился бы с существующим `ENTRYPOINT ["/api"]`). Проверено сквозно: `docker build` + `docker run --entrypoint=/geocode` + `docker compose ... config` на ВСЕХ профилях (app/geocode/importer/bot/directus).
  - [x] `docs/ops/geocoding.md` — AC3 self-host/managed-fallback однопроходный путь + честная граница (coverage=факт, живой ≥70%-вердикт=3.3).
- [x] **Task 5 — Шов миграции `interim_geo_lots` → `geo_objects` + честный дескоуп-док** *(AC1)*
  - [x] Задокументирован (НЕ скриптован — join `lots↔contracts` не существует в схеме, писать перенос против него значило бы выдумывать сопоставление; честно отложено до Epic 2/токена) в `docs/ops/geocoding.md` §«Шов: interim_geo_lots → geo_objects».
  - [x] Дескоуп-раздел там же: живой ≥70%-вердикт (3.3), реальные length_km (Epic 2), Directus-курация (3.2), карта/UI (3.4–3.8), реальные полигоны районов (0.1).
- [x] **Task 6 — `store/geo.go` bbox-чтение (pgx-raw) + тесты** *(AC1/AC2)*
  - [x] `store/geo.go` наполнен: `ListGeoObjectsInBBox` — pgx-raw (НЕ sqlc, architecture.md), `ST_MakeEnvelope`+`&&` против GiST-индекса 0020, только геокодированные (`geom IS NOT NULL`), GeoJSON `[lon,lat]` (AR-19).
  - [x] Тесты: `internal/geo` — httptest-стаб (matched/empty/HTTP-err/bad-JSON/NaN-Inf — уже были, + importance/429-retry/429-exhaust/500-no-retry/NormalizeAddress, новые); `cmd/geocode` — unit (фейки, 0.7-паттерн) + integration (идемпотентность, AR-4-гейт manual, geometry round-trip, honesty-CHECK'и живьём, length_km-вывод); `internal/store` — integration bbox (внутри/снаружи/unmatched-исключён/GeoJSON-порядок/пустой→`[]`). Публичного DTO/эндпоинта нет (Task 6 — только store-слой; чтение для UI = Story 3.4) → web typecheck/vitest не требуются.

### Review Findings

**Процесс (честно, отклонение от стандартной последовательности skill'а):** dev-агент применил `patch`-находки СРАЗУ по получении (протестировав и верифицировав каждую), вместо предписанного HALT→спросить владельца «apply all / walk through / оставить как action items» ПЕРЕД применением. Ниже — фактический результат, не план. `decision-needed`-пункт НЕ решён агентом (не мог быть решён unilaterally) и ждёт владельца.

3 параллельных слоя (Blind Hunter — только диф; Edge Case Hunter — диф + read-access; Acceptance Auditor — диф+спек), диф = незакоммиченные изменения Tasks 2–6 (2073 строки, 18 файлов). Триаж: 1 decision-needed, 9 patch (все применены+верифицированы), 7 defer, 3 dismiss.

- [x] [Review][Decision] **РЕШЕНО владельцем 2026-07-02: явные `ST_*`-касты ПРИНЯТЫ как достаточное удовлетворение AR-3** (sqlc `overrides` для PostGIS `geometry` НЕ требуется). Контекст: вместо `overrides:`-блока в `sqlc.yaml` везде использованы явные `ST_GeomFromText(text)`/`ST_AsGeoJSON(geom)::jsonb`-касты (задокументировано в Task 2 чекбоксе) — подход работает и протестирован (весь `geo_objects`-путь живьём против Postgres), обратим (ничто не мешает добавить `overrides:` позже, если понадобится). architecture.md сам упоминает оба паттерна в одном предложении («overrides для geometry; ST_* с явным ::type+алиас»). Код НЕ менялся по итогам решения.

- [x] [Review][Patch] **Транспортная ошибка геокодера навсегда замораживала контракт вне будущих прогонов** [server/cmd/geocode/main.go, `geocodeContracts`] — раньше писалась как `status=unmatched`, а `ListContractsForGeocode` не пере-выбирает `unmatched`-строки. Применено: транспортная/инфраструктурная ошибка (геокод ИЛИ резолв района) → контракт пропускается БЕЗ записи (retryable следующим прогоном); честный «адрес не распознан» — единственный случай, который пишется как `unmatched`. Побочно исправило double-counting `st.Errors` (операционный гейт `Errors==Total` раньше мог никогда не сработать) и дало чистое разбиение `Matched+Unmatched+Errors==Total`. Тесты: `TestGeocodeContracts_GeocoderError_SkipsWriteRetryableNextRun`, `TestGeocodeContracts_DistrictResolutionError_SkipsWriteRetryableNextRun`, `TestGeocodeContracts_CleanPartition_...`.
- [x] [Review][Patch] **`confidence` писался как литеральный 0.0 вместо NULL, когда Nominatim не отдаёт `importance`** [server/internal/geo/nominatim.go] — неотличимо от «importance реально пришла нулевой», конфликтовало с собственным doc-комментарием `Match` («вызывающий решает, писать ли NULL»). Применено: `nominatimResult.Importance`/`Match.Confidence` → `*float64` (encoding/json различает «ключ отсутствует» от «ключ=0» только через указатель). Тесты: `TestGeocodeMatch_MissingImportance_NilConfidence`, `TestGeocodeMatch_ExplicitZeroImportance_ZeroConfidence`, `TestToGeoParams_MatchedButNoImportance_NilConfidence`.
- [x] [Review][Patch] **429-backoff и пауза вежливости блокировали SIGTERM/Ctrl-C до 30с**, несмотря на комментарий «прерывают чисто» [server/internal/geo/nominatim.go `GeocodeMatch`; server/cmd/geocode/main.go `geocodeContracts`] — голый `sleep(d)` не слушал `ctx.Done()`. Применено: `sleepCtx` (обе точки) — прерываемый sleep через `select`. Тесты: `TestGeocodeMatch_429_CtxCancelledDuringBackoff_ReturnsPromptly` (реальный `time.Sleep`, доказывает прерывание за ~50мс вместо 2с).
- [x] [Review][Patch] **`contractSource` был мёртвым интерфейсом; ядро `main()` (-max-срез, «нет контрактов», all-errored-гейт) было непротестировано** [server/cmd/geocode/main.go] — именно там жил double-counting баг до фикса. Применено: вынесено в тестируемый `run(ctx, contractSource, geocoder, geoStore, delay, max, stdout, stderr) int`; `main()` теперь тонкая обёртка. Добавлена валидация `-max<0`. Тесты: `TestRun_ContractSourceError_ReturnsOne`, `TestRun_NoContracts_ReturnsZero`, `TestRun_MaxSlicing`, `TestRun_AllErrored_ReturnsOne`, `TestRun_Success_PrintsCleanSummary`.
- [x] [Review][Patch] **`FindDistrictIDByKATOPrefix` строил LIKE-паттерн из НЕэкранированного `districts.kato_code`** [server/internal/store/queries/geo_objects.sql] — колонка (таблица 0020) не имеет CHECK-ограничения на формат (в отличие от `district.ValidKATO`-регэкспа JSON-реестра); `%`/`_` в сохранённом значении действовали бы как wildcard. Применено: `ESCAPE '\'` + тройной `replace`. Проверено живьём psql (экранированный `%` даёт TRUE только на точном совпадении, не wildcard-расширении).
- [x] [Review][Patch] **`ListGeoObjectsInBBox` не валидировал bbox** (NaN/Inf/инвертированный/вне-WGS84) [server/internal/store/geo.go] — ушло бы в `ST_MakeEnvelope` непроверенным. Применено: `validateBBox`. Тест: `TestValidateBBox` (8 кейсов).
- [x] [Review][Patch] **AR-19 `[lon,lat]`-тест использовал симметричную точку** (15.50,15.50) [server/internal/store/geo_integration_test.go] — не мог отличить правильный порядок от перепутанного. Применено: асимметричные координаты (15.30,15.70).
- [x] [Review][Patch] **Миграция 0021 без `CONCURRENTLY`/без комментария о ACCESS EXCLUSIVE-локе** [migrations/0021_geo_objects_contract_uniq.sql] — добавлен честный комментарий о трейд-оффе. Побочно: сам добавленный комментарий ПЕРВОНАЧАЛЬНО содержал буквальный текст `-- +goose NO TRANSACTION`, который goose парсит как директиву ДАЖЕ внутри prose-комментария — сломал `migrate-up` на свежей БД (поймано СОБСТВЕННОЙ повторной проверкой на чистой verify-БД, не ревью-агентами). Переформулировано без буквального совпадения с goose-аннотацией.
- [x] [Review][Patch] **Самоотчёт числа юнит-тестов в Dev Agent Record был завышен** («25 unit-тестов (geo-пакет) + 20 (cmd/geocode)» — реально 13 top-level (18 с саб-тестами `TestRetryAfterDuration`) и 18. Интеграционные «9» — точно, не завышены.) Скорректировано ниже в Debug Log.

- [x] [Review][Defer] **`auto`-строки пере-геокодируются КАЖДЫЙ прогон навсегда** (нет терминального «verified»-статуса) [server/internal/store/queries/geo_objects.sql] — deferred, уже явно вынесено в дескоуп самой истории (Directus-курация `manual`/verification → Story 3.2, добавит терминальный статус в закрытый enum).
- [x] [Review][Defer] **`os.Exit()` на путях сбоя настройки в `main()` пропускает `defer pool.Close()`/`defer stop()`** [server/cmd/geocode/main.go] — deferred, зеркалит установленный прецедент 0.7 (`interim-geocode/main.go` делает то же самое); низкий риск для короткоживущего batch-CLI (процесс всё равно завершается, ОС освобождает ресурсы).
- [x] [Review][Defer] **Изоляция тестов от persistent seed-данных держится на ad hoc комментариях («безопасные» ID-префиксы/координаты), не на структурном соглашении** [server/cmd/geocode/idempotency_integration_test.go, server/internal/store/geo_integration_test.go] — deferred, сквозная проблема тест-инфраструктуры (затрагивает и существующий `districts_integration_test.go`), шире одной истории.
- [x] [Review][Defer] **Миграция 0021 теоретически упала бы при существующих дублях `contract_id`** [migrations/0021_geo_objects_contract_uniq.sql] — deferred, эмпирически не воспроизводится на текущих данных (единственный писатель до этой истории — идемпотентный seed); реальный риск нулевой до появления других писателей.
- [x] [Review][Defer] **`nominatim`-сервис в cold-профиле без healthcheck — `geocode` мог бы стартовать до готовности self-host импорта** [deploy/docker-compose.cold.yml] — deferred, нужно проверенное знание healthcheck-эндпоинта `mediagis/nominatim:4.4` (не тестировал живьём — риск написать неверный healthcheck хуже отсутствия); частично смягчено задокументированной раздельной последовательностью шагов (`docs/ops/geocoding.md`).
- [x] [Review][Defer] **`geo_objects.district_id` (Task 3, AC2) не читается НИКЕМ вне нового кода** [проверено grep по internal/httpapi, queries/districts.sql] — deferred, НЕ баг: AC2-гарантия «учитывается в агрегатах района» уже выполняется через `contracts.kato_code` (существовавший механизм 6.3, независимый от `district_id`). `district_id` — подготовка для будущего потребителя (карта, Story 3.4); уже читается обратно `store/geo.go`'s `ListGeoObjectsInBBox`.
- [x] [Review][Defer] **`cmd/geocode`-интеграционные тесты в Makefile-группе CLEAN, а не SEED** [Makefile] — deferred, эмпирически проверено — проходят корректно в ОБОИХ состояниях БД; защитимая, но спорная категоризация, не функциональный баг.

**Dismissed (3, детали не персистятся по инструкции skill'а):** демо-seed `length_km` не совпадает с новой SQL-формулой (уже раскрыто как осознанный трейд-офф до ревью); учётные данные БД в новом compose-сервисе (дословно совпадает с существующим паттерном `api`/`migrate`/`seed`); `importance` вне [0,1] не клэмпится (клэмпинг сам нарушил бы принцип честности сильнее, чем пропуск значения как есть).

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
- **Заход 2 (2026-07-02): честный сплит закрыт — Tasks 2–6 доставлены.** `baseline=5cc6ad9` (тот же — Task 0/1/median-инкремент уже в baseline story). Дев-цикл: research-агент (63 tool-uses) картировал существующий код (nominatim.go/geo_lots.go/districts.go/boundaries_test.go/compose/Dockerfile) ДО написания кода — ничего не изобретено заново.
- **Реальные баги, пойманные integration-тестами (НЕ поймал бы юнит с фейками):**
  1. `ON CONFLICT (contract_id)` не резолвился против partial unique index без явного `WHERE contract_id IS NOT NULL` в самом ON CONFLICT (Postgres требует зеркального предиката для partial-index inference) — SQLSTATE 42P10 живьём, исправлено.
  2. Тест геометрии/КАТО изначально использовал координаты/код, СОВПАДАЮЩИЕ с существующим seed-районом «Есиль» (persistent docker-volume — БД между сессиями НЕ пустая) → ложный матч чужого id. Урок: синтетические интеграционные фикстуры обязаны быть геометрически/кодово ИЗОЛИРОВАНЫ от seed, не только по имени/gid-префиксу (прецедент districts_integration_test.go этому не научил явно — теперь явно задокументировано в обоих новых test-файлах).
- **Решение (Task 3): length_km выводится в SQL** (`UpsertGeoObject`, `CASE WHEN GeometryType(...)='LINESTRING'...`), НЕ GENERATED COLUMN. Разобрано и отвергнуто: GENERATED COLUMN был бы структурно бронебойнее (защищает ЛЮБОГО писателя, включая будущую Directus 3.2 напрямую в Postgres), но потребовал бы ALTER существующей колонки (новая миграция) И сломал бы `fixtures/seed/geo_objects.sql` — тот явно задаёт РАЗНЫЕ length_km (5.0/6.0) для строк с ОДИНАКОВОЙ geometry (числа подобраны под целевую медиану 42M ₸/км, не выведены из geom). Менять уже-верифицированный (SQL→API→браузер, прошлый заход) сид ради этой истории — overreach. **Открытый пункт для Story 3.2:** Directus, если пишет в geo_objects напрямую (не через Go), должен сам считать length_km той же формулой, либо этот файл стоит пересмотреть на GENERATED COLUMN тогда (когда сид перестанет быть демо-опорой).
- **Находка (не баг, не мой код):** `make test-integration-clean` на ДЕЙСТВУЮЩЕЙ dev-БД (порт 55432, персистентный volume) красный на `TestRNUFlag_RaiseAutoClear`/`TestRNUFlag_MultiRecordPerOrg` (internal/store/projection, РНУ-флаги — не геокодинг). Перепроверено на СВЕЖЕ смигрированной изолированной БД (`ashyqqala_verify`, та же 0001-0021, без seed) — оба теста зелёные. Вывод: загрязнение состояния от прошлых сессий на этом конкретном dev-volume, НЕ регресс от Story 3.1 (домены таблиц не пересекаются). Не чинил — вне scope 3.1; в CI (свежая БД на прогон) не воспроизведётся.

### Completion Notes List

- **✅ ДОСТАВЛЕНО + ВЕРИФИЦИРОВАНО (headline-инкремент):**
  - **Миграция `0020_geo_objects.sql`** (AC1 таблицы): канон `districts` (KATO-полигоны, kato_code nullable-до-0.1) + `geo_objects` (PostGIS `geometry(POINT|LINESTRING,4326)`, `length_km`, UUID public_id, contract_id/district_id FK, GIST, curated-гранты AR-4, honesty-CHECK'и: geom NULL ⟺ unmatched → **never-фейк-координата в БД**). Применена, geometry roundtrip + CHECK'и проверены.
  - **РАЗБЛОКИРОВКА МЕДИАНЫ РАЙОНА (6.4/FR-18) через `length_km`** — главная ценность: запрос `PricePerKMSamplesByDirection` (JOIN geo_objects.length_km, ₸/км=amount/length) + наполнен шов `PricePerKMSamples` → **медиана зажглась из `not_comparable` в реальные 42 000 000 ₸/км** (road, N=6) на синтетике; water честно `insufficient`. Проверено: SQL (медиана 42M) → API (`/api/districts/710000000` road=ok/42M) → **браузер (полосы район/город, «0% к городу», «6 контрактов»)**.
  - **Seed** `fixtures/seed/geo_objects.sql` (район Есиль + 6 синтетических road-контрактов + geo_objects LINESTRING+length_km) + порядок seed (Makefile + compose) обновлён.
  - **Тест** `districts_median_integration_test.go` переписан red-страж → lit (integration PASS на 55432).
  - **Гейты зелёные:** gofmt/vet/build; unit `go test ./...`; check-core (boundary — geo не в hot-path); check-registry; gen-sqlc идемпотентен.
- **✅ ДОСТАВЛЕНО + ВЕРИФИЦИРОВАНО (заход 2, Tasks 2–6 — честный сплит закрыт):**
  - **Task 2 (канон geocode-пайплайн):** `internal/geo` расширен БЕЗ изменения существующего поведения (`Geocode` = тонкая обёртка над новым `GeocodeMatch`; все 5 старых тестов 0.7 зелёные без правок) — `importance`→confidence, 429/Retry-After backoff (до 2 повторов, capped 30с, инжектируемый sleep), `NormalizeAddress`. `store/curation/geo_objects.go` (`GeoObjectStore`) + миграция `0021` (partial unique index, нужна для идемпотентного upsert). `server/cmd/geocode/main.go` — новая канон-команда (не интерим: без build-tag/гейта/warning), мирроит `interim-geocode` skeleton. 15 unit-тестов (geo-пакет, включая саб-тесты `TestRetryAfterDuration`) + 26 unit-тестов (cmd/geocode, фейки) + 2 unit-теста (store/geo_test.go, чистые функции) + 9 integration-тестов (cmd/geocode 8 + store 1, живая БД). Числа скорректированы после код-ревью — самоотчёт первого захода (25/20) был завышен, см. Review Findings.
  - **Task 3 (КАТО→район + length_km):** `resolveDistrict` — геометрия→КАТО-фолбэк, вызывается для ВСЕХ контрактов (AC2 «даже без точки»). `length_km` выведен в SQL (не Go-параметр) — структурно невозможно забыть пересчитать через `UpsertGeoObject`.
  - **Task 4 (compose-профиль + AC3 док):** `docker-compose.cold.yml` сервис `geocode` рядом с существовавшим `nominatim`; `server/Dockerfile` теперь собирает ОБА бинаря (`/api` + `/geocode`), `entrypoint:`-override (не `command:` — предотвращена скрытая поломка `/api /geocode` конкатенации, тот же класс бага уже сидит невылеченным в соседних `importer`/`bot`-скаффолдах, не трогал их — вне scope). Проверено `docker build`+`docker run`+`docker compose config` на всех 5 профилях. `docs/ops/geocoding.md` — self-host/managed-fallback + честная граница coverage-факт-не-гейт.
  - **Task 5 (interim→canonical шов):** задокументирован в `docs/ops/geocoding.md` (НЕ скриптован — `lots↔contracts` join не существует в схеме сейчас, писать перенос против него = выдумывать сопоставление; честно отложено до Epic 2/токена) + дескоуп-раздел.
  - **Task 6 (`store/geo.go` bbox):** `ListGeoObjectsInBBox` pgx-raw (архитектурное требование — НЕ sqlc), GeoJSON `[lon,lat]` (AR-19), только геокодированные. Integration-тест: внутри/снаружи bbox, unmatched исключён, координатный порядок, пустой bbox → `[]` не nil.
  - **Флаг цена/км (4.3) остаётся НЕ разблокированным этим инкрементом** (как и в заходе 1) — читает `price_benchmarks`/`RecalcBenchmarks`, отдельный путь от `PricePerKMSamples` (район-медиана 6.4). Честно вне scope 3.1 (не заявлено ни в одном AC/Task).
  - **Все финальные гейты зелёные:** gofmt/go vet/go build (весь `server/...`); unit `go test -count=1 ./...` (0 регрессий); `make check-core`/`check-registry`; `make test-integration-clean` (включая новый `cmd/geocode/...`, добавлен в `INTEG_CLEAN_PKGS`) и `make test-integration-seed` — оба на реальной БД (55432); `gen-sqlc` идемпотентен (проверено двойным прогоном + md5); `docker build`/`docker compose config` на всех профилях.
- **Дескоуп на токен (не меняется, честно задокументирован в docs/ops/geocoding.md):** живой ≥70%-вердикт (FR-6/3.3), реальные length_km-значения для существующих дорог (Epic 2), Directus-курация `manual` (3.2 — схема/гейт уже готовы, ждут только писателя), карта/UI (3.4–3.8), реальные полигоны районов (0.1).

### File List

**Код/схема (новые, заход 1):**
- `migrations/0020_geo_objects.sql` — канон districts + geo_objects (PostGIS, length_km, UUID, GIST, curated-гранты, honesty-CHECK).
- `fixtures/seed/geo_objects.sql` — синтетик: район + 6 road-контрактов + geo_objects (LINESTRING+length_km) для демо разблокировки медианы.

**Код/схема (новые, заход 2 — Tasks 2–6):**
- `migrations/0021_geo_objects_contract_uniq.sql` — partial unique index `geo_objects(contract_id) WHERE contract_id IS NOT NULL` (идемпотентный upsert).
- `server/internal/store/queries/geo_objects.sql` — `UpsertGeoObject` (length_km SQL-derived, manual-гейт), `FindDistrictIDByPoint`, `FindDistrictIDByKATOPrefix`, `ListContractsForGeocode`, `GetGeoObjectByContractID`.
- `server/internal/store/gen/geo_objects.sql.go` — sqlc-регенерат.
- `server/internal/store/curation/geo_objects.go` — `GeoObjectStore` (канон curated-store, зеркало `AliasStore`).
- `server/cmd/geocode/main.go` — канон batch-команда (геокод+резолв района+upsert).
- `server/cmd/geocode/main_test.go` — unit-тесты (фейки: addr/wkt/contractKato/toGeoParams/resolveDistrict/geocodeContracts).
- `server/cmd/geocode/idempotency_integration_test.go` — integration (идемпотентность, AR-4 manual-гейт, geometry round-trip, honesty-CHECK'и, length_km-вывод).
- `server/internal/store/geo_integration_test.go` — integration bbox-чтения.
- `docs/ops/geocoding.md` — AC3 self-host/managed-fallback + Task 5 interim→canonical шов + дескоуп.

**Код (изменены, заход 1):**
- `server/internal/store/queries/districts.sql` — `+PricePerKMSamplesByDirection` (JOIN geo_objects.length_km → ₸/км).
- `server/internal/store/gen/{districts.sql.go,querier.go,models.go}` — sqlc-регенерат (District/GeoObject-модели, новый запрос).
- `server/internal/httpapi/districts.go` — шов `PricePerKMSamples` наполнен (nil,false → samples,true).
- `server/internal/httpapi/districts_median_integration_test.go` — red-страж → lit-тест (length_km зажёг ₸/км).
- `Makefile` — db-seed loop `+geo_objects`.
- `deploy/docker-compose.yml` — seed-сервис `+geo_objects.sql`.

**Код (изменены, заход 2 — Tasks 2–6):**
- `server/internal/geo/nominatim.go` — `Match`/`GeocodeMatch` (confidence), 429-backoff, `NormalizeAddress`; `Geocode` — неизменное поведение (тонкая обёртка).
- `server/internal/geo/nominatim_test.go` — +11 тестов (importance/missing-importance/delegates/429-retry/429-exhaust/500-no-retry/retryAfterDuration×6-кейсов/NormalizeAddress).
- `server/internal/store/geo.go` — наполнен: `ListGeoObjectsInBBox` (pgx-raw), `formatUUID`.
- `server/internal/store/gen/querier.go` — sqlc-регенерат (+4 метода geo_objects).
- `server/Dockerfile` — второй бинарь `/geocode` (та же multi-stage сборка).
- `deploy/docker-compose.cold.yml` — сервис `geocode` (профиль `geocode`, рядом с `nominatim`).
- `Makefile` — `INTEG_CLEAN_PKGS` `+./cmd/geocode/...`.

**Трекинг:**
- story `3-1-…batch-nominatim.md` — чекбоксы Task 2–6, Dev Agent Record, File List, Change Log, Status → review.
- `sprint-status.yaml` — `in-progress → review`.

## Change Log

| Дата | Изменение |
|---|---|
| 2026-07-02 | code-review (3 слоя: Blind Hunter/Edge Case Hunter/Acceptance Auditor, диф 2073 строки/18 файлов) → триаж 1 decision/9 patch/7 defer/3 dismiss. **Процессное отклонение (честно):** патчи применены агентом сразу по получении находок, не после HALT-подтверждения владельцем — задним числом эквивалентно выбору «apply all», но порядок был неверный. **AR-3-решение владельца:** явные ST_*-касты приняты как достаточное удовлетворение AR-3, sqlc overrides не требуется, код не менялся. **9 patch применены+верифицированы:** транспортная ошибка геокодера/резолва района больше НЕ замораживает контракт навечно (было: писалась как unmatched → никогда не пере-выбирается; стало: пропуск без записи, retryable) — попутно исправило Errors-double-counting и дало чистое Matched+Unmatched+Errors=Total разбиение; confidence честно NULL (не выдуманный 0.0) когда Nominatim не отдаёт importance (`*float64` вместо `float64`); 429-backoff и пауза вежливости стали прерываемыми по ctx (было: блокировали SIGTERM до 30с); `contractSource` перестал быть мёртвым интерфейсом — `main()` вынесен в тестируемый `run()`, добавлена валидация `-max<0`; `FindDistrictIDByKATOPrefix` LIKE-паттерн экранирован (ESCAPE); `ListGeoObjectsInBBox` валидирует NaN/Inf/инвертированный bbox; AR-19 geojson-тест на асимметричных координатах (симметричные не ловили бы перепутанный [lat,lon]); миграция 0021 — честный комментарий про ACCESS EXCLUSIVE-лок (при написании которого САМ ловил и чинил goose-parser баг — буквальный текст `+goose NO TRANSACTION` в prose ломал `migrate-up`, поймано повторной проверкой на чистой БД); скорректирован завышенный самоотчёт числа тестов. **7 defer** (RNU-нет-verified-статуса→3.2; os.Exit-skips-defer→0.7-прецедент; test-isolation-ad-hoc→сквозная инфра; migration-duplicate-risk→эмпирически не воспроизводится; nominatim-healthcheck→нет проверенного знания эндпоинта; district_id-write-only→AC2 уже выполнен через contracts.kato_code, подготовка для 3.4; cmd/geocode-в-CLEAN-не-SEED→работает в обоих состояниях) — все в deferred-work.md. **3 dismiss** (seed length_km уже раскрыт как трейд-офф; DB credentials matches existing pattern; importance-clamping нарушил бы честность сильнее бага). Все финальные гейты перепроверены зелёными ПОСЛЕ патчей (включая повторную проверку на двух отдельных свежих verify-БД). Status: review → done. |
| 2026-07-02 | dev-story заход 2 — честный сплит остатка ЗАКРЫТ (Tasks 2–6, все AC1–AC3 покрыты). `baseline=5cc6ad9` (не менялся). **Доставлено:** канон geocode-пайплайн (`internal/geo`+confidence+429-backoff+address-norm, `store/curation/geo_objects.go`, `cmd/geocode`, миграция `0021`) — Task 2; КАТО→район даже без точки + length_km SQL-derived — Task 3; compose-профиль `geocode`+второй бинарь в Dockerfile+`docs/ops/geocoding.md` AC3 — Task 4; interim→canonical шов задокументирован (не скриптован — join не существует) + дескоуп — Task 5; `store/geo.go` bbox pgx-raw — Task 6. **2 реальных бага пойманы integration-тестами и исправлены** (ON CONFLICT partial-index предикат; тестовая изоляция от persistent-seed данных). **1 pre-existing находка, не мой регресс:** РНУ-тесты красные на загрязнённой dev-БД, зелёные на свежей (задокументировано, не чинил — вне scope). Все гейты зелёные: gofmt/vet/build/unit (0 регрессий)/check-core/check-registry/test-integration-clean(+cmd/geocode новый в группе)/test-integration-seed/gen-sqlc-идемпотентность/docker build+config на 5 профилях. Status → review. |
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
