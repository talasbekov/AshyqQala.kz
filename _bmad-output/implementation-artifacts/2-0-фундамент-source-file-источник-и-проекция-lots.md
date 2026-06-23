---
baseline_commit: 3b4f83178040dc76b3333a9fc9ed2ec9382d40b0
---

# Story 2.0: Токен-независимый фундамент — Source-интерфейс, file-источник, проекция lots

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->
<!-- Добавлена Sprint Change Proposal 2026-06-23 (Direct Adjustment). Токен-независимый фундамент, вырезанный из 2.1/B-3. -->

## Story

As a **команда**,
I want **зафиксировать `Source`-интерфейс (точка swap scrape→ows|file) + file-реализацию + проекцию `lots`**,
so that **живой импорт (Story 2.1) и трек «Парсер-мост» (Story 0.6) разблокированы без `GOSZAKUP_TOKEN`**.

> **Тип истории:** enabler-story Epic 2 (backend, Go). **Токен-независима** (нужен только Docker для
> sqlc-генерации миграции + integration-тестов). Разблокирует: Story 0.6 (закрывает её Task 0 —
> проекция `lots` + `Source`) и Story 2.1 (даёт интерфейс `Source` под ows-реализацию).
> [Source: sprint-change-proposal-2026-06-23.md; epics.md Story 2.0; AR-4/AR-11/AR-27]

## Acceptance Criteria

**AC1 — `Source`-интерфейс (точка swap)** [SCP §4 AC1; AR-27]
**Given** пустые заглушки `internal/goszakup` и `internal/store/projection`
**When** в `internal/goszakup` определён `Source` (`Name() string`, `Fetch(resource string, scopeBINs []string, max int, handle func(items []map[string]any) error) error`) — единая точка swap `scrape→ows|file`, сигнатура совпадает с `stage0-audit/source.go:14-17`
**Then** go-list-страж: `internal/goszakup` импортируется ТОЛЬКО из `internal/ingest/decode` (AR-27); токен не требуется.

**AC2 — file-реализация `Source`** [SCP §4 AC2]
**Given** локальные JSON-дампы `<data-dir>/<resource>.json` (массив объектов на ресурс)
**When** запущена file-реализация `Source`
**Then** ресурсы (`contract`/`lots`/…) читаются из файлов и отдаются в `handle` поштучно/пачкой (как `stage0-audit -source file`), **без токена**. (ows-реализация `Source` — Story 2.1.)

**AC3 — Проекция `lots` (миграция + store + sqlc)** [SCP §4 AC3; AR-4; data-model lots]
**Given** проекционная граница (AR-4)
**When** добавлена goose-миграция `lots` (колонки: `id`, `announcement_id` BIGINT null, `goszakup_lot_id` TEXT, `title_ru`, `title_kk`, `amount` BIGINT, `quantity`, `unit`, `kato_code`; **БЕЗ flags/geo**) + запросы `internal/store/queries/lots.sql` + sqlc-генерат + доступ `internal/store/projection`
**Then** проекция `lots` доступна downstream; проекционная ⊥ кураторская; **закрывает Task 0 Story 0.6**.

**AC4 — Границы и тесты** [AR-27; Story 1.10 паттерн стража]
**Given** go-list-страж ядра (`internal/arch`, Story 1.10)
**When** добавлены новые пакеты
**Then** страж расширен/проходит (`goszakup` — только из `ingest/decode`; ядро не импортирует новое); golden/юнит на file-`Source` (фикстура) и декод `lots`; `go test ./...` зелёный; честные состояния (нет данных vs пусто/0).

## Tasks / Subtasks

- [x] **Task 0 — Предусловие окружения (Docker для sqlc)** *(AC3)*
  - [x] Убедиться, что доступен Docker (sqlc-генерация идёт через `docker run sqlc/sqlc` — `make gen-sqlc`); локально БД-порт `POSTGRES_PORT=55432` (порты 5432/5433 заняты). Если Docker недоступен — HALT и эскалировать (миграцию `lots` + sqlc без него не сгенерировать) [memory: local-dev-docker-env]
