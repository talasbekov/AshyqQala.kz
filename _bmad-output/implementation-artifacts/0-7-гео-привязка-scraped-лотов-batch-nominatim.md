---
baseline_commit: 7cdde03392c464954e4a86b29a14b8bfe9c9517c
---

# Story 0.7: ⏳ Гео-привязка scraped-лотов (batch-Nominatim)

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->
<!-- ⏳ ВРЕМЕННАЯ история трека «Парсер-мост» (Sprint Change Proposal 2026-06-20). Удаляется/замещается при получении токена (Epic 2/3 живой импорт + Гейт №0). -->

## Story

As a **команда платформы AshyqQala.kz**,
I want **геокодировать scraped-лоты Астаны (из проекции `lots`, 0.6) в гео-объекты через эфемерный
batch-Nominatim, за тем же флагом/изоляцией трека «Парсер-мост»**,
so that **реальные точки Астаны попадают на карту (0.8) ДО живого импорта, при этом негеокодированные
лоты остаются честно видимыми, а координаты никогда не выдумываются**.

> **Тип истории:** ⏳ ВРЕМЕННАЯ enabler-story Эпика 0, трек «Парсер-мост». Это **стадия `geo`** в
> несущем downstream `decode → lots → geo → map` (architecture.md:124), который НЕ меняется при swap
> `scrape → ows`. История **обратима**: интерим-команда геокодинга удаляется по критерию удаления
> (получен `GOSZAKUP_TOKEN`); сам механизм `internal/geo` и данные `geo_objects` — переиспользуются
> Epic 3 (Story 3.1), не выбрасываются. **Только гео-привязка лотов** — договоры/флаги/медианы ждут токен.

## Acceptance Criteria

**AC1 — Batch-Nominatim пишет координаты scraped-лотов в гео-таблицу; автопокрытие — ФАКТ, не гейт**
**Given** scraped-лоты в проекции `lots` (Story 0.6) с полями `title_ru`/`name_ru` (адресная строка) и `ref_kato`
**When** запущена эфемерная команда batch-геокодинга (механика Story 3.1, по AR-22) за флагом трека
**Then** для распознанного адреса пишется гео-объект (`geocode_status=auto`) с координатой; **доля
автопокрытия фиксируется как ФАКТ (НЕ гейт сейчас)** — печатается/логируется число + **систематические
пропуски отмечены** (например провал по новостройкам/пригородам — отдельной строкой, как Шаг B+ Гейта №0).
[Source: epics.md:775-789; architecture.md:468-474 (AR-22); architecture.md:489-493 (Шаг B+, распределение пропусков)]

**AC2 — Негеокодированный лот остаётся честно видимым; координата НЕ выдумывается**
**Given** лот, чей адрес Nominatim не сматчил
**When** batch завершился
**Then** лот **не получает выдуманную координату**: гео-объект помечается `geocode_status=unmatched`
(или строка не создаётся), лот остаётся доступен в списке/поиске; контейнер региона честно отражает
`container_state=not_geocoded`. Никогда не `0,0` и не «центр города» вместо реального матча.
[Source: epics.md:787-789; architecture.md:343 (data-state-ungeocoded, перечёркнутый пин); architecture.md:885-887 (container_state); architecture.md:546-549 (value_state geocode_pending/failed)]

**AC3 — Структурная изоляция и обратимость трека (производное от контракта «Парсер-мост»)**
**Given** несущий принцип изоляции временного кода и обратимости (как в Story 0.6)
**When** прогон CI-стража и swap `scrape → ows`
**Then** интерим-команда геокодинга **изолирована от прод-бинарей** (`cmd/api`/`cmd/importer` её не
импортируют — go-list-страж, как AC2 у 0.6); зафиксирован **критерий удаления** (получен `GOSZAKUP_TOKEN`)
и путь восстановления §6.1/§6.4; механизм `internal/geo` и данные `geo_objects` при удалении интерим-команды
**сохраняются** (downstream не выбрасывается).
[Source: epics.md:747-753, 771-773; architecture.md:197-200; sprint-change-proposal-2026-06-20.md:103-107, 177]

## Tasks / Subtasks

- [x] **Task 0 — ПРЕДУСЛОВИЕ/РЕШЕНИЕ: целевая гео-таблица и КАТО→район (ВОЗМОЖНЫЙ БЛОКЕР — см. «Зависимости»)** *(AC1, AC2)*
  - [x] Подтвердить факт: **`geo_objects` и `districts` НЕ существуют** (миграции только `0001_extensions`/`0002_projection`/`0003_projection_lots`; `server/internal/geo/` — пустой `doc.go`; `server/internal/store/geo.go` — заглушка). PostGIS уже включён (`0001_extensions.sql`).
  - [x] **Решение владельцу (рекомендация дева ниже, см. «Открытые вопросы»):** (а) построить **минимальную** интерим-таблицу гео под трек «Парсер-мост» (кураторская граница AR-4, с **nullable `lot_id`** — договоров нет), ИЛИ (б) HALT-предусловие на Story 3.1 (канонический `geo_objects` — скоуп Epic 3). **Рекомендация: (а) минимальная, чётко помеченная «interim», обратимая.**
  - [x] **Де-скоуп КАТО→район:** полноценные полигоны `districts` Астаны — скоуп Epic 3 (3.1). В 0.7 — **только lat/lon** (точка), `district_id` оставить `NULL`/отложить; присвоение района по КАТО-коду — Epic 3. НЕ строить полигоны районов здесь.
  - [x] Если владелец выбирает HALT — зафиксировать и остановиться (как делала 0.6 до Story 2.0). Иначе — продолжить с минимальной интерим-таблицей.
