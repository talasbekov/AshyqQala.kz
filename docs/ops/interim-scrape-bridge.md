# Интерим-источник: scrape-мост лотов (Story 0.6)

> **Статус:** ВРЕМЕННЫЙ (трек «Парсер-мост», Sprint Change Proposal 2026-06-20; §6.1/§6.4-override
> согласован владельцем 2026-06-23). Этот документ — операционная инструкция и **критерий удаления**.
> Контекст и потолок парсера — `docs/ops/stage0-access.md` §6.

## Что это

Временный импортёр лотов Астаны из **публичного портала** `goszakup.gov.kz` в проекцию `lots` —
чтобы двигаться без `GOSZAKUP_TOKEN`. Реализован в `server/tools/scrape/` (+ команда
`server/tools/scrape/cmd/interim-import`), **за build-tag `scrape`** (вне дефолтного билда).

Поток: `scrape.Source` → **тот же** `internal/ingest/decode` (что и боевой `ows`) → `internal/store/projection` (`UpsertLot`).

## ⚠️ Юридическая рамка (§6.1/§6.4)

Парсинг — осознанное **временное отклонение** от несущего принципа «официальный канал, не парсинг».
Поэтому защита встроена в код:

- **Гейт-флаг.** Команда запускается ТОЛЬКО при `ASHYQQALA_INTERIM_SCRAPE=1`; без флага — отказ (exit 2).
- **Громкий warning.** При запуске печатается баннер о §6.1/§6.4 и критерии удаления.
- **Изоляция (несущая).** Весь код — за `//go:build scrape`. CI-страж `internal/arch`
  (`TestHotPathDoesNotImportScrape`) доказывает машинно: `cmd/api` и `cmd/importer` **не импортируют**
  `tools/scrape` даже транзитивно; страж краснеет, если импорт появится.
- **Только лоты.** Договоры/участники/РНУ/journal требуют токен (`Fetch` иных ресурсов → честная ошибка).
- **Честность.** Отсутствующих полей (`title_kk`, `quantity`, `unit`, договор) парсер НЕ выдумывает → `NULL`.
  Выборка keyword-смещена (round-robin дорога/вода) → направления помечать «предв.».

## Запуск (осознанно)

```bash
cd server
ASHYQQALA_INTERIM_SCRAPE=1 DATABASE_URL=postgres://... \
  go run -tags scrape ./tools/scrape/cmd/interim-import -max 200
```

Идемпотентность: натуральный ключ `goszakup_lot_id` синтезируется стабильно из
`(объявление|наименование)` (`scrape-<sha256-16>`), повторный прогон не плодит дубли (UPSERT по ключу).

## Тесты

- `go test -tags scrape ./tools/scrape/...` — парсер на записанной HTML-фикстуре, стабильность ключа,
  гейт флага, `schema_hash`-golden (контракт полей scrape→decode).
- `go test -count=1 ./internal/arch/...` — изоляция от прод-бинарей (AC2).
- `DATABASE_URL=... go test -tags 'scrape integration' ./tools/scrape/...` — идемпотентность в БД.

## Гео-стадия: batch-геокодинг лотов (Story 0.7)

Стадия `geo` конвейера `decode → lots → geo → map`: геокодит scraped-лоты в координаты для ранней
карты (0.8). Реализация: `server/internal/geo/` (порт Nominatim-клиента, переиспользуется Epic 3) +
команда `server/tools/scrape/cmd/interim-geocode` (за тем же build-tag `scrape`, тем же гейтом и стражем).

```bash
cd server
ASHYQQALA_INTERIM_SCRAPE=1 DATABASE_URL=postgres://... [NOMINATIM_URL=http://localhost:8080] \
  go run -tags scrape ./tools/scrape/cmd/interim-geocode
```

- **Целевая таблица — `interim_geo_lots`** (миграция `0004`): МИНИМАЛЬНАЯ интерим-таблица (lat/lon `DOUBLE`,
  не PostGIS-geometry; связь по стабильному `goszakup_lot_id`, без hard FK к проекции — AR-4). **НЕ
  канонический `geo_objects`** (его строит Epic 3/Story 3.1: полная geometry, district, Directus-курация).
- **Автопокрытие — ФАКТ, не гейт.** Команда печатает `сматчено/всего` числом, **без exit-кода**. Выборка
  keyword-смещена → доля «предв.», НЕ репрезентативна для гейта FR-6 (тот — на живых данных, Гейт №0).
