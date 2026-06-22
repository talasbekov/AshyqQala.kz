---
baseline_commit: bba6c126bd75911df01957efe4378396a211c619
---

# Story 1.4: registry/ + двухосевой enum честных состояний + render-каркас

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a **команда платформы AshyqQala.kz**,
I want **единый источник честных состояний и нейтральной прозы (`registry/`) + типизированный двухосевой enum + каркас `render()`**,
so that **честность и нейтральность гарантируются СТРУКТУРОЙ (закрытый union + CI-страж taboo), а не дисциплиной разработчика** (несущие гардрейлы AR-12/AR-14/AR-16).

> **Тип истории:** enabler-story Эпика 1 (backbone-контракты, расширение Winston). Без FR напрямую,
> но несёт гардрейлы **нейтральности** (FR-12) и **честности/без-домысла** (FR-10) на уровне
> структуры. Ставит фундамент для карточек (Epic 5), флагов (Epic 4) и i18n (1.6).
> **Предусловия выполнены:** монорепо `server/` (1.1), `Field{value,state}` + OpenAPI-арбитр (1.3),
> пустой каркас `registry/` и `ci-registry.yml` — всё СУЩЕСТВУЕТ и проверено (см. «Что переиспользовать»).
> Task 0-HALT не требуется.

## Acceptance Criteria

**AC1 — `registry/values` наполнен и читается в рантайме (не codegen)**
**Given** каркас `registry/values/` (сейчас спит — только `.gitkeep`)
**When** определены `honest_states` (`value_state`: `ok`/`no_data`/`insufficient_sample`/`not_comparable`/`stale`/`geocode_pending`/`geocode_failed`/`source_conflict`/`redacted`/`not_applicable`/`error`; `flag_state`: `raised`/`not_raised`/`insufficient_data`/`not_published`), `glossary{kk,ru}` (метка для каждого состояния), `taboo_lexicon` (RU+KZ)
**Then** они читаются Go-загрузчиком **в рантайме** (парс файла при старте, НЕ генерация Go-кода из registry); неизвестное состояние/отсутствующий файл → честная ошибка старта, не «пусто».

**AC2 — каркас `render(flag_id, evidence, params, locale)` — закрытый union с ОБЯЗАТЕЛЬНОЙ default-веткой**
**Given** каркас `render()` в `server/internal/render/`
**When** рендерится любое `value_state`/`flag_state`
**Then** `switch` по закрытому enum имеет **обязательную `default`-ветку** → текст «неизвестно, см. методику» (НЕ пустая строка, НЕ паника); прозу несёт только `render()` (единая точка нейтральности); функция чистая (одни входы → один текст). Тест полноты: новый член enum без своей ветки ловится property-тестом.

**AC3 — CI doc-нейтральности красный на taboo-строке в любой локали (блок merge)**
**Given** `ci-registry.yml` (сейчас placeholder-echo)
**When** в сгенерированной прозе/glossary любой из двух локалей встречается taboo-строка (RU: «нарушение/коррупция/виновен…»; KZ: «бұзушылық/сыбайлас жемқорлық/кінәлі…»)
**Then** тест красный и блокирует merge; матчер **нормализует регистр и морфологию** (NFKC + casefold + совпадение по корню/стему, чтобы «нарушение/нарушения/нарушений» ловились одним корнем) и гомоглифы (лат/кир-двойники).

**AC4 — перекрёстная согласованность enum (registry ↔ OpenAPI ↔ Go)**
**Given** три источника одного enum: `registry/values/honest_states`, `docs/api-contracts/openapi.yaml` (`StringField.state`), типизированные Go-консты
**When** прогон перекрёстного теста
**Then** множества членов `value_state` **РАВНЫ** во всех трёх; рассинхрон → красный. (Оси `i18n.kk`/`i18n.ru`/`tokens` добавляются по мере появления — 1.5/1.6; тест писать расширяемым, но проверять ТОЛЬКО существующие оси — не падать на ещё не созданных файлах и не «молча пропускать».)

## Tasks / Subtasks

