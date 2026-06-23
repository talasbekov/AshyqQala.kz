---
baseline_commit: b5258b5bd7f8d6e1bccbb16b89e0255ef8b7d513
---

# Story 1.10: Общие контракты ядра (чистые сигнатуры, decode-на-дрейф, курация-stub, границы)

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a **команда**,
I want **зафиксировать общие контракты в backbone**,
so that **будущие потребители не приватизируют ядро и не создают обратных зависимостей**.

> **Тип истории:** enabler-story Epic 1 (backbone), **backend-only (Go)**. Идёт на синтетике, не ждёт
> токен/гейт №0. Это **каркас контрактов**, НЕ расчётное ядро: объявить чистые сигнатуры, доказать
> границы импортов go-list-тестом, реализовать `decode`+`schema_hash` на синтетическом дрейфе с
> golden-снапшотом, собрать курация-stub под будущую S-0-приёмку Epic 2.
> [Source: epics.md:1023–1041,830–832; AR-13/AR-11/AR-27/AR-4/AR-10]

## Acceptance Criteria

**AC1 — Чистые сигнатуры + go-list границы** [epics.md:1031–1033]
**Given** `internal/`
**When** определены чистые сигнатуры `median(samples) → (value, state)`, `flag(inputs, params) → state`,
`normalize → BIN` (без store/IO/реального времени) и `internal/clock` (Real|Fixed)
**Then** go-list CI-тест запрещает им импортировать `store`/`httpapi`/`goszakup`/реальное время.

**AC2 — decode + schema_hash на дрейф** [epics.md:1035–1037]
**Given** `internal/decode` + `schema_hash`
**When** прогон на синтетике, имитирующей дрейф схемы
**Then** изменение зафиксированного контракта → стоп+алерт (не тихий 0); golden-снапшот декодера.

**AC3 — Курация-stub** [epics.md:1039–1041]
**Given** курация-stub
**When** смоделирована ручная правка проекционной строки (без полного Directus-UI)
**Then** механика «правка переживает перезапись» доступна для будущей S-0-приёмки (Epic 2).

## Tasks / Subtasks

- [x] **Task 1 — `internal/clock` (Real|Fixed) (AC1)**
  - [x] Наполнить `server/internal/clock/` (заглушка `doc.go`): интерфейс `Clock { Now() time.Time }`, `Real` (через `time.Now`), `Fixed` (детерминированное «сейчас» для тестов). Это **единственное** место, где ядро берёт время [AR-13; architecture.md:298,437]
  - [x] Юнит-тест: `Fixed.Now()` детерминирован; `Real` возвращает текущее. `clock` — единственный пакет ядра, которому разрешён `import "time"`
- [x] **Task 2 — Чистые сигнатуры median/flag/normalize (AC1)**
  - [x] `server/internal/median/`: `Median(samples) → (value, registry.ValueState)`. При `len(samples) < min_sample (=5)` → `value=nil` + `StateInsufficientSample` (медиана НЕ показывается, НЕ NaN/0); иначе `StateOK` + медиана. Чистая, детерминированная, **без store/IO/времени** [epics.md:1032; FR-18/FR-20; data-model min_sample=5]
  - [x] `server/internal/flags/`: `Flag(inputs, params) → registry.FlagState` — **сигнатура-каркас** (без реальных формул 4 флагов — Epic 4). `params` — минимальный тип-задел, согласованный с `render.Params` (не плодить конкурирующее определение). Дата-зависимость (если нужна) — ТОЛЬКО через `clock.Clock` аргументом [epics.md:1032; render.go Params]
  - [x] `server/internal/normalize/`: `Normalize(name) → BIN` — чистая сигнатура-контракт приведения к каноническому БИН. Содержательная нормализация (статусы auto/manual/conflict) — Epic 2 Story 2.3; здесь только контракт [epics.md:1032; FR-2]
  - [x] Все три берут типы состояний из `internal/registry` (`ValueState`/`FlagState`) — **НЕ переопределять** [registry.go]
  - [x] (смежно) `server/internal/benchmark/` — если затрагивается, тот же инвариант чистоты; реальное наполнение `price_benchmarks` — Epic 4
