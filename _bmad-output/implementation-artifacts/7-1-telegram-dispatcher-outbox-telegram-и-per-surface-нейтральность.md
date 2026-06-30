---
baseline_commit: e99bf88
---

# Story 7.1: Telegram Dispatcher (outbox→Telegram) и per-surface нейтральность

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a команда,
I want надёжную вежливую доставку событий в Telegram с нейтральностью,
so that бот не нарушает гардрейл нейтральности и не спамит источник (Telegram).

Реализует **FR-24…FR-26 (часть: транспорт доставки)** через `Dispatcher`-контракт outbox (2.7). Поверхность — **исходящая** доставка (worker `outbox.ProcessBatch` → `Dispatcher.Send` → Telegram). Подписки/fan-out (FR-24 целиком), смена/отписка (FR-25), антифлуд-дайджест (FR-26) — истории 7.2–7.4.

## ⚠️ Внешний блокер (читать ПЕРВЫМ) — токен-независимый build, живая доставка ждёт токен/3.7

**Реальная отправка в Telegram требует bot-токена + inbound-bootstrap бота (Story 3.7, Epic 3 — НЕ построен: `server/internal/bot/` = только `doc.go`-стаб).** Поэтому, по прецеденту 4.1/5.1/6.4 (движок + честно-пусто/мок, живое ждёт токен):

- **Что строит 7.1 ТОКЕН-НЕЗАВИСИМО (демонстрируемо на mock-транспорте в тестах):**
  1. `TelegramDispatcher`, реализующий `outbox.Dispatcher` (`Send(ctx, Event) error`) — **ноль правок 2.7** (compile-time `var _ outbox.Dispatcher = …`).
  2. Транспорт за **интерфейсом** (`TelegramTransport`/`MessageSender`) → live HTTP `sendMessage` — тонкий шов, мокается в тестах.
  3. **Чистое Telegram-форматирование**: проза ТОЛЬКО через `render` (short-форма) → MarkdownV2-экранирование + обрезка по лимиту 4096 + эмодзи-префикс. Без ручной конкатенации строк ботом.
  4. **Per-surface neutrality-тест на ФИНАЛЬНОЙ строке** (после MarkdownV2/обрезки/эмодзи): `registry.FindTaboo(loc, final) == ∅` + negative-control (краснеет на «нарушение»/«бұзушылық») в ОБЕИХ локалях.
  5. **Классификация отказов** (429 / chat blocked / chat deleted → permanent-vs-transient) + политика max-attempts/dead-letter (закрывает долг 2.7) + **deactivation-шов** (`SubscriptionDeactivator`, no-op по умолчанию; реальная таблица подписок — 7.2).
  6. **Троттл** (вежливость к Telegram) детерминированно через `Clock`.

- **Что ОТЛОЖЕНО до токена/3.7/7.2 (честный шов, НЕ выдумываем):** живой HTTP `sendMessage` (bot-токен); polling/inbound-bootstrap бота (3.7/Epic 3); таблица подписок + реальная деактивация + fan-out по `subscription_id` (7.2). Эти швы задокументированы и заполняются БЕЗ слома формы `TelegramDispatcher`.

## Acceptance Criteria

1. **AC1 — `TelegramDispatcher` реализует контракт `Dispatcher` (ноль правок 2.7) + троттл + однократность.**
   **Given** интерфейс `outbox.Dispatcher` (`Send(ctx, outbox.Event) error`, `dispatcher.go:11`) и worker `ProcessBatch` (дедуп по `event_id` уже в O-3)
   **When** реализуется Telegram-доставка (S-1)
   **Then** `TelegramDispatcher` реализует ту же сигнатуру (`var _ outbox.Dispatcher = TelegramDispatcher{}`, ноль изменений в `outbox`-пакете); троттл (вежливость к Telegram, детерминированно через `Clock`); однократность наследуется из дедупа `event_id` (O-3) — повтор события не двоит у получателя.

