---
baseline_commit: 7cdde03392c464954e4a86b29a14b8bfe9c9517c
---

# Story 0.6: ⏳ Временный scrape-источник лотов за флагом

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->
<!-- ⏳ ВРЕМЕННАЯ история трека «Парсер-мост» (Sprint Change Proposal 2026-06-20). Удаляется/замещается при получении токена (Epic 2 живой импорт). -->

## Story

As a **команда платформы AshyqQala.kz**,
I want **временно подавать scraped-лоты Астаны в проекцию `lots` через тот же `ingest/decode`, за явным флагом и вне продового бинаря**,
so that **можно двигаться без `GOSZAKUP_TOKEN`, не выбрасывая downstream при последующем swap на `ows`** (несущий принцип обратимости).

> **Тип истории:** ⏳ ВРЕМЕННАЯ enabler-story Эпика 0, трек «Парсер-мост». Внесена Sprint Change
> Proposal 2026-06-20 (осознанное временное отклонение от §6.1/§6.4, ратифицировано владельцем).
> **Только лоты** — договоры/участники/РНУ/акты/journal недоступны без токена → карточки, 3/4 флага
> и медианы **ждут токен** (не в скоупе этой истории). История **обратима**: при получении токена
> `Source` переключается `scrape → ows`, интерим-команда удаляется по критерию удаления.

## Acceptance Criteria

**AC1 — Интерим-команда пишет scraped-лоты в `lots` за флагом, с громким warning**
**Given** существующий парсер (`Source` = `ows|file|scrape`) и проекция `lots`
**When** запущена отдельная `tools/`-команда интерим-импорта с флагом окружения `ASHYQQALA_INTERIM_SCRAPE=1`
**Then** scraped-лоты Астаны пишутся в проекцию `lots` через тот же путь `ingest/decode`; **без флага команда не выполняется** (явно отказывает); при запуске печатается **громкий warning** о временном отклонении §6.1/§6.4.

**AC2 — CI-страж доказывает изоляцию парсинга от продового бинаря**
**Given** несущий принцип изоляции парсинга («скрейпинг невозможен по структуре»)
**When** прогон CI-стража (go-list / grep по графу импортов)
**Then** доказано машинно: `cmd/api` и `cmd/importer` **НЕ импортируют** `tools/scrape` (парсер вне горячего пути); страж падает (red), если импорт появится.

**AC3 — Обратимость и критерий удаления зафиксированы**
**Given** будущий `GOSZAKUP_TOKEN`
**When** источник переключается `scrape → ows`
**Then** это **один флаг** `Source`; downstream (`decode → lots → geo → map`) **не меняется**; критерий удаления интерим-команды и путь восстановления §6.1/§6.4 зафиксированы (в коде/доке).

## Tasks / Subtasks

- [x] **Task 0 — Предусловие: подтвердить фундамент монорепо (БЛОКЕР, см. «Зависимости»)** *(AC1)* — ✅ **ПРОЙДЕН на HEAD `7cdde03` (Story 2.0)**; прежний HALT (2026-06-22/23 на `b5258b5`) был ДО Story 2.0 и устарел.
  - [x] Подтверждено на `7cdde03`: монорепо `server/` (`ashyqqala/server`) ✓; `server/tools/scrape/` (`.gitkeep`) ✓; `cmd/{api,importer}` ✓; **проекция `lots`** — `migrations/0003_projection_lots.sql` + `internal/store/projection/lots.go` (`UpsertLot`/`GetLotByID`) + sqlc `gen.UpsertLotParams` ✓; пакет **`internal/ingest/decode`** — `DecodeLot`/`DecodeLotsFrom`/`type Lot` + `SchemaHash` ✓; **`internal/goszakup`** — `type Source` (точка swap) + `FileSource` ✓.
  - [x] HALT снят: все 4 несущих компонента AC1–AC3 существуют (построены Story 2.0, коммит `7cdde03`). Фундамент в этой истории НЕ создаётся (соблюдён запрет анти-churn).
- [x] **Task 1 — Перенести парсер в `server/tools/scrape/` за build-tag (AC1, AC2)** — ✅ `scrape.go`
  - [x] Перенесён `scrapeSource`→`scrape.Source` (`parseRows`/`parseMoney`/round-robin по `terms`) в `server/tools/scrape/` вне продового бинаря. `astanaKatos` НЕ перенесён (в исходном Fetch не использовался — единый код КАТО; убран, чтобы не плодить unused-код/lint).
  - [x] Build-tag `//go:build scrape` на всех исполняемых файлах + `doc.go` (без тега, «one-off, НЕ прод; §6.1/§6.4», критерий удаления).
  - [x] `scrape.Source` СТРУКТУРНО удовлетворяет `internal/goszakup.Source` (`Name()`+`Fetch(resource, scopeBINs, max, handle)`) **без импорта goszakup** — точка swap `scrape→ows`.