- [x] **Task 3 — go-list CI-страж границ импортов (AC1)**
  - [x] Написать архитектурный тест (Go-тест по образцу `ci-registry.yml` — страж как `go test`, не bash): через `go list -f '{{ join .Imports "..." }}'` (ПРЯМЫЕ импорты) проверить, что `internal/{median,flags,normalize,benchmark}` НЕ импортируют `internal/store`, `internal/httpapi`, `internal/goszakup`, `time` (напрямую), драйверы `pgx`/`chi`. Разрешено: stdlib (кроме `time`), `internal/registry`, `internal/clock`. Красный при нарушении [AR-27; epics.md:1033]
  - [x] Включить страж в `.github/workflows/ci-server.yml` (сейчас он только TODO-комментарий, строки 41–42) и в `Makefile` (таргет `check-core` по образцу `check-registry`)
- [x] **Task 4 — `internal/decode` + `schema_hash` на синтетическом дрейфе (AC2)**
  - [x] Наполнить `server/internal/ingest/decode/`: `SchemaHash` по **нормализованной key-sorted** схеме (иначе хеш «мигает»); функция decode синтетического ows_v2-образного объекта → доменный тип [AR-11; architecture.md:349]
  - [x] **Политика «стоп+алерт, не тихий 0»:** при изменении зафиксированного контракта схемы (несовпадение `schema_hash` / пропажа обязательного поля) — вернуть ошибку (detect-and-halt), НЕ молчаливый `0`/`""`. Это инверсия политики stage0 (аудит=тихо → прод=стоп) [AR-11; epics.md:1037]
  - [x] **golden-снапшот декодера** (`fixtures/golden/decode/`): прогон на синтетике + тест декодера; синтетика, имитирующая дрейф → стоп+алерт. **Только синтетика** — живой ows_v2-декод и реальный snapshot B-1 — Story 2.1 (которая берёт `internal/decode` «из 1.10»); `internal/goszakup` НЕ наполнять [epics.md:1059; architecture.md:766–767]
  - [x] Концепт `fieldCandidates` переиспользовать из `stage0-audit/models.go` (другой Go-модуль — **импортировать нельзя**, перенести/переписать минимально)
- [x] **Task 5 — Курация-stub: «правка переживает перезапись» (AC3)**
  - [x] Модель границы (AR-4/AR-10): ПРОЕКЦИОННЫЕ таблицы (derived из снапшота, импортёр перестраивает) ⊥ КУРАТОРСКИЕ (ручные правки; импортёр только читает) — раздельные пакеты `internal/store/projection` и `internal/store/curation` (граница в Go, не только в БД) [architecture.md:308–313,778–781]
  - [x] Наполнить `server/internal/store/curation/` минимальным stub: смоделировать ручную правку проекционной строки (без Directus-UI) и механику, при которой правка **переживает перезапись** проекции (re-import только читает курацию)
  - [x] Минимальная миграция кураторской таблицы (напр. `migrations/0003_curated.sql`) под stub, если требуется; держать минимальной
  - [x] Тест механики: проекция → ручная курация → перезапись проекции → правка цела. Может быть `//go:build integration` (testcontainers, как `store/contracts_integration_test.go`) ИЛИ детерминированная in-memory симуляция (CI без docker). **Полная S-0-приёмка «2 импорта + курация» — Story 2.6**, не здесь [epics.md:1041,1152–1166; AR-10]
- [x] **Task 6 — Тесты и финализация**
  - [x] Юнит/property: `median` — property «insufficient однозначен» (нельзя `value` при `insufficient`, и наоборот); закрытые union покрыты + default-ветка (по образцу `render_test.go`); table-driven с русскими сообщениями [architecture.md:598; AR-26]
  - [x] golden decode-тест; go-list-страж зелёный (Task 3); честный-fail тесты (рассинхрон → ошибка, не «пусто»)
  - [x] Зелёные: `cd server && go build ./... && go vet ./... && go test ./...`; `gofmt -l .` пусто; `make lint` (golangci-lint v2.5.0). Обновить File List, Change Log, Completion Notes

### Review Findings