2. **AC2 — текст ТОЛЬКО через `render` short-форму (бот не конкатенирует руками).**
   **Given** общий lexicon-контракт (Epic 4: `render.Renderer`, glossary)
   **When** Telegram-поверхность рендерит текст события (flag.raised / contract.created)
   **Then** проза берётся ТОЛЬКО через `render` (short-форма `flag.<id>.short` или существующий `FlagLine`/`Text`; architecture.md:630, 776) — ноль текстовых литералов в bot-коде; смысловое ядро {рамка, число, flag_id, methodology_version, locale} совпадает с web/OG (cross-surface, architecture.md:625-632).

3. **AC3 — per-surface neutrality-тест на ФИНАЛЬНОЙ строке + lexicon-linter exit≠0.**
   **Given** Telegram-форматирование (MarkdownV2-экранирование, обрезка по 4096, эмодзи-префиксы)
   **When** проверяется вывод
   **Then** ассерт на ФИНАЛЬНОЙ строке ПОСЛЕ форматирования (не на сырой прозе до диспатча): `registry.FindTaboo(loc, final)` пуст в ОБЕИХ локалях для всех состояний флага; **negative-control** доказывает, что матчер краснеет (на «нарушение» ru / «бұзушылық» kk — иначе страж фиктивен, см. [[guards-must-prove-red]]); lexicon-linter падает `exit≠0` при ≥1 taboo.

4. **AC4 — отказ доставки → детерминированный retry / деактивация (без бесконечного ретрая).**
   **Given** отказ Telegram (429 throttle / 403 chat blocked / chat deleted)
   **When** `Dispatcher.Send` возвращает ошибку
   **Then** **transient** (429/5xx/сеть) → растит `attempts`, retry детерминирован через `Clock` (O-4 backoff), **с потолком max-attempts → dead-letter** (poison-строка не ре-поллится вечно — закрывает долг 2.7); **permanent** (403 blocked / chat not found) → деактивация подписки через `SubscriptionDeactivator`-шов (no-op сейчас, реальная таблица — 7.2), НЕ бесконечный ретрай.

## Scope Fence — что НЕ входит (читать перед стартом)

- ❌ **Живой HTTP `sendMessage`** (bot-токен) — за интерфейсом транспорта, мок в тестах; live → токен.
- ❌ **Polling / inbound-bootstrap бота (3.7)** — `internal/bot` остаётся стабом до Epic 3.
- ❌ **Таблица подписок + fan-out по `subscription_id` (FR-24 целиком) → 7.2** — 7.1 строит ТРАНСПОРТ доставки одного события одному chat_id (через шов), не подписочную модель. `SubscriptionDeactivator` — интерфейс с no-op, реальная таблица 7.2.
- ❌ **Смена района/отписка (FR-25) → 7.3; антифлуд-дайджест (FR-26) → 7.4.**
- ❌ **Формирование самих событий** (flag.raised/contract.created эмиттеры) — эмиссия живёт в импорте (Epic 2/token); 7.1 потребляет уже-записанные outbox-строки.

## Tasks / Subtasks

- [x] **Task 1 — `TelegramDispatcher` + транспортный шов (AC1).**
  - [x] Новый пакет (предложение: `server/internal/bot/telegram/` или `server/internal/notify/telegram/`) с `TelegramDispatcher`, реализующим `outbox.Dispatcher`; `var _ outbox.Dispatcher = TelegramDispatcher{}`.
  - [x] Интерфейс транспорта `MessageSender` (`SendMessage(ctx, chatID, text, opts) error`) — live HTTP-реализация за токеном (шов), мок/fake для тестов.
  - [x] Троттл (rate-limit) детерминированно через `clock.Clock` (вежливость к Telegram; architecture.md:440).
  - [x] **НЕ менять пакет `outbox`** (контракт зафиксирован 2.7).