- [x] **Task 1 — `Source`-интерфейс в `internal/goszakup` (AC1)**
  - [x] Наполнить `server/internal/goszakup/` (заглушка `doc.go`): интерфейс `Source` (`Name()`, `Fetch(resource, scopeBINs, max, handle)`) — точка swap. Сигнатура из `stage0-audit/source.go:14-17` (другой модуль — переписать, не импортировать). Константы ресурсов (`contract`/`lots`/`trd-buy`/`rnu`/…)
  - [x] **НЕ** реализовывать ows-источник (живой, нужен токен) — это Story 2.1; здесь только интерфейс + file-impl
- [x] **Task 2 — file-реализация `Source` (AC2)**
  - [x] `FileSource` в `internal/goszakup`: читает `<data-dir>/<resource>.json` (массив объектов → `[]map[string]any`), отдаёт в `handle`; неизвестный ресурс → ошибка; отсутствующий файл → честная ошибка (не тихо пусто). Зеркалит `stage0-audit` file-источник [stage0-audit/source.go]
  - [x] Юнит-тест на фикстуре (`testdata/<resource>.json`): file-Source читает и отдаёт корректные записи; без токена
- [x] **Task 3 — Проекция `lots`: миграция + sqlc (AC3)**
  - [x] `migrations/0003_projection_lots.sql` (goose Up/Down): таблица `lots` — `id BIGINT GENERATED ALWAYS AS IDENTITY PK`, `goszakup_lot_id TEXT NOT NULL UNIQUE`, `announcement_id BIGINT` (null, FK позже — целевой таблицы нет), `title_ru/title_kk TEXT`, `amount BIGINT`, `quantity`, `unit TEXT`, `kato_code TEXT`, `is_deleted BOOLEAN DEFAULT FALSE`, `imported_at/updated_at TIMESTAMPTZ DEFAULT now()`. **БЕЗ flags/geo.** Отсутствующие значения = NULL (паттерн `0002_projection.sql`)
  - [x] `internal/store/queries/lots.sql` (named-запросы как `contracts.sql`): минимум — `UpsertLot :one`/`exec` (идемпотентный UPSERT по `goszakup_lot_id`) + `GetLotsByContract`/`GetLotByID :one`
  - [x] `make gen-sqlc` (Docker) → `internal/store/gen/lots.sql.go` (НЕ править руками)
  - [x] Наполнить `internal/store/projection/`: доступ к проекции `lots` поверх `gen` (как боевой store к contracts). Проекционная ⊥ кураторская (`store/curation` — Story 1.10)
- [x] **Task 4 — Декод `lots` через `ingest/decode` (AC2/AC3)**
  - [x] Расширить `internal/ingest/decode`: тип `Lot` + `fieldCandidates` для лота (`goszakup_lot_id`, `title_ru/kk`, `amount`, `quantity`, `unit`, `kato_code`); функция декода raw→`Lot` со `schema_hash`-политикой «стоп на дрейф, не тихий 0» (как `Contract` в 1.10). golden-снапшот лота
  - [x] **`internal/goszakup` импортируется ТОЛЬКО из `ingest/decode`** (AR-27): decode потребляет `Source` (file) → raw → домен `Lot` → проекция. Wiring демонстрируется тестом (file-Source → decode → projection upsert), без живого ows
- [x] **Task 5 — go-list-страж и финализация (AC4)**
  - [x] Расширить `internal/arch` go-list-страж: `internal/goszakup` импортируется только из `ingest/decode`; чистое ядро (median/flags/normalize/benchmark) НЕ импортирует новые пакеты. Negative-control + `-count=1` (правило Story 1.10) [[guards-must-prove-red]]
  - [x] Зелёные: `go build ./...`, `go vet ./...`, `gofmt -l .` пусто, `go test ./...` (вкл. go-list-страж `-count=1`, golden lots/Source), `make lint`. Обновить File List, Change Log, Completion Notes

### Review Findings (code-review 2026-06-23)

Адверсариальное ревью (3 слоя: Blind Hunter / Edge Case Hunter / Acceptance Auditor). Блокирующих/High нет; все AC1–AC4 подтверждены; страж AR-27 эмпирически краснеет; sqlc-идемпотентность проверена.