- [x] **Task 2 — Интерим-команда `tools/` с флагом и громким warning (AC1)** — ✅ `cmd/interim-import/main.go`
  - [x] Отдельный `main` за build-tag `scrape`, **вне** `cmd/api`/`cmd/importer`.
  - [x] Гейт `ASHYQQALA_INTERIM_SCRAPE=1` (строгий «=1»): без флага — отказ (exit 2); с флагом — громкий warning §6.1/§6.4 + критерий удаления. Логика вынесена в `interimEnabled`/`loudWarning` (тестируемо, без сети/БД).
  - [x] Параметры: `-max`, дефолты вежливости (`politeDelayDefault` 1100мс), `AstanaKATO` — перенос дефолтов `stage0-audit`.
- [x] **Task 3 — Запись scraped-лотов в проекцию `lots` через `ingest/decode` (AC1)** — ✅
  - [x] `decode.DecodeLotsFrom(scrapeSrc, hash, max)` → ТОТ ЖЕ decode, что у будущего `ows` (не пишем в БД напрямую) → `projection.LotStore.UpsertLot`.
  - [x] Маппинг: `name_ru→title_ru`, `amount→amount`, `ref_kato→kato_code`; объявления в парсере нет → `announcement_id=NULL`; отсутствующие `title_kk`/`quantity`/`unit` = NULL (не выдуманы). **Синтетический стабильный `id`** (`scrape-<sha256-16>` из `anno|name`) — портал реального `lot_id` не отдаёт, а `goszakup_lot_id` — `NOT NULL UNIQUE` ключ UPSERT.
  - [x] Идемпотентность: натуральный ключ детерминирован → повтор не плодит дубли (юнит `TestSyntheticLotID_Stable` + интеграционный `TestScrapeIdempotency_Integration`).
- [x] **Task 4 — CI-страж изоляции `tools/scrape` от прода (AC2)** — ✅ `internal/arch/boundaries_test.go`
  - [x] `TestHotPathDoesNotImportScrape`: `go list -deps` cmd/api & cmd/importer НЕ содержат `tools/scrape` (транзитивно). Гоняется существующим CI-шагом `go test -count=1 ./internal/arch/...`.
  - [x] Red при нарушении ДОКАЗАН: временный `import _ tools/scrape` в `cmd/importer` → FAIL; восстановлено → green. Чистый предикат `importsScrape` + negative-control `TestImportsScrape` (см. [[guards-must-prove-red]]).
- [x] **Task 5 — Обратимость и критерий удаления (AC3)** — ✅
  - [x] Зафиксировано в `doc.go` + `docs/ops/interim-scrape-bridge.md`: swap `scrape→ows` = один флаг `Source`; downstream неизменен; критерий удаления = «получен `GOSZAKUP_TOKEN`»; путь восстановления §6.1/§6.4 → Sprint Change Proposal 2026-06-20.
  - [x] `docs/ops/interim-scrape-bridge.md` создан, согласован с `docs/ops/stage0-access.md` §6 (потолок: lots-only, keyword-bias).
- [x] **Task 6 — Тесты и финализация** — ✅
  - [x] Юнит-тесты парсера на **записанной HTML-фикстуре** (`testdata/search-result.html`, без сети): `parseRows`/`parseMoney`/маппинг/дедуп детерминированы; `schema_hash`-golden (контракт scrape→decode).
  - [x] Тест гейта флага (`TestInterimEnabled_GateRequiresFlag`) + warning (`TestLoudWarning_…`).
  - [x] Тест идемпотентности в `lots` (`//go:build scrape && integration`, skip без `DATABASE_URL`).
  - [x] `go build/vet/test/gofmt` (дефолт + `-tags scrape`) зелёные; go-list-страж зелёный; CI-шаг `-tags scrape` добавлен; File List/Change Log/Completion Notes обновлены.

## Dev Notes

### Контекст истории (зачем эта история и почему временная)