- [x] **Task 2 — Telegram-форматирование через `render` (AC2/AC3).**
  - [x] `render`-short форма для Telegram: проверить наличие/добавить glossary-ключ `flag.<id>.short` (сейчас НЕТ — решение: добавить short-ключи ИЛИ переиспользовать `FlagLine`; см. Open-Q4). Никаких литералов в bot-коде.
  - [x] MarkdownV2-экранирование (зарезервированные символы Telegram: ``_ * [ ] ( ) ~ ` > # + - = | { } . !``), обрезка по 4096 (по границе, не рвать MarkdownV2-разметку), эмодзи-префикс.
  - [x] Чистая функция форматирования (входы → финальная строка; без IO) — тестируема и детерминирована.
- [x] **Task 3 — per-surface neutrality-тест (AC3).**
  - [x] Тест на ФИНАЛЬНОЙ строке после MarkdownV2/обрезки/эмодзи: `Reg.FindTaboo(loc, final)==∅` для всех flag_state × обе локали (паттерн `render/cross_surface_test.go:55-57`).
  - [x] **Negative-control** (FindTaboo краснеет на «нарушение»/«бұзушылық», обе локали — `cross_surface_test.go:72-77`).
  - [x] Активировать отложенный cross-surface golden-snapshot равенства смыслового ядра web↔Telegram (deferred-work:101) ИЛИ зафиксировать, почему откладывается до 7.2-поверхности.
- [x] **Task 4 — классификация отказов + max-attempts/dead-letter + deactivation-шов (AC4).**
  - [x] Классификатор ошибок Telegram: transient (429/5xx/сеть) vs permanent (403 blocked / chat not found).
  - [x] Политика max-attempts → dead-letter (закрывает долг 2.7 deferred-work:275): порог попыток → пометка/квартин (колонка/таблица dead-letter ИЛИ статус), poison-строка не ре-поллится вечно.
  - [x] `SubscriptionDeactivator`-интерфейс с no-op-дефолтом (реальная таблица 7.2; прецедент `RecalcHook` 2.4 / `PricePerKMSamples` 6.4). Permanent-отказ → вызов деактивации.
  - [x] Решить (Open-Q3): делать ли в 7.1 реструктуризацию `ProcessBatch` lock-across-IO (claim+commit→deliver-вне-tx→mark отдельной tx) + свежий `clk.Now()` + ограничение amplification (deferred-work:276/277/279), ИЛИ отложить до live-delivery (на mock-транспорте moot).
- [x] **Task 5 — тесты + гейты.**
  - [x] Unit: dispatcher (мок-транспорт, троттл через `clock.Fixed`), форматирование (MarkdownV2/4096/эмодзи golden), классификатор отказов, max-attempts/dead-letter, deactivation-шов (no-op + вызов).
  - [x] Neutrality (Task 3) + negative-control.
  - [x] `var _`-контракт; arch-страж AR-27 (если новый пакет тащит запрещённые импорты — проверить go-list-страж).
  - [x] Все гейты: `go build/vet/gofmt/test -count=1 ./...`, check-registry, check-core.
- [x] **Task 6 — миграция (если нужна).** Dead-letter может потребовать колонку/таблицу (`notifications_outbox.dead_at`/`dead_reason` ИЛИ отдельная `notifications_dead_letter`). Если да — миграция `0019_*` (следующий номер после 0018). Если порог держится в памяти/статусе без схемы — без миграции (подтвердить в Open-Q3).

## Dev Notes

### Несущие опоры (citable)

- **Контракт `Dispatcher` зафиксирован 2.7 — НЕ менять:** `Send(ctx context.Context, e outbox.Event) error` [Source: server/internal/outbox/dispatcher.go:11-13]. Референс-реализация `LoggingDispatcher` (нейтральный конверт, без прозы) [dispatcher.go:15-35].
- **`outbox.Event`** несёт `EventID/Type/OccurredAt/SubjectRef/Payload/V` — машиночитаемый конверт, НЕ прозу [Source: server/internal/outbox/event.go:73-80]. Типы: `flag.raised`, `contract.created` [event.go:16-17]. `SubjectRef` = URN `<entity>:<public_id>` [event.go:65-68].
- **Worker `ProcessBatch`** (O-2): PollUnsent `FOR UPDATE SKIP LOCKED` → `Send` → MarkSent / BumpAttempt(`backoff(attempts)`, линейный 30s→30m); дедуп O-3 (`event_id`); retry детерминирован O-4 (`Clock`, НЕ `now()` в SQL) [Source: server/internal/outbox/worker.go:44-118; architecture.md:434-438].
- **Проза ТОЛЬКО через `render`:** `render.Renderer.FlagLine(flagID, fs, loc)` [render.go:46-52], `Text(loc,key)` [render.go:67], `RenderFlagState` [render.go:56-63]. `og`/`bot` берут прозу ТОЛЬКО отсюда (нейтральность структурой) [Source: architecture.md:776; render.go:22-24].
- **Нейтральность-линтер:** `registry.FindTaboo(loc, text)` — casefold + фолд гомоглифов + матч по корню; возвращает совпавшие taboo-корни [Source: server/internal/registry/neutrality.go:36-58]. Паттерн теста: `cross_surface_test.go:55-77` (assert ∅ + negative-control обе локали).
- **All-Surfaces Consistency:** Telegram = сокращение через registry short-форму (`flag.x.short`); смысловое ядро {рамка, число, flag_id, methodology_version, locale} совпадает web/OG/Telegram [Source: architecture.md:625-632].
- **Анти-паттерн (запрещено):** прямой вызов Telegram из импорта — только через outbox (`flag.raised` event_id той же tx) [Source: architecture.md:673]. Telegram = polling, через outbox [architecture.md:801].
- **Долги 2.7, которые 7.1 закрывает/решает** [Source: deferred-work.md:275-280, 101]: dead-letter/max-attempts (275); lock-across-IO restructure (276); at-least-once amplification (277); `NewEventID`/`SubjectURN` пустые части (278/280); батч-`now` reuse (279); cross-surface golden neutrality (101). 7.1 — естественное место для (275) + классификации; (276/277/279) — см. Open-Q3.

### Ключевые решения (Decisions) — ✅ ПОДТВЕРЖДЕНЫ владельцем 2026-06-30 (все дефолты приняты)

- **D1 (токен-независимость):** строить dispatcher + форматирование + нейтральность + классификацию на МОК-транспорте; live HTTP/polling/bootstrap → токен/3.7. Дефолт: да (прецедент 6.4).
- **D2 (deactivation-шов):** `SubscriptionDeactivator` интерфейс + no-op (реальная таблица 7.2). Дефолт: да (прецедент `RecalcHook` 2.4 / `PricePerKMSamples` 6.4).
- **D3 (lock-across-IO):** реструктуризацию `ProcessBatch` (claim→deliver-вне-tx→mark) + свежий `clk.Now()` + amplification отложить до live-delivery (на mock moot); в 7.1 — max-attempts/dead-letter + классификация. Дефолт: отложить restructure, сделать политику.
- **D4 (short-форма):** добавить glossary-ключи `flag.<id>.short` (architecture.md:630) ИЛИ переиспользовать `FlagLine`. Дефолт: переиспользовать `FlagLine` если short-вариант не нужен по длине; добавить `.short` только если 4096/UX требует.

### Project Structure Notes

- Новый пакет доставки: предложение `server/internal/bot/telegram/` (под существующим `internal/bot`) ИЛИ `server/internal/notify/`. НЕ класть в `outbox` (контракт чужой). `outbox/` владелец: importer пишет, bot читает [architecture.md:733].
- Транспорт (HTTP `sendMessage`) — за интерфейсом; live-реализация может жить за build-tag/конфигом токена (прецедент interim-scrape `//go:build`), но dispatcher+форматирование+тесты — БЕЗ тега (переиспользуемы).
- Миграция (если dead-letter в схеме): `migrations/0019_*` (0018 — district index 6.3; следующий свободный — 0019).
- Конфиг: `internal/config` для bot-токена/throttle-параметров (токен из env, не в логи — architecture.md:586 «НИКОГДА токены/telegram_chat_id в логах»).

### Gotchas (из ревью/прошлых историй)

- **Контракт `Dispatcher` НЕ менять** — иначе слом 2.7 (`var _` + outbox-тесты покраснеют). Расширять через нового реализатора, не сигнатуру.
- **Нейтральность — на ФИНАЛЬНОЙ строке** (после MarkdownV2/обрезки/эмодзи), НЕ на сырой прозе: экранирование/обрезка не должны протащить taboo или сломать рамку (AC3 явно).
- **Стражи обязаны краснеть** [[guards-must-prove-red]]: neutrality-тест + negative-control обе локали; `-count=1` для go-list/golden-стражей.
- **Токены/`telegram_chat_id` НИКОГДА в логи** [architecture.md:586]. `LoggingDispatcher` логирует только нейтральный конверт (event_id/type/ref/v) — Telegram-dispatcher так же.
- **at-least-once by design** (O-3 дедуп у получателя по event_id) — не строить exactly-once; ограничение amplification — опционально (D3).
- **Энтанглмент-урок (ретро Epic 6 AI-3):** коммитить 7.1 отдельной историей сразу после code-review (не копить).
- **Integration-шов БД:** конвенция `//go:build integration` + `DATABASE_URL` + `t.Skip` (НЕ testcontainers); локально БД на 55432 (чужой проект занял порт — env-блокер, как у 6.3/6.4). Dead-letter integration-тест скипается без `DATABASE_URL`.

### Тестирование (стандарты)

- Go unit (мок-транспорт, `clock.Fixed`), golden для MarkdownV2/обрезки/эмодзи, neutrality + negative-control (обе локали), `var _`-контракт, classifier-таблица (429/403/5xx/сеть), max-attempts/dead-letter, deactivation-шов (no-op + вызов на permanent).
- Гейты: `go build/vet/gofmt`, `go test -count=1 ./...`, check-core, check-registry. Integration (dead-letter persistence) — `//go:build integration`, скип без `DATABASE_URL`.

### References

- [Source: _bmad-output/planning-artifacts/epics.md:1734-1757] — Story 7.1 AC (epic-level).
- [Source: _bmad-output/planning-artifacts/architecture.md:434-440, 577, 625-632, 673, 733, 776, 801] — outbox O-2/3/4, all-surfaces consistency, анти-паттерн, структура.
- [Source: server/internal/outbox/dispatcher.go, event.go, worker.go] — контракт/конверт/воркер.
- [Source: server/internal/render/render.go:22-67] — Renderer (FlagLine/Text).
- [Source: server/internal/registry/neutrality.go:36-58; server/internal/render/cross_surface_test.go:55-77] — линтер + паттерн теста.
- [Source: _bmad-output/implementation-artifacts/deferred-work.md:101, 275-280] — долги 2.7 → Epic 7.
- Prior: [Source: _bmad-output/implementation-artifacts/2-7-*.md] — outbox/Dispatcher история; [[sprint-sequencing-epic1-first]], [[guards-must-prove-red]], [[local-dev-docker-env]].

## Open Questions — ✅ ВСЕ ПОДТВЕРЖДЕНЫ владельцем 2026-06-30 (дефолты приняты, дев следует им без переспроса)

1. **(D1/Блокер) Подтвердить: 7.1 = токен-независимый build** (dispatcher + форматирование + нейтральность + классификация на МОК-транспорте; live HTTP `sendMessage` + polling + bot-bootstrap 3.7 → токен/Epic 3)? Дефолт: да (прецедент 6.4).
2. **(D2) deactivation-шов `SubscriptionDeactivator` (no-op сейчас, таблица подписок 7.2)** vs полный descope деактивации в 7.2? Дефолт: шов (прецедент `RecalcHook`/`PricePerKMSamples`). Без шва AC4 «permanent→деактивация» нечем выразить токен-независимо.
3. **(D3) lock-across-IO restructure `ProcessBatch` (claim→deliver-вне-tx→mark) + свежий `clk.Now()` + amplification (deferred-work:276/277/279)** — делать в 7.1 ИЛИ отложить до live-delivery? Дефолт: ОТЛОЖИТЬ (на мок-транспорте moot; реструктуризация tx-семантики вслепую без живого транспорта рискованна), в 7.1 — max-attempts/dead-letter (275) + классификация.
4. **(D4) short-форма Telegram: добавить glossary `flag.<id>.short`** (architecture.md:630) ИЛИ переиспользовать `FlagLine`? Дефолт: переиспользовать `FlagLine`, добавить `.short` только если лимит 4096/UX требует.
5. **(миграция) dead-letter в СХЕМЕ (колонка `notifications_outbox.dead_at`/`dead_reason` или таблица) → миграция 0019** vs порог в памяти/статусе без схемы? Дефолт: лёгкая колонка `dead_at`/`dead_reason` в `notifications_outbox` (персистентный квартин переживает рестарт воркера).

## Review Findings (code-review 2026-06-30, 3 параллельных уровня: Blind / Edge Case / Acceptance)

**Acceptance Auditor: PASS** — все AC1–AC4 + D1–D5 + Scope Fence + Guardrails MET, гейты независимо перепроверены зелёными. Хантеры: 0 Critical/High; 1 Medium (throttle), остальное Low. Триаж: 0 decision, 6 patch, 2 defer, 4 dismiss.

**Все 6 patch ПРИМЕНЕНЫ + верифицированы (2026-06-30):** P1 throttle `last=now+wait`+nil-Clock-гард+mutex+advancing-clock тест; P2 `\` в reserved-наборе; P3 raw-prose FindTaboo + escaped-сравнение; P4 `knownFlagState`-гард (unknown→FlagRaised); P5 `truncateUTF16` (UTF-16-лимит Telegram); P6 заголовок 0001-0019. Гейты зелёные: `go build/vet/gofmt/test -count=1 ./...`; check-registry/core. Defer'ы → `deferred-work.md` (per-recipient дубли→7.2; throttle-в-tx→D3/live-delivery).

### Patch

- [x] [Review][Patch] **Throttle недо-троттлит ~2× под burst + nil-Clock паника + не потокобезопасен** (Blind#1/Edge#4/#6). `Throttle.Wait` ставит `last = now` ДО сна → слитое время засчитывается в следующий gap (реальные часы; Fixed-clock тест маскирует). Фикс: `last = now.Add(wait)` (логическое время отправки) + гард nil `Clock`→`clock.Real{}` (как у Sleep) + `sync.Mutex` + тест на ПРОДВИГАЮЩИХСЯ часах. [bot/throttle.go; dispatcher_test.go TestThrottle_MinGap]
- [x] [Review][Patch] **`escapeMarkdownV2` не экранирует сам `\`** (Blind#3/Edge#3). Литеральный `\` в прозе + следующий reserved → `\\.`→Telegram-парс-ошибка. Фикс: добавить `\` в `markdownV2Reserved` (→ удвоение). Latent (glossary без `\`), но набор неполон. [bot/format.go]
- [x] [Review][Patch] **Neutrality-страж можно обмануть экранированием reserved-несущего taboo-корня** (Edge#9c). `FindTaboo` (substring по нормализованному тексту, `\` сохраняется) НЕ найдёт `анти\-коррупц`, если корень содержит reserved-символ → escaping маскирует taboo. Сейчас все корни — чистая кириллица (не триггерит), но дыра в гарантии AC3. Фикс: ассертить `FindTaboo` ТАКЖЕ на сырой прозе (`renderEvent`, до escaping) + контроль reserved-несущего корня. Заодно `TestRenderEvent_ProseViaRender` сравнивает unescaped vs escaped — чинить (Blind#4). [bot/neutrality_test.go]
- [x] [Review][Patch] **Гарбедж `flag_state` мислейблится как «insufficient data»** (Edge#7). `renderEvent` сбрасывает в FlagRaised только ПУСТОЙ state; непустой мусор → `FlagLine` (бинарный `==FlagRaised`) → «недостаточно данных» (нечестно). Фикс: валидировать `flag_state` против `AllFlagStates()`, неизвестный (для flag.raised) → FlagRaised (тип события авторитетен). [bot/format.go renderEvent]
- [x] [Review][Patch] **Обрезка по рунам, а Telegram считает UTF-16** (Edge#2). 🔔 = 2 UTF-16-юнита (1 руна) → финал в 4096 рун = до 4097 UTF-16 → Telegram 400 «too long». Latent (проза крошечная). Фикс: бюджетировать лимит в UTF-16. [bot/format.go FormatTelegram/truncate]
- [x] [Review][Patch] **Заголовок integration-теста занижает миграции (0001-0016 vs нужно 0019)** (Edge#10, trivial). [outbox/outbox_integration_test.go:7]

### Defer

- [x] [Review][Defer→7.2] **Частичный fan-out: transient-сбой одного chat → ре-доставка уже-успешным (дубли)** (Blind#2/Edge#1) — event-level ретрай воркера переигрывает ВСЕХ получателей (дедуп O-3 по `event_id`, не `(event_id,chat_id)`). Нужно per-recipient delivery-state (таблица доставок) → Story 7.2. Latent (NoRecipients). Доп: не-идемпотентный 7.2-`Deactivator` на повторе → его ошибка как transient → цикл до dead-letter — учесть в 7.2. → deferred-work.
- [x] [Review][Defer→live-delivery] **Throttle.Sleep внутри открытой tx держит FOR UPDATE-локи** (Edge#5) — это часть УЖЕ отложенного D3 (lock-across-IO restructure, deferred-work:276/279): доставка+троттл вне tx. Subsumed.

### Dismissed (след)

- nil `Renderer` паника (Blind#5) — ЛОЖНОЕ: `render.glossaryOr` nil-safe (`if rd.Reg != nil`→fallback «неизвестно»), не паникует.
- `MarkSent` + `sent++` при 0 получателей / только-permanent (Edge#8) — функционально верно (событие обработано; нет подписчиков ≠ сбой). Семантика метрики «finalized vs delivered» — наблюдаемость/7.2.
- monopoly/rnu без glossary-summary → fallback (Acceptance obs#1) — пробел glossary Epic 4/5, не 7.1; fallback нейтрален; тот же на web/OG (cross-surface сохранён).
- rich-content (число/deep-link) вне `flagPayload` (Acceptance obs#2) — по Scope Fence отложено в 7.2.

## Dev Agent Record

### Agent Model Used

Claude Opus 4.8 (1M context) — dev-story (TDD, токен-независимый build).

### Debug Log References

- `go build ./...` ✓; `go vet ./...` ✓; `gofmt -l` чисто.
- `go test -count=1 ./internal/bot/ ./internal/outbox/` ✓ (формат/нейтральность/диспетчер/throttle/shouldDeadLetter).
- `go test -count=1 ./...` — весь серверный набор зелёный.
- `go test -tags=integration -count=1 ./internal/outbox/` — компилируется, честный skip без `DATABASE_URL` (dead-letter integration-тест; конвенция репо, БД на 55432 чужого проекта — env-блокер, как 6.3/6.4).
- `make gen-sqlc` идемпотентен (md5 до/после совпали — нет дрейфа `gen/`); `make check-registry` + `make check-core` (arch-страж AR-27) зелёные.

### Completion Notes List

Реализована FR-24…26 (часть: транспорт доставки) как **токен-независимый build** (прецедент 6.4/5.1): движок диспетчера + форматирование + нейтральность + классификация на МОК-транспорте; live HTTP/polling/bot-bootstrap(3.7) и таблица подписок(7.2) отложены за швами.

- **Task 1 (AC1):** пакет `internal/bot` — `TelegramDispatcher` реализует `outbox.Dispatcher` (`var _ outbox.Dispatcher = TelegramDispatcher{}`, **ноль правок контракта 2.7**) над швами `MessageSender` (транспорт; live HTTP за токеном) / `RecipientResolver` (fan-out 7.2; дефолт `NoRecipients`) / `SubscriptionDeactivator` (деактивация 7.2; дефолт `NoopDeactivator`). Троттл `Throttle` (min-interval, детерминизм через `clock.Clock`+мок-Sleep).
- **Task 2 (AC2):** проза ТОЛЬКО через `render` (`FlagLine`/`Text`, **D4 reuse FlagLine** — `flag.*.short` не добавлял) → `escapeMarkdownV2` (зарезерв. символы) → обрезка 4096 рун (по границе руны) → нейтральный эмодзи-префикс 🔔 → срез висячего `\`. Чистые функции, ноль литералов в боте.
- **Task 3 (AC3):** `TestTelegramNeutrality_FinalString` — `FindTaboo==∅` на ФИНАЛЬНОЙ строке для всех flag_state × обе локали + contract.created; **negative-control** (краснеет на «нарушение»/«бұзушылық» в финале, обе локали). Cross-surface равенство ядра web↔OG↔Telegram энфорсится переиспользованием единого `render.FlagLine` + `TestRenderEvent_ProseViaRender` (финал содержит точную `FlagLine`) → отдельный tri-surface golden-snapshot (deferred-work:101) избыточен (единый источник).
- **Task 4 (AC4):** классификация `ErrChatUnavailable` (permanent: 403/chat-not-found → деактивация подписки + НЕ ретрай) vs transient (429/5xx/сеть → возврат ошибки → воркер растит attempts O-4). **max-attempts/dead-letter** (D5): migration `0019` + `MarkDead` + `ProcessBatchN(maxAttempts)` + чистый `shouldDeadLetter`; poison-строка при потолке выпадает из поллинга (закрывает долг 2.7 deferred-work:275). **D3 (lock-across-IO restructure) ОТЛОЖЕН до live-delivery** (на mock-транспорте moot — подтверждено владельцем).
- **Task 5/6:** тесты (unit + neutrality + dead-letter integration env-gated) + миграция `0019`. Все гейты зелёные, sqlc идемпотентен.

**Не wired в live cmd** (LoggingDispatcher остаётся S-0-получателем): `TelegramDispatcher` без живого `MessageSender` (токен) и подписчиков (7.2) ничего не доставит — продовая проводка ждёт токен/3.7/7.2 (честный шов, не выдумываем).

### File List

**Backend (Go) — NEW (`internal/bot`):**
- `server/internal/bot/transport.go` — `MessageSender` + `ErrChatUnavailable`/`IsPermanent`
- `server/internal/bot/recipients.go` — `RecipientResolver` + `NoRecipients` (шов 7.2)
- `server/internal/bot/deactivate.go` — `SubscriptionDeactivator` + `NoopDeactivator` (шов 7.2)
- `server/internal/bot/format.go` — MarkdownV2-экранирование/обрезка 4096/эмодзи/`formatEvent` (render)
- `server/internal/bot/throttle.go` — `Throttle` (вежливость, clock-детерминизм)
- `server/internal/bot/dispatcher.go` — `TelegramDispatcher` (реализует `outbox.Dispatcher`)
- `server/internal/bot/format_test.go`, `neutrality_test.go`, `dispatcher_test.go` — NEW (unit/AC3/AC1/AC4)

**Backend — MOD (outbox dead-letter):**
- `server/internal/outbox/worker.go` — `ProcessBatchN(maxAttempts)`+`ProcessBatch`-обёртка, `shouldDeadLetter`/`clampReason`, dead-letter ветка
- `server/internal/store/queries/notifications_outbox.sql` — `MarkDead` + `PollUnsent`(+`dead_at IS NULL`) + `GetOutboxByEventID`(+dead-поля)
- `server/internal/store/gen/notifications_outbox.sql.go` — GEN (`make gen-sqlc`)
- `server/internal/outbox/outbox_test.go` — MOD (`TestShouldDeadLetter`)
- `server/internal/outbox/outbox_integration_test.go` — MOD (`TestProcessBatchN_DeadLetter`, env-gated)

**Миграция:**
- `migrations/0019_outbox_dead_letter.sql` — NEW (`dead_at`/`dead_reason` + частичный индекс)

**Артефакты:**
- `_bmad-output/implementation-artifacts/sprint-status.yaml` — MOD (7-1 → review)
- `_bmad-output/implementation-artifacts/deferred-work.md` — MOD (долг 2.7 dead-letter отмечен resolved)

### Change Log

- 2026-06-30 — Реализована Story 7.1 (FR-24…26 транспорт): `internal/bot.TelegramDispatcher` (реализует `outbox.Dispatcher`, ноль правок 2.7) над швами транспорт/получатели/деактивация (мок/no-op → токен-независимо); Telegram-форматирование (MarkdownV2/4096/эмодзи) через `render`; per-surface neutrality-тест на финальной строке + negative-control; классификация permanent/transient + deactivation-шов; **max-attempts/dead-letter** (migration 0019 + `ProcessBatchN`, закрывает долг 2.7). Live-доставка/bot-bootstrap(3.7)/подписки(7.2) отложены за швами (D1/D2). D3 (lock-across-IO restructure) отложен до live-delivery. Статус → review.
- 2026-06-30 — code-review (3 слоя): Acceptance PASS (AC/D MET); 6 patch применены (throttle pre-sleep/nil-Clock/mutex, `\`-escape, neutrality raw-prose-страж, unknown flag_state-гард, UTF-16-обрезка, integration-заголовок), 2 defer→deferred-work (per-recipient дубли→7.2; throttle-в-tx→D3), 4 dismiss. Гейты зелёные. Статус → done.