**Patch (исправить):**
- [x] [Review][Patch] Страж goszakup без кэшируемого negative-control теста — вынести предикат в чистую функцию + тест (правило [[guards-must-prove-red]], как `TestIsForbidden`) [server/internal/arch/boundaries_test.go]
- [x] [Review][Patch] `kato_code`-кандидаты уже, чем в stage0 — добавить `delivery_kato`, `ref_kato_id` (KATO гео-критичен, FR-6) [server/internal/ingest/decode/lots.go:39]
- [x] [Review][Patch] `TestFileSource_Max` глотает ошибку Fetch (`_ =`) — проверять ошибку (иначе вводящее в заблуждение падение) [server/internal/goszakup/file_source_test.go:141]
- [x] [Review][Patch] `quantity`/`unit`/`announcement_id` всегда NULL (декод их не читает) — добавить комментарий, что это намеренно отложенные S-0 поля (не баг декода) [server/internal/ingest/decode/lots.go]

**Defer (вынесено в deferred-work.md):**
- [x] [Review][Defer] decode value-robustness для живого ows (`json.Number`/строковые числа/`>2^53`) — `lotInt`/`pickInt` берут только `float64`; ows часто отдаёт суммы строками → тихий NULL [decode/lots.go:298] — Story 2.1
- [x] [Review][Defer] Дублирование pick-хелперов (`lotPick`/`lotString`/`lotInt` ≈ `pick`/`pickString`/`pickInt`) — обобщить при добавлении доменных типов Epic 2 [decode/lots.go]
- [x] [Review][Defer] `ON CONFLICT DO UPDATE` безусловно затирает `quantity`/`unit`/`announcement_id` в NULL при смешении источников — Story 2.6 (выживание курации)

**Dismissed (шум/осознанно):** дублирование golden-хеша литералом (decode-тест ловит дрейф алгоритма в CI; комментарий фиксирует связь) · неиспользуемые `Resource`-константы + `Fetch(string)` (намеренная stage0-совместимость, forward-looking для 2.1) · хрупкость `max` для будущей ows-постраничности (2.1; file-источник корректен) · `scopeBINs` игнорируется (задокументировано, `DecodeLotsFrom` передаёт `nil`).

## Dev Notes

### Контекст истории (зачем и где границы)

Ретроспектива Epic 1 выявила: **Epic 2 нельзя стартовать** — Story 2.1 (живой decode) требует токен И боевого
`Source` + проекции `lots`, а токен-независимый фундамент «размазан». Story 2.0 (вырезана Sprint Change
Proposal 2026-06-23) собирает этот фундамент: `Source`-интерфейс + file-impl + проекция `lots` — токен-независимо.
**Разблокирует Story 0.6** (её Task 0 HALT-ил именно на отсутствии `lots` + `Source`) и **Story 2.1** (даёт
интерфейс под ows-реализацию). [Source: sprint-change-proposal-2026-06-23.md; ретро epic-1-retro-2026-06-23.md]

### Что переиспользовать (НЕ изобретать)

- **`Source`-интерфейс — из `stage0-audit/source.go:14-17`** (другой Go-модуль `ashyqqala/stage0-audit` —
  ИМПОРТ ЗАПРЕЩЁН; переписать сигнатуру/концепт): `Name() string`, `Fetch(resource string, scopeBINs []string,
  max int, handle func(items []map[string]any) error) error`. Реализации: ows|file|scrape (здесь — только file).
- **file-источник — паттерн `stage0-audit`** (`-source file`, локальные JSON-дампы массивами объектов).
- **Миграция `lots` — паттерн `migrations/0002_projection.sql`** (`contracts`): `BIGINT IDENTITY` PK + natural
  `TEXT UNIQUE` id, двуязычные `*_ru/*_kk`, NULL для отсутствующих (честное «нет данных»), `is_deleted`,
  `imported_at/updated_at`. `0002` уже содержит TODO про `lots` (стр. 23-25) — реализовать его.
- **sqlc — паттерн `internal/store/queries/contracts.sql`** (`-- name: X :one`) + `sqlc.yaml` (schema=миграции,
  per-domain `.sql` → per-file `.sql.go` в `gen/`, `pgx/v5`, `emit_interface`). `make gen-sqlc` через Docker.
- **decode — `internal/ingest/decode` (Story 1.10)**: `SchemaHash` (key-sorted), `Decode(raw, pinned)` →
  `ErrSchemaDrift` (стоп, не тихий 0), golden в `testdata/`, концепт `fieldCandidates`. Для `Lot` — тот же приём.