- [x] **Task 1 — Наполнить `registry/values` (AC1)**
  - [x] Решить формат файлов registry: **рекомендация — JSON** (`encoding/json`, stdlib, без новой зависимости; согласуется с i18n/`tokens.json`). Архитектура допускает «YAML+JSON». **Если выбран YAML — это новая прямая зависимость → требует одобрения владельца (HALT-условие dev-story).** Зафиксировать решение в `doc.go`/Change Log.
  - [x] `registry/values/honest_states.{json|yaml}`: списки `value_state` (11 членов) и `flag_state` (4 члена) — точное написание из AC1 (lower_snake). Каждый член — стабильный ключ.
  - [x] `registry/values/glossary-kk.{json|yaml}` и `glossary-ru.{json|yaml}` (или один файл `key→{kk,ru}`): метка/короткое пояснение для КАЖДОГО `value_state` и `flag_state`. KZ — локаль по умолчанию. Суффикс языка — только `_kk`/`_ru`, **никогда `_kz`** (`kz` = страна, не язык — сторож архитектуры).
  - [x] `registry/values/taboo_lexicon.{json|yaml}`: корни/стемы запретных терминов в обеих локалях (RU: нарушение, коррупция, виновен, …; KZ: бұзушылық, сыбайлас жемқорлық, кінәлі, …) — хранить корни (`нарушен`, `коррупц`, `виновн`), чтобы морфология ловилась без полного стеммера. Каждый термин — нормализованная форма (NFKC, lower).
  - [x] Заменить `.gitkeep` в `registry/values/` на реальные файлы (НЕ удалять каталог).
- [x] **Task 2 — Go-загрузчик registry + типизированные enum (AC1, AC4)**
  - [x] Создать пакет `server/internal/registry` (сейчас ОТСУТСТВУЕТ): рантайм-загрузчик, читает `registry/values/*` при старте; путь к корню registry — через config (по образцу относительного пути к `openapi.yaml` в тестах, см. ниже) или env/флаг.
  - [x] Закрытые типы-enum: `type ValueState string`, `type FlagState string` + консты для всех 11+4 членов + срез `AllValueStates()/AllFlagStates()` для тестов полноты. Это **рукописные** типы (не codegen); равенство с registry-файлом гарантирует перекрёстный тест (AC4), а не генерация.
  - [x] Загрузчик валидирует: файл существует, JSON/YAML парсится, множество в файле == множество Go-констант (иначе honest fail на старте). Экспортировать `Glossary` (по локали) и `TabooLexicon`.
- [x] **Task 3 — Подключить типизированный `value_state` к wire (AC4, без регрессий)**
  - [x] Заменить `State string` на `State registry.ValueState` в `server/internal/httpapi/field.go` (`Field[T]`); `okField`→`ValueStateOK`, `noData`→`ValueStateNoData`. JSON-тег и форма провода НЕ меняются (строка на проводе).
  - [x] Расширить `docs/api-contracts/openapi.yaml` `StringField.state.enum` с `[ok, no_data]` до полного списка 11 `value_state`; добавить схему/enum для `flag_state` (задел карточки флага). Обновить комментарий «расширяется из registry в Story 1.4» → «синхронно с registry (AC4)».
  - [x] Прогнать существующий `server/internal/httpapi/contracts_test.go` (kin-openapi) — ДОЛЖЕН остаться зелёным (регрессия 1.3). Тело контракта на проводе не меняется.
- [x] **Task 4 — Каркас `render()` — закрытый union + обязательный default (AC2)**
  - [x] В `server/internal/render/` (сейчас `doc.go`-заглушка): чистая функция `Render(flagID string, evidence Evidence, params MethodologyParams, locale registry.Locale) string`. `evidence`/`params` — **минимальные типы-задел** (полная иммутабельная модель `methodology_params` — Story 4.1; per-flag шаблоны — Story 4.x; здесь только каркас).
  - [x] `switch` по `FlagState` (и helper по `ValueState`) с **обязательной `default`**-веткой → glossary-ключ «неизвестно, см. методику» (никогда `""`, никогда `panic`). Рамка вывода — нейтральная («сигнал, требующий проверки» / «тексеруді талап ететін сигнал») из glossary.
  - [x] **Числовые литералы в шаблонах запрещены** — любые пороги через плейсхолдеры `{...}` из `params` (в каркасе можно без чисел; запрет зафиксировать комментарием/линт-заделом). `render()` — чистая, БЕЗ побочных эффектов (никакого outbox/IO; адаптеры OG/bot — Epic 5/7).
  - [x] `type Locale string` (`KK`/`RU`, KK по умолчанию).
- [x] **Task 5 — Наполнить `ci-registry.yml`: doc-нейтральность + перекрёстный тест + generated==regenerated (AC3, AC4)**
  - [x] Расширить `paths` триггера в `.github/workflows/ci-registry.yml` с `registry/**` до `registry/**`, `server/**`, `web/**`, и сам workflow.
  - [x] Шаг **doc-нейтральности**: Go-тест/CLI сканирует glossary + выводы `render()` для всех состояний × обеих локалей против `taboo_lexicon`; матчер: NFKC → casefold → нормализация гомоглифов → совпадение по корню. Любое совпадение → exit≠0 (red, блок merge).
  - [x] Шаг **перекрёстного теста** (AC4): равенство `value_state` в registry ↔ openapi.yaml ↔ Go-консты (расширяемо под будущие оси i18n/tokens — проверять только существующие).
  - [x] Шаг **generated==regenerated** (перенесён из `deferred-work.md`): `make gen-sqlc` + `make gen-web`, затем `git diff --exit-code` по сгенерированным файлам — дрейф sqlc/миграций/OpenAPI↔TS красный.
  - [x] Добавить make-таргет (напр. `make check-registry` / `gen-registry`-нет — registry читается в рантайме) для локального прогона стражей.