- [x] **Task 1 — Перенести Nominatim-клиент в `server/internal/geo/` (AC1)**
  - [x] Перенести `stage0-audit/geocoder.go` → `server/internal/geo/nominatim.go` (модуль stage0 НЕ импортируется — перенос концепта, как `decode`/`scrape`): интерфейс `Geocoder` (`Geocode(query) (lat, lon float64, ok bool, err error)`), `nominatim` + `NewNominatim(base, ua, viewbox, bounded)`, freeform `/search?q=&format=json&limit=1&countrycodes=kz&viewbox=&bounded=1`, парс `lat`/`lon`-строк, helper `truncate`.
  - [x] Перенести константу Astana viewbox из `stage0-audit/config.go:30` (`"71.20,51.30,71.78,51.00"`) — пометить `VERIFY` (bbox Астаны).
  - [x] (опц.) добавить парс `importance` из ответа Nominatim → `confidence` гео-объекта (в исходном клиенте confidence НЕ парсится — `ok` = «≥1 результат»).
- [x] **Task 2 — Минимальная кураторская гео-таблица + store (AC1, AC2)** *(зависит от Task 0 решения «а»)*
  - [x] Миграция (например `migrations/0004_curated_geo_lots.sql`, **goose Up/Down**) — интерим-таблица гео под лоты: `id` (bigint identity), `lot_id` (FK→`lots.id`, **nullable** до закрепления связи), `geom geometry(Point,4326)` (**nullable** для unmatched), `kato_code TEXT`, `geocode_status TEXT` (`auto`/`unmatched`), `confidence DOUBLE PRECISION` nullable, `address_text TEXT`, `geocoded_at TIMESTAMPTZ`. **Кураторская граница (AR-4):** отдельная миграция/гранты от проекционных; импортёр только читает. Пометить таблицу комментарием «INTERIM, трек Парсер-мост; канонический `geo_objects` — Epic 3».
  - [x] sqlc-запросы + `store`-обёртка (зеркалит `projection.LotStore`): `UpsertGeoLot` (идемпотентно по `lot_id`), при необходимости `GetGeoByLotID`.
  - [x] Честность: unmatched → `geocode_status=unmatched`, `geom=NULL` (НЕ `0,0`); `confidence=NULL` если не парсится.
- [x] **Task 3 — Запрос перечисления лотов для геокодинга (AC1)**
  - [x] Добавить `ListLots :many` (или `ListUngeocodedLots`) в `server/internal/store/queries/lots.sql` (фильтр `NOT is_deleted`), регенерировать sqlc (v1.31, через Docker — см. Makefile `gen-sqlc`), выставить на `LotStore`. Сейчас есть только `GetLotByID`/`UpsertLot` — итерации нет.
- [x] **Task 4 — Эфемерная команда batch-геокодинга за флагом и warning (AC1, AC3)**
  - [x] Создать `tools/`-команду (зеркалит `server/tools/scrape/cmd/interim-import/`): **build-tag `//go:build scrape`** (тот же трек → вне прод-бинаря), гейт `ASHYQQALA_INTERIM_SCRAPE=1` (без флага — отказ exit 2), **громкий warning** §6.1/§6.4 + критерий удаления, fail-fast `pool.Ping` ДО геокодинга.
  - [x] Цикл: `ListLots` → для каждого `Geocode(title_ru)` с **вежливой паузой** (≥1100мс для публичного Nominatim; для self-host — меньше) → `UpsertGeoLot` (matched → точка+`auto`; no-match → `unmatched`).
  - [x] Конфиг геокодера (`NOMINATIM_URL`, опц. UA/delay/viewbox) — читать через `os.Getenv` в команде (НЕ расширять `cmd/api` `config.Config` — он намеренно узкий). По AR-22 целевой Nominatim — **self-host эфемерный** (профиль `geocode`), публичный — только для разовой выборки.
- [x] **Task 5 — Автопокрытие как ФАКТ + распределение пропусков (AC1)**
  - [x] Посчитать и **вывести** долю `matched/total` как ЧИСЛО (лог/stdout), **без гейта/exit-кода** (в отличие от Гейта №0 / Story 0.2). Зафиксировать систематические пропуски (например доля unmatched по типу адреса) — честная диагностика, не вердикт.
  - [x] Пометить, что выборка **keyword-смещена** → доля непоказательна (метить «предв.»); это не репрезентативный гео-% для FR-6 (тот — Гейт №0 на живых данных, Story 0.3).