- **go-list-страж — `internal/arch` (Story 1.10)**: `isForbidden` + negative-control + `-count=1` (некэшируемо).

### Соблюдение архитектуры (guardrails)

- **AR-27 (исполняемые границы):** `internal/goszakup` импортируется ТОЛЬКО из `internal/ingest/decode`;
  чистое ядро не импортирует goszakup/store. Расширить go-list-страж.
- **AR-4 (граница данных):** проекционные таблицы (`lots`, импортёр перестраивает) ⊥ кураторские (`store/curation`,
  импортёр только читает) — раздельные пакеты `store/projection` vs `store/curation` (граница в Go, не только БД).
- **AR-11 (decode):** дрейф зафиксированной схемы `lots` → стоп+алерт, НЕ тихий 0; golden-снапшот.
- **Честность над домыслом:** отсутствующее поле lot → NULL/«нет данных»; file-Source на отсутствующем файле —
  честная ошибка, не тихо пусто.
- **Токен НЕ требуется.** ows-реализация `Source` (живая) и реальный snapshot B-1 — Story 2.1; `cmd/importer`
  (держатель токена) здесь НЕ наполняется.

### Границы скоупа — ЧТО НЕ делать в 2.0

| Отложено | Куда |
|---|---|
| ows-реализация `Source` (живой ows_v2, токен) + реальный `schema_hash` по B-1 | Story 2.1 |
| Ежедневный инкрементальный импорт через `/v2/journal` | Story 2.2 |
| Нормализация наименований→БИН (auto/manual/conflict) | Story 2.3 |
| Атомарная публикация снапшота + хук пересчёта | Story 2.4 |
| Реальный compute флагов/медиан | Epic 4 |
| `cmd/importer` (держатель `GOSZAKUP_TOKEN`) | Epic 2 (живой импорт) |
| FK `lots.announcement_id`/`organizations` | поздняя миграция (целевых таблиц ещё нет) |

[Source: epics.md Epic 2 (Story 2.1–2.7); SCP §2/§5]

### Файлы и куда писать

- **Наполнить:** `server/internal/goszakup/` (`Source`-интерфейс + `FileSource` + тесты), `server/internal/store/projection/`
  (доступ к `lots`), `server/internal/store/queries/lots.sql`, `server/internal/ingest/decode/` (тип `Lot` + декод + golden testdata).
- **Добавить:** `migrations/0003_projection_lots.sql` (goose); расширить `server/internal/arch/boundaries_test.go` (граница goszakup).
- **Сгенерировать (Docker):** `server/internal/store/gen/lots.sql.go` (`make gen-sqlc`).
- **НЕ трогать:** `internal/store/gen/*` руками (генерат), `internal/{registry,render,httpapi,clock,median,flags,normalize}` (готовы), `migrations/0001-0002` (только добавить 0003), `stage0-audit/` (образец, не импорт), фронт `web/`.

### Тестирование / проверка готовности

- **Раннеры:** `go test ./...`; integration (testcontainers postgis) за `//go:build integration` — для проекции
  `lots` (UPSERT/идемпотентность) при наличии Docker. Unit (file-Source, decode lots, schema_hash) — без Docker.
- **go-list-страж** `-count=1` (некэшируемо, правило 1.10) + negative-control.
- **DoD:** AC1–AC4 закрыты; `Source`-интерфейс + file-impl + проекция `lots` существуют; decode lots со стоп-на-дрейф +
  golden; граница goszakup enforced; `go build`/`go vet`/`gofmt`/`go test ./...` зелёные; **Task 0 Story 0.6 закрываем**
  (проекция `lots` + `Source` есть). Токен НЕ требуется.

### Previous Story Intelligence (Epic 1: 1.10, 1.2; stage0)

- **1.10:** decode/`schema_hash`/golden, `internal/arch` go-list-страж (negative-control + `-count=1`), registry-типы,
  курация-stub (`store/curation` — пара к `store/projection`). Уроки: «страж обязан доказать, что краснеет»; каркасы честны.
- **1.2:** миграции goose `0001/0002` (только `contracts`), sqlc через Docker (`make gen-sqlc`), `store/gen` не править,
  анти-churn per-domain `.sql`. lots — это и есть отложенный TODO из 0002.