- **Честность (AC2).** Негеокодированный лот → `geocode_status=unmatched`, `lat/lon=NULL` (НИКОГДА не `0,0`).
  Потребитель (0.8) показывает «без точки на карте» / `container_state=not_geocoded`.
- **AR-22 (эфемерный Nominatim).** Целевой геокодер — self-host batch (профиль `geocode`), гасится после
  прогона; `NOMINATIM_URL` пуст → публичный Nominatim (≤1 req/sec) только для разовой выборки.
- **Тесты:** `go test ./internal/geo/...` (клиент на httptest), `go test -tags scrape ./tools/scrape/cmd/interim-geocode/...`
  (гейт, warning, честность unmatched→NULL, покрытие), `DATABASE_URL=… go test -tags 'scrape integration' …`
  (идемпотентность UPSERT).

## Карта-стадия: ранняя карта лотов (Story 0.8)

Стадия `map` конвейера `decode → lots → geo → map`: отдаёт scraped-лоты с интерим-гео на публичную карту.
**В отличие от 0.6/0.7, код карты НЕ за build-tag `scrape`** — это обычный read-only путь прод-бинаря
`cmd/api`, читающий `interim_geo_lots` через `internal/store` (НЕ импортируя `tools/scrape`; страж
изоляции остаётся зелёным). Реализация:

- **Сервер:** `GET /api/lots` (`server/internal/httpapi/map.go`, `MapLotsHandler`) — sqlc-запрос
  `ListLotsWithGeo` (`queries/map.sql`, **LEFT JOIN** `lots↔interim_geo_lots` по `goszakup_lot_id`).
  Координаты на проводе — bare `[lon,lat]` (nullable; `null` ⇒ нет точки, НЕ `0,0`). Маппинг честных
  состояний: `auto`+координата → `geocode_state=ok`; `unmatched`/NULL → `geocode_failed`; нет гео-строки
  (LEFT JOIN NULL) → `geocode_pending`. Только лоты — контракты/флаги/медианы ждут токен.
- **Фронт:** `web/src/features/map/` — `useLots`/`splitLots` (точки ⊥ негео), DOM-маркеры реальных лотов,
  `MapStatePlaque` (`container_state=no_contracts`: «временные/частичные данные» + «предв., keyword-bias»),
  `LotPreviewSheet` (данные лота + «ожидает официального источника» — БЕЗ фетча несуществующего контракта).
- **Честность:** негеокодированный лот → честно в списке «без точки на карте» (`Icon ungeocoded`), координата
  не выдумывается; пустой `/api/lots` → честная плашка контейнера (не «всё чисто»).
- **Тесты:** `go test ./internal/httpapi/...` (httptest + OpenAPI-валидация), web `vitest`/Playwright
  (`route.fulfill` мок `/api/lots`).

## 🔚 Критерий удаления и обратимость (AC3)

- **Критерий удаления:** получен `GOSZAKUP_TOKEN` (закрытие Story 0.1).
- **Действие:** источник переключается `scrape → ows` — это **один флаг** `Source` (Story 2.1, `owsSource`);
  downstream `decode → lots → geo → map` **не меняется** (работа парсер-моста не выбрасывается).
- **Удаляется:** пакет `server/tools/scrape/`, команды `cmd/interim-import` (0.6) и `cmd/interim-geocode`
  (0.7), таблица `interim_geo_lots` (вместе с build-tag `scrape`). **Сохраняется:** `internal/geo`
  (механизм геокодинга переиспользует Epic 3); данные мигрируют в канонический `geo_objects`.
- **Карта (0.8):** эндпоинт `/api/lots` и фронт `features/map/` **переживают swap** (читают те же `lots`,
  наполняемые уже из `ows`); снимаются лишь плашки «временные/частичные данные»/«предв.», а интерим-таблица
  `interim_geo_lots` замещается каноническим `geo_objects` (Epic 3 — кластеры/bbox/гейт покрытия).
- **Путь восстановления §6.1/§6.4:** удаление интерим-кода возвращает «официальный канал, не парсинг»
  в исходную силу. См. Sprint Change Proposal 2026-06-20 и `docs/ops/stage0-access.md` §6.

---

*Сосед по каталогу:* `docs/ops/stage0-access.md` (Story 0.1, §6 — потолок парсера),
`docs/ops/stage0-verdict-20260620.json` (Story 0.2, пример вердикта гейта №0),
`docs/ops/preflight-decisions.md` (Story 0.4 — ключ идемпотентности, частичный Go, демо, снапшот ows_v2).