- [x] **Task 6 — Честная видимость негеокодированного (AC2)**
  - [x] Убедиться, что unmatched-лот остаётся в `lots` и доступен (список/поиск появятся в 0.8/Epic 6) — 0.7 НЕ удаляет лот и НЕ ставит фейк-координату. Гео-объект `unmatched` (или его отсутствие) → потребитель (0.8) показывает «без точки на карте» / `container_state=not_geocoded`.
  - [x] Зафиксировать контракт для 0.8: как отличать matched (точка) от unmatched (честное состояние).
- [x] **Task 7 — Тесты (детерминированно, без живой сети)** *(AC1, AC2)*
  - [x] Юнит-тест Nominatim-клиента на **записанном JSON-ответе** (httptest-сервер или фикстура): matched → (lat,lon,true); пустой ответ → (0,0,false,nil) — НЕ ошибка; HTTP-ошибка → err. Без живого запроса в тесте.
  - [x] Тест маппинга matched/unmatched → `UpsertGeoLot`-параметры: unmatched → `geom=NULL`/`unmatched`, НЕ `0,0` (гардрейл честности).
  - [x] Интеграционный (`//go:build … && integration`, skip без `DATABASE_URL`): `ListLots` → geocode(stub) → `UpsertGeoLot` идемпотентно (повтор не плодит дубли).
  - [x] Тест гейта флага команды + содержимого warning (как у 0.6).
  - [x] Архитектурный go-list-страж: прод-бинари не тянут команду геокодинга (расширить/переиспользовать `internal/arch`); `go build/vet/test/gofmt` (дефолт + `-tags scrape`) зелёные.
- [x] **Task 8 — Обратимость, критерий удаления, доки (AC3)**
  - [x] Зафиксировать в `doc.go`/`docs/ops/` (рядом с `interim-scrape-bridge.md`): критерий удаления интерим-команды геокодинга = «получен `GOSZAKUP_TOKEN`»; `internal/geo`+данные сохраняются; путь восстановления §6.1/§6.4. Обновить `docs/ops/interim-scrape-bridge.md` разделом про гео-стадию.

## Dev Notes

### Контекст истории (зачем и почему временная)

0.7 — стадия **`geo`** временного конвейера «Парсер-мост»: 0.6 положила scraped-лоты в проекцию `lots`;
0.7 геокодирует их в гео-объекты; 0.8 покажет на карте. Это **тонкий вынос механики Story 3.1
(batch-Nominatim) на интерим-данные**, эфемерно, до получения токена. Цель — ранняя осязаемая ценность
(реальные точки Астаны на карте) без живого импорта, при строгой честности (негеокодированное видимо,
координаты не выдумываются) и обратимости (swap на `ows` не выбрасывает downstream).
[Source: epics.md:775-789; sprint-change-proposal-2026-06-20.md:134-137; architecture.md:124, 192-200]

### ⚠️ Зависимости и предусловия (КРИТИЧНО — Task 0)

0.7 ссылается на канонический `geo_objects` (Epic 3), которого **сейчас нет**. Состояние фундамента:

| Нужный компонент | Статус сейчас | Канонический владелец |
|---|---|---|
| `server/internal/geo/` (batch-геокодер) | 🟡 пустой `doc.go` — **это и есть тело 0.7** | Story 0.7 (строим) |
| Nominatim-клиент | ✅ есть в `stage0-audit/geocoder.go` (перенести) | перенос |
| `geo_objects` (кураторская гео-таблица) | 🔴 НЕ существует (миграций нет) | **Epic 3 (Story 3.1/3.2)** |
| `districts` (KATO-полигоны Астаны) + КАТО→район | 🔴 НЕ существует вовсе | **Epic 3 (Story 3.1/3.2)** |
| `ListLots` (перечисление лотов) | 🔴 нет (только `GetLotByID`/`UpsertLot`) | строим (мелочь) |
| Конфиг геокодера (`NOMINATIM_URL`) | 🔴 нет (`config` узкий, только api) | строим в команде |

> **Вывод для dev-агента:** как и у 0.6, есть **Task 0-предусловие**. Два несущих компонента
> (`geo_objects`, `districts`) — канонически Epic 3. **Рекомендация (см. «Открытые вопросы»):** построить
> МИНИМАЛЬНУЮ интерим-гео-таблицу под лоты (nullable `lot_id`, lat/lon-only, кураторская граница),
> чётко помеченную «interim/Парсер-мост», обратимую; КАТО→район — де-скоупить в Epic 3. НЕ строить
> канонический `geo_objects`/полигоны районов здесь (анти-churn: это схема-авторитет Epic 3). Если
> владелец предпочитает не предвосхищать схему Epic 3 — HALT-предусловие на 3.1 (честный исход, как 0.6
> до Story 2.0). Честность над домыслом: не имитировать несуществующую схему молча.
[Source: data-model:49 (geo_objects); epics.md:1222-1240 (Story 3.1); architecture.md:711, 778 (curated 0003); migrations/ (факт отсутствия)]

### Что переиспользовать (НЕ изобретать заново)