- [x] **Task 6 — Тесты и финализация**
  - [x] Юнит: загрузчик registry (happy + отсутствующий файл + рассинхрон множеств → honest fail).
  - [x] **Property-тест полноты** (AC2): для КАЖДОГО члена `AllValueStates()`/`AllFlagStates()` `Render` возвращает непустой текст; синтетический «неизвестный» член → попадает в default-ветку (не паника, не пусто).
  - [x] Тест taboo-матчера: ловит склонения/множественное число RU («нарушение/нарушения») и KZ-формы («бұзушылық/бұзушылықтар»); НЕ ложно-срабатывает на нейтральных словах; гомоглиф лат-`a`/кир-`а`.
  - [x] Перекрёстный тест (AC4) — зелёный на согласованных, красный на искусственном рассинхроне.
  - [x] `cd server && go build ./... && go vet ./... && go test ./...` зелёные; `gofmt` чисто; обновить File List, Change Log, Completion Notes.

### Review Findings

_Code review 2026-06-22 (Blind Hunter + Edge Case Hunter + Acceptance Auditor). Триаж: 1 decision-needed, 4 patch, 1 defer, 8 dismissed. Ложные находки отброшены: `kin-openapi` якобы новая зависимость (уже в `go.mod`, используется `contracts_test.go`); `gen-sqlc` якобы no-op (миграции `0001/0002` существуют); FlagField orphan и неиспользуемые `flagID/params` — намеренный задел; cross-axis транзитивность — математически корректна._

- [x] [Review][Decision] NFKC-нормализация аппроксимирована (AC3) — матчер taboo использует `ToLower` + 12-символьную таблицу гомоглифов + матч по корню вместо полного NFKC (AC3 и architecture называют «NFKC»). Сделано осознанно, чтобы не вводить `golang.org/x/text` (новая зависимость → требует одобрения владельца). Гомоглиф-фолд неполный и однонаправленный (лат→кир); intra-word разделители/zero-width не ловятся. Риск низкий: проза АВТОРСКАЯ (не внешний ввод). Решение владельца: (a) принять аппроксимацию, зафиксировав against AC3; (b) добавить `golang.org/x/text` и реализовать настоящий NFKC [server/internal/registry/neutrality.go]
- [x] [Review][Patch] `Load` не сверяет полноту glossary — известное состояние без glossary-ключа грузится БЕЗ ошибки и молча рендерится как «неизвестно» (маскирует пропущенный перевод). Контракт «честно падает на старте» (AC1) требует honest-fail. Сейчас ловится лишь транзитивно тестами пакета render [server/internal/registry/registry.go:Load]
- [x] [Review][Patch] CI `generated==regenerated` не ловит НОВЫЕ untracked сгенерированные файлы — `git diff --exit-code` видит только tracked; заменить на `git status --porcelain` по пути [.github/workflows/ci-registry.yml]
- [x] [Review][Patch] `TestRenderValueState_AllKnown` объединяет 3 проверки в одно `||`-условие (слабая диагностика) — разнести как в `TestRenderFlagState_AllKnown` [server/internal/render/render_test.go]
- [x] [Review][Patch] `TestNeutrality_RenderOutputs_NoTaboo` прогоняет составной `Render` только для `FlagRaised` — пройти все `flag_state` [server/internal/render/render_test.go]
- [x] [Review][Defer] Матчер нейтральности однонаправленный (нет проверки ПРИСУТСТВИЯ нейтральной рамки) [server/internal/registry/neutrality.go] — deferred, pre-existing scope: активируется в Epic 4 (per-flag шаблоны прозы); сейчас рамка структурно гарантирована в `Render`.

**Разрешение (2026-06-22):**
- **Decision (NFKC):** владелец принял аппроксимацию — без новой зависимости `golang.org/x/text`; зафиксировано against AC3 (риск низкий: проза авторская). Кода не менял.
- **P1:** добавлен `checkGlossaryComplete` в `Load` (honest-fail на отсутствующей метке известного состояния) + `TestLoad_GlossaryIncomplete_HonestError`.
- **P2:** CI-страж `generated==regenerated` переведён на `git status --porcelain` (ловит и untracked).
- **P3:** проверки в `TestRenderValueState_AllKnown` разнесены (точная диагностика).
- **P4:** `TestNeutrality_RenderOutputs_NoTaboo` прогоняет составной `Render` по всем `flag_state`.
- **Defer:** двунаправленный матчер → Epic 4 (записано в `deferred-work.md`).
- Проверено: `go build`/`go vet`/`gofmt`/`go test ./...` зелёные; `make lint`/`make test` OK; codegen идемпотентен.