_Code review 2026-06-23 — адверсариальные слои (Blind Hunter, Edge Case Hunter, Acceptance Auditor). AC2/AC3 подтверждены полностью. По AC1 найден РЕАЛЬНЫЙ (эмпирически воспроизведён) дефект enforcement. Итог: 5 patch, 4 defer, ~5 dismissed._

- [x] [Review][Patch][CRITICAL] go-list-страж маскировался кэшем `go test` (нарушение в median/flags/normalize проходило `go test ./...` как `ok (cached)` — результат `exec go list` не в инпутах тест-кэша). Подрывал основной deliverable AC1. **fixed:** отдельный CI-шаг `go test -count=1 ./internal/arch/...` (ci-server.yml) + `make check-core` с `-count=1`. **Эмпирически проверено:** под `-count=1` внедрённый `import "time"` в median → FAIL; под обычным `go test` — нет. Попытка blank-import core-пакетов оказалась неэффективной (кэш не инвалидировался) — отброшена, мех-зм честно задокументирован в комментарии теста. [server/internal/arch/boundaries_test.go, .github/workflows/ci-server.yml, Makefile]
- [x] [Review][Patch] `median` чётная выборка: `(s[n/2-1]+s[n/2])/2` переполняет int64 на больших суммах (отрицательная медиана под `ok`) → overflow-safe `s[n/2-1] + (s[n/2]-s[n/2-1])/2` [server/internal/median/median.go]
- [x] [Review][Patch] `flags.Flag` по дефолту `not_raised` при наличии clock — нечестное «всё чисто» (каркас НЕ считает формулы). Честнее `insufficient_data` всегда (нельзя оценить до Epic 4) [server/internal/flags/flags.go]
- [x] [Review][Patch] go-list-страж не доказывает, что умеет КРАСНЕТЬ → вынести предикат в чистый `isForbidden(imp)` + табличный тест (time/store → forbidden; registry/clock → allowed) [server/internal/arch/boundaries_test.go]
- [x] [Review][Patch] golden decode не пинит хеш-литерал (`pinned` пересчитывается из входа) → дрейф алгоритма `SchemaHash` не ловится. Пин эталонной константы + assert [server/internal/ingest/decode/decode_test.go]
- [x] [Review][Defer] decode value-robustness: present-но-null / число строкой / float→int64 усечение → тихий пропуск суммы (на УРОВНЕ ЗНАЧЕНИЯ). Field-set дрейф (AC2) ловится; квирки реального ows_v2 — Story 2.1 — deferred
- [x] [Review][Defer] курация: конфликт overrides (last-wins, есть `source_conflict`) + пустой `Field` без валидации → реальная курация/Directus — Epic 2 — deferred
- [x] [Review][Defer] `clock.Fixed{}` zero-value / typed-nil не отличаются от валидного в `flags` → актуально в Epic 4 (реальное использование clock) — deferred
- [x] [Review][Defer] страж: `benchmark` вакуумно-зелёный (пуст); `decode`/`clock` вне pure-core (корректно). benchmark наполняется Epic 4 — deferred

## Dev Notes

### Контекст истории (зачем и где границы)

Epic 1 фиксирует общие контракты «позвонка», чтобы будущие эпики не приватизировали ядро и не создавали
обратных зависимостей. Story 1.10 — **последняя в Epic 1** и закрывает backbone-контракты: чистые
сигнатуры (median/flag/normalize), часы, decode-границу со `schema_hash`, курацию-stub и **исполняемые
границы импортов** (go-list). Это **каркас**, не расчёт. [Source: epics.md:1023–1041,438,519–523]

**Все три потребителя дословно ссылаются «из 1.10»:** Story 2.1 (живой decode, epics.md:1059), Story 2.6
(S-0-приёмка с `Clock`, :1162), Story 4.1 (реальный median, :1374). Поэтому контракты должны быть
стабильны и чисты.

### Фактическое состояние кода (всё — заглушки, наполнять)