- **Nominatim-клиент — `stage0-audit/geocoder.go`** (перенос в `internal/geo`): `Geocoder.Geocode(query) (lat, lon, ok, err)`; `NewNominatim(base, ua, viewbox, bounded)` (дефолты: `nominatim.openstreetmap.org`, UA, timeout 20s); freeform `/search?q=` + `countrycodes=kz` + `viewbox`/`bounded`; lat/lon — **строки** в ответе → `ParseFloat`; пустой результат → `(0,0,false,nil)` (НЕ ошибка); confidence НЕ парсится (можно добавить `importance`). [Source: stage0-audit/geocoder.go:15-88]
- **Astana viewbox** — `stage0-audit/config.go:30` `DefaultAstanaViewbox = "71.20,51.30,71.78,51.00"` (`lon_left,lat_top,lon_right,lat_bottom`), VERIFY. [Source: stage0-audit/config.go:30]
- **Эфемерная команда-скелет — `server/tools/scrape/cmd/interim-import/main.go`** (Story 0.6): build-tag `//go:build scrape`, гейт-флаг `ASHYQQALA_INTERIM_SCRAPE=1` + `loudWarning` §6.1/§6.4 + критерий удаления, `pgxpool.New`+fail-fast `Ping`, `DATABASE_URL` через `os.Getenv`, NULL-честный pgtype-маппинг. **Зеркалить 1:1.** [Source: server/tools/scrape/cmd/interim-import/main.go]
- **Lots read shape** — `projection.LotStore`/`gen.Lot` (`title_ru`/`title_kk`/`kato_code` — вход геокодинга); `politeDelayDefault = 1100ms` (`server/tools/scrape/scrape.go:23`) — прецедент вежливой паузы. [Source: server/internal/store/projection/lots.go; server/internal/store/gen/models.go]
- **go-list-страж изоляции** — `server/internal/arch/boundaries_test.go` (`TestHotPathDoesNotImportScrape` + чистый предикат + negative-control) — расширить на команду геокодинга. [Source: server/internal/arch/boundaries_test.go]

### Соблюдение архитектуры (guardrails)

- **AR-22 — эфемерный batch-Nominatim, НЕ 24/7.** Геокодер — холодный/batch (профиль `geocode`), не в горячем пути; результат кэшируется в гео-таблице, контейнер гасится. Self-host (адреса не уходят наружу — суверенность) или managed-fallback — оба дают тот же артефакт. [Source: architecture.md:462-474; epics.md:260-265]
- **AR-4 — кураторская ⊥ проекционная граница.** Гео-таблица — **кураторская** (отдельные миграция/гранты; импортёр только читает). Команда геокодинга — НЕ импортёр (batch-запись гео допустима). 0.6-импортёр лотов гео НЕ пишет. [Source: epics.md:197-199; architecture.md:308-313]
- **Честность над домыслом (несущий).** Negеокодированное → `unmatched`/`geom=NULL` + `container_state=not_geocoded` + «перечёркнутый пин» в UI (0.8); НИКОГДА не выдуманная координата (`0,0`/центр города). [Source: architecture.md:343, 546-549, 885-887; prd.md гардрейл честности]
- **Автопокрытие = ФАКТ, не гейт.** В отличие от Гейта №0 (Story 0.2/0.3, exit-код на `<0.70`), 0.7 только репортит число. Реальный серверный гейт автопокрытия — Story 3.3 (per-район `geo_coverage`). Причины: Гейт №0 токен-блокирован/отложен; выборка keyword-смещена → её % непоказателен. [Source: epics.md:785; sprint-change-proposal:135; architecture.md:493; epics.md:1262-1276]
- **Keyword-bias «предв.».** Выборка scrape отобрана словами направлений → доля направлений непоказательна; гео-% интерима не репрезентативен — метить «предв.». [Source: architecture.md:194-195; sprint-change-proposal:31-32]
- **Изоляция + обратимость (AC3).** build-tag трека `scrape`, go-list-страж; критерий удаления = токен; `internal/geo`+данные сохраняются, удаляется только интерим-команда. [Source: architecture.md:197-200; sprint-change-proposal:177]
- **Только stdlib в `internal/geo`-клиенте** (как stage0): `net/http`, `encoding/json`, `net/url`, `strconv`, `time`. Запись в БД — через pgx (уже в модуле). [Source: stage0-audit/geocoder.go imports]

### Модель данных (куда пишем)

Канонический `geo_objects` (Epic 3): `id, contract_id(FK), geom(POINT|LINESTRING), district_id(FK),
address_text, length_km, geocode_status(auto/manual/unmatched), confidence, geocoded_by, geocoded_at`
(data-model:49). **Несоответствие для интерима:** канон FK-ит к `contract_id`, а у нас лоты без
договоров → нужна связь по **`lot_id`** (nullable). Поэтому интерим-таблица минимальна и помечена
«interim»; канонический `geo_objects` со всеми FK строит Epic 3. `length_km`/LINESTRING (цена/км, флаг 2)
— токен-блокированы, в 0.7 дремлют (точки POINT достаточно). [Source: data-model:49; epics.md:1230]

### Файлы и куда писать результат

