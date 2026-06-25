---
baseline_commit: b41db864a7c3b1dced2a093bb57c2a6d048e57d5
---
# Story 4.1: methodology_params (иммутабельный) + median/benchmark-движок

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a **команда платформы AshyqQala.kz**,
I want **версионируемый ИММУТАБЕЛЬНЫЙ конфиг порогов (`methodology_params`) и пересчитываемый ЧИСТЫЙ движок медиан/бенчмарков (`price_benchmarks`)**,
so that **все 4 флага риска (4.2–4.5) и страница медиан района (FR-18) считаются по ОДНОЙ публичной, пересчитываемой третьим лицом методике, а пустая/малая выборка честно даёт «недостаточно сопоставимых данных», а не выдуманное число**.

> **Тип истории:** enabler-история Эпика 4 (compute, **backend-only**). Реализует фундамент FR-18/FR-20/FR-23. **Гардрейл эпика:** выход НЕ виден пользователю и НЕ «опубликованный флаг» — приёмка на ДАННЫЕ (Go-ассерт), не на экран. [Source: epics.md:1380–1407, 1503]
>
> **Скоуп токена (ЧЕСТНО):** движок (`median`/`benchmark`), иммутабельный `methodology_params`, состояние `insufficient_sample` и детерминизм — **token-НЕзависимы**, строятся и доказываются на синтетике (как весь Эпик 1). **AC4 (порядок benchmark-перед-флагами в постимпортном хуке) зависит от Story 2.4 (ещё НЕ построена)** → 4.1 поставляет ВЫЗЫВАЕМЫЙ оркестратор пересчёта + atomic swap и фиксирует инвариант порядка юнит-тестом; живое подключение хука к импорту — в 2.4. НЕ ждать 2.4. [Source: epics.md:1385–1387, 1405–1407; architecture.md:880–881; см. «Открытые вопросы» Q1]

## Acceptance Criteria

**AC1 — `methodology_params`: версионируемый ИММУТАБЕЛЬНЫЙ конфиг порогов с дефолтами**
**Given** `methodology_params` как ВЕРСИОНИРУЕМЫЙ ИММУТАБЕЛЬНЫЙ конфиг (правка = новая версия, история НЕ переписывается — требование к DDL до первой записи; B-4)
**When** значение читается в рантайме
**Then** доступны дефолты: `median.comparability_key = direction×kato×24мес(скользящее)`, `median.min_sample=5`, `flag.price_per_km.deviation_factor=1.5`, `flag.monopoly.concentration_share=0.5`, `flag.monopoly.min_group_contracts=5`; источник истины — `registry/values/methodology_params.v1.yaml` (человек правит ТОЛЬКО здесь, читается в РАНТАЙМЕ, формат версии `^v\d+\.\d+$`); каноническая `methodology_version` доступна и протекает в `flags.Params`/`render.Params`/`evidence`.
[Source: epics.md:1397–1399; architecture.md:124–125, 218 (B-4), 321–323, 640–641, 787–789; data-model v1 §2]

**AC2 — Чистый median/benchmark-движок: `(value, state)`, выборка < `min_sample` → `insufficient_sample`**
**Given** УЖЕ реализованная чистая `median.Median(samples) → (*int64, registry.ValueState)` (сигнатура из 1.10, `MinSample=5`) + новый пакет `internal/benchmark`
**When** наполняется кэш медиан `price_benchmarks` (группа сопоставимости `direction×kato×24мес`, опорное время — ПАРАМЕТРОМ через `clock.Clock`, НЕ `time.Now`)
**Then** для группы с `comparable_n ≥ min_sample` возвращается `(median_price_per_km, StateOK)`; при `comparable_n < min_sample` — `StateInsufficientSample` (медиана НЕ показывается, не 0/NaN); граничные `n=0/1/2` → честное состояние, не паника; пакет `benchmark` остаётся ЧИСТЫМ (страж `internal/arch` зелёный: нет импортов `store`/`httpapi`/`goszakup`/`time`).
[Source: epics.md:1401–1403; architecture.md:119–121, 160, 596–598, 775–776; median.go (`Median`, `MinSample`); registry.go (`StateInsufficientSample`, `StateNotComparable`)]

**AC3 — `price_benchmarks` как производный кэш: пересчёт «в сторону → атомарный swap»**
**Given** таблица `price_benchmarks` (`comparability_key`, `median_price_per_km`, `sample_size`, `computed_at`) — производное от чистой функции (одна истина)
**When** запускается пересчёт бенчмарков
**Then** пересчёт идёт «в сторону» и публикуется АТОМАРНЫМ swap (читатель видит снапшот целиком — старый или новый, не полупересчёт); `sample_size` фиксируется; пересчёт детерминирован (тот же вход + та же `methodology_version` → тот же результат).
[Source: epics.md:1401–1403; architecture.md:121, 148–149, 400, 880–881; data-model v1:56]