Скелет `server/` существует; целевые пакеты — **однострочные `doc.go`-заглушки** (тел нет), наполняет 1.10:
`internal/median`, `internal/flags`, `internal/normalize`, `internal/benchmark`, `internal/clock`,
`internal/ingest/decode`, `internal/store/curation`, `internal/store/projection`. `schema_hash` отдельного
пакета НЕТ — он внутри `internal/ingest/decode`. go-list-страж НЕ написан (только TODO в `ci-server.yml:41–42`).
[Source: agents — фактическое чтение server/internal/]

### Что переиспользовать (НЕ изобретать, НЕ дублировать)

- **`internal/registry` (Story 1.4):** закрытые union-типы — `ValueState` (11 членов: `ok/no_data/
  insufficient_sample/not_comparable/stale/geocode_pending/geocode_failed/source_conflict/redacted/
  not_applicable/error`), `FlagState` (4: `raised/not_raised/insufficient_data/not_published`), `Locale`
  (`KK`/`RU`). `median`→`ValueState`, `flag`→`FlagState`. **НЕ переопределять** эти типы.
- **`internal/render` (Story 1.4):** `Renderer`/`Render`/`RenderFlagState` + типы-задел `Evidence`/`Params`.
  `flag(inputs, params)` — `params` согласовать с `render.Params` (не плодить конкурирующее определение).
- **Образцы стиля (копировать, не изобретать):** закрытый enum — `internal/apierr/codes.go`,
  `internal/registry/registry.go`; чистый `switch` + обязательная `default` — `internal/render/render.go`;
  DB-free table-тест против артефакта — `internal/httpapi/contracts_test.go`; honest-fail тесты —
  `registry_test.go::TestLoad_*_HonestError`; build-tag integration — `store/contracts_integration_test.go`.
- **Концепт `fieldCandidates`** — из `stage0-audit/models.go` (логическое поле → список кандидатов имён);
  модуль `ashyqqala/stage0-audit` ОТДЕЛЬНЫЙ — импортировать нельзя, переписать минимально для decode.

### Соблюдение архитектуры (guardrails)

- **Чистота ядра (AR-13):** `median/flag/normalize/benchmark` — чистые функции, детерминизм, **без
  store/IO/`time.Now`**. Время — только через `clock.Clock` аргументом. [architecture.md:121,776]
- **Исполняемые границы (AR-27, go-list):** ядро НЕ импортирует `store`/`httpapi`/`goszakup`/реальное время;
  `goszakup` — только из `ingest/decode`; `og`/`bot` — прозу только через `render`. Страж = Go-тест, красный
  блокирует merge. [architecture.md:774–777]
- **decode честность (AR-11):** «стоп+алерт, не тихий 0» при дрейфе схемы; `schema_hash` по key-sorted
  нормализованной схеме; golden-снапшот. [architecture.md:150–151,349]
- **Граница данных (AR-4/AR-10):** проекционные ⊥ кураторские (разные гранты И раздельные Go-пакеты);
  курируемое переживает ре-импорт (импортёр только читает курацию). [architecture.md:308–313,778–781]
- **Честные состояния (AR-16):** `median` при `n<min_sample` → `insufficient_sample` (не 0/NaN). HTTP-коды
  не используются (это чистые функции). [epics.md:238–242]

### Границы скоупа — ЧТО ОТКЛАДЫВАЕТСЯ (НЕ делать в 1.10)

| Отложено | Куда |
|---|---|
| Реальный compute median/benchmark/4 флагов (формулы) | Epic 4 (4.1–4.6); median-сигнатура «из 1.10» наполняется в 4.1 |
| Живой decode реального ows_v2 + реальный `schema_hash` по B-1 + nightly | Story 2.1 (берёт `internal/decode` из 1.10) |
| Полная S-0-приёмка «2 импорта + курация» (testcontainers) | Story 2.6 |
| Содержательная нормализация наименований→БИН (auto/manual/conflict) | Story 2.3 (FR-2) |
| Иммутабельный движок `methodology_params` (версионирование) | Story 4.1 |
| Наполнение `internal/goszakup` (клиент ows_v2) | после B-1 / Epic 2 |
| Реальный Directus-UI курации | Epic 2/3 |

[Source: epics.md:1051–1069,1152–1166,1362–1380,1091–1109; architecture.md:124–127,766–767]