- **Создаём (NEW):** `server/internal/geo/nominatim.go` (+ `nominatim_test.go`); `server/internal/geo/` доменная запись гео (по решению Task 0); миграция `migrations/0004_curated_geo_lots.sql` (интерим); sqlc-запросы (`queries/geo_lots.sql` или в `lots.sql`); `tools/`-команда геокодинга (build-tag `scrape`) + тесты; (опц.) фикстура Nominatim-ответа в `testdata/`.
- **Меняем (UPDATE):** `server/internal/store/queries/lots.sql` (+`ListLots`); `server/internal/arch/boundaries_test.go` (страж новой команды); `.github/workflows/ci-server.yml` (если новый build-tag/шаг); `docs/ops/interim-scrape-bridge.md` (раздел гео).
- **НЕ трогать:** `stage0-audit/` (источник переноса); проекционный путь 0.6 (`tools/scrape`); прод-бинари (`cmd/api`/`cmd/importer`).
- **НЕ создавать:** канонический `geo_objects` со всеми FK, полигоны `districts`, Directus-очередь гео (3.2), серверный гейт автопокрытия (3.3), полную карту (3.4 → это 0.8). Анти-churn: это скоуп Epic 3.

### Тестирование / проверка готовности

- **Детерминизм без сети:** Nominatim-клиент тестировать на `httptest`-сервере / записанном JSON (matched/empty/HTTP-error). Живой Nominatim — отдельно, вручную, под флагом (вежливый delay).
- **Гардрейл-тест:** unmatched → `geom=NULL`/`unmatched`, не `0,0`.
- **Идемпотентность:** интеграционный (`integration`-tag, skip без `DATABASE_URL`) — повтор не плодит дубли.
- **Изоляция:** go-list-страж (red при импорте команды из прода) + `-count=1`.
- **DoD:** AC1–AC3 закрыты; автопокрытие репортится числом (без гейта); негеокодированное честно видимо; build/vet/test/gofmt (дефолт + `-tags scrape`) зелёные; критерий удаления зафиксирован.

### Previous Story Intelligence (0.6 — прямой предшественник, и 0.2)

- **0.6 (done):** положила scraped-лоты в `lots` за `//go:build scrape` + гейт `ASHYQQALA_INTERIM_SCRAPE=1` + `loudWarning`; синтетический стабильный `goszakup_lot_id` (`scrape-<sha256-32>`); CI-страж изоляции `TestHotPathDoesNotImportScrape` (red доказан); `docs/ops/interim-scrape-bridge.md`. **0.7 зеркалит этот паттерн** (тот же трек, флаг, изоляция, обратимость). Ревью 0.6 дало defer-уроки (в `deferred-work.md`): честность сумм→NULL, fail-fast Ping, value-robustness→2.1 — наследовать стиль честности. [Source: 0-6-…md; deferred-work.md]
- **0.2 (done):** машинный Гейт №0 (`-format json`, exit-код на `geo<0.70`). 0.7 НЕ гейтит — контраст важен: 0.7 репортит факт, 0.2/0.3 — гейт на живых данных. [Source: 0-2-…md]
- **Гардрейл честности 0.1/0.6:** неподтверждённое → честное состояние, не выдумка. Наследовать в гео (`unmatched`, не `0,0`). [Source: memory data-source-interim-parser-first; guards-must-prove-red]

### Project Structure Notes

- **Прямой конфликт с текущим состоянием:** канонические `geo_objects`/`districts` (Epic 3) не существуют; 0.7 пулит механику 3.1 вперёд эфемерно. Разрешается Task 0 (минимальная интерим-таблица + де-скоуп района) — не дублировать схему-авторитет Epic 3, помечать «interim», держать обратимым.
- Геокодер — `internal/geo` (домен), команда — `tools/` за build-tag (вне горячего пути). Согласованность downstream с боевым геокодингом (Epic 3) — через ту же `internal/geo`-механику и кэш гео-результатов.
- `server/internal/store/geo.go` — заглушка S-0 («реализация в 3.x»); 0.7 либо наполняет минимально (интерим), либо рядом — по Task 0.

### Внешние знания / web-research (осознанно ограничено)

История не вводит новых библиотек: Nominatim-клиент — stdlib (перенос); запись — pgx (в модуле). Nominatim
`/search` API (`q`, `format=json`, `viewbox`, `bounded`, `countrycodes`) **зашит в проверенный
`geocoder.go`** — не брать из памяти модели. Self-host Nominatim (`mediagis/nominatim`, osmium-extract,
`IMPORT_STYLE`) — ops-механика AR-22, при реальном self-host прогоне сверять с актуальным образом; для
тестов 0.7 живой Nominatim не нужен (httptest-фикстура). [Source: architecture.md:468-474; stage0-audit/geocoder.go]

## Открытые вопросы / решения владельцу (зафиксировать до/во время dev)