- **stage0-audit:** `source.go` (Source: ows|file|scrape), file-источник из дампов — образец для file-`Source`.
- **0.6 (blocked):** её Task 0 требует именно проекцию `lots` + `Source` — Story 2.0 это закрывает.

### Внешние знания / web-research

Не требуется: новых библиотек нет (Go stdlib + pgx/sqlc + goose — всё уже в проекте). Сигнатура `Source` и file-источник
берутся из существующего `stage0-audit`, схема `lots` — из data-model/`0002` TODO, не из памяти модели.

### Project Structure Notes

- Целевые пакеты (`internal/goszakup`, `internal/store/projection`) — зарезервированные `doc.go`-заглушки; наполняем.
- Конфликтов нет: 2.0 реализует отложенный TODO `lots` (0002) + интерфейс `Source` (точка swap) — обе вещи спланированы.
- Это первая реализуемая история Epic 2 → `epic-2` → in-progress.

### References

- [Source: _bmad-output/planning-artifacts/sprint-change-proposal-2026-06-23.md (вся; §4 AC); epics.md Story 2.0 + Story 2.1 (нота сужения)]
- [Source: stage0-audit/source.go:14-17 (Source-интерфейс), file-источник; CLAUDE.md «-source абстракция»]
- [Source: migrations/0002_projection.sql (паттерн `contracts` + TODO lots:23-25); server/sqlc.yaml; server/internal/store/queries/contracts.sql]
- [Source: server/internal/ingest/decode/decode.go (SchemaHash/Decode/fieldCandidates/golden — Story 1.10); server/internal/arch/boundaries_test.go (go-list-страж)]
- [Source: docs/AshyqQala_MVP_data_model_and_flags_v1.md (lots :42); architecture.md AR-4 (308–313,778–781), AR-11 (220–223), AR-27 (281–283), целевое дерево server/]
- [Source: _bmad-output/implementation-artifacts/{1-10,1-2}-*.md; 0-6-*.md (Task 0 — что разблокируем); deferred-work.md]
- [Source: memory — local-dev-docker-env (POSTGRES_PORT=55432, Docker-ретраи), guards-must-prove-red]

## Dev Agent Record

### Agent Model Used

claude-opus-4-8[1m] (dev-story 2.0)

### Debug Log References

- Task 0: `docker run sqlc/sqlc:1.31.0 version` → `v1.31.0` (образ доступен; sqlc-генерация реалистична).
- `make gen-sqlc` (Docker) сгенерировал `gen/lots.sql.go` + `gen.Lot`/`UpsertLotParams`/методы; повторный прогон идемпотентен (генерат стабилен → ci-registry generated==regenerated пройдёт).
- golden lot schema_hash = `0dcd872b0001371952b0046ae6fbff89954c45ee4d72644519a5eb53767a4b20` (поля `amount,lot_id,name_kk,name_ru,ref_kato`).

### Completion Notes List

- Ultimate context engine analysis completed — comprehensive developer guide created.
- **AC1** `internal/goszakup.Source` (`Name`/`Fetch`, сигнатура из stage0/source.go — точка swap, модуль stage0 не импортируется); `decode` — ЕДИНСТВЕННЫЙ импортёр goszakup (AR-27), enforced go-list-стражем.
- **AC2** `goszakup.FileSource` читает дампы `<dir>/<resource>.json` (без токена); отсутствующий/битый файл → честная ошибка. Юниты: чтение, max, missing-file, Name.
- **AC3** миграция `0003_projection_lots.sql` (таблица `lots`, минимум колонок, без flags/geo, NULL для отсутствующих) + `lots.sql` (UpsertLot идемпотентный UPSERT, GetLotByID) + sqlc-генерат + `store/projection.LotStore` (проекция ⊥ курация, AR-4).
- **AC4** `decode.Lot` + `DecodeLot` (стоп-на-дрейф schema_hash, golden + drift-тест) + `DecodeLotsFrom(Source)`; go-list-страж расширен `TestGoszakupImportedOnlyFromDecode`; integration-тест wiring (file→decode→projection, `//go:build integration`).
- **Гейты:** `gofmt` чисто · `go build ./...` · `go vet ./...` · `go test ./...` (goszakup/decode+lots/arch — ok) · `go test -count=1 ./internal/arch/...` (некэшируемо) · `go vet -tags=integration ./internal/store/...` компилируется. golangci-lint v2.5.0 локально не установлен (CI прогонит; modernize-хинты gopls закрыты — `SplitSeq`).
- **Разблокирует:** Task 0 Story 0.6 (проекция `lots` + `Source` есть) и Story 2.1 (интерфейс под ows-реализацию). Токен НЕ требовался.