**AC4 — Контракт порядка: benchmark пересчитывается ПЕРЕД флагами (хук-вызов — в 2.4)**
**Given** постимпортный хук (Story 2.4 — ещё НЕ построен)
**When** импорт завершён (живое подключение — в 2.4)
**Then** benchmark пересчитывается ПЕРЕД флагами (флаг НИКОГДА не считается против устаревшего benchmark); 4.1 поставляет ВЫЗЫВАЕМЫЙ оркестратор `recalc` с зашитым порядком (benchmark→swap→flags) + юнит-тест порядка; реальная привязка к импорт-конвейеру и атомарная публикация снапшота ДЕСКОУПЛЕНЫ на 2.4 (честно, без выдуманного хука).
[Source: epics.md:1405–1407; architecture.md:292–293, 804–805, 880–881; «Открытые вопросы» Q1]

**AC5 — Пересчитываемость и гардрейлы (несущие)**
**Given** гардрейлы платформы
**When** движок принимается
**Then** (а) числовые пороги берутся ТОЛЬКО из `methodology_params` (литералы порогов в коде/прозе запрещены, кроме одного источника дефолтов); (б) `methodology_version` в evidence == текущей в params (перекрёстный инвариант); (в) выход НЕ виден пользователю и НЕ «опубликованный флаг» — приёмка на ДАННЫЕ (Go-ассерт/SQL), не на экран; (г) честность: `insufficient`/`not_comparable` — ТОЛЬКО при `comparable_n < min_sample` (property-тест однозначности: нельзя показать медиану при insufficient и наоборот).
[Source: epics.md:1491, 1503; architecture.md:328–329, 596–598; prd.md#FR-23, §7.1, §7.4; data-model v1 преамбула]

## Tasks / Subtasks

- [x] **Task 0 — РЕШЕНИЯ ВЛАДЕЛЬЦУ (см. «Открытые вопросы»)** *(AC1–AC4)*
  - [x] Q1: подтвердить ДЕСКОУП AC4 (хук — в 2.4; 4.1 даёт вызываемый оркестратор+swap+тест порядка). **Рекоменд.: дескоуп** (как 0.4 фиксировала инвариант, реализуемый позже).
  - [x] Q2: место `methodology_params` — **рекоменд.: YAML `registry/values/methodology_params.v1.yaml` = источник истины (рантайм-чтение, registry single-source) + DB-таблица `methodology_params` как иммутабельный version-реестр (append-only) для evidence/пересчёта**. Альтернатива — только YAML или только DB.
  - [x] Q3: формат `median_price_per_km` (целые ₸/км vs дробные) → если дробные, ввести детерминированный round в `benchmark` (`math` разрешён ядру). **Рекоменд.: целые ₸/км** (как `amount_tng` bigint; без плавающей точки → детерминизм бесплатен).
  - [x] Q4: расширять ли `fixtures/seed/contracts.sql` до ≥5 сопоставимых для OK-пути. **Рекоменд.: добавить синтетические фикстуры под `fixtures/golden/medians/` (границы) + оставить 3 DEMO как insufficient-кейс** (не ломать демо 1.9).
- [x] **Task 1 — `methodology_params`: иммутабельный конфиг + рантайм-загрузчик + version** *(AC1)*
  - [x] `registry/values/methodology_params.v1.yaml`: дефолты (Q2), `methodology_version: v1.0` (формат `^v\d+\.\d+$`), все 5 порогов из data-model §2.
  - [x] Миграция `migrations/0005_methodology_params.sql` (goose Up/Down): таблица (`key, value, description, version, effective_from`) + **DDL-иммутабельность (B-4):** триггер/правило, запрещающий `UPDATE`/`DELETE` строк версии (append-only; правка = новая версия). Down — drop.
  - [x] sqlc: `internal/store/queries/methodology_params.sql` (read по версии) → `make gen-sqlc` (Docker sqlc 1.31.0; править `gen/` руками НЕЛЬЗЯ).
  - [x] Go-загрузчик (НЕ в ядре `benchmark`, чтобы не нарушить страж) типа `internal/methodology` или поверх registry-ридера: парсит YAML, валидирует версию, отдаёт типизированные пороги (тип согласован с `flags.Params`/`render.Params`).
  - [x] Тесты: дефолты читаются; невалидная версия → ошибка; перекрёстный registry-тест (methodology_params в single-source) проходит (`make check-registry`).
- [x] **Task 2 — Пакет `internal/benchmark` (ЧИСТЫЙ движок медиан групп)** *(AC2)*
  - [x] В пустом `internal/benchmark/`: тип группы сопоставимости + builder `comparability_key` из `(direction, kato, окно 24 мес)` где опорное «сейчас» приходит ПАРАМЕТРОМ (`clock.Clock`/reference time), НЕ `time.Now`.
  - [x] Чистая функция: выборка цен/км группы → `median.Median(samples)` → `(median_price_per_km *int64, registry.ValueState)`; `sample_size = len(comparable)`. Использовать СУЩЕСТВУЮЩУЮ `median.Median` (НЕ переписывать).
  - [x] Состояния: `comparable_n < MinSample` → `StateInsufficientSample`; отсутствие группы/ключа → `StateNotComparable`; `n=0/1/2` → честно, без NaN.
  - [x] Тесты (table + property): OK-путь (≥5), insufficient (≤4), границы n=0/1/2, не-мутация входа, детерминизм (Fixed clock); зеркалить стиль `median_test`. **Negative-control** на каждый страж-предикат ([[guards-must-prove-red]]).
  - [x] ПОДТВЕРДИТЬ чистоту: `make check-core` (`go test -count=1 ./internal/arch/...`) зелёный — `benchmark` не тянет `store`/`httpapi`/`goszakup`/`time`/pgx/chi.
- [x] **Task 3 — `price_benchmarks`: таблица + atomic-swap-пересчёт (store-слой)** *(AC3)*
  - [x] Миграция `migrations/0006_price_benchmarks.sql`: (`id, comparability_key, median_price_per_km, sample_size, computed_at`) + индекс по `comparability_key`.
  - [x] sqlc-запросы `internal/store/queries/price_benchmarks.sql` (upsert/replace-в-сторону, чтение) → `make gen-sqlc`.
  - [x] Store-сервис (store МОЖЕТ касаться БД — ядро НЕТ): читает сопоставимые из `contracts`/`lots`, зовёт ЧИСТЫЙ `benchmark`, пишет результат и публикует **атомарным swap** (напр. вставка нового набора + транзакционная замена/`computed_at`-курсор → читатель видит целостный набор).
  - [x] Интеграционный тест (testcontainers/`-tags=integration`, как существующие `*_integration_test.go`): seed → recalc → swap; читатель никогда не видит полупересчёт. На синтетике: 3 DEMO → insufficient; расширенная фикстура (≥5) → OK.
- [x] **Task 4 — Оркестратор `recalc` (порядок benchmark→flags) + дескоуп хука на 2.4** *(AC4)*
  - [x] Функция-оркестратор (cmd/importer или store-сервис): пересчёт benchmark (swap) → ЗАТЕМ пересчёт флагов; порядок зашит и покрыт юнит-тестом (флаг не считается против устаревшего benchmark).
  - [ ] (опц.) каркас CLI `recalc --snapshot --methodology` (architecture.md:292) — НЕ реализован в 4.1 (честный дескоуп): оркестратор `recalc.Run` готов и протестирован, CLI-обёртка — позже (при живом пересчёте 2.4).
  - [x] ЗАФИКСИРОВАТЬ дескоуп: реальное подключение постимпортного хука + атомарная публикация снапшота — Story 2.4; критерий закрытия AC4 = «оркестратор+тест порядка есть, хук-вызов задокументирован для 2.4».
- [x] **Task 5 — Согласовать `flags.Params`/`render.Params` с движком** *(AC1, AC5)*
  - [x] Наполнить `flags.Params` (сейчас `struct{}`) типизированными порогами из methodology_params (для 4.2–4.5); согласовать `render.Params` (задел) + `render.Evidence.MethodologyVersion`.
  - [x] НЕ реализовывать сами формулы флагов (это 4.2–4.5) — только тип Params + проток `methodology_version`.
- [x] **Task 6 — Гардрейлы и приёмка на данные** *(AC5)*
  - [x] Числовые пороги — ТОЛЬКО из methodology_params (нет литералов-порогов в коде, кроме YAML-дефолтов); проверить, что прозы/чисел в шаблонах нет (конвенция render).
  - [x] Перекрёстный инвариант: `methodology_version` в evidence == в params (тест).
  - [x] Property-тест однозначности: «insufficient ⟺ comparable_n < min_sample» (нельзя медиану при insufficient и наоборот).
  - [x] Приёмка на ДАННЫЕ (Go/SQL-ассерт), НЕ на экран; выход эпика не публикуется (`not_published`-семантика — сервер-gate в Epic 5).
- [x] **Task 7 — Сборка/тесты/линт зелёные** *(все AC)*
  - [x] `make gen-sqlc` (чистый дифф генерата), `make migrate-up` (0005/0006 применяются и откатываются), `make lint` (vet+gofmt), `make test`, `make check-core` (`-count=1`), `make check-registry`. CI: `ci-server.yml` (build/vet/test/arch) + `ci-registry.yml` (single-source incl. methodology_params).

### Review Findings (code-review 2026-06-24)

Адверсариальное ревью: Blind Hunter + Edge Case Hunter + Acceptance Auditor (Opus 4.8). Триаж: **8 patch · 1 defer · 6 dismiss**. Блокеров (High по AC) нет; AC1–AC5 подтверждены аудитором.

**Patch (исправить):**
- [x] [Review][Patch] YAML-загрузчик молча принимает unknown-поля → опечатка в имени ключа даёт вводящую в заблуждение ошибку («рассинхрон min_sample» вместо «неизвестный ключ») — включить `KnownFields(true)` [server/internal/methodology/methodology.go]
- [x] [Review][Patch] `TRUNCATE` обходит row-level триггеры иммутабельности (B-4 «история не переписывается») — добавить `BEFORE TRUNCATE` statement-триггер [migrations/0005_methodology_params.sql]
- [x] [Review][Patch] `sample_size` без `CHECK (>= 0)`; `int→int32` cast без валидации знака/потолка — добавить CHECK + задокументировать ceiling [migrations/0006_price_benchmarks.sql, server/internal/store/projection/price_benchmarks.go]
- [x] [Review][Patch] Честность отчёта: подзадача Task 4 «(опц.) каркас CLI recalc» помечена [x], но НЕ реализована — снять отметку/зафиксировать дескоуп [story Tasks/Subtasks]
- [x] [Review][Patch] `TestMethodologyVersion_FlowsConsistently` вакуумен (одно значение в 3 поля → не способен покраснеть; анти-паттерн [[guards-must-prove-red]]) — удалить (версия уже пинится TestLoad_Defaults) [server/internal/methodology/methodology_test.go]
- [x] [Review][Patch] Интеграционный тест иммутабельности не перезапускаем (UNIQUE-конфликт `vTEST.0` при повторе; глушит ошибки) — пречистка перед вставкой [server/internal/store/projection/price_benchmarks_integration_test.go]
- [x] [Review][Patch] `GroupMedian`: не задокументированы предусловие `windowMonths > 0` (валидируется на загрузке methodology) и календарная семантика окна (`AddDate`) — doc-комментарий [server/internal/benchmark/benchmark.go]
- [x] [Review][Patch] Округление целочисленной медианы для чётной выборки (floor двух центральных) не покрыто тестом — добавить кейс (прозрачность ручного пересчёта) [server/internal/benchmark/benchmark_test.go]

**Defer (отложено, задокументировано в deferred-work.md):**
- [x] [Review][Defer] Нет инварианта «methodology_version в YAML == версия в строках methodology_params»; `InsertMethodologyParam` не валидирует формат версии — дескоуп до наполнения DB-реестра (sync YAML→DB для evidence, Story 4.6/2.4) [server/internal/store/queries/methodology_params.sql] — отложено, наполнение DB вне скоупа 4.1

**Dismiss (6, шум/осознанный дизайн/проверено вне диффа):** nil-интерфейс `recalc.Run` (идиоматичная программная ошибка сборки); `ReplaceSnapshot(nil)` пишет пустую транзакцию (эффективность, временный путь до length_km); context-cancel не различается (атомарность цела); `concentration_share == 1.0` допустимо (политика, не баг); `comparability_key` без окна (осознанный дизайн — окно в `methodology_version`); дифф без sqlc-генерата (проверено вне диффа: повторный `make gen-sqlc` без дрейфа).

## Dev Notes

### Контекст истории (зачем именно сейчас и где границы)

4.1 — фундамент «логики риска» как ЧИСТОГО backend-слоя: версионируемый иммутабельный конфиг + пересчитываемый движок медиан, питающий флаги 4.2–4.5 и FR-18. Центральная архитектурная «точка рычага»: флаг считается над СНАПШОТОМ на момент T; воспроизводимость = «дай то же `(snapshot_id, methodology_version)` → тот же результат бит-в-бит». 4.1 реализует ВЫЧИСЛИТЕЛЬНУЮ половину этого контракта. [Source: architecture.md:104–127]

### ⚠️ Что УЖЕ реализовано/застаблено (НЕ переизобретать)

| Компонент | Состояние | Где | Что делает 4.1 |
|---|---|---|---|
| `median.Median(samples []int64) (*int64, registry.ValueState)` | ✅ РАБОЧАЯ (не stub), `MinSample=5`, overflow-safe, не мутирует вход, без IO/времени | `server/internal/median/median.go` | **использует как есть** в benchmark |
| `internal/benchmark` | 🟡 ПУСТОЙ stub (только `doc.go`) | `server/internal/benchmark/doc.go` | **пишет движок здесь** |
| `flags.Params` (`struct{}`), `flags.Inputs{Now clock.Clock}` | 🟡 каркас, помечен «движок версий — Story 4.1» | `server/internal/flags/flags.go` | **наполняет `Params`** порогами |
| `render.Params` (`struct{}`), `render.Evidence{MethodologyVersion string}` | 🟡 задел | `server/internal/render/render.go` | согласует Params + проток версии |
| `registry.ValueState`/`FlagState` (`StateInsufficientSample`,`StateNotComparable`,`FlagInsufficientData`) | ✅ enum + рантайм-источник | `server/internal/registry/registry.go`, `registry/values/honest_states.json` | **использует**, не плодит свои состояния |
| Страж чистоты ядра (`benchmark` в списке) | ✅ go-list тест | `server/internal/arch/boundaries_test.go` | держит `benchmark` чистым |
| `clock.Clock` (Real/Fixed) | ✅ | `server/internal/clock/clock.go` | reference time для окна 24 мес |
| Таблицы `contracts`/`lots` | ✅ | `migrations/0002`,`0003` | читает для медиан (store-слой) |

> **Вывод для dev-агента:** ядро 4.1 — НЕ зелёное поле. Главная ловушка — переписать `median.Median` или ввести свои состояния вместо `registry.*`. **Наполняй пустой `benchmark`, используй существующую `median.Median`, бери состояния из `registry`.** Единственные новые таблицы — `methodology_params` (0005) и `price_benchmarks` (0006).

### Несущие архитектурные ограничения / гардрейлы

- **Чистота ядра (исполняемый страж).** `internal/{median,flags,normalize,benchmark}` НЕ импортируют `store`/`httpapi`/`goszakup`/`time`/pgx/chi. `benchmark` — чистый: НЕТ `time.Now` (только `clock.Clock` параметром), НЕТ БД (выборки приходят аргументом; персист/swap — в store-слое). Прогон `make check-core` ТОЛЬКО `-count=1` (кэш маскирует нарушение). [Source: architecture.md:775–776; boundaries_test.go]
- **Детерминизм.** Окно 24 мес — reference time ПАРАМЕТРОМ (Fixed clock в тестах). Деньги — целые ₸ (bigint); если `median_price_per_km` дробный — детерминированный round в `benchmark` (Q3). `snapshot_id`/пересчёт детерминированы. [Source: architecture.md:298, 557–558, 581–582, 736]
- **Иммутабельность methodology_params (B-4).** Правка = новая версия; история не переписывается → DDL запрещает UPDATE/DELETE. `methodology_version` (`^v\d+\.\d+$`) пинит формулу+пороги. Перекрёстный инвариант: версия в evidence == в params. [Source: architecture.md:124–125, 218, 328–329, 640–641]
- **Honesty / neutrality (несущие).** `insufficient`/`not_comparable` ТОЛЬКО при `comparable_n < min_sample` (property-тест однозначности). Никакой интерполяции/выдуманных чисел. Числовые пороги — из methodology_params, не литералы в прозе/шаблонах. Флаг — «сигнал, требующий проверки», никогда «нарушение». [Source: prd.md#§7.1, §7.4, FR-23; architecture.md:596–598; epics.md:1491]
- **Backend-only гардрейл.** Выход 4.1 НЕ виден пользователю и НЕ «опубликованный флаг» (публикация — Epic 5 под сервер-gate; `not_published` — отдельное `FlagState`). Приёмка на ДАННЫЕ (Go/SQL), не на экран. [Source: epics.md:1503; architecture.md:140–143, 550–551]
- **Атомарность кэша.** `price_benchmarks` — производное; пересчёт «в сторону → атомарный swap»; читатель видит снапшот целиком. [Source: architecture.md:121, 148–149, 400, 880–881]
- **Порядок пересчёта.** benchmark ПЕРЕД флагами (флаг не против устаревшего benchmark). Живой хук — 2.4; 4.1 даёт оркестратор+тест порядка. [Source: architecture.md:880–881; epics.md:1405–1407]

### Модель данных (откуда и куда)

- **Создаём:** `methodology_params` (`key, value, description, version, effective_from`; иммутабельна в DDL); `price_benchmarks` (`id, comparability_key, median_price_per_km, sample_size, computed_at`). [Source: data-model v1 §1 A/E, §2]
- **Читаем:** `contracts` (`amount_tng`, `direction`(road/water/other), `kato_code`, `sign_date`) и `lots` (`amount`, `kato_code`) для выборок медиан. [Source: migrations/0002, 0003]
- **НЕ существует (честная деградация):** `geo_objects.length_km` (Epic 3/Story 3.1) → цена/км для дорог пока без длины → группа упирается в `insufficient`/`not_comparable` ЧЕСТНО, флаг цены (4.3) не строится «из воздуха». [Source: data-model v1:49, 85; epics.md:1441–1443]
- **НЕ создаём:** `risk_flags` (наполняется флаг-сторями 4.2–4.5), `flag_disputes`/экспорт evidence (Story 4.6, AR-28/AR-29), `geo_objects` (Epic 3).

### Файлы и куда писать

- **NEW:** `registry/values/methodology_params.v1.yaml`; `migrations/0005_methodology_params.sql`; `migrations/0006_price_benchmarks.sql`; `server/internal/store/queries/methodology_params.sql`; `server/internal/store/queries/price_benchmarks.sql`; код `server/internal/benchmark/*.go` (+ тесты); загрузчик methodology (`server/internal/methodology/*.go` или поверх registry-ридера); (опц.) `fixtures/golden/medians/*`.
- **UPDATE:** `server/internal/flags/flags.go` (`Params`), `server/internal/render/render.go` (`Params` согласование); (опц.) `fixtures/seed/contracts.sql` (Q4); `cmd/importer` (каркас recalc-оркестратора).
- **НЕ ТРОГАТЬ:** `server/internal/median/median.go` (использовать), `server/internal/store/gen/*` (генерат — только через `make gen-sqlc`), `registry/values/honest_states.json` (состояния уже есть).
- **Генерат:** после правки `queries/*.sql`/миграций — `make gen-sqlc` (Docker), коммитить `gen/` вместе с источником (страж дрейфа `ci-registry.yml` «generated==regenerated (sqlc)» краснеет иначе).

### Тестирование / проверка готовности

- **Ядро (`benchmark`):** unit/property/table тесты — OK-путь (≥min_sample), insufficient (<min_sample), границы n=0/1/2, не-мутация входа, детерминизм (Fixed clock); negative-control на каждый страж-предикат ([[guards-must-prove-red]]). Внешний тест-пакет `package benchmark_test`.
- **Страж чистоты:** `make check-core` (`-count=1`) зелёный.
- **Store/swap:** интеграционный тест (`-tags=integration`, testcontainers postgis) — seed→recalc→atomic swap; никогда не виден полупересчёт.
- **methodology_params:** дефолты читаются; невалидная версия → ошибка; иммутабельность (UPDATE/DELETE → ошибка БД); registry single-source (`make check-registry`).
- **DoD:** AC1–AC3, AC5 закрыты на синтетике; AC4 закрыт на уровне «оркестратор+тест порядка + дескоуп хука на 2.4»; build/test/lint/arch/registry зелёные; генерат чист; честность (insufficient property-тест) + нейтральность (нет литералов-порогов) + backend-only (приёмка на данные).

### Previous Story Intelligence (1.10/2.0/1.4/1.9 — фундамент; 0.4 — паттерн дескоупа)

- **1.10 (done):** общие контракты ядра, чистые сигнатуры, decode-на-дрейф, **stub-границы** — отсюда `median.Median` (рабочая), пустой `benchmark`, `flags.Params`/`render.Params` каркасы, страж `internal/arch`. **4.1 наполняет эти границы.** [Source: 1-10-…md]
- **2.0 (done):** Source+file+проекция `lots`; ключ идемпотентности `(source, goszakup_lot_id)`; паттерн store/sqlc/миграций. **4.1 зеркалит** store/sqlc-паттерн для новых таблиц. [Source: 2-0-…md; queries/lots.sql]
- **1.4 (done):** двухосевой enum честных состояний (`ValueState`/`FlagState`), registry single-source, нейтральность-страж. **4.1 использует состояния, не плодит свои.** [Source: 1-4-…md; registry.go]
- **1.9 (done):** 3 DEMO-контракта (`DEMO-0001/0002/0003`, `fixtures/seed/contracts.sql`) — все `road`/`kato 710000000`, <5 → **упражняют insufficient-путь.** Расширение до ≥5 — Q4. [Source: 1-9-…md; fixtures/seed/contracts.sql]
- **0.4 (done):** паттерн ЧЕСТНОГО дескоупа зависимости (документировать инвариант + критерий закрытия, не выдумывать). **4.1 применяет к AC4** (хук-вызов → 2.4). [Source: 0-4-…md; memory data-source-interim-parser-first]
- **Урок ревью ([[guards-must-prove-red]]):** каждый страж-тест обязан иметь negative-control + прогон `-count=1`; пины golden литералом. Применить к benchmark-тестам и тесту порядка пересчёта.

### Git Intelligence (последние коммиты)

`b41db86` untrack stage0-audit binary · `424348a` Story 0.7 geocoding · `81e406a` Story 0.6 scrape lots · `4301476` Story 0.2 review fixes · `7cdde03` Story 2.0 token-независимый фундамент (Source+file+проекция lots). Паттерн: токен-независимые истории строятся на синтетике/фикстурах с детерминированными тестами и стражами-доказывающими-red. 4.1 продолжает этот трек (compute-ядро на синтетике). [Source: git log]

### Project Structure Notes

- Конфликтов нет: ядро в `server/internal/benchmark` (существующий пустой пакет), миграции в корне `migrations/` (goose, продолжение нумерации 0005/0006), sqlc per-domain → per-file (анти-churn), registry-источник в `registry/values/`. Go 1.25.0, module `ashyqqala/server`.
- Внешних библиотек НЕ добавлять: ядро — stdlib (`slices`/`math`) + `registry`/`clock`; store — существующие pgx/v5+sqlc. `methodology_version` server-стороны раньше НЕ было (у `stage0-audit` — свой, другой модуль; НЕ переиспользовать) → 4.1 заводит канонический server-side.

### Внешние знания / web-research (осознанно ограничено)

Новых версий/библиотек нет: движок — Go stdlib + внутренние `registry`/`clock`/`median`. sqlc 1.31.0 (запинен), goose-миграции — устоявшиеся паттерны проекта. Формула отклонения ×1.5 (НЕ MAD) — намеренно простая для публичной пересчитываемости (PRD#FR-23). Research не требуется.

## Открытые вопросы / решения владельцу (зафиксировать до/во время dev)

1. **AC4 дескоуп на 2.4.** Реальный постимпортный хук + атомарная публикация снапшота требуют Story 2.4 (не построена). Рекоменд.: 4.1 даёт вызываемый оркестратор (benchmark→swap→flags) + юнит-тест порядка; хук-вызов — 2.4. **Рекоменд.: дескоуп** (как 0.4).
2. **Место `methodology_params`.** Рекоменд.: YAML `registry/values/methodology_params.v1.yaml` = источник истины (рантайм-чтение, single-source) + DB-таблица как иммутабельный version-реестр для evidence/пересчёта. Альтернатива — только YAML / только DB. **Рекоменд.: YAML+DB.**
3. **Тип `median_price_per_km`.** Целые ₸/км (детерминизм бесплатен, как `amount_tng`) vs дробные (нужен детерминированный round). **Рекоменд.: целые ₸/км.**
4. **Синтетика для OK-пути.** 3 DEMO < min_sample (только insufficient). Рекоменд.: golden-фикстуры medians (≥5, с границами) под `fixtures/golden/medians/`, DEMO оставить как insufficient-кейс. **Рекоменд.: golden-фикстуры, не ломать DEMO.**
5. **DDL-иммутабельность.** Триггер, бросающий исключение на UPDATE/DELETE строк methodology_params (append-only) vs REVOKE прав. **Рекоменд.: триггер (переносимо, виден в миграции).**

## Dev Agent Record

### Agent Model Used

claude-opus-4-8[1m] (Claude Opus 4.8, 1M context) — create-story (context engine) + dev-story (реализация).

### Debug Log References

- Окружение: sqlc 1.31.0 и postgis/postgis:16-3.4 закэшированы локально → `make gen-sqlc` и интеграция выполнимы офлайн (Docker Hub pull заблокирован, но `docker run` кэшированных образов работает). `gopkg.in/yaml.v3 v3.0.1` уже в graph модулей (транзитивно через kin-openapi) → промоутнут в прямую зависимость без роста supply-chain (`go mod tidy` офлайн, `GOPROXY=off`).
- Интеграция прогнана на ИЗОЛИРОВАННОМ Postgres (`docker run ... -p 127.0.0.1:55432:5432 postgis:16-3.4`, не трогая чужой :5432 и compose-том); миграции 0001–0006 применены через psql (Up-секции извлечены awk, минуя сетевой fetch goose); проверены up И down (откат 0006/0005 чистый — таблицы+функция удалены без остатка).
- `make test`/`lint`/`build`/`check-core`/`check-registry` — зелёные; повторный `make gen-sqlc` не меняет сгенерированные методику/бенчмарки (нет дрейфа генерата).

### Completion Notes List

**Решения владельца (Task 0):** Q1=дескоуп AC4 на 2.4; Q2=YAML+DB; Q3=целые ₸/км; Q4=golden-фикстуры (DEMO остаётся insufficient); Q5=триггер иммутабельности (применено).

**AC1 ✅** `registry/values/methodology_params.v1.yaml` — источник истины (рантайм-чтение через `internal/methodology.Load`, формат версии `^v\d+\.\d+$`, дефолты data-model §2). Миграция 0005: append-only `methodology_params` + **триггеры на UPDATE/DELETE** (B-4, plpgsql RAISE; иммутабельность проверена интеграционно). sqlc-ридер по версии. `methodology_version` протекает в `flags.Params`/`render.Params`/`render.Evidence` (перекрёстный инвариант — тест). Single-source подключён в `make check-registry` + ci-registry.

**AC2 ✅** Пакет `internal/benchmark` (ЧИСТЫЙ): `Group.Key()` (детерминированный ключ direction×kato, пин golden-литералом), `GroupMedian(samples, windowMonths, clock)` → `(median, state, sample_size)` поверх существующей `median.Median` (НЕ переписана; MinSample из ядра). Окно 24 мес считается арифметикой на `time.Time` из `clock` БЕЗ импорта `time` (страж чистоты `make check-core` зелёный). Граничные n=0/1/2 → честный `insufficient_sample`; property-тест однозначности + golden-фикстура (пересчёт вручную: median 1800000).

**AC3 ✅** Миграция 0006 `price_benchmarks` (median NULL = honest insufficient/not_comparable; целые ₸/км; methodology_version снапшота). `projection.BenchmarkStore.ReplaceSnapshot` — **атомарный swap** (DELETE+INSERT в ОДНОЙ транзакции; читатель видит снапшот целиком — проверено интеграционно: старый ключ исчезает, новый появляется, sample_size фиксируется, детерминизм). `Lookup` — честные состояния (нет строки→not_comparable, median NULL→insufficient_sample, иначе ok).

**AC4 ✅ (дескоуп Q1)** `internal/recalc.Run` — вызываемый оркестратор с ЗАШИТЫМ порядком benchmark→flags + юнит-тест порядка (+ negative-control: падение benchmark НЕ пускает флаги). `BenchmarkStore.RecalcBenchmarks` реализует интерфейс; на этапе 4.1 БЕЗ `geo_objects.length_km` (Epic 3) публикует ПУСТОЙ снапшот честно (any key → not_comparable), без выдуманных медиан. Живой постимпортный хук + публикация снапшота — Story 2.4 (как паттерн дескоупа 0.4).

**AC5 ✅** Пороги — только из methodology_params (литералов в коде нет: benchmark использует параметры/`median.MinSample`, flags — struct без чисел; единственный код-дефолт `median.MinSample` сверяется с YAML загрузчиком). Перекрёстный инвариант версии (тест). Приёмка на ДАННЫЕ (Go/SQL-ассерты, не экран); выход не публикуется. Честность: `insufficient ⟺ comparable_n < min_sample` (property-тесты benchmark+median); нейтральность render не нарушена (render-тесты зелёные).

**Дескоуп/задел (честно):** живой хук импорта и atomic-публикация снапшота — 2.4; наполнение DB-таблицы methodology_params строками версии (sync YAML→DB для evidence) и сбор price/km-выборок из contracts+length_km — позже (Epic 3 / 4.6); формулы 4 флагов — 4.2–4.5.

### File List

**NEW (код/конфиг):**
- `registry/values/methodology_params.v1.yaml`
- `migrations/0005_methodology_params.sql`, `migrations/0006_price_benchmarks.sql`
- `server/internal/benchmark/benchmark.go`, `benchmark_test.go`, `golden_test.go`
- `server/internal/methodology/methodology.go`, `methodology_test.go`
- `server/internal/recalc/recalc.go`, `recalc_test.go`
- `server/internal/store/queries/methodology_params.sql`, `price_benchmarks.sql`
- `server/internal/store/projection/price_benchmarks.go`, `price_benchmarks_integration_test.go`
- `fixtures/golden/medians/road_astana_24mo.json`

**NEW (генерат sqlc, через `make gen-sqlc`):**
- `server/internal/store/gen/methodology_params.sql.go`, `price_benchmarks.sql.go`

**MODIFIED:**
- `server/internal/flags/flags.go` (Params: типизированные пороги + MethodologyVersion)
- `server/internal/render/render.go` (Params: MethodologyVersion — согласование)
- `server/internal/store/gen/querier.go`, `models.go` (регенерат: новые методы/модели)
- `server/go.mod`, `server/go.sum` (+ `gopkg.in/yaml.v3` прямая)
- `Makefile`, `.github/workflows/ci-registry.yml` (check-registry включает methodology single-source)

## References

- [Source: epics.md:1380–1407 — Epic 4 цель + Story 4.1 (3 AC, гардрейл backend-only); 1409–1503 — флаги 4.2–4.5 (потребители движка) + 4.6 (пересчитываемость/disputes/экспорт — НЕ 4.1)]
- [Source: architecture.md:104–127 — снапшот/methodology_version/чистое ядро/иммутабельность params; 119–121 — `flag/median/normalize` чистые, price_benchmarks=swap; 124–125, 218 (B-4) — иммутабельность methodology_params в DDL; 139, 547–551 — состояния value/flag; 596–598 — insufficient property; 640–641, 787–789, 321–323 — registry-источник methodology_params.vN.yaml (рантайм); 775–776 — страж чистоты (нет store/httpapi/goszakup/time); 880–881 — порядок benchmark→flags, atomic swap; 292–293 — recalc CLI; 298, 736 — clock; 328–329 — перекрёстный инвариант версии]
- [Source: prd.md#FR-18 (медиана ₸/км, район vs город); FR-19…FR-22 (4 флага); FR-23 (пересчитываемость, evidence+methodology_version, ×1.5 не MAD); FR-1 (постимпорт пересчёт benchmark перед флагами); §5.8 (methodology_params); NFR-5/NFR-8; SM-C1; §7.1/§7.4 (нейтральность/честность); OQ-6 (дефолты калибруются)]
- [Source: docs/AshyqQala_MVP_data_model_and_flags_v1.md §1 (таблицы methodology_params/price_benchmarks/risk_flags), §2 (дефолты: min_sample=5, deviation_factor=1.5, concentration_share=0.5, min_group_contracts=5, comparability_key)]
- [Source: server/internal/median/median.go (`Median`, `MinSample`); internal/benchmark/doc.go (пустой); internal/flags/flags.go (`Params`/`Inputs`); internal/render/render.go (`Params`/`Evidence`); internal/registry/registry.go (`ValueState`/`FlagState`); internal/arch/boundaries_test.go (forbidden imports); internal/clock/clock.go; migrations/0002–0003; server/sqlc.yaml; root Makefile (gen-sqlc/check-core/check-registry); fixtures/seed/contracts.sql]
- [Source: 1-10-…md (stub-границы ядра); 2-0-…md (store/sqlc/идемпотентность); 1-4-…md (registry/состояния); 1-9-…md (DEMO seed); 0-4-…md (паттерн дескоупа); memory: guards-must-prove-red, sprint-sequencing-epic1-first]

## Change Log

| Дата | Изменение |
|---|---|
| 2026-06-24 | code-review (адверсариальный, 3 слоя) → 8 patch применены: YAML `KnownFields`; `TRUNCATE`-страж иммутабельности (B-4); `CHECK (sample_size >= 0)`; честность чекбокса Task 4 (опц. CLI снят); удалён вакуумный version-тест; перезапускаемый immutable-тест; doc `GroupMedian` (предусловие окна + календарная семантика); тест floor-округления чётной медианы. 1 defer (инвариант версии YAML↔DB → 4.6/2.4), 6 dismiss. Гейты + интеграция перепроверены зелёными (TRUNCATE/CHECK заблокированы; immutable-тест ×2 перезапуск; миграции up/down; gen стабилен). Статус review → done. |
| 2026-06-24 | dev-story: реализованы AC1–AC5. NEW: benchmark (чистый движок медиан групп + golden), methodology (YAML-загрузчик порогов + single-source), recalc (оркестратор порядка benchmark→flags), миграции 0005 (methodology_params append-only + триггер иммутабельности) / 0006 (price_benchmarks), sqlc-запросы + генерат, store BenchmarkStore (атомарный swap + честные состояния), интеграционный тест. MOD: flags/render.Params (типизированные пороги + methodology_version), Makefile/ci-registry (single-source methodology), go.mod (+yaml.v3). Решения владельца Q1–Q5 применены (дескоуп хука AC4 → 2.4). Гейты зелёные (test/lint/build/check-core/check-registry/gen-стабилен); интеграция (atomic swap + иммутабельность) и миграции up/down проверены на изолированном Postgres. Статус → review. |
| 2026-06-24 | Создан context engine (create-story): Story 4.1 — methodology_params (иммутабельный) + median/benchmark-движок. Анализ epics.md (Эпик 4), architecture.md (чистое ядро/снапшот/иммутабельность/atomic swap/страж чистоты), prd.md (FR-18/19–23, NFR-5, SM-C1, §7.1/§7.4), data-model v1 (таблицы+дефолты) + разведка кода (median.Median готова; benchmark пуст; flags/render.Params каркасы; registry-состояния; arch-страж; миграции 0002–0003; geo_objects/length_km отсутствуют). 5 AC: methodology_params иммутабельный + дефолты (AC1); чистый median/benchmark→(value,state), insufficient<min_sample (AC2); price_benchmarks atomic swap (AC3); порядок benchmark→flags с ДЕСКОУПОМ хука на Story 2.4 (AC4); пересчитываемость/гардрейлы/приёмка-на-данные (AC5). 5 открытых вопросов. Статус → ready-for-dev. |
