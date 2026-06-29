# Story 2.7: Transactional outbox (LoggingDispatcher)

Status: ready-for-dev

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

- [ ] **T1. Таблица + конверт + транзакционный Enqueue (O-1)** (AC1)
  - [ ] Миграция `0016_notifications_outbox.sql` (goose): `id` identity PK; `event_id` UUID **UNIQUE** (дедуп O-3); `type` TEXT; `occurred_at` TIMESTAMPTZ; `subject_ref` TEXT (URN); `payload` JSONB; `v` INT (версия payload); `sent_at` TIMESTAMPTZ NULL; `attempts` INT NOT NULL DEFAULT 0 (CHECK ≥0); `available_at` TIMESTAMPTZ NOT NULL DEFAULT now() (видимость O-4); `created_at`. Индекс для поллинга: `(available_at) WHERE sent_at IS NULL`.
  - [ ] sqlc-запросы `internal/store/queries/notifications_outbox.sql` (`make gen-sqlc` через Docker; `store/gen` не править): `EnqueueEvent` (INSERT … `ON CONFLICT (event_id) DO NOTHING`), `PollUnsent` (FOR UPDATE SKIP LOCKED), `MarkSent`, `BumpAttempt`.
  - [ ] `internal/outbox`: `Event` (конверт), `Enqueue(ctx, q gen.DBTX, e Event) error` — пишет в tx ВЫЗЫВАЮЩЕГО (атомарность с бизнес-объектом).
  - [ ] Integration-тест O-1: бизнес-вставка+Enqueue в одной tx → rollback ⇒ НЕТ ни бизнес-строки, ни outbox-строки; commit ⇒ обе есть.
- [ ] **T2. Dispatcher-контракт + LoggingDispatcher** (AC3)
  - [ ] `Dispatcher` интерфейс `Send(ctx, Event) error`; `LoggingDispatcher{Log *slog.Logger}` (логирует, возвращает nil); `var _ Dispatcher = LoggingDispatcher{}`.
  - [ ] Unit-тест: LoggingDispatcher.Send не ошибается, логирует id/type/subject_ref (нейтрально, без прозы флага).
- [ ] **T3. Воркер доставки (O-2) + SKIP LOCKED** (AC2)
  - [ ] `ProcessBatch(ctx, pool, d Dispatcher, clk clock.Clock, limit int) (sent int, err error)`: tx → `PollUnsent($now, limit)` (FOR UPDATE SKIP LOCKED) → для каждой `Dispatcher.Send`; успех → `MarkSent($now)`; ошибка Send → `BumpAttempt(attempts+1, available_at=$now+backoff)`; commit.
  - [ ] Integration-тест O-2: enqueued событие → ProcessBatch → `sent_at` проставлен, Dispatcher вызван 1×. SKIP LOCKED: два конкурентных ProcessBatch не шлют одну строку дважды (counting-dispatcher).
- [ ] **T4. Дедуп (O-3) + детерминизм retry/visibility (O-4)** (AC2)
  - [ ] O-3: повторный Enqueue того же `event_id` — no-op (ON CONFLICT DO NOTHING); integration-тест: 2× Enqueue → 1 строка.
  - [ ] O-4: backoff — чистая функция `backoff(attempts) time.Duration` (простая воспроизводимая, напр. `base * attempts` с потолком); воркер берёт «сейчас» из `clk` (НЕ `now()` в SQL для границы видимости). Unit-тест backoff детерминизм; integration: упавший Send → attempts=1 + available_at сдвинут (не виден до $now+backoff с `clock.Fixed`).
- [ ] **T5. Гейты, CI, честность**
  - [ ] Расширить CI-job `integration` (ci-server.yml, Story 2.6): добавить `./internal/outbox/...` в прогон (красный=блок merge).
  - [ ] `make build`/`lint`/`check-core`(-count=1)/`check-registry` зелёные; negative-control на каждый страж (`-count=1`); честный конвейер без прозы флага.

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

{{agent_model_name_version}}

### Debug Log References

### Completion Notes List

### File List