Story 0.1 устанавливает официальный канал (`GOSZAKUP_TOKEN` → ows_v2), но **заблокирована на
человеко-операционном получении токена** (AC1, действие 👤). Чтобы двигаться без токена, владелец
санкционировал **временный парсер публичного портала как источник лотов** (Sprint Change Proposal
2026-06-20). Эта история — первая в треке «Парсер-мост»: она ставит интерим-импорт лотов так, чтобы
**работа не выбрасывалась** при последующем переключении на `ows` (один флаг `Source`), а юр-долг
был минимизирован (флаг + изоляция + критерий удаления).
[Source: _bmad-output/planning-artifacts/sprint-change-proposal-2026-06-20.md]
[Source: _bmad-output/planning-artifacts/epics.md#Трек-Парсер-мост (строки 747–773)]
[Source: _bmad-output/planning-artifacts/architecture.md#Промежуточное-отклонение (строки 178–200)]

### ⚠️ Зависимости и предусловия (КРИТИЧНО — потенциальный БЛОКЕР)

**Эта история ссылается на компоненты целевого монорепо, которых СЕЙЧАС не существует.** В репозитории
есть только модуль `stage0-audit/` (`ashyqqala/stage0-audit`, package main, только stdlib). Монорепо
`server/` (со всеми `cmd/*`, `internal/*`, `tools/*`, `migrations/*`, БД) ещё не создан.

AC1–AC3 предполагают наличие:

| Нужный компонент | Создаётся в истории | Статус сейчас |
|---|---|---|
| Монорепо `server/` (`ashyqqala/server`), `server/tools/`, `cmd/{api,importer}` | **Story 1.1** (скелет + compose db) | 🔴 backlog |
| Проекция `lots` (миграция `0002_projection`) + `internal/store` | **Story 1.2** (миграции + sqlc) | 🔴 backlog |
| `internal/ingest/decode` (единый вход ows_v2→домен + schema_hash) | **Story 2.1** (живая граница декодирования) | 🔴 backlog |
| `internal/goszakup` `Source` (`ows\|file`) — точка swap | арх. «после B-1» / 2.1 | 🔴 нет |

> **Вывод для dev-агента:** Task 0 — предусловие. Если перечисленного нет, **HALT** и эскалировать
> владельцу решение о последовательности. Sprint Change Proposal говорит «трек идёт **параллельно**
> Epic 1», но по факту 0.6 сидит **downstream** фундамента Epic 1 (1.1, 1.2) и части Epic 2 (2.1).
> Реалистичная последовательность: сначала 1.1 → 1.2 (+ проекция `lots`) → минимальный `ingest/decode`,
> затем 0.6. **Не создавать монорепо/миграции в рамках этой истории** — это чужой скоуп (1.1/1.2),
> дублирование сломает анти-churn структуру. Честность над домыслом: не имитировать несуществующие пакеты.

[Source: _bmad-output/planning-artifacts/architecture.md#S-0-минимальный-каркас (строки 757–767)]
[Source: _bmad-output/planning-artifacts/epics.md#Epic-1 (строки 825–844)]

### Что переиспользовать (НЕ изобретать заново)

**Парсер уже написан** — `stage0-audit/scrape.go`. Эта история **переносит** его в целевую структуру
(`server/tools/scrape/` за build-tag), а не пишет парсер заново.

- **`scrapeSource`** (`stage0-audit/scrape.go:21–46`): портал `https://goszakup.gov.kz/ru`, таймаут,
  UA, delay вежливости, `maxPages`.
- **`Fetch(resource, _, max, handle)`** (`scrape.go:122–170`): **только `lots`** (иначе ошибка
  «недоступен без токена»); round-robin по `terms = ["дорог","водоснабж","автодорог","водопровод","канализац"]`
  с пагинацией по `/search/lots?filter[kato]=…&filter[name]=…&page=…`.
- **`parseRows`** (`scrape.go:172–206`): парсит таблицу `#search-result`; колонки лота → запись
  `{name_ru, amount, ref_kato, trd_buy_number_anno}`; дедуп по `anno|name`.
- **`parseMoney`** (`scrape.go:88–99`), **`astanaKatos()`** (`scrape.go:101–120`, справочник
  `/search/getKato?name=Астана`, фолбэк `710000000`).
- **Интерфейс `Source`** (`stage0-audit/source.go:14–17`): `Name() string` + `Fetch(resource string,
  scopeBINs []string, max int, handle func(items []map[string]any) error) error`. Боевой
  `internal/goszakup` должен дать тот же интерфейс → swap `scrape → ows` = один флаг.
- **Дефолты вежливости/гео** из `stage0-audit/config.go`: Astana viewbox `71.20,51.30,71.78,51.00`,
  delay (политика Nominatim ≤1 req/sec) — переиспользовать.

### Соблюдение архитектуры (guardrails)

- **Структурная изоляция парсинга (несущая).** `tools/scrape` живёт в `server/tools/` за build-tag +
  `doc.go` «one-off, не прод»; **вне** `cmd/api`/`cmd/importer`. Прод-бинарь физически не может
  импортировать парсер. Это AC2 и исполняемая граница (CI go-list-тест), а не комментарий.
  [Source: architecture.md строки 299–300, 738–739, 774, 419]
- **Временное отклонение — за флагом и громким warning.** `ASHYQQALA_INTERIM_SCRAPE=1` обязателен;
  без флага — отказ; warning о §6.1/§6.4. [Source: architecture.md строки 190–191]
- **Обратимость через `Source` + единый downstream.** `decode → lots → geo → map` тот же для scrape и
  ows → swap = один флаг. Работа парсер-моста при получении токена **не выбрасывается** (downstream),
  выбрасывается только интерим-команда. [Source: architecture.md строки 192–193, 197–200]
- **Только лоты.** Договоры/участники/РНУ/акты/journal недоступны без токена → карточки, 3/4 флага,
  медианы вне скоупа. Парсер `Fetch` уже отказывает на не-`lots`. [Source: architecture.md строки 188–189]
- **Честность над домыслом.** Поля, которых нет в парсере (`title_kk`, `quantity`, `unit`, договор),
  пишутся как NULL/«нет данных», НЕ выдумываются. Keyword-bias выборки → направления непоказательны,
  метить «предв.» (это всплывёт в 0.8 на карте). [Source: architecture.md строки 194–195; CLAUDE.md гардрейл честности]
- **Юр-риск активен СЕЙЧАС.** Реальный парсинг → владелец юр-риска назначается в Story 0.9 (не до
  запуска, а сейчас). Эта история — техническая; юр-допустимость закрывает 0.9.

### Модель данных (куда пишем)

Целевая таблица — проекция **`lots`**: `id, announcement_id(FK,null), goszakup_lot_id, title_ru,
title_kk, amount, quantity, unit, kato_code`. Маппинг из scrape см. Task 3. Проекционные таблицы
(`0002_projection`) ⊥ кураторские (`0003_curated`) — разные гранты; интерим-импорт пишет в
**проекционную** `lots`. [Source: docs/AshyqQala_MVP_data_model_and_flags_v1.md (строка 42 — lots);
architecture.md строки 778–781]

### Файлы и куда писать результат

- **Код (целевая структура, создаётся при наличии фундамента):** `server/tools/scrape/` (перенос
  `scrape.go` + `doc.go` + build-tag), `tools/`-команда интерим-импорта, маппинг в `internal/ingest/decode`,
  go-list-страж (тест в `server/` + шаг в `.github/workflows/ci-server.yml`).
- **Док:** обновить `docs/ops/` (интерим-источник: флаг, warning, критерий удаления, путь
  восстановления), согласовать с `docs/ops/stage0-access.md` §6 (потолок парсера: lots-only, keyword-bias).
- **НЕ трогать:** существующий `stage0-audit/` (его `scrape.go` — источник для переноса, но stage0-audit
  остаётся отдельным инструментом гейта №0); прикладной MVP-код вне скоупа этой истории.
- **НЕ создавать** в этой истории: монорепо-скелет (1.1), миграции/проекцию `lots` (1.2) — это чужой скоуп.

### Тестирование / проверка готовности

- Целевой стек тестов (Epic 1+): Go `go test` (+golden +property), testcontainers postgis для слоя
  данных, архитектурный go-list-тест. [Source: architecture.md строки 689–691, 815]
- Для **этой** истории: (а) юнит-тест парсера на **записанной HTML-фикстуре** (детерминизм, без живого
  запроса в тесте); (б) тест гейта флага; (в) тест идемпотентности декода в `lots`; (г) go-list-страж
  изоляции (AC2). Живой прогон парсера — отдельно, вручную, под флагом (вежливый delay).
- DoD: AC1–AC3 закрыты; страж red при нарушении изоляции; отсутствующие поля честно NULL; критерий
  удаления и путь восстановления зафиксированы.

### Previous Story Intelligence (0.1, 0.2, Sprint Change Proposal)

- **Story 0.1** (in-progress, blocked на токене): подтвердила эмпирически потолок парсера разовым
  `-source scrape -sample 30` — **lots-only** (поля `amount, name_ru, ref_kato, trd_buy_number_anno`);
  `contract/trd-buy/rnu/acts/subject/journal` → «недоступен без токена». Выборка **keyword-смещена**
  (`scrape.go:129`). Зафиксировано в `docs/ops/stage0-access.md` §6 и в долгой памяти проекта.
- **Story 0.2** (review): машиночитаемый Go/No-Go вердикт `stage0-audit` (`verdict.go`/`verdict_test.go`)
  — пример того, что у `stage0-audit` ЕСТЬ тест (`verdict_test.go`), хотя у основного инструмента
  тестов нет; перенос в `server/` принесёт нормальный `go test`.
- **Sprint Change Proposal 2026-06-20:** ратифицировал отклонение; критерий успеха трека — (а) ранняя
  карта Астаны на реальных лотах (0.8); (б) нулевая утечка scrape в прод (CI-страж зелёный — AC2 этой
  истории); (в) downstream переиспользуем (swap доказан на `Source` — AC3); (г) Epic 1 на синтетике не
  заблокирован.

### Git-контекст

Прикладного кода/наследуемых паттернов в git ещё нет (скаффолд-коммиты + `stage0-audit`). Опираться на
`stage0-audit/` (источник переноса) и планировочные артефакты, а не на git-историю. Последний релевантный
коммит — `796bd54` (план спринта + Story 0.1).

### Внешние знания / web-research

Не требуется: история не вводит новых библиотек (переиспользует существующий stdlib-парсер + целевой
стек Go из Epic 1). Специфику живого портала (структура таблицы `#search-result`, `/search/getKato`)
**не берём из памяти модели** — она уже зашита в проверенный `scrape.go`; при изменении разметки портала
тест на HTML-фикстуре поймает дрейф.

### Project Structure Notes

- **Прямой конфликт с текущим состоянием репозитория:** целевые пути (`server/tools/scrape/`,
  `internal/ingest/decode`, проекция `lots`) **не существуют** — есть только `stage0-audit/`. Это не
  ошибка планирования, а следствие того, что Epic 1 (фундамент) ещё не реализован. Разрешается Task 0
  (предусловие/HALT) + рекомендацией владельцу по последовательности (см. «Зависимости»).
- Парсер **остаётся** в `tools/` (НЕ в `internal/`) — юр-принцип невозможен к нарушению по структуре.
- Интерим-импорт — отдельная `tools/`-команда, **вне** горячего пути (`cmd/api`/`cmd/importer`).
- Согласованность downstream с боевым импортом (Epic 2) обеспечивается единым `ingest/decode` и
  интерфейсом `Source`.

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-0.6 (строки 755–773); трек 747–753]
- [Source: _bmad-output/planning-artifacts/architecture.md#Промежуточное-отклонение (строки 178–200)]
- [Source: _bmad-output/planning-artifacts/architecture.md#Целевое-дерево (строки 685–755, особенно tools/scrape 738–739)]
- [Source: _bmad-output/planning-artifacts/architecture.md#S-0-каркас (757–767); #Architectural-Boundaries (769–781)]
- [Source: _bmad-output/planning-artifacts/sprint-change-proposal-2026-06-20.md (весь; §4.2, §5)]
- [Source: stage0-audit/scrape.go (scrapeSource :21; Fetch :122; parseRows :172; parseMoney :88; astanaKatos :101)]
- [Source: stage0-audit/source.go (интерфейс Source :14–17)]
- [Source: stage0-audit/config.go (Astana viewbox :30; RoadKeywords/WaterKeywords; delay вежливости)]
- [Source: docs/AshyqQala_MVP_data_model_and_flags_v1.md (lots :42; geo_objects :49)]
- [Source: docs/ops/stage0-access.md §6 — потолок парсера (lots-only, keyword-bias)]
- [Source: CLAUDE.md — `-source` абстракция, scrape «legally constrained», гардрейлы честности/нейтральности]

## Dev Agent Record

### Agent Model Used

claude-opus-4-8[1m] (Claude Opus 4.8, 1M context) — dev-story workflow.

### Debug Log References

- Task 0 (проверка фундамента монорепо, 2026-06-22):
  - `server/` модуль `ashyqqala/server` ✓; `cmd/{api,importer,stage0}` ✓; `tools/{osm,scrape,tiles}` ✓ — каталоги-скелет от Story 1.1.
  - `migrations/0002_projection.sql` содержит ТОЛЬКО таблицу `contracts`; `lots` — TODO («когда появятся organizations/lots — B-3 / Epic 2»). 🔴 проекции `lots` нет.
  - `server/internal/ingest/decode/` — только `doc.go` (заглушка, 2 строки). 🔴 decode-граница не реализована (Story 2.1, backlog).
  - `server/internal/goszakup/` — только `doc.go` (заглушка). 🔴 интерфейс `Source` не определён (Story 2.1, backlog).
  - `server/internal/store/projection/` — только `doc.go` (заглушка). 🔴 store для `lots` нет.
  - `server/tools/scrape/` — только `.gitkeep` (ожидаемо: наполняет данная история).
- Task 0 (повторная проверка фундамента, 2026-06-23): состояние БЕЗ ИЗМЕНЕНИЙ относительно 2026-06-22.
  - `migrations/0002_projection.sql`: `CREATE TABLE contracts` (стр. 5); `lots` остаётся TODO-комментарием (стр. 23–25). 🔴 проекции `lots` нет.
  - `server/internal/ingest/decode/doc.go` — 2 строки (заглушка). 🔴
  - `server/internal/goszakup/doc.go` — заглушка; `grep "type Source"` по `server/internal/` → не найдено. 🔴 точка swap не существует.
  - `server/internal/store/projection/doc.go` — заглушка. 🔴
  - `server/tools/scrape/` — только `.gitkeep`. HEAD репо — `b5258b5`, рабочее дерево чистое.
- Task 0 (третья проверка, 2026-06-23 ПОСЛЕ Story 2.0, HEAD `7cdde03`): ✅ ВСЁ НА МЕСТЕ.
  - `migrations/0003_projection_lots.sql` — `CREATE TABLE lots` (`goszakup_lot_id NOT NULL UNIQUE`, ключ UPSERT). ✓
  - `server/internal/store/projection/lots.go` — `LotStore.UpsertLot`/`GetLotByID` (+ sqlc `gen.UpsertLotParams`, `queries/lots.sql`). ✓
  - `server/internal/ingest/decode/{decode,lots}.go` — `SchemaHash`/`DecodeLot`/`DecodeLotsFrom`/`type Lot`. ✓
  - `server/internal/goszakup/{source,file_source}.go` — `type Source` + `FileSource`. ✓
  - Реализация: `go build/vet/test/gofmt` (дефолт) зелёные; `-tags scrape` build/vet/test зелёные; `go test -count=1 ./internal/arch/...` зелёный; red-способность AR-27-стража scrape доказана (внедрён импорт в `cmd/importer` → FAIL → откат → green). `schema_hash` scrape = `548e4755…3af5ca9` (golden).

### Completion Notes List

- **РЕАЛИЗОВАНО (AC1–AC3 закрыты; HALT снят).** Прежний Task-0-HALT (2026-06-22/23 на `b5258b5`) был ДО Story 2.0. На HEAD `7cdde03` (Story 2.0) фундамент существует: проекция `lots` (миграция 0003 + `projection.LotStore`/sqlc), `internal/ingest/decode` (`DecodeLot`/`DecodeLotsFrom`/`SchemaHash`), `internal/goszakup.Source`+`FileSource`. Предусловие перепроверено фактически и пройдено. Юр-блокер §6.1/§6.4 снят владельцем 2026-06-23.
- **AC1** — интерим-команда `cmd/interim-import` за флагом `ASHYQQALA_INTERIM_SCRAPE=1` (без флага exit 2) + громкий warning §6.1/§6.4; пишет scraped-лоты в `lots` через **тот же** `ingest/decode`, что и боевой ows. Логика гейта/варнинга тестируема (`interimEnabled`/`loudWarning`).
- **AC2** — изоляция парсера НЕСУЩАЯ: весь `tools/scrape` за `//go:build scrape` (вне дефолтного билда). `internal/arch.TestHotPathDoesNotImportScrape` доказывает машинно, что `cmd/api`/`cmd/importer` не тянут `tools/scrape` (`.Deps`, транзитивно); **red-способность доказана** (внедрил импорт в importer → FAIL → восстановил). CI-шаг `go test -count=1 ./internal/arch/...` уже это гоняет.
- **AC3** — обратимость: `scrape.Source` структурно совместим с `goszakup.Source` (swap = один флаг, downstream `decode→lots→geo→map` неизменен); критерий удаления (`получен GOSZAKUP_TOKEN`) и путь восстановления §6.1/§6.4 — в `doc.go` + `docs/ops/interim-scrape-bridge.md`.
- **Решения дева (латитюд):** (1) **синтетический стабильный `id`** (`scrape-<sha256(anno|name)[:16]>`) — портал не отдаёт `lot_id`, а `goszakup_lot_id` = `NOT NULL UNIQUE` ключ UPSERT; стабильность ключа = идемпотентность. (2) scrape эмитит decode-совместимые записи (+синтет.`id`) → `schema_hash` зафиксирован golden-тестом (дрейф полей → `ErrSchemaDrift`). (3) `astanaKatos` не перенесён (в исходном `Fetch` не использовался; убран ради чистого lint). (4) CI гоняет `tools/scrape` явным `-tags scrape`-шагом (иначе тег-код не компилируется в дефолтном CI).
- **Гардрейлы:** честность — отсутствующие поля (`title_kk`/`quantity`/`unit`/договор) → NULL, не выдуманы; keyword-bias выборки задокументирован («предв.»); только лоты (иные ресурсы → честная ошибка). Фундамент (1.2/2.1) НЕ дублировался.
- **Не выполнено в этой среде (требует внешних ресурсов):** живой прогон парсера против портала (сеть + вежливость) и интеграционный тест в реальной БД (`-tags 'scrape integration'`, нужен `DATABASE_URL`+миграции). Оба компилируются/скипаются чисто; идемпотентность ключа покрыта детерминированным юнит-тестом.

### File List

- `server/tools/scrape/doc.go` — **новый.** Пакетная док-строка (без build-tag): §6.1/§6.4, изоляция, обратимость, критерий удаления, потолок.
- `server/tools/scrape/scrape.go` — **новый** (`//go:build scrape`). Перенос парсера: `Source` (структурно = `goszakup.Source`), `parseRows`/`parseMoney`/`cellText`/`get`, синтетический `syntheticLotID`, `RecordKeys`, `AstanaKATO`.
- `server/tools/scrape/scrape_test.go` — **новый** (`//go:build scrape`). Парсер на HTML-фикстуре, стабильность ключа, `parseMoney`, `schema_hash`-golden, scrape→`DecodeLot` round-trip.
- `server/tools/scrape/testdata/search-result.html` — **новый.** Записанная фикстура страницы `/search/lots` (детерминизм, без сети).
- `server/tools/scrape/cmd/interim-import/main.go` — **новый** (`//go:build scrape`). Интерим-команда: гейт-флаг + warning + scrape→decode→`UpsertLot`; `interimEnabled`/`loudWarning`/`toParams`.
- `server/tools/scrape/cmd/interim-import/main_test.go` — **новый** (`//go:build scrape`). Тест гейта флага + содержимого warning.
- `server/tools/scrape/idempotency_integration_test.go` — **новый** (`//go:build scrape && integration`). Идемпотентность в БД (skip без `DATABASE_URL`).
- `server/internal/arch/boundaries_test.go` — изменён: `TestHotPathDoesNotImportScrape` (AC2) + чистый предикат `importsScrape` + negative-control `TestImportsScrape`.
- `.github/workflows/ci-server.yml` — изменён: шаг `scrape build-tag` (`go build/vet/test -tags scrape ./tools/scrape/...`); обновлён комментарий go-list-стража (AC2).
- `docs/ops/interim-scrape-bridge.md` — **новый.** Операционная инструкция: флаг, warning, запуск, тесты, критерий удаления, обратимость; согласован со `stage0-access.md` §6.
- `_bmad-output/implementation-artifacts/0-6-…md` — изменён: frontmatter `baseline_commit`, Task 0–6 чекбоксы, Dev Agent Record, File List, Change Log, Status → review.
- `_bmad-output/implementation-artifacts/sprint-status.yaml` — изменён: статус `ready-for-dev → in-progress → review`.

## Change Log

| Дата | Изменение |
|---|---|
| 2026-06-21 | Создан context engine для Story 0.6 (трек «Парсер-мост»). Исчерпывающий анализ: epics 0.6–0.9, architecture (отклонение + целевое дерево + границы), модель данных `lots`, код `stage0-audit/scrape.go`+`source.go`, предыдущие истории 0.1/0.2 + Sprint Change Proposal. **Зафиксирован критичный блокер:** AC1–AC3 зависят от ещё не созданного монорепо (Story 1.1 скелет, 1.2 проекция `lots`+sqlc, 2.1 `ingest/decode`) — Task 0 = предусловие/HALT + эскалация владельцу по последовательности. Статус → ready-for-dev. |
| 2026-06-22 | Запуск dev-story. **Task 0 (предусловие) — HALT.** Проверено фактическое состояние репо: скелет монорепо (Story 1.1, done) есть, но проекция `lots` (миграция + store), `internal/ingest/decode` и `internal/goszakup` `Source` — пустые заглушки/отсутствуют (1.2 сделала только `contracts`; `lots` → TODO Epic 2/B-3; `ingest/decode`+`Source` → скоуп Story 2.1, backlog). Историю нельзя реализовать поверх несуществующего фундамента; создавать его здесь запрещено (чужой скоуп 1.2/2.1, анти-churn). Эскалировано владельцу: решить последовательность (сначала проекция `lots` + Story 2.1, затем 0.6). Статус остаётся ready-for-dev (реализация не начата). |
| 2026-06-23 | Повторный запуск dev-story (перенаправлен с заблокированной на токене Story 0-1). **Task 0 (предусловие) — HALT сохраняется.** Фактическое состояние перепроверено и БЕЗ ИЗМЕНЕНИЙ: `0002_projection.sql` — только `contracts` (`lots` = TODO стр. 23–25); `ingest/decode`, `goszakup` `Source`, `store/projection` — заглушки `doc.go`; `tools/scrape/` — только `.gitkeep`. Три из четырёх несущих компонентов AC1–AC3 отсутствуют. Создавать фундамент в этой истории запрещено (скоуп 1.2/Epic 2 B-3 и 2.1; анти-churn). Статус остаётся ready-for-dev (реализация не начата). Эскалация владельцу: блокер 0-6 — не токен, а ПОСЛЕДОВАТЕЛЬНОСТЬ — нужен фундамент (проекция `lots` + минимальные `ingest/decode` и `Source`) до старта парсер-моста. |
| 2026-06-23 | **dev-story (после Story 2.0): HALT снят, реализовано.** `baseline_commit=7cdde03`. Task 0 перепроверен фактически: фундамент построен Story 2.0 (проекция `lots`+`LotStore`, `decode.DecodeLot`/`SchemaHash`, `goszakup.Source`+`FileSource`) → предусловие пройдено. Юр-блокер §6.1/§6.4 снят владельцем. Реализованы Task 1–6: перенос парсера в `server/tools/scrape/` за `//go:build scrape` (структурно = `goszakup.Source`, синтетический стабильный `id`); интерим-команда `cmd/interim-import` (гейт `ASHYQQALA_INTERIM_SCRAPE=1` + warning §6.1/§6.4); scrape→ЕДИНЫЙ `ingest/decode`→`UpsertLot` (schema_hash-golden); CI-страж изоляции `TestHotPathDoesNotImportScrape` (red-способность доказана) + negative-control; doc.go + `docs/ops/interim-scrape-bridge.md` (обратимость/критерий удаления); HTML-фикстура + юнит-тесты + интеграционный (skip без `DATABASE_URL`); CI-шаг `-tags scrape`. Дефолт `go build/vet/test/gofmt` + `-tags scrape` + `-count=1 ./internal/arch/...` — зелёные. AC1–AC3 закрыты. Статус → review. |

### Change Log (доп.)

| Дата | Изменение |
|---|---|
| 2026-06-23 | Code review (3 слоя). Все AC1–AC3 MET (Acceptance Auditor: accept). Применены 6 patch: честность `parseMoney`→nil/NULL (не 0); `io.ReadAll`-ошибка не глотается; fail-fast `Ping` БД; синтет.`id` 64→128 бит; regex-якорь `\sid=`; golden negative-control + comma-ok. 4 defer → `deferred-work.md` (слабый ключ при пустом anno, regex-хрупкость, точность amount→2.1, неатомарный импорт). ~6 dismissed (AC2-«vacuity» — false-positive: страж краснеет правильно). Все проверки зелёные. **Статус → done.** |

## Review Findings (Code Review — 2026-06-23)

> Адверсариальное ревью (3 слоя). Все AC (AC1–AC3) и гардрейлы — **MET** (Acceptance Auditor подтвердил, red-способность AC2-стража перепроверена). 6 patch, 4 defer, ~6 dismissed.

### Patch (все 6 применены 2026-06-23 — build/vet/test/gofmt дефолт + `-tags scrape` + `-count=1 arch` зелёные)

- [x] [Review][Patch] **Честность:** `parseMoney→(float64,bool)`; `parseRows` эмитит `"amount": nil` при `!ok` (ключ остаётся → schema_hash стабилен; `decode`→NULL, не 0). Тест `TestParseRows_UnparseableAmount_NULL` доказывает: `"-"`→NULL, не 0 ✅ [server/tools/scrape/scrape.go]
- [x] [Review][Patch] `io.ReadAll` ошибка больше не проглочена — возвращается `fmt.Errorf("чтение тела…")` ✅ [server/tools/scrape/scrape.go]
- [x] [Review][Patch] fail-fast `pool.Ping` (5s) ДО scrape — коннект-фейл БД не дёргает портал зря (зеркалит `cmd/api`) ✅ [server/tools/scrape/cmd/interim-import/main.go]
- [x] [Review][Patch] синтетический `id` расширен 16→32 hex (64→128 бит) — birthday-коллизия исчезающе мала ✅ [server/tools/scrape/scrape.go]
- [x] [Review][Patch] regex привязан `\sid="search-result"` — не матчит `data-id="search-result"` (фикстура по-прежнему парсится) ✅ [server/tools/scrape/scrape.go]
- [x] [Review][Patch] golden `schema_hash` negative-control `TestScrapeSchemaHash_RedOnDrift` (±1 поле → иной хеш) + `comma-ok` type-assert в тесте ✅ [server/tools/scrape/scrape_test.go]

### Defer

- [x] [Review][Defer] слабый synthetic key при пустом `anno` (две разные строки с одинаковым `name` → коллизия/тихий drop) — присущая tokenless-ограниченность; `ows` `lot_id` решает при swap (критерий удаления) [server/tools/scrape/scrape.go] — deferred → Story 2.1/ows-swap
- [x] [Review][Defer] regex-HTML хрупкость (вложенная `</table>` усекает захват; преждевременный `exhausted` при странице из дублей) — присуще regex-скрейпингу; ows API решает [server/tools/scrape/scrape.go] — deferred → ows-swap
- [x] [Review][Defer] точность `amount` (`int64(float64)`-усечение копеек / `>2^53`) — сворачивается в существующий defer «decode value-robustness → Story 2.1» [server/tools/scrape/scrape.go] — deferred → Story 2.1
- [x] [Review][Defer] неатомарный импорт в БД (UpsertLot в цикле без транзакции; повтор идемпотентен по ключу) — приемлемо для интерим-тула [server/tools/scrape/cmd/interim-import/main.go] — deferred, документировано

### Dismissed (false-positive / by-design / out-of-scope)

- AC2-страж «vacuity / go list error» — **FALSE POSITIVE**: `doc.go` без build-tag → страж краснеет ПРАВИЛЬНЫМ сообщением (доказано: внедрён импорт в importer → boundary-FAIL, не go-list-ошибка).
- `strings.FieldsSeq` Go-floor — `go 1.25` (go.mod), уже используется в репо (Story 1.10).
- `maxPages` кап «unlimited» — намеренная защитная граница. · дубль `toParams`/`toScrapeParams` — кросс-пакетный, test-only. · `announcement_id` NULL — по спеке (таблицы объявлений нет). · stdout русская проза не JSON — интерим-тул без потребителя; stderr/stdout split корректен.
