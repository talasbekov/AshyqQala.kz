---
baseline_commit: 87cb59538bf538d02b2ac09f29c825c11aa99af3
---
# Story 2.7: Transactional outbox (LoggingDispatcher)

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a **команда**,
I want **надёжную доставку событий о новых контрактах/флагах**,
so that **уведомления (Telegram, Epic 7) однократны и не теряются**.

## Acceptance Criteria

> Дословно из эпика [Source: _bmad-output/planning-artifacts/epics.md#Story 2.7]. Уточнения помечены *(уточнение 2.7)*.

**AC1 — транзакционная запись + конверт (O-1)**
- **Given** таблица `notifications_outbox` и `Dispatcher`-интерфейс **When** записывается бизнес-объект и outbox-строка **Then** обе пишутся в ОДНОЙ транзакции (**O-1**: откат ⇒ нет ни той, ни той); конверт `{event_id, type, occurred_at, subject_ref, payload, v}`, `subject_ref`=URN `<entity>:<public_id>`.
- *(уточнение 2.7)* `Enqueue` принимает транзакцию вызывающего (`gen.DBTX`/`pgx.Tx`), чтобы запись события шла В ТОЙ ЖЕ tx, что и бизнес-объект (как `orgnorm.Apply` / `ReplaceSnapshot`). `subject_ref` — ПУБЛИЧНЫЙ id (natural `goszakup_*_id`, стабилен между ре-импортами), НЕ внутренний bigint (прецедент evidence_export 5.6). Имя `type` = `<aggregate>.<event>` (lower-dot), напр. `flag.raised`.

**AC2 — воркер доставки (O-2) + дедуп (O-3) + детерминизм (O-4)**
- **Given** воркер outbox **When** он поллит неотправленные **Then** `FOR UPDATE SKIP LOCKED` → `Dispatcher.Send` → `sent_at` (**O-2**); ошибка растит `attempts`; дедуп по `event_id` (**O-3**); retry/visibility-timeout детерминированы через инъекцию `Clock` (**O-4**).
- *(уточнение 2.7)* O-3 дедуп = `event_id` UNIQUE + `ON CONFLICT (event_id) DO NOTHING` на Enqueue (повторная вставка того же события — no-op). O-4: воркер берёт «сейчас» ТОЛЬКО из `clock.Clock` (поллит `WHERE sent_at IS NULL AND available_at <= $now`; на ошибке `attempts++` и `available_at = $now + backoff(attempts)`), backoff — простая воспроизводимая формула. SKIP LOCKED → два воркера не双-шлют одну строку.

**AC3 — Dispatcher-контракт + LoggingDispatcher (S-0)**
- **Given** S-0 **When** Dispatcher реализован **Then** это `LoggingDispatcher` (Telegram-реализация — Epic 7); сигнатура `Dispatcher` зафиксирована как контракт.
- *(уточнение 2.7)* `LoggingDispatcher.Send` логирует событие (slog) и возвращает nil (успех); реальная Telegram-доставка + fan-out по подпискам (`subscription_id`) — Epic 7. `var _ Dispatcher = LoggingDispatcher{}` (compile-time контракт).

**Гардрейл честности/нейтральности:** payload outbox-события НЕ генерирует прозу флага (нейтральность — через `render` на поверхности bot, Epic 7); 2-7 несёт нейтральный конверт (id/type/ref), не текст. Стражи обязаны краснеть (negative-control, `-count=1`). [Source: CLAUDE.md#guardrails; [[guards-must-prove-red]]]

## Tasks / Subtasks

- [x] **T1. Таблица + конверт + транзакционный Enqueue (O-1)** (AC1)
  - [x] Миграция `0016_notifications_outbox.sql` (goose): `id` identity PK; `event_id` UUID **UNIQUE** (дедуп O-3); `type` TEXT; `occurred_at` TIMESTAMPTZ; `subject_ref` TEXT (URN); `payload` JSONB; `v` INT (версия payload); `sent_at` TIMESTAMPTZ NULL; `attempts` INT NOT NULL DEFAULT 0 (CHECK ≥0); `available_at` TIMESTAMPTZ NOT NULL DEFAULT now() (видимость O-4); `created_at`. Индекс для поллинга: `(available_at) WHERE sent_at IS NULL`.
  - [x] sqlc-запросы `internal/store/queries/notifications_outbox.sql` (`make gen-sqlc` через Docker; `store/gen` не править): `EnqueueEvent` (INSERT … `ON CONFLICT (event_id) DO NOTHING`), `PollUnsent` (FOR UPDATE SKIP LOCKED), `MarkSent`, `BumpAttempt` (+ `CountOutbox`/`GetOutboxByEventID` для тестов).
  - [x] `internal/outbox`: `Event` (конверт), `Enqueue(ctx, q gen.DBTX, e Event) error` — пишет в tx ВЫЗЫВАЮЩЕГО (атомарность с бизнес-объектом).
  - [x] Integration-тест O-1: бизнес-вставка+Enqueue в одной tx → rollback ⇒ НЕТ ни бизнес-строки, ни outbox-строки; commit ⇒ обе есть.
- [x] **T2. Dispatcher-контракт + LoggingDispatcher** (AC3)
  - [x] `Dispatcher` интерфейс `Send(ctx, Event) error`; `LoggingDispatcher{Log *slog.Logger}` (логирует, возвращает nil); `var _ Dispatcher = LoggingDispatcher{}`.
  - [x] Unit-тест: LoggingDispatcher.Send не ошибается, логирует id/type/subject_ref (нейтрально, без прозы флага — табу-слова проверены).
- [x] **T3. Воркер доставки (O-2) + SKIP LOCKED** (AC2)
  - [x] `ProcessBatch(ctx, pool, d Dispatcher, clk clock.Clock, limit int) (sent int, err error)`: tx → `PollUnsent($now, limit)` (FOR UPDATE SKIP LOCKED) → для каждой `Dispatcher.Send`; успех → `MarkSent($now)`; ошибка Send → `BumpAttempt(attempts+1, available_at=$now+backoff)`; commit.
  - [x] Integration-тест O-2: enqueued событие → ProcessBatch → `sent_at` проставлен, Dispatcher вызван 1×; повторный проход не передоставляет. SKIP LOCKED: пока tx1 держит FOR UPDATE-лок, tx2 пропускает строку (0 строк) — двойной доставки нет.
- [x] **T4. Дедуп (O-3) + детерминизм retry/visibility (O-4)** (AC2)
  - [x] O-3: повторный Enqueue того же `event_id` — no-op (ON CONFLICT DO NOTHING); integration-тест: 2× Enqueue → 1 строка (negative-control: другой event_id → 2 строки).
  - [x] O-4: backoff — чистая функция `backoff(attempts) time.Duration` (линейная `base*attempts` с потолком); воркер берёт «сейчас» из `clk` (НЕ `now()` в SQL для границы видимости). Unit-тест backoff детерминизм/монотонность/потолок; integration: упавший Send → attempts=1 + available_at сдвинут (скрыт до $now+backoff с `clock.Fixed`; со сдвинутым clk — доставлен).
- [x] **T5. Гейты, CI, честность**
  - [x] Расширить CI-job `integration` (ci-server.yml, Story 2.6): добавить `./internal/outbox/...` в прогон (красный=блок merge).
  - [x] `build`/`lint`/`check-core`(-count=1)/`check-registry` зелёные; negative-control на каждый страж (`-count=1`); честный конвейер без прозы флага. Red-proof: дубль event_id → UNIQUE-violation; миграция реверсивна (down/up).

### Review Findings

> Адверсариальное код-ревью 2026-06-29 (3 слоя: Blind/Edge/Acceptance). Все 3 AC + O-1..O-4 + оба гардрейла + скоуп-дисциплина — PASS. Триаж: 0 decision / 7 patch / 4 defer / 5 dismiss.

**Patch (ПРИМЕНЕНЫ 2026-06-29 — валидация конверта + честность воркера, всё в новом коде):**
- [x] [Review][Patch] Валидировать нулевой `OccurredAt` в Enqueue (иначе `0001-01-01` в NOT NULL) [server/internal/outbox/enqueue.go] — blind+edge+auditor
- [x] [Review][Patch] Валидировать пустой `SubjectRef` в Enqueue (load-bearing поле AC1; валидация асимметрична) [server/internal/outbox/enqueue.go] — blind+edge+auditor
- [x] [Review][Patch] Payload-гард по `len(payload)==0`→`{}` + отвергать невалидный JSON (пустой/битый slice сейчас минует `==nil` → ошибка JSONB) [server/internal/outbox/enqueue.go] — edge
- [x] [Review][Patch] Валидировать `limit <= 0` в ProcessBatch (LIMIT 0 = вечный idle / LIMIT -1 = DB-ошибка) [server/internal/outbox/worker.go] — blind+edge
- [x] [Review][Patch] Возвращать `0` (не `sent`) на пред-commit error-путях ProcessBatch (rollback отменил всё → честная метрика) [server/internal/outbox/worker.go] — blind+edge
- [x] [Review][Patch] Клампить верхний край `backoff` до умножения (overflow int64 при огромном attempts → отрицательная задержка) [server/internal/outbox/worker.go] — blind+edge
- [x] [Review][Patch] Усилить страж нейтральности: событие с проза-payload → проверить, что её НЕТ в логе (доказывает «payload не логируется») [server/internal/outbox/outbox_test.go] — auditor

**Defer (Epic 7 — реальная эмиссия/Telegram/сетевая доставка; в S-0 эмиттеров нет):**
- [x] [Review][Defer] Нет dead-letter/max-attempts: poison-строка ретраится вечно + overflow `attempts` [server/internal/outbox/worker.go] — defer → Epic 7
- [x] [Review][Defer] `Dispatcher.Send` под FOR UPDATE-локом всего батча (lock-across-IO для сетевой доставки) [server/internal/outbox/worker.go] — defer → Epic 7 (текущий батч-в-tx соответствует AC2 для S-0)
- [x] [Review][Defer] Откат всего батча при поздней DB-ошибке передоставляет уже-Send-нутые строки (at-least-once amplification) [server/internal/outbox/worker.go] — defer → Epic 7 (ограничить/дедуп у получателя)
- [x] [Review][Defer] `NewEventID()` без/с пустыми частями коллапсирует в один не-нулевой id (тихое склеивание событий) [server/internal/outbox/event.go] — defer → Epic 7 (эмиттер обязан давать стабильный непустой дискриминатор)

**Dismiss (5):** `clk.Now()`-zero stall (прод = `clock.Real`, не возвращает zero); negative/huge `V` (emitter-controlled, нет эмиттеров, `v` — внутренняя версия); «детерминизм overstated» (O-4 — про poll/retry-границу, она clock-injected; enqueue-`now()` намеренно делает событие сразу видимым); EnqueueEvent без insert-vs-dedup сигнала (идемпотентность = контракт O-3); ctx-cancel инфлирует attempts (само-корректируется: BumpAttempt на том же отменённом ctx тоже падает → rollback).

### Review Findings — повторное ревью (2026-06-29)

> Независимый повторный прогон (3 слоя: Blind/Edge/Acceptance, Opus). Подтвердил тщательность ревью #1: тяжёлые пункты (lock-across-IO, dead-letter, at-least-once, NewEventID) уже отловлены/отложены ранее. HIGH нет; все 3 AC + O-1..O-4 — PASS. Триаж: 0 decision / 3 patch / 5 defer (3 повтор + 2 новых) / 3 dismiss.

**Patch (ПРИМЕНЕНЫ 2026-06-29 — дополняют патчи ревью #1; gofmt/vet/build/`go test -count=1 ./...` зелёные):**
- [x] [Review][Patch] Дополнить гард `limit` верхней границей (`limit > math.MaxInt32` → int32-каст оборачивается в отрицательный/мусорный LIMIT); ревью #1 закрыло только `limit <= 0` [server/internal/outbox/worker.go] — blind+edge
- [x] [Review][Patch] Требовать JSON-ОБЪЕКТ в payload: валидный не-объект (`123`/`[1,2]`/`"x"`) минует `json.Valid` и тихо пишется в JSONB, хотя конверт по контракту — объект `{...}` [server/internal/outbox/enqueue.go] — edge (+ negative-control в TestEnqueue_ValidatesEnvelope)
- [x] [Review][Patch] Страж нейтральности: табу теперь из канона `registry.FindTaboo` (casefold + homoglyph-fold, ru+kk) вместо хардкод-подмножества — `assertNoTaboo` + proves-red `TestNeutralityGuard_CanonicalMatcher_ProvesRed` [server/internal/outbox/outbox_test.go] — auditor

**Defer — НОВЫЕ (Epic 7; добавлены в deferred-work.md):**
- [x] [Review][Defer] Батч-старт `now` переиспользован для планирования ретрая (`available_at=now+backoff`) и `sent_at` → при долгих/медленных батчах (Epic 7) эрозия backoff + ранний `sent_at` [server/internal/outbox/worker.go] — blind (бандл с lock-across-IO)
- [x] [Review][Defer] `SubjectURN` с пустыми частями (`SubjectURN("contract","")→"contract:"`) проходит проверку непустоты → URN без public_id [server/internal/outbox/enqueue.go, event.go] — edge (бандл с NewEventID-гардом)

**Defer — повторно подтверждены (уже в deferred-work.md:269-272, ревью #1):** dead-letter/max-attempts; `Dispatcher.Send` под локом батча (lock-across-IO); at-least-once amplification; `NewEventID()` пустые части.

**Dismiss (3):** `available_at DEFAULT now()` на вставке против `clk.Now()` (намеренно/задокументировано — enqueue-`now()` делает событие сразу видимым; O-4 — про poll/retry-границу; ops-нюанс: skew app↔БД → мониторить в Epic 7); `ORDER BY (available_at,id)` шире индекса `(available_at)` (perf-only, пренебрежимо на масштабе MVP); во frontmatter спеки нет `context:` (References присутствуют inline, на код не влияет).

## Dev Notes

### §0. Что есть / чего нет (это GREENFIELD-инфра, чистая и токен-независимая)

- **Только стаб** `server/internal/outbox/doc.go` (`package outbox` + комментарий) — кода нет, строим с нуля. [Source: server/internal/outbox/doc.go]
- **Реестра `event_types` НЕТ** (`registry/values/` = glossary/honest_states/methodology/normalize_lexicon/taboo; check-registry его не покрывает). → 2-7 определяет имена событий КОНСТАНТАМИ в `internal/outbox` (`<aggregate>.<event>`); registry-каталог `event_types` + cross-test (architecture: «event_types == эмиттеры») — forward на Epic 7, когда появятся РЕАЛЬНЫЕ эмиттеры/Telegram (в S-0 эмиттеров нет — LoggingDispatcher только логирует). [Source: architecture.md:645,792; Вопрос №1]
- **Прецеденты переиспользования:** транзакционная атомарность (одна `pgx.Tx`) — `orgnorm.Apply` (2-4) / `BenchmarkStore.ReplaceSnapshot`; Clock-инъекция — `internal/clock` (Real/Fixed); sqlc/goose/migration — как 0015; `subject_ref`=публичный id — `httpapi/evidence_export.go:50`.

### §1. Архитектура outbox (load-bearing — O-1..O-4)

[Source: architecture.md#API & Communication Patterns (строки 432-438)]
- **O-1:** запись бизнес-объекта + outbox-строки в ОДНОЙ транзакции (откат ⇒ нет ни той, ни той).
- **O-2:** воркер поллит неотправленные (`FOR UPDATE SKIP LOCKED`), вызывает `Dispatcher.Send`, ставит `sent_at`; на ошибке растёт `attempts`.
- **O-3:** дедуп по `event_id` (повтор не двоит у получателя).
- **O-4:** retry/visibility-timeout детерминированы через инъекцию `Clock`.
- **Получатель — ИНТЕРФЕЙС:** в S-0 `LoggingDispatcher`; Telegram-реализация — Epic 7.

[Source: architecture.md#Communication Patterns (строки 577-579)]
- **Конверт:** имя `<aggregate>.<event>` (lower-dot); `{event_id (uuid), type, occurred_at, subject_ref, payload, v}`; `subject_ref` = URN `<entity>:<public_id>` (singular, напр. `contract:44071234`); дедуп по `event_id`; версия payload — `v`.
- **Natural id в subject_ref** (стабилен между ре-импортами) [Source: architecture.md:526].
- **Владелец:** importer ПИШЕТ outbox, bot ЧИТАЕТ [Source: architecture.md:733,799]. (Реальная эмиссия `flag.raised` в tx пересчёта + bot-доставка — Epic 7; 2-7 = механизм + LoggingDispatcher.)

### §2. Модель данных — ВАЖНОЕ расхождение

`docs/AshyqQala_MVP_data_model_and_flags_v1.md:64` описывает `notifications_outbox` как `{id, subscription_id(FK), event_type, payload, sent_at, status}` — это ПОЗДНЯЯ (Epic 7) форма с fan-out по подпискам. **2-7 следует АРХИТЕКТУРНОМУ конверту** (`event_id/occurred_at/subject_ref/v/attempts/available_at`), БЕЗ `subscription_id` (таблицы подписок ещё нет — Epic 7). subscription-fan-out добавится в Epic 7. [Source: data-model:64 vs architecture.md:577-579; Вопрос №2]

### §3. Стек/конвенции

Go 1.25 · pgx v5 · sqlc 1.31 (Docker; `store/gen` не править) · PostgreSQL 16 · goose v3 — следующая миграция **`0016`** (`-- +goose Up/Down`). UUID: `gen_random_uuid()` (pgcrypto/pg16 встроено) для `event_id` дефолта ИЛИ генерация в Go (`event_id` детерминируется эмиттером для дедупа — предпочесть Go-генерацию/детерминированный id у эмиттера, не БД-random, чтобы повтор был дедуплицируем). [Source: Makefile; migrations/0001..0015]

### §4. Тестовая дисциплина

- **Integration — конвенция репо:** `//go:build integration` + `DATABASE_URL` + `t.Skip`; миграции внешне; `-p 1`; локально `POSTGRES_PORT=55432` [[local-dev-docker-env]]. Образцы: `price_benchmarks_integration_test.go`, `orgnorm/s0_acceptance_integration_test.go`, `pipeline/lock_integration_test.go` (SKIP LOCKED-конкуренция через две acquired-conn). [Source: server/internal/ingest/pipeline/lock_integration_test.go].
- **Стражи краснеют:** дедуп (2× Enqueue→1 строка — упадёт без UNIQUE), SKIP LOCKED (без него — двойная доставка), O-4 (без Clock-сдвига строка видна сразу) — каждый с negative-control, прогон `-count=1`. [[guards-must-prove-red]]
- **CI:** добавить `./internal/outbox/...` в job `integration` (Story 2.6) — иначе outbox-integration не гейтится (path job сейчас = `./internal/ingest/... ./internal/store/projection/...`). [Source: .github/workflows/ci-server.yml]

### Project Structure Notes

- `internal/outbox/` — `Event`, `Dispatcher`, `LoggingDispatcher`, `Enqueue`, `ProcessBatch`, `backoff`. Пакет НЕ чистое ядро (импортирует pgx/clock/slog — допустимо, не в `pureCorePackages`). НЕ импортирует `goszakup` (граница AR-27). Текст событий — нейтральный конверт; проза — `render` на поверхности (Epic 7), НЕ здесь.
- `internal/store/queries/notifications_outbox.sql` → sqlc gen.
- `internal/clock` — `ProcessBatch` берёт `clock.Clock` (О-4 детерминизм); чистый `backoff()` — без `time.Now`.
- Миграция `0016`; индекс поллинга частичный (`WHERE sent_at IS NULL`).

### Previous Story Intelligence

- **2-4/2-6:** транзакционная атомарность одной `pgx.Tx` (Apply/ReplaceSnapshot) — образец Enqueue; `WithSingleJobLock`/SKIP LOCKED-конкуренция — образец воркер-теста; Clock-инъекция (`FreezeClock`, `clock.Fixed`) — O-4. Уроки ревью: стражи краснеют (negative-control доказывает ПРИЧИНУ); честные состояния; не переоценивать «закрыто»; гранты — `app_importer` пишет проекции (если позже навесить роли — outbox-таблица под importer-write, bot — read; forward).
- **5.6 evidence_export:** `subject_ref` = публичный goszakup-id, не bigint — переиспользовать принцип.

### Git Intelligence

Миграции до `0015`; sqlc-gen в `store/gen`; integration build-tag + DATABASE_URL; CI job `integration` добавлен в 2.6. Коммитить 2.7 одним логическим коммитом.

### References

- [Source: epics.md#Story 2.7; architecture.md#Communication Patterns (432-438, 577-579), #Project Structure (733), #Boundaries (799)]
- [Source: docs/AshyqQala_MVP_data_model_and_flags_v1.md:64 (поздняя форма — Epic 7)]
- [Source: server/internal/outbox/doc.go; server/internal/clock/clock.go; server/internal/ingest/orgnorm/apply.go; server/internal/ingest/pipeline/lock_integration_test.go; server/internal/httpapi/evidence_export.go; server/internal/store/queries/*.sql; migrations/0015_roles_grants.sql; .github/workflows/ci-server.yml; Makefile]
- [Source: deferred-work.md; [[guards-must-prove-red]], [[local-dev-docker-env]]]

### Вопросы владельцу (сохранены, не блокируют dev)

1. **Реестр `event_types` + check-registry cross-test** — строить сейчас (минимальный каталог + перекрёстный тест «типы == эмиттеры») или forward на Epic 7? Рекомендую: КОНСТАНТЫ в `internal/outbox` сейчас; registry-каталог — когда появятся реальные эмиттеры/Telegram (S-0 эмиттеров нет). Подтвердить.
2. **Форма `notifications_outbox`** — архитектурный конверт (event_id/occurred_at/subject_ref/v/attempts/available_at, БЕЗ subscription_id) vs data-model (subscription_id fan-out). Рекомендую архитектурный (subscription_id — Epic 7). Подтвердить.
3. **Backoff O-4** — простая воспроизводимая формула (линейный `base*attempts` с потолком vs экспонента). Рекомендую линейный с потолком (этос «простые воспроизводимые формулы»). Подтвердить.
4. **Эмиссия реальных событий** — 2-7 = МЕХАНИЗМ + LoggingDispatcher + тест-эмиттер; реальная эмиссия `flag.raised`/`contract.created` в tx пересчёта/импорта + Telegram-доставка + fan-out по подпискам = Epic 7. Подтвердить, что 2-7 не тащит живую эмиссию/Telegram.

## Dev Agent Record

### Agent Model Used

claude-opus-4-8[1m] (Claude Opus 4.8, 1M context)

### Debug Log References

- `make gen-sqlc` (Docker `sqlc/sqlc:1.31.0`) → `event_id`→`pgtype.UUID`, timestamptz→`pgtype.Timestamptz`, jsonb→`[]byte`.
- Unit: `go test -count=1 ./internal/outbox/...` → ok.
- Integration (dev DB :55432, миграция 0016 применена): `go test -tags=integration -count=1 -p 1 ./internal/outbox/...` → ok (6 тестов).
- Полный CI-набор на ЧИСТОЙ эфемерной БД (postgis :55433, миграции 0001-0016): `./internal/ingest/... ./internal/store/projection/... ./internal/outbox/...` → ok (без регрессий, outbox зелён на чистой БД).
- Red-proof стражей: сырой дубль `event_id` → UNIQUE-violation (`already exists`); миграция реверсивна (goose down→up). Negative-control встроен в каждый integration-страж.
- Гейты: `make lint` (go vet + gofmt), `make check-core` (-count=1), `make check-registry`, `go build ./...`, `go test ./...` — все зелёные.

### Completion Notes List

- **AC1 (O-1):** конверт `{event_id, type, occurred_at, subject_ref, payload, v}` в `notifications_outbox` (миграция 0016). `Enqueue(ctx, q gen.DBTX, e Event)` пишет в tx ВЫЗЫВАЮЩЕГО (атомарность с бизнес-объектом — прецедент `orgnorm.Apply`). `subject_ref` = URN `<entity>:<public_id>` (публичный goszakup id через `SubjectURN`, не bigint). `event_id` детерминируется эмиттером (`NewEventID` — UUIDv5/SHA-1, stdlib-only, без uuid-зависимости) → повтор дедуплицируем.
- **AC2 (O-2/O-3/O-4):** `ProcessBatch` — одна tx: `PollUnsent` (FOR UPDATE SKIP LOCKED) → `Dispatcher.Send` → `MarkSent`/`BumpAttempt`. O-3 = `event_id` UNIQUE + `ON CONFLICT DO NOTHING`. O-4 = «сейчас» только из `clock.Clock` (граница видимости `available_at`, НЕ `now()` в SQL); `backoff(attempts)` — чистая линейная формула `30s*attempts` с потолком 30m.
- **AC3:** `Dispatcher` зафиксирован контрактом; `LoggingDispatcher` логирует нейтральный конверт (id/type/subject_ref/v) и возвращает nil; `var _ Dispatcher = LoggingDispatcher{}`. Telegram + fan-out по подпискам — Epic 7.
- **Гардрейл нейтральности:** конверт не несёт прозу флага; unit-тест проверяет отсутствие табу-слов в логе. Проза — через `render` на поверхности (Epic 7).
- **Вопросы владельцу (рекомендации приняты как дефолт, не блокируют):** №1 — имена событий КОНСТАНТАМИ в `internal/outbox` (`TypeFlagRaised`/`TypeContractCreated`); реестр `event_types` + cross-test — forward на Epic 7 (в S-0 эмиттеров нет). №2 — АРХИТЕКТУРНЫЙ конверт без `subscription_id` (fan-out — Epic 7). №3 — линейный backoff с потолком. №4 — 2-7 несёт МЕХАНИЗМ + LoggingDispatcher, без живой эмиссии/Telegram.

### File List

- `migrations/0016_notifications_outbox.sql` (new)
- `server/internal/store/queries/notifications_outbox.sql` (new)
- `server/internal/store/gen/notifications_outbox.sql.go` (new, sqlc-генерат)
- `server/internal/store/gen/models.go` (modified, sqlc-генерат — добавлен `NotificationsOutbox`)
- `server/internal/store/gen/querier.go` (modified, sqlc-генерат)
- `server/internal/outbox/event.go` (new — `EventID`/`NewEventID`/`Event`/`SubjectURN`/type-константы)
- `server/internal/outbox/dispatcher.go` (new — `Dispatcher`/`LoggingDispatcher`)
- `server/internal/outbox/enqueue.go` (new — транзакционный `Enqueue`)
- `server/internal/outbox/worker.go` (new — `ProcessBatch`/`backoff`)
- `server/internal/outbox/outbox_test.go` (new — unit: eventid/backoff/dispatcher/enqueue-валидация)
- `server/internal/outbox/outbox_integration_test.go` (new — O-1/O-2/O-3/O-4 + SKIP LOCKED)
- `.github/workflows/ci-server.yml` (modified — `./internal/outbox/...` в job `integration`)

### Change Log

- 2026-06-29: Story 2.7 реализована — транзакционный outbox (`notifications_outbox` 0016) + `internal/outbox` (Enqueue/ProcessBatch/Dispatcher/LoggingDispatcher), O-1..O-4 покрыты unit+integration (negative-control, -count=1), CI-job `integration` расширен outbox-пакетом. Status → review.
- 2026-06-29: Код-ревью (3 слоя Blind/Edge/Acceptance) — 0 decision / 7 patch / 4 defer / 5 dismiss; все AC + O-1..O-4 + гардрейлы PASS. 7 патчей применены: валидация конверта (occurred_at/subject_ref/payload), `limit>0`-гард, честный `sent=0` на error-путях, overflow-кламп `backoff`, усиление стража нейтральности (payload-проза не логируется). 4 defer → Epic 7 (dead-letter, lock-across-IO, at-least-once amplification, NewEventID-коллизия). Все гейты + integration на чистой БД зелёные. Status → done.