### Файлы и куда писать

- **Наполнить (заглушки → код):** `server/internal/clock/`, `server/internal/median/`, `server/internal/flags/`,
  `server/internal/normalize/`, `server/internal/ingest/decode/`, `server/internal/store/curation/`
  (+ возможно `server/internal/store/projection/`, `server/internal/benchmark/`).
- **Добавить:** go-list-страж (Go-тест, напр. `server/internal/arch/boundaries_test.go` или в каждом пакете),
  golden-фикстуры (`fixtures/golden/decode/`), возможно `migrations/0003_curated.sql` (минимальная кураторская
  таблица), таргет `check-core` в `Makefile`.
- **Изменить:** `.github/workflows/ci-server.yml` (включить go-list-страж — сейчас TODO-комментарий).
- **НЕ трогать:** `internal/httpapi/*`, `internal/store/gen/*` (sqlc-генерат), `internal/registry/*`,
  `internal/render/*` (1.4 — использовать как контракт типов, не переписывать), `stage0-audit/` (другой модуль),
  фронт `web/` (1.10 — backend-only).

### Тестирование / проверка готовности

- **Раннеры:** `go test ./...` (unit/property/golden co-located `*_test.go`); integration — за `//go:build
  integration` (не в обычном прогоне). Запуск через `Makefile` (`make test`, `make lint`).
- **Ключевые тесты:** property «insufficient однозначен» для `median`; golden decode-снапшот + дрейф→стоп;
  go-list-страж границ; honest-fail (рассинхрон → ошибка, не «пусто»); закрытый union + default-ветка.
- **DoD:** AC1–AC3 закрыты; `go build`/`go vet`/`go test ./...` зелёные; `gofmt -l .` пусто; `make lint`
  чистый; go-list-страж включён в CI и красный при нарушении границ; decode на дрейфе даёт стоп+алерт;
  курация-stub доказывает выживание правки; ядро не импортирует запрещённое.

### Previous Story Intelligence (1.4, 1.9, 1.1/1.2)

- **1.4 (registry/render):** дал `ValueState`/`FlagState`/`Locale` + `render` с обязательной default-веткой;
  CI-страж нейтральности = Go-тест в `ci-registry.yml` (образец «страж как go test»). Числа в прозе запрещены.
- **1.9:** `flag`/evidence были РУЧНЫЕ на фронте (без compute); 1.10 даёт backend-контракт `flag(inputs,
  params)`, который Epic 4 наполнит реальной логикой. `render.Params` — задел, согласовать.
- **1.1/1.2:** монорепо `server/` (модуль `ashyqqala/server`, Go 1.25), миграции goose `0001/0002` (только
  `contracts`), sqlc-генерат в `store/gen` (не править), `ci-server.yml`/`ci-registry.yml` с path-фильтрами.

### Внешние знания / web-research

Не требуется: история не вводит новых библиотек (stdlib Go + существующий `internal/registry`). `normalize`
в 1.10 — только сигнатура-каркас, поэтому `golang.org/x/text` (NFKC) пока не нужен (реальная нормализация —
Story 2.3); если понадобится — это первое прямое использование, взвесить против «без новых зависимостей».

### Project Structure Notes

- Фундамент готов (1.1–1.9): все целевые пакеты ядра зарезервированы как `doc.go`-заглушки — наполняем, не
  пересоздаём. Предусловного HALT нет.
- Конфликтов нет: 1.10 наполняет зарезервированные контракты-каркасы и пишет недостающий go-list-страж.
- 1.10 — backend-only; фронт не затрагивается. После 1.10 Epic 1 завершён → возможна `retrospective`.

### References