1. **Целевая гео-таблица (Task 0, несущее).** Рекоменд.: **минимальная интерим-таблица** (`0004_curated_geo_lots`, nullable `lot_id`, lat/lon-only, кураторская граница, помечена «interim») — НЕ канонический `geo_objects` (схема-авторитет Epic 3/3.1). Альтернатива — HALT-предусловие на 3.1. **Рекомендуется минимальная интерим (обратимая).**
2. **КАТО→район.** Рекоменд.: **де-скоупить** в Epic 3 (полигоны `districts` — тяжёлая токен/Epic-3 работа); в 0.7 только точка lat/lon, `district_id=NULL`. Подтвердить.
3. **Связь гео↔лот.** `lot_id` (nullable FK) как интерим-связь (договоров нет). Подтвердить, что не отравляет токен-схему (канон Epic 3 заменит).
4. **Self-host vs managed Nominatim.** AR-22 — self-host эфемерный (суверенность); managed — документированный фолбэк. Для 0.7 (механика + клиент) выбор инфраструктуры можно отложить: клиент работает с любым `NOMINATIM_URL`. Подтвердить дефолт для интерим-прогона.
5. **Build-tag.** Переиспользовать `scrape` (тот же трек) или ввести `interim`? Рекоменд.: **тот же `scrape`** (один трек «Парсер-мост», один страж). Подтвердить.
6. **confidence.** Парсить `importance` Nominatim в `confidence` или оставить `NULL` для интерима? Рекоменд.: для интерима достаточно `NULL` (точность не используется до Epic 3/3.2). Подтвердить.

## Dev Agent Record

### Agent Model Used

claude-opus-4-8[1m] (Claude Opus 4.8, 1M context) — create-story (context engine).

### Debug Log References

- Task 0 (предусловие/решение, 2026-06-23): `geo_objects`/`districts` отсутствуют (миграции 0001-0003); `internal/geo` — пустой `doc.go`. **Владелец принял рекомендованные дефолты** (dev-story с rec-defaults): минимальная интерим-таблица + де-скоуп района → НЕ HALT.
- Тулинг: sqlc регенерация через Docker (`make gen-sqlc`, sqlc 1.31.0) — РАБОТАЕТ и воспроизводима (no-op regen → пустой diff). Docker доступен.
- TDD geo-клиент: `nominatim_test.go` (httptest) → matched/empty(ok=false)/HTTP-error/bad-JSON — 4/4 PASS. При переносе исправлены проглоченные ошибки `io.ReadAll`/`ParseFloat` (честность, как ревью 0.6).
- sqlc после `0004_interim_geo_lots.sql` + `geo_lots.sql` + `ListLots`: сгенерированы `InterimGeoLot`, `UpsertGeoLotParams` (Lat/Lon/Confidence `pgtype.Float8`), `ListLots`. `go build ./...` зелёный.
- Команда `interim-geocode` (tags scrape): гейт+warning+honesty+coverage — тесты PASS (после правки warning: добавлен литерал «keyword-bias»). Интеграционный (`scrape integration`) компилируется и скипается без `DATABASE_URL`.
- Финал: дефолт `gofmt`/`vet`/`build`/`test` (geo+store+arch, без регрессий); `-tags scrape` build/vet/test (3 пакета); `-count=1 ./internal/arch/...` (страж покрывает interim-geocode) — всё зелёное.

### Completion Notes List

- **РЕАЛИЗОВАНО (AC1–AC3 закрыты).** Task 0 разрешён владельцем (rec-defaults): построена МИНИМАЛЬНАЯ интерим-гео-таблица `interim_geo_lots` (lat/lon `DOUBLE`, не PostGIS-geometry; связь по стабильному `goszakup_lot_id`, без hard FK к проекции — AR-4), НЕ канонический `geo_objects` (Epic 3). КАТО→район де-скоуплен в Epic 3 (только точка).
- **AC1** — команда `interim-geocode` (build-tag `scrape`, гейт `ASHYQQALA_INTERIM_SCRAPE=1` + warning §6.1/§6.4 + критерий удаления, fail-fast Ping): `ListLots` → `geo.Nominatim.Geocode(title_ru)` → `UpsertGeoLot`; **автопокрытие печатается ЧИСЛОМ, без exit-гейта** (контраст с Гейтом №0/0.2), помечено «предв., keyword-bias».
- **AC2** — честность: unmatched → `geocode_status=unmatched`, `lat/lon=NULL` (НИКОГДА не 0,0); пустой `title` не геокодится; тест `TestGeocodeLots_CoverageAndHonesty` + `TestToGeoParams_UnmatchedIsNull` доказывают.
- **AC3** — изоляция: весь `tools/scrape` (вкл. `interim-geocode`) за `//go:build scrape`; go-list-страж `TestHotPathDoesNotImportScrape` (prefix `tools/scrape` уже покрывает; добавлен явный positive-case в negative-control); критерий удаления + обратимость в `doc`/`docs/ops/interim-scrape-bridge.md`; `internal/geo` сохраняется (переиспользует Epic 3).
- **Решения дева:** (1) интерим-таблица `interim_geo_lots` (lat/lon doubles, не geometry — проще для sqlc; ключ `goszakup_lot_id`). (2) команда под `tools/scrape/cmd/interim-geocode` (тот же трек/тег/страж — ноль изменений стража). (3) `internal/geo` БЕЗ build-tag (механизм для Epic 3); только команда тегирована. (4) перенос geocoder.go с починкой проглоченных ошибок. (5) `confidence=NULL` (importance не парсится, Q6). (6) CI не менялся — существующий `-tags scrape ./tools/scrape/...` уже ловит новую команду, `internal/geo` — в дефолтном `go test ./...`.
- **Не выполнено в этой среде (внешние ресурсы, не блокер review):** живой Nominatim-прогон (сеть/вежливость/self-host AR-22) и интеграционный тест в реальной БД (`-tags 'scrape integration'`, нужен `DATABASE_URL` + миграции 0001-0004). Оба компилируются/скипаются чисто; логика покрыта детерминированными юнит-тестами (httptest + fakes).