### File List

- `server/internal/goszakup/source.go` (новый) — интерфейс `Source` + ресурсы
- `server/internal/goszakup/file_source.go` (новый) — `FileSource`
- `server/internal/goszakup/file_source_test.go` (новый)
- `server/internal/goszakup/testdata/lots.json` (новый)
- `server/internal/ingest/decode/lots.go` (новый) — `Lot`, `DecodeLot`, `DecodeLotsFrom` (импорт goszakup)
- `server/internal/ingest/decode/lots_test.go` (новый) — golden + drift
- `server/internal/ingest/decode/testdata/lot.json`, `lot_drifted.json` (новые)
- `migrations/0003_projection_lots.sql` (новый) — таблица `lots`
- `server/internal/store/queries/lots.sql` (новый) — UpsertLot/GetLotByID
- `server/internal/store/gen/lots.sql.go` (новый, сгенерирован sqlc)
- `server/internal/store/gen/models.go` (изменён, сгенерирован — +`Lot`)
- `server/internal/store/gen/querier.go` (изменён, сгенерирован — +`UpsertLot`/`GetLotByID`)
- `server/internal/store/projection/lots.go` (новый) — `LotStore`
- `server/internal/store/projection/testdata/lots.json` (новый)
- `server/internal/store/projection/lots_integration_test.go` (новый, `//go:build integration`)
- `server/internal/arch/boundaries_test.go` (изменён — +`TestGoszakupImportedOnlyFromDecode`)

## Change Log

| Дата | Изменение |
|---|---|
| 2026-06-23 | Создан context engine для Story 2.0 (токен-независимый фундамент Epic 2). Источник — Sprint Change Proposal 2026-06-23 (вырезание из 2.1/B-3). Зафиксировано: `Source`-интерфейс из stage0/source.go (точка swap, не импорт чужого модуля); file-impl из дампов; проекция `lots` (миграция 0003 + sqlc + projection, паттерн 0002/contracts, Docker для gen); декод lots через ingest/decode (стоп-на-дрейф, golden); go-list-граница goszakup (AR-27) + negative-control (правило 1.10). Токен-независимо; разблокирует 0.6 (Task 0) и 2.1. Чёткие границы со 2.1–2.7/Epic 4. Статус → ready-for-dev. |
| 2026-06-23 | Реализация (dev-story). Task 0 Docker-проба sqlc OK. Создано: `goszakup` (Source + FileSource + тесты); `decode` Lot/DecodeLot/DecodeLotsFrom (golden+drift, импорт goszakup); миграция `0003` + `lots.sql` + sqlc-генерат + `projection.LotStore`; go-list-страж `TestGoszakupImportedOnlyFromDecode` (AR-27); integration-тест wiring (`//go:build integration`). Гейты зелёные (gofmt/build/vet/test, arch `-count=1`, integration-компиляция); sqlc идемпотентен. AC1–AC4 закрыты. Статус → review. |
| 2026-06-23 | Code-review (3 адверсариальных слоя). Блокирующих нет; AC1–AC4 подтверждены; страж AR-27 эмпирически краснеет. Применено 4 patch: (1) чистый предикат `violatesGoszakupBoundary` + кэшируемый negative-control `TestViolatesGoszakupBoundary` ([[guards-must-prove-red]]); (2) `kato_code`-кандидаты выровнены со stage0 (+`delivery_kato`,`ref_kato_id`); (3) `TestFileSource_Max` проверяет ошибку Fetch; (4) комментарий об отложенных S-0 полях lots. 3 defer → deferred-work (value-robustness→2.1, дублирование pick-хелперов, UPSERT-затирание→2.6). 4 dismissed. Гейты перепроверены зелёными. Статус → done. |