- [Source: epics.md#Story-1.10 (1023–1041); backbone (438,519–523); AR-13 (228–230), AR-11 (220–223), AR-27 (281–283), AR-4 (197–199), AR-10 (215–217), AR-16 (238–242), AR-26 (276–280); потребители 2.1 (1051–1069), 2.6 (1152–1166), 4.1 (1362–1380)]
- [Source: architecture.md — чистое ядро (121,776); clock (298,437); decode/schema_hash (150–151,349,426–427); go-list границы (774–777); курация (308–313,778–781); целевое дерево (723–737); methodology_params иммутабельность (124–127); тест-каркас (342–352,815–817)]
- [Source: docs/AshyqQala_MVP_data_model_and_flags_v1.md — min_sample=5, ×1.5, share=0.5 (72–81); addendum.md (28–31,36)]
- [Source: server/internal/{registry/registry.go,render/render.go,apierr/codes.go,httpapi/contracts_test.go} — образцы типов/стиля; doc.go-заглушки median/flags/normalize/benchmark/clock/ingest/decode/store/{curation,projection}]
- [Source: .github/workflows/{ci-server.yml (go-list TODO :41–42),ci-registry.yml (страж как go test)}; Makefile (test/lint/check-registry); stage0-audit/models.go (fieldCandidates — образец, не импорт)]
- [Source: _bmad-output/implementation-artifacts/{1-4,1-9,1-1,1-2}-*.md; CLAUDE.md — гардрейлы честности/нейтральности, чистое ядро, go-list границы]

## Dev Agent Record

### Agent Model Used

claude-opus-4-8[1m] (Claude Opus 4.8, 1M context) — dev-story workflow.

### Debug Log References

- `cd server && go build ./...` OK · `go vet ./...` OK · `gofmt -l .` пусто · `go test ./...` — все пакеты `ok` (arch/clock/flags/median/normalize/ingest·decode/store·curation + регрессия registry/render/httpapi).
- go-list-страж (`internal/arch`) отработал: чистое ядро не импортирует store/httpapi/goszakup/время — тест зелёный.
- Модернизация под Go 1.25 (чисто для golangci v2.5.0): `slices.Sort`, `maps.Copy`, `strings.FieldsSeq`; убран тривиальный тест (staticcheck SA4000). `.golangci` конфига нет → дефолтные линтеры (errcheck/govet/ineffassign/staticcheck/unused).

### Completion Notes List

- **AC1 — чистые сигнатуры + границы:** `internal/clock` (`Clock`/`Real`/`Fixed`); `internal/median` (`Median([]int64)→(*int64, ValueState)`, n<MinSample=5 → insufficient_sample, без store/IO/времени); `internal/flags` (`Flag(Inputs, Params)→FlagState`, время через `clock`, формулы 4 флагов — Epic 4); `internal/normalize` (`Normalize(string)→BIN`, реальная нормализация — Story 2.3). go-list-страж `internal/arch` запрещает ядру импорт `store/httpapi/goszakup/time`/драйверов — гоняется `go test ./...` (+ `make check-core`), красный при нарушении.
- **AC2 — decode + schema_hash:** `internal/ingest/decode` — `SchemaHash` (key-sorted), `Decode(raw, pinned)` → `ErrSchemaDrift` (стоп+алерт, НЕ тихий 0) при дрейфе зафиксированной схемы; golden в `testdata/` (contract.json + drifted.json). Концепт `fieldCandidates` перенесён из stage0 (модуль не импортируется). Живой ows_v2-декод — Story 2.1.
- **AC3 — курация-stub:** `internal/store/curation.Apply(projection, overrides)` — кураторская правка хранится отдельно и **переживает перезапись** проекции импортёром (AR-10); тест доказывает на двух «импортах». Полная S-0-приёмка (реальные импорты + Directus + БД) — Story 2.6. DB-backed кураторская таблица не создавалась (Epic 2): механика-stub детерминированная, CI-safe.
- **Переиспользовано:** `registry.ValueState`/`FlagState` (типы состояний), стиль закрытых union + табличных тестов (1.4). Удалены 6 устаревших `doc.go`-заглушек наполненных пакетов (package-doc теперь в реальных файлах).
- **Не делал (граница):** реальный compute (Epic 4), живой decode/импорт (Story 2.1), полная S-0-приёмка (2.6), реальная нормализация (2.3), движок `methodology_params` (4.1), `internal/goszakup`. Фронт не затрагивался.
- **Epic 1 завершён** этой историей (1-1…1-10) → возможна `retrospective`.

### File List

**Новые:**
- `server/internal/clock/{clock.go,clock_test.go}` — Clock (Real|Fixed).
- `server/internal/median/{median.go,median_test.go}` — чистая медиан-сигнатура + property insufficient.
- `server/internal/flags/{flags.go,flags_test.go}` — чистая сигнатура-каркас флага (время через clock).
- `server/internal/normalize/{normalize.go,normalize_test.go}` — чистая сигнатура normalize→BIN.
- `server/internal/ingest/decode/{decode.go,decode_test.go,testdata/contract.json,testdata/drifted.json}` — decode + schema_hash + golden/дрейф.
- `server/internal/store/curation/{curation.go,curation_test.go}` — курация-stub (правка переживает перезапись).
- `server/internal/arch/{doc.go,boundaries_test.go}` — go-list-страж границ ядра.

**Изменённые:**
- `Makefile` — таргет `check-core` (go-list-страж).
- `.github/workflows/ci-server.yml` — комментарий: go-list/golden/schema_hash/property теперь активны через `go test ./...`.

**Удалённые:** `server/internal/{clock,median,flags,normalize,ingest/decode,store/curation}/doc.go` — устаревшие заглушки (package-doc перенесён в реальные файлы).

> _`internal/{benchmark,store/projection,goszakup}/doc.go` оставлены как заглушки (наполняются позже: Epic 4 / Epic 2)._

## Change Log

| Дата | Изменение |
|---|---|
| 2026-06-23 | code-review (3 слоя): AC2/AC3 подтверждены; по AC1 вскрыт РЕАЛЬНЫЙ enforcement-дефект. 5 patch / 4 defer / ~5 dismissed. **Применены 5 patch:** (1) 🔴 go-list-страж маскировался кэшем `go test` → отдельный CI-шаг `-count=1` + `make check-core -count=1` (эмпирически доказано: внедрённый `import "time"` → FAIL); (2) median overflow-safe (`lo+(hi-lo)/2`); (3) `flags.Flag` → честный `insufficient_data` (не ложное not_raised); (4) negative-control стража (чистый `isForbidden` + табличный тест); (5) пин эталонного `schema_hash` в golden. Defer (decode value-robustness→2.1, конфликт курации→Epic 2, clock zero-value→Epic 4, benchmark-покрытие) → `deferred-work.md`. Гейты зелёные: build/vet/gofmt/`go test -count=1 ./...` (10 пакетов ok). Статус → done. **Epic 1 закрыт полностью (1-1…1-10).** |
| 2026-06-23 | dev-story: реализованы общие контракты ядра (backend, Go). Чистые сигнатуры `median`/`flag`/`normalize` + `clock` (Real|Fixed) — без store/IO/времени; go-list-страж `internal/arch` (запрет импорта store/httpapi/goszakup/time, гоняется `go test ./...` + `make check-core`); `internal/ingest/decode` + `schema_hash` (стоп+алерт на синтетическом дрейфе, golden в testdata); курация-stub `store/curation.Apply` (правка переживает перезапись, AR-10). Переиспользованы registry-типы; удалены 6 устаревших doc.go-заглушек. AC1–AC3 закрыты. Гейты зелёные: `go build`/`go vet`/`gofmt`/`go test ./...` (все пакеты ok). Статус → review. **Epic 1 завершён (1-1…1-10).** |
| 2026-06-23 | Создан context engine для Story 1.10 (общие контракты ядра, backend). Параллельный анализ (3 субагента): epics.md (спека 1.10 + AR-11/13/27/4/10 + потребители 2.1/2.6/4.1), architecture.md (чистое ядро, clock, decode/schema_hash, go-list границы, курация), фактический код server/ (все целевые пакеты — doc.go-заглушки; registry даёт ValueState/FlagState; go-list-страж не написан). Зафиксировано: 1.10 — КАРКАС контрактов (не compute); чистые сигнатуры median/flag/normalize + clock(Real|Fixed); go-list CI-страж границ (написать первым); decode+schema_hash на СИНТЕТИЧЕСКОМ дрейфе + golden; курация-stub под S-0-приёмку Epic 2. Переиспользовать registry-типы, не дублировать. Чёткие границы со Epic 2/4. Статус → ready-for-dev. |