### File List

- `server/internal/geo/nominatim.go` — **новый.** Порт Nominatim-клиента (`Geocoder`, `Nominatim`, `Geocode`, `AstanaViewbox`, `PoliteDelayDefault`, `truncate`); без build-tag (переиспользует Epic 3).
- `server/internal/geo/nominatim_test.go` — **новый.** httptest-тесты клиента (matched/empty/HTTP-error/bad-JSON).
- `server/internal/geo/doc.go` — **удалён** (заглушка; пакетная док-строка переехала в `nominatim.go`).
- `migrations/0004_interim_geo_lots.sql` — **новый.** Минимальная интерим-гео-таблица `interim_geo_lots` (goose Up/Down).
- `server/internal/store/queries/geo_lots.sql` — **новый.** `UpsertGeoLot`/`GetGeoLotByLotID` (sqlc).
- `server/internal/store/queries/lots.sql` — изменён: `+ListLots :many`.
- `server/internal/store/gen/*` — **регенерировано sqlc** (`geo_lots.sql.go` новый; `lots.sql.go`/`models.go`/`querier.go` обновлены — `ListLots`, `UpsertGeoLot`, `InterimGeoLot`). НЕ править руками.
- `server/internal/store/projection/geo_lots.go` — **новый.** `GeoLotStore` (`UpsertGeoLot`/`GetGeoLotByLotID`).
- `server/internal/store/projection/lots.go` — изменён: `+ListLots`.
- `server/tools/scrape/cmd/interim-geocode/main.go` — **новый** (`//go:build scrape`). Команда batch-геокодинга (гейт/warning/Ping/цикл/coverage); helpers `interimEnabled`/`loudWarning`/`addr`/`toGeoParams`/`geocodeLots`.
- `server/tools/scrape/cmd/interim-geocode/main_test.go` — **новый** (`//go:build scrape`). Гейт, warning, honesty-маппер, цикл-покрытие (fakes).
- `server/tools/scrape/cmd/interim-geocode/idempotency_integration_test.go` — **новый** (`//go:build scrape && integration`). Идемпотентность в БД (skip без `DATABASE_URL`).
- `server/internal/arch/boundaries_test.go` — изменён: `interim-geocode` добавлен в positive-cases negative-control стража изоляции.
- `docs/ops/interim-scrape-bridge.md` — изменён: раздел «Гео-стадия (0.7)» + критерий удаления.
- `_bmad-output/implementation-artifacts/0-7-…md` — изменён: frontmatter `baseline_commit`, чекбоксы, Dev Agent Record, File List, Change Log, Status → review.
- `_bmad-output/implementation-artifacts/sprint-status.yaml` — изменён: `ready-for-dev → in-progress → review`.

## Change Log

| Дата | Изменение |
|---|---|
| 2026-06-23 | Создан context engine (create-story): двух-агентная аналитика архитектуры/кода; зафиксирован Task-0-выбор (минимальная интерим-гео-таблица vs HALT на 3.1) + 6 открытых вопросов. Статус → ready-for-dev. |
| 2026-06-23 | dev-story (rec-defaults). `baseline_commit=7cdde03`. Task 0 разрешён владельцем (минимальная интерим-таблица, де-скоуп района). Реализованы Task 1–8: порт Nominatim-клиента в `internal/geo` (+httptest); миграция `0004_interim_geo_lots` + sqlc (`UpsertGeoLot`/`ListLots`/`InterimGeoLot`); `GeoLotStore`+`ListLots`; команда `interim-geocode` (build-tag `scrape`, гейт+warning+Ping, цикл geocode→upsert, **автопокрытие как ФАКТ без гейта**, честный unmatched→NULL); страж изоляции расширен; интеграционный тест; `docs/ops/interim-scrape-bridge.md` (гео-стадия). Дефолт + `-tags scrape` + `-count=1 arch` зелёные. AC1–AC3 закрыты. Статус → review. |

### Change Log (доп.)

| Дата | Изменение |
|---|---|
| 2026-06-23 | Code review (3 слоя). AC2/AC3 MET, AC1 был PARTIAL. Применены 5 patch: NaN/Inf/вне-диапазона→ok=false (честность); систематические-пропуски-строка (AC1 закрыт); ctx-протяжка + signal.NotifyContext; `-max`+пустой-lots+operational-outage-exit; миграция CHECK + address_text=реальный-запрос + io.LimitReader + sleep-on-network. 3 defer → `deferred-work.md` (лоссовый адрес, 429-backoff, неатомарный batch). Все проверки зелёные. **Статус → done.** |

## Review Findings (Code Review — 2026-06-23)

> Адверсариальное ревью (3 слоя). AC2/AC3 — **MET**; **AC1 — PARTIAL** (нет распределения систематических пропусков). 5 patch, 3 defer, ~3 dismissed. Acceptance Auditor: «substantially met, fix AC1 или owner-descope».