## Dev Notes

### Контекст истории (зачем backbone-контракт сейчас)

Это четвёртая история Эпика 1 (фундамент). После каркаса (1.1), миграции `contracts`+sqlc (1.2) и
контракт-эндпоинта с честным конвертом `{value,state}` (1.3) — здесь ставится **единый источник
истины** для честных состояний и нейтральной прозы. Несущая идея архитектуры: **нейтральность и
честность обеспечиваются СТРУКТУРОЙ, а не дисциплиной** — закрытый union с обязательной default-веткой
+ CI-страж taboo делают нарушение гардрейлов технически невозможным, а не «не рекомендованным».
[Source: _bmad-output/planning-artifacts/epics.md#Story-1.4 (строки 898–916)]
[Source: _bmad-output/planning-artifacts/architecture.md — AR-12 (registry), AR-14 (render), AR-16 (двухосевой enum)]

### Что переиспользовать (НЕ изобретать заново) — всё проверено в коде

- **Каркас `registry/` СУЩЕСТВУЕТ, но спит** — три каталога с `.gitkeep`, чьи шапки называют скоуп 1.4:
  `registry/values/.gitkeep` («methodology_params, honest_states, glossary{kk,ru}, taboo_lexicon… наполняется Story 1.4»),
  `registry/rules/.gitkeep` (naming.yaml), `registry/methodology/.gitkeep` (шаблоны флага — Story 4.x).
  **Заменять `.gitkeep` на реальные файлы**, каталоги не пересоздавать. [Source: registry/values/.gitkeep, registry/rules/.gitkeep]
- **Честный конверт уже есть и ждёт enum** — `server/internal/httpapi/field.go:10-19`:
  ```go
  type Field[T any] struct {
      Value *T     `json:"value"`
      State string `json:"state"`   // ← заменить на registry.ValueState
  }
  func okField[T any](v T) Field[T] { return Field[T]{Value: &v, State: "ok"} }
  func noData[T any]() Field[T]     { return Field[T]{State: "no_data"} }
  ```
  Комментарий прямо говорит: «Полный value_state-enum … приходит из registry в Story 1.4; здесь — ok/no_data».
  Это файл UPDATE — расширить, не ломая wire-форму (строка на проводе). [Source: server/internal/httpapi/field.go:10-19]
- **OpenAPI-арбитр ждёт расширения** — `docs/api-contracts/openapi.yaml:54-64`: `StringField.state` →
  `enum: [ok, no_data] # расширяется из registry в Story 1.4`. Расширить до 11 `value_state`; добавить `flag_state`.
  [Source: docs/api-contracts/openapi.yaml:54-64, :8]
- **Паттерн закрытого enum + честный ответ** — `server/internal/apierr/codes.go:10-18`: закрытый
  `type Code string` (UPPER_SNAKE) + `Write()`; зеркалится в OpenAPI (`openapi.yaml:108`). **Скопировать стиль**
  для `ValueState`/`FlagState` (но lower_snake — это значения данных, не коды ошибок). [Source: server/internal/apierr/codes.go]
- **Прецедент закрытого state-enum с честным insufficient** — `stage0-audit/verdict.go:25-30`
  (`stateOK`/`stateInsufficient`/`stateNoData`). **ТОЛЬКО как образец** — это другой Go-модуль
  (`ashyqqala/stage0-audit`), импортировать из `server/` НЕЛЬЗЯ. [Source: stage0-audit/verdict.go:25-30]
- **`ci-registry.yml` — placeholder, который 1.4 наполняет** — `.github/workflows/ci-registry.yml:1-23`:
  paths только `registry/**`, тело — echo. Шапка: «Перекрёстный тест (~7 осей: + server/**, web/**) включается
  в Story 1.4 … doc-нейтральность (taboo RU/KZ), generated==regenerated». [Source: .github/workflows/ci-registry.yml]
- **Образец Go-теста против файла-артефакта** — `server/internal/httpapi/contracts_test.go`: DB-free,
  таблично, грузит `openapi.yaml` относительным путём `../../../docs/api-contracts/openapi.yaml` через
  `openapi3.NewLoader().LoadFromFile(...)` (kin-openapi уже в go.mod, `getkin/kin-openapi v0.140.0`),
  сообщения об ошибках по-русски. **Перекрёстный тест (AC4) писать в этом стиле** (относительные пути к
  repo-root `registry/values/*` и `docs/api-contracts/openapi.yaml`). [Source: server/internal/httpapi/contracts_test.go]

### Двухосевой enum — точное написание (источник истины, не передеривать)

`value_state` (почему нет ЗНАЧЕНИЯ, 11): `ok` · `no_data` · `insufficient_sample` · `not_comparable` ·
`stale` · `geocode_pending` · `geocode_failed` · `source_conflict` · `redacted` · `not_applicable` · `error`.
`flag_state` (почему нет/есть ФЛАГА, 4): `raised` · `not_raised` · `insufficient_data` · `not_published`.
Оси **независимы**: `value_state=ok` может сочетаться с `flag_state=insufficient_data` и т.п.
**Честные состояния — НЕ ошибки**: всегда HTTP 200 + типизированное состояние; 4xx/5xx только для
технических сбоев (`apierr`). Никогда не отдавать 404 на `insufficient_sample`.
[Source: _bmad-output/planning-artifacts/architecture.md AR-16 (~строки 546–553); epics.md:907]

### Соблюдение архитектуры (guardrails)

- **registry = единый позвонок, человек правит ТОЛЬКО здесь** (AR-12). Имена/enum из registry;
  «рождение имени вне registry = fail». registry/values **читается в рантайме** — НЕ генерировать
  Go-код из registry (генераторов ровно два: `openapi-typescript` и tokens-codegen — оба не про registry-values).
  [Source: architecture.md AR-12 (~636–642, 698–703, 785–795)]
- **`render()` — единственная точка прозы** (AR-14): нет API «написать текст флага» в обход render.
  Чистая функция → один слой → адаптеры web/OG/Telegram (Epic 5/7) не переписывают смысл. Закрытый
  union + обязательная default. [Source: architecture.md AR-14 (~590–593, 731, 776)]
- **Нейтральность** (FR-12): каждый флаг — «сигнал, требующий проверки» / «тексеруді талап ететін сигнал»;
  запрещены «нарушение/коррупция/виновен» (RU) и «бұзушылық/сыбайлас жемқорлық/кінәлі» (KZ).
  [Source: prd.md:312, :172-175 (FR-12); epics.md UX-DR27; docs/AshyqQala_MVP_data_model_and_flags_v1.md:5]
- **Честность над домыслом** (FR-10): отсутствующее → `no_data`/«нет данных»; малая выборка →
  `insufficient_sample`/«недостаточно сопоставимых данных»; никогда `0`/`—`/выдуманное; не интерполировать.
  [Source: prd.md:329, :160-162 (FR-10)]
- **Без `_kz` как языка**: суффикс языка только `_kk`/`_ru`. **Без числовых литералов** в шаблонах прозы.
  [Source: architecture.md (~594–595, 641); CLAUDE.md гардрейлы]

### Зависимость YAML vs JSON (решение в Task 1 — важно для dev-агента)

`server/go.mod` НЕ содержит прямой YAML-библиотеки (есть лишь indirect `go.yaml.in/yaml/v2` через
kin-openapi — на него опираться нельзя). Архитектура допускает «YAML+JSON». **Рекомендация: registry/values
как JSON** — `encoding/json` (stdlib), без новой зависимости, и формат совпадает с i18n/`tokens.json` (оси
перекрёстного теста). Если владелец предпочитает YAML — это **новая прямая зависимость**, которая по правилу
dev-story требует одобрения (HALT-условие «new dependencies require user approval»). Не добавлять YAML-либу молча.

### Границы скоупа (НЕ делать в этой истории — предотвратить scope creep)

- **`methodology_params` (иммутабельный движок)** — Story 4.1. Здесь `render()` принимает `params` как
  **минимальный тип-задел**, без версионирования/иммутабельности.
- **Per-flag шаблоны прозы** (`registry/methodology/`, citizen/journalist) — Story 4.x. Здесь только каркас
  `render()` + default-ветка, без текстов конкретных флагов.
- **Сами 4 флага** (FR-19…FR-23) — Epic 4.
- **i18n web-локали** (`web/.../kk.json`, `ru.json`) и **резолвер `lang`** — Story 1.6 (`server/internal/lang`
  сейчас заглушка). Перекрёстный тест (AC4) НЕ должен падать на ещё не созданных осях — проверять только
  существующие (registry↔OpenAPI↔Go), оставив хук для i18n/tokens.
- **Дизайн-токены** (`tokens.json`/`gen-tokens`) — Story 1.5.
- **`registry/rules/naming.yaml`** — упомянут в `.gitkeep` как 1.4, но НЕ в AC1–AC3. Опционально/вторично:
  добавить, если дёшево; не блокировать историю на нём.
- **Не трогать** `stage0-audit/` (другой модуль) и прикладной MVP-код вне перечисленных файлов.

### Файлы и куда писать результат

- **Новое:** `registry/values/{honest_states,glossary-kk,glossary-ru,taboo_lexicon}.{json|yaml}`;
  `server/internal/registry/*.go` (загрузчик + типы-enum + тесты); `server/internal/render/render.go`(+тест);
  тест(ы) перекрёстной согласованности и taboo-нейтральности (в `server/internal/registry` или отдельный пакет).
- **UPDATE:** `server/internal/httpapi/field.go` (типизировать `State`); `docs/api-contracts/openapi.yaml`
  (расширить enum); `.github/workflows/ci-registry.yml` (наполнить стражи + paths); `Makefile` (таргет проверки);
  возможно `server/go.mod` (только если выбран YAML — с одобрения).
- **Сохранить зелёным:** `server/internal/httpapi/contracts_test.go` (регрессия wire-формата 1.3).

### Тестирование / проверка готовности

- Стек тестов: Go `go test ./...` (DB-free; интеграционные — за `//go:build integration`, здесь не нужны),
  golden/property где уместно; CI — `ci-server.yml` (gofmt/vet/build/test/golangci-lint v2.5.0) + `ci-registry.yml`
  (этой истории). [Source: Makefile, .github/workflows/ci-server.yml, ci-registry.yml]
- DoD: AC1–AC4 закрыты; default-ветка `render()` доказана property-тестом; taboo-матчер ловит морфологию RU+KZ
  и красит CI; перекрёстный enum-тест зелёный/красный корректно; `contracts_test` без регрессий; отсутствующие
  данные честно `no_data`, не выдуманы; решение YAML/JSON зафиксировано.

### Previous Story Intelligence (1.3, 1.2, 1.1)

- **1.3 (done):** ввела `Field{value,state}` (`field.go`), OpenAPI-арбитр (`openapi.yaml`, валидация
  kin-openapi в `contracts_test.go`), `apierr` закрытый каталог. Wire: деньги строкой, даты ISO8601 `Z`,
  `{value:null,state:"no_data"}` — НИКОГДА «0». 1.4 расширяет именно эти артефакты, не ломая их.
- **1.2 (done):** миграция `contracts` + sqlc (per-domain `.sql`, анти-churn; `gen.Contract`, `gen.GetContractByID`).
  Codegen sqlc через Docker (`make gen-sqlc`), Go 1.25, модуль `ashyqqala/server`.
- **1.1 (done):** монорепо-скелет (все `internal/*` как `doc.go`-заглушки), compose-db, Makefile (таргеты
  `gen`/`gen-sqlc`/`gen-web`/`test`/`lint`), три CI-workflow (`ci-server`/`ci-registry`/`ci-web`).
- **`deferred-work.md`:** страж **generated==regenerated** явно отложен В ЭТУ историю (`ci-registry.yml`) —
  ловить дрейф sqlc/миграций и OpenAPI↔TS. Включить в Task 5. [Source: _bmad-output/implementation-artifacts/deferred-work.md]

### Git-контекст

Свежие коммиты — план спринта и истории 0.1/1.x (`bba6c12` 1.3, `cedd1ce` 1.2). Применимые паттерны —
в коде `server/` (см. «Что переиспользовать»), а не в git-истории. Прикладной MVP-код вне `server/` ещё не создан.

### Внешние знания / web-research

Не требуется: история не вводит новых runtime-библиотек (всё на stdlib + уже подключённый kin-openapi для
тестов; YAML — только с одобрения). Морфологический матчер taboo реализуется **без** внешнего стеммера —
нормализация NFKC + casefold + совпадение по корню/стему (корни хранятся в `taboo_lexicon`); полноценный
KZ-морфоанализатор избыточен для гардрейла. Гомоглифы — таблица лат/кир-двойников.

### Project Structure Notes

- Конфликтов с целевым деревом нет: `registry/` (repo-root), `server/internal/{registry,render}`,
  `docs/api-contracts/openapi.yaml`, `.github/workflows/ci-registry.yml` — все на штатных местах архитектуры.
  [Source: architecture.md — целевое дерево (~636–642, 698–703, 731, 694–696)]
- `server/internal/registry` — НОВЫЙ пакет (сейчас отсутствует); `server/internal/render` — наполнение заглушки.
- Перекрёстный тест растёт по осям (1.5 tokens, 1.6 i18n) — писать расширяемым, но честным к текущему составу.

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-1.4 (строки 898–916); AR-12/AR-14/AR-16, UX-DR27]
- [Source: _bmad-output/planning-artifacts/architecture.md — AR-12 registry (~636–642, 698–703, 785–795), AR-14 render (~590–593, 731, 776), AR-16 двухосевой enum (~546–553), перекрёстный тест ~7 осей (~644–650), ci-registry (~694–696)]
- [Source: _bmad-output/planning-artifacts/prds/prd-AshyqQala.kz-2026-06-17/prd.md — FR-10 (:160-162), FR-12 (:172-175), нейтральность (:312), честность (:329), NFR-6 i18n KZ-default (:305-306)]
- [Source: docs/AshyqQala_MVP_data_model_and_flags_v1.md — honest_states/value_state/flag_state (§2), methodology_params, evidence/methodology_version, нейтральность (:5)]
- [Source: server/internal/httpapi/field.go:10-19; contracts.go; contracts_test.go (паттерн kin-openapi)]
- [Source: docs/api-contracts/openapi.yaml:8, :54-64, :108]
- [Source: server/internal/apierr/codes.go:10-18 (паттерн закрытого enum)]
- [Source: server/internal/render/doc.go; server/internal/lang/doc.go (заглушки)]
- [Source: .github/workflows/ci-registry.yml (placeholder), ci-server.yml; Makefile (таргеты, версии тулов)]
- [Source: registry/values/.gitkeep, registry/rules/.gitkeep, registry/methodology/.gitkeep (скоуп-шапки)]
- [Source: _bmad-output/implementation-artifacts/deferred-work.md (generated==regenerated → 1.4)]
- [Source: stage0-audit/verdict.go:25-30 (прецедент закрытого state-enum; другой модуль, не импортировать)]
- [Source: CLAUDE.md — гардрейлы нейтральности/честности, registry single-source, Russian comments]

## Dev Agent Record

### Agent Model Used

claude-opus-4-8[1m] (Claude Opus 4.8, 1M context) — create-story (context engine).

### Debug Log References

- `cd server && go build ./...` → OK; `go vet ./...` → OK; `gofmt -l .` → пусто.
- `go test ./internal/registry/... ./internal/render/... ./internal/httpapi/...` → ok (включая регрессию wire-формата 1.3).
- `make test` (полный server) → ok; `make check-registry` → ok; `make lint` → OK.
- `make gen-web` → schema.gen.ts регенерён (value_state→11, добавлен FlagField); идемпотентен (md5 стабилен при повторе).
- `make gen-sqlc` (Docker sqlc 1.31.0) → без diff (`server/internal/store/gen` не менялся — миграции/запросы не трогал).

### Completion Notes List

**Контекст-инжиниринг (create-story):**
- Анализ проведён тремя параллельными исследовательскими агентами (architecture / существующий код server/ / PRD+модель данных) + личная верификация файлов UPDATE. Все цитаты сверены с реальными путями/строками.
- Предусловия 1.4 подтверждены существующими (1.1/1.2/1.3 done). Task 0-HALT не требовался.

**Реализация (dev-story):**
- **AC1** — `registry/values/{honest_states,glossary-kk,glossary-ru,taboo_lexicon}.json` (формат **JSON** — stdlib, без новой зависимости; решение зафиксировано). Пакет `server/internal/registry`: рантайм-загрузчик `Load(root)` + закрытые типы `ValueState`(11)/`FlagState`(4)/`Locale`(kk,ru). Загрузчик **честно падает** на старте при отсутствии файла или рассинхроне множества honest_states с Go-константами (`TestLoad_MissingDir_HonestError`, `TestLoad_Desync_HonestError`). registry читается в РАНТАЙМЕ (не codegen).
- **AC2** — каркас `render()` в `server/internal/render/`: `Render(flagID, evidence, params, locale)` + `RenderFlagState`/`RenderValueState` — **закрытый union с ОБЯЗАТЕЛЬНОЙ default-веткой** (неизвестный член → «неизвестно, см. методику» из glossary, никогда пусто/паника; работает и при `Reg==nil`). Чистая функция, без IO. Property-тесты полноты: каждый известный член → своя glossary-строка (не fallback); синтетический неизвестный → default.
- **AC3** — `.github/workflows/ci-registry.yml` наполнен: doc-нейтральность + перекрёстный тест + generated==regenerated, paths расширены на `server/**`/`web/**`/`docs/api-contracts/**`/`migrations/**`. Матчер taboo (`FindTaboo`): casefold + фолд гомоглифов (лат→кир) + матч по корню — ловит склонения/мн.ч. RU и KK-формы (`TestFindTaboo_Morphology`); вся glossary и весь вывод render свободны от taboo (`TestNeutrality_*`). **NFKC аппроксимирован** (без `golang.org/x/text` — чтобы не вводить новую зависимость; задокументировано в `neutrality.go`).
- **AC4** — перекрёстное равенство множеств `value_state`/`flag_state` в **registry ↔ OpenAPI ↔ Go-константы** (`TestCrossAxis_ValueState/FlagState`, через kin-openapi — уже в go.mod). OpenAPI `StringField.state.enum` расширен до 11; добавлена схема `FlagField` (flag_state). Тест расширяем под будущие оси i18n/tokens (1.5/1.6), проверяет только существующие.
- **Регрессия 1.3 сохранена:** `Field[T].State` стал `registry.ValueState`, но на проводе остаётся lower_snake-строкой — `httpapi/contracts_test.go` зелёный без изменений; `schema.gen.ts` регенерён под расширенный enum.
- **Гардрейлы соблюдены:** нейтральность (рамка «сигнал, требующий проверки»/«тексеруді талап ететін сигнал», CI-страж taboo) и честность (honest-fail вместо «пусто», value:null→честное состояние) обеспечены структурой.
- **Осознанно вне скоупа (не сделано):** `registry/rules/naming.yaml` (не в AC1–AC3; `.gitkeep` помечает на потом); иммутабельный движок `methodology_params` (Story 4.1); per-flag шаблоны прозы (4.x); оси i18n/tokens перекрёстного теста (1.5/1.6).

### File List

- `registry/values/honest_states.json` — **новый**. Двухосевой enum (источник истины, рантайм).
- `registry/values/glossary-ru.json`, `registry/values/glossary-kk.json` — **новые**. Нейтральные метки состояний + рамка + «неизвестно».
- `registry/values/taboo_lexicon.json` — **новый**. Корни запретного лексикона RU+KK.
- `registry/values/.gitkeep` — **удалён** (заменён реальными файлами).
- `server/internal/registry/registry.go` — **новый**. Рантайм-загрузчик + закрытые типы ValueState/FlagState/Locale + honest-fail.
- `server/internal/registry/neutrality.go` — **новый**. Нормализация (casefold+гомоглифы) + `FindTaboo` (матч по корню).
- `server/internal/registry/registry_test.go` — **новый**. Load/honest-fail, перекрёстный тест (AC4), doc-нейтральность glossary, морфология матчера.
- `server/internal/render/render.go` — **новый**. Каркас `render()`: закрытый union + обязательная default-ветка; чистая функция.
- `server/internal/render/render_test.go` — **новый**. Полнота union, default-ветка, нейтральность вывода.
- `server/internal/httpapi/field.go` — **изменён**. `State string` → `registry.ValueState` (wire не изменился).
- `docs/api-contracts/openapi.yaml` — **изменён**. `StringField.state.enum` → 11 value_state; добавлена схема `FlagField` (flag_state).
- `web/src/shared/api/schema.gen.ts` — **изменён (сгенерирован)**. Регенерён `make gen-web` под расширенный enum + FlagField.
- `.github/workflows/ci-registry.yml` — **изменён**. Наполнены сторожа: doc-нейтральность, перекрёстный тест, generated==regenerated; расширены paths.
- `Makefile` — **изменён**. Добавлен таргет `check-registry`.
- `_bmad-output/implementation-artifacts/1-4-...-render-каркас.md` — **изменён**. Frontmatter `baseline_commit`, чекбоксы, Dev Agent Record, File List, Change Log, Status.
- `_bmad-output/implementation-artifacts/sprint-status.yaml` — **изменён**. `1-4`: ready-for-dev → in-progress → review.

## Change Log

| Дата | Изменение |
|---|---|
| 2026-06-22 | create-story: контекст-инжиниринг (3 параллельных агента + верификация). Статус → ready-for-dev. |
| 2026-06-22 | dev-story: реализованы AC1–AC4. `registry/values/*.json` (JSON, без новой зависимости), пакет `server/internal/registry` (загрузчик + закрытые enum + honest-fail + taboo-матчер), каркас `server/internal/render` (закрытый union + обязательная default-ветка), типизирован `httpapi.Field.State`, расширен OpenAPI (11 value_state + FlagField) и регенерён `schema.gen.ts`, наполнен `ci-registry.yml` (нейтральность + перекрёстный тест + generated==regenerated), таргет `make check-registry`. Полный `go test ./...` + `make lint` зелёные; wire-регрессия 1.3 сохранена. Статус → review. |
| 2026-06-22 | code-review (3 adversarial-слоя): 1 decision-needed, 4 patch, 1 defer, 8 dismissed. Decision (NFKC) — принята аппроксимация (без новой зависимости). Применены 4 патча: полнота glossary в `Load` (+тест), CI-страж на `git status --porcelain`, разнесена диагностика теста value_state, прогон всех flag_state в neutrality-тесте. Defer (двунаправленный матчер) → `deferred-work.md` (Epic 4). `go test ./...`/`make lint` зелёные. Статус → done. |