### Patch (все 5 применены 2026-06-23 — дефолт + `-tags scrape` + `-count=1 arch` + sqlc-регенерация зелёные)

- [x] [Review][Patch] **Честность:** `Geocode` отбраковывает NaN/Inf (`ParseFloat` их принимает!) + вне-диапазона lat∉[-90,90]/lon∉[-180,180] → `ok=false` (не выдуманная точка). Тест `TestGeocode_NonFiniteOrOutOfRange_NotMatch` ✅ [server/internal/geo/nominatim.go]
- [x] [Review][Patch] **AC1 закрыт:** `geocodeLots` собирает образцы unmatched-названий; команда печатает строку «систематические пропуски: N; примеры…»; coverage-строка помечает «без точки (вкл. ошибок=M)» ✅ [tools/scrape/cmd/interim-geocode/main.go]
- [x] [Review][Patch] ctx протянут: `Geocode(ctx,q)` + `NewRequestWithContext`; `signal.NotifyContext(SIGINT/SIGTERM)`; цикл проверяет `ctx.Err()` → Ctrl-C прерывает чисто ✅ [nominatim.go, main.go]
- [x] [Review][Patch] `-max`-флаг (cap лотов) + диагностика пустого `lots` + **operational-exit** при `Errors==Total` (геокодер недоступен ≠ честный 0%; НЕ coverage-гейт) ✅ [main.go]
- [x] [Review][Patch] миграция: `CHECK(geocode_status IN ('auto','unmatched'))` + `CHECK((lat IS NULL)=(lon IS NULL))` (honesty в БД); `address_text` = реальный запрос (воспроизводимость, тест); `io.LimitReader` 1MB; `time.Sleep` только после сетевого запроса ✅ [migrations/0004…sql, nominatim.go, main.go]

### Defer

- [x] [Review][Defer] `addr()` приклеивает «, Астана» к названию лота (не адресу) — геокодинг procurement-тайтлов лоссов по сути; покрыто «предв./keyword-bias» + coverage-как-факт [tools/.../main.go] — deferred → качество адреса решает ows/Epic 3
- [x] [Review][Defer] HTTP 429/Retry-After без backoff (публичный Nominatim может забанить) [server/internal/geo/nominatim.go] — deferred → self-host AR-22 / Epic 3 (для интерима — delay + outage-exit)
- [x] [Review][Defer] неатомарный partial-write при падении БД в середине batch (повтор идемпотентен по `goszakup_lot_id`) [tools/.../main.go] — deferred, приемлемо для интерима

### Dismissed

- `internal/geo` не покрыт стражем от импорта scrape — теоретично (geo — переиспользуемый механизм, scrape он импортировать не станет; ничего из прода geo пока не тянет).
- header/query-инъекция через title — `url.Values.Encode()` экранирует (clean). · SQL-инъекция — sqlc `$N` (clean). · honesty-маппер NULL-vs-(0,0) — корректен (Valid-флаг). · `gen/*` — sqlc-генерат (вне ревью).

## References

- [Source: epics.md:775-789 — Story 0.7 (текст + AC); трек «Парсер-мост» 747-753]
- [Source: epics.md:1222-1240 — Story 3.1 (механика batch-Nominatim, geo_objects, КАТО→район); 1262-1276 — Story 3.3 (реальный серверный гейт)]
- [Source: architecture.md:468-474 — AR-22 эфемерный Nominatim batch; 462-467 — AR-21 hot/cold, профиль geocode]
- [Source: architecture.md:308-313, 397-398 — AR-4 кураторская⊥проекционная, гранты; 711, 778 — curated миграция 0003]
- [Source: architecture.md:343 — data-state-ungeocoded (перечёркнутый пин); 546-549 — value_state geocode_pending/failed; 885-887 — container_state=not_geocoded]
- [Source: architecture.md:171, 488-493 — ≥70%-гейт (FR-6/OQ-4), Шаг B+, распределение пропусков; 194-195 — keyword-bias «предв.»]
- [Source: architecture.md:124, 192-200 — downstream decode→lots→geo→map, обратимость swap; 462-474 — гео-инфра]
- [Source: sprint-change-proposal-2026-06-20.md:134-137 — Story 0.7 (факт-не-гейт, предв.); 103-107, 177 — путь восстановления/удаление интерим]
- [Source: docs/AshyqQala_MVP_data_model_and_flags_v1.md:49 — geo_objects колонки; 85 — length_km→цена/км]
- [Source: stage0-audit/geocoder.go:15-88 — Nominatim-клиент (перенос); config.go:30 — Astana viewbox]
- [Source: server/tools/scrape/cmd/interim-import/main.go — скелет эфемерной команды (0.6); internal/store/projection/lots.go — LotStore; internal/arch/boundaries_test.go — go-list-страж]
- [Source: 0-6-…md — Previous Story Intelligence (трек, флаг, изоляция, обратимость); deferred-work.md — defer-уроки 0.6]
- [Source: CLAUDE.md — geocoder politeness, VERIFY (Astana bbox/KATO), гардрейлы честности/нейтральности]
