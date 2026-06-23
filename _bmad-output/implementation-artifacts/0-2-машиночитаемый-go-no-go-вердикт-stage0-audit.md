---
baseline_commit: 796bd548f2f033d33b2de3c6870b7e4a100eeca6
---

# Story 0.2: Машиночитаемый Go/No-Go-вердикт stage0-audit

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a **команда платформы AshyqQala.kz**,
I want **чтобы инструмент `stage0-audit/` отдавал машиночитаемый Go/No-Go-вердикт (`-format json`) и
писал версионируемый baseline-артефакт**,
so that **гейт №0 проверяется в CI программно (exit-код), а регрессии гео-покрытия доказуемы
артефактом-в-git, а не «решаются на совещании»**.

> **Тип истории:** enabler-story Эпика 0 (без FR; несёт AR-2 и gate для Epic 2/3). Это
> **чисто инженерная** история: добавляет машиночитаемый слой поверх уже работающих Шагов A–D.
> **🔑 КЛЮЧЕВОЕ ОТЛИЧИЕ ОТ 0.1:** эта история **НЕ заблокирована на живом `GOSZAKUP_TOKEN`.**
> Вся машинерия (JSON-вердикт, exit-код, артефакт, CI-контракт, юнит-тесты) **строится и
> проверяется на синтетических фикстурах** (`-source file`). Реальный прогон вердикта на живом
> токене с настоящими числами — это **Story 0.3**, не эта. Здесь мы делаем *детектор*; 0.3
> *нажимает кнопку*. Не повторяй HALT-на-токене из 0.1.

## Acceptance Criteria

**AC1 — `-format json` отдаёт машиночитаемый вердикт**
**Given** существующий инструмент `stage0-audit/`
**When** он запущен с `-format json`
**Then** на stdout выводится валидный JSON-объект с обязательными ключами верхнего уровня:
`oq1_volume`, `oq4_geo_coverage` (содержит **доверительный интервал** и **размер выборки**),
`oq6_sample`, `verdict` (∈ `go | go_with_fallback | no_go`), `methodology_version`.
**And** текстовый режим (без флага или `-format text`) работает по-прежнему — Шаги A–D и
печать таблицы не сломаны.

**AC2 — Гео-гейт даёт exit-код для CI**
**Given** порог гео `cfg.GeoGate = 0.70`
**When** `oq4_geo_coverage.coverage < 0.70` (или гео измерить не удалось — выборки нет)
**Then** `verdict ≠ go` **и** процесс завершается `exit 1` (пригодно для CI-хука «красный = блок merge»).
**And** при `coverage ≥ 0.70` → `verdict = go` и `exit 0`.

**AC3 — Версионируемый baseline-артефакт**
**Given** завершённый прогон с `-format json`
**When** задан путь вывода артефакта (`-verdict-out`)
**Then** тот же JSON записывается в файл с именем по схеме `stage0-verdict-YYYYMMDD.json`
(детерминированный порядок ключей — чистый git-diff), пригодный для коммита как baseline под
`docs/ops/`.

## Tasks / Subtasks

- [x] **Task 1 — Структура `Verdict` + чистая функция вывода вердикта (AC1, AC2)** — ✅ `verdict.go`
  - [x] `const MethodologyVersion = "stage0-1.0"` в новом `verdict.go` (+ пины порогов через комментарий). [Source: architecture.md:108,124–127]
  - [x] Тип `Verdict` (+ `OQ1Volume`/`OQ4GeoCoverage`/`OQ6Sample`) с `json:`-тегами snake_case; топ-уровень — **struct** (стабильный порядок ключей подтверждён тестом `deterministicKeys`).
  - [x] `wilsonInterval(successes, n int, z float64)` (Wilson 95%, `math` only); `n ≤ 0 → [0,1]` (честная неопределённость, не `[0,0]`). Проверено: 124/200 → CI≈[0.551,0.684].
  - [x] **Чистая** `deriveVerdict(r Report, cfg Config, opts verdictOpts) (Verdict, int)` — без `os.Exit`; читает готовые поля `Report`, ничего не пересчитывает.
  - [x] Гардрейл честности: `geocodeAttempted < 3` (n∈{0,1,2}) → `state=insufficient_sample`, `coverage=null`, `no_go`/exit 1. НЕ 0% за измерение. Проверено тестом `honestyNoFabricated0`. [Source: prd.md:328–330; architecture.md:160]
- [x] **Task 2 — Флаг `-format` и ветка вывода (AC1)** — ✅ `main.go`
  - [x] Флаг `-format` (`text|json`, дефолт `text`) + валидация неизвестного значения (exit 2).
  - [x] В `main()` после `runAudit`: `json` → `deriveVerdict` + `json.MarshalIndent` + trailing `\n` на stdout; иначе `rep.print(cfg)`.
  - [x] `runAudit`/`Report.print` по сути не тронуты (только `[warn]` → stderr, необходимо для чистого JSON на stdout). Вердикт переиспользует поля `Report`.
- [x] **Task 3 — Exit-код для CI (AC2)** — ✅
  - [x] `os.Exit(code)` после вывода и записи артефакта; `go/go_with_fallback → 0`, `no_go → 1`.
  - [x] **Q5 принят:** `json` гейтит **всегда**; `text` по умолчанию exit **не меняет** (обратная совместимость), включается явным `-gate`. Проверено: text→exit0, text+`-gate`→exit1, json→exit1.
- [x] **Task 4 — Baseline-артефакт (AC3)** — ✅
  - [x] Флаг `-verdict-out` (дефолт `""` = только stdout).
  - [x] Имя `stage0-verdict-YYYYMMDD.json` + флаг `-verdict-date` (override); каталог → дописать имя, `.json` → как есть (`resolveVerdictPath`).
  - [x] Записываемый JSON **байт-в-байт** идентичен stdout (`diff` пустой).
  - [x] Пример-артефакт на фикстурах закоммичен: `docs/ops/stage0-verdict-20260620.json` (`data_source: file:./data`). Живой baseline — Story 0.3.
- [x] **Task 5 — Юнит-тесты (первые в модуле) (AC1, AC2)** — ✅ `verdict_test.go`, 11 кейсов
  - [x] `deriveVerdict` table-driven: 0.85→go/0; **граница 0.70→go (≥)**; 0.55→no_go/1; attempted=0→no_go/1+insufficient; attempted=2→insufficient (n∈{0,1,2}); fallback@0.55→go_with_fallback/0; fallback не спасает insufficient. + проверка 5 обязательных ключей в JSON.
  - [x] `wilsonInterval`: `n=0 → [0,1]`; 124/200 → CI≈[0.551,0.684] с допуском; `0≤lo≤p≤hi≤1`.
  - [x] `go build ./...`, `go vet ./...`, `go test ./...`, `gofmt -l` — все зелёные.
- [x] **Task 6 — CI-контракт и финализация** — ✅
  - [x] README: раздел «Машиночитаемый вердикт (`-format json`) и CI-гейт» — контракт 5 ключей, таблица exit-кодов, CI-хук `… || echo "No-Go: блок merge"`, флаги вердикта.
  - [x] `.github/workflows/*.yml` **НЕ создан** (CI монорепо — Epic 1+, Story 1.1); здесь только exit-код + артефакт + документированный контракт.
  - [x] Обновлены File List / Change Log; open-questions владельцу зафиксированы в Dev Agent Record (Q1–Q6 приняты по рекомендациям).

## Dev Notes

### Контекст истории (зачем именно вердикт)

Эпик 0 снимает ведущий риск проекта (по Минто): **данных по Астане (дороги+вода) физически
достаточно и они геокодируемы** (OQ-1 × OQ-4 × OQ-6 — один риск в трёх проекциях). Детектор —
`stage0-audit/` — уже считает Шаги A–D и печатает человекочитаемую таблицу Go/No-Go. Story 0.1
открыла официальный канал (токен — в процессе). **Эта история превращает человекочитаемую таблицу
в машинный контракт:** один JSON, один exit-код, один версионируемый артефакт — чтобы гейт стал
*тестируемым в CI*, а не устной договорённостью.
[Source: architecture.md:162–177 «Гейт №0»; epics.md:539–548 «Epic 0»; epics.md:659–678 «Story 0.2»]

**Точная цель из Epic 0:** «машиночитаемый Go/No-Go-вердикт stage0-audit на живом токене
(`-format json`: `oq1_volume`, `oq4_geo_coverage` с доверительным интервалом и размером выборки,
`oq6_sample`, `verdict: go|no-go`), закоммиченный baseline-артефакт (`stage0-verdict-YYYYMMDD.json`),
CI-хук (`geo_coverage < 0.70 → No-Go блокирует merge в Epic 2/3`)».
[Source: epics.md:540–548]

### Что переиспользовать (НЕ изобретать заново)

**Инструмент существует — `stage0-audit/` (Go 1.25, ТОЛЬКО stdlib, модуль `ashyqqala/stage0-audit`).**
Эта история **добавляет** JSON-слой, **не переписывает** аудит. Категорически:

- **НЕ пересчитывать метрики.** Все числа уже в структуре `Report` (`main.go:184–201`) после
  `runAudit`. `deriveVerdict` только **читает** её поля и оформляет в `Verdict`. [Source: stage0-audit/main.go:184–201]
- **НЕ ломать текстовый вывод.** `Report.print(cfg)` (`main.go:401–489`) и таблица Go/No-Go
  (`main.go:473–488`) остаются как есть — это эталон, с которым JSON должен совпадать по числам.
- **Порог гео уже есть:** `cfg.GeoGate = 0.70` (`main.go:54`, объявлен `config.go:16`). Использовать
  его, не хардкодить «0.70» в новом коде. [Source: stage0-audit/config.go:16; main.go:54]
- **`encoding/json` уже импортируется** в модуле (`models.go`, `client.go`, `source.go` и др.) —
  но только для **декодирования**. Для вывода добавить теги и `json.MarshalIndent`. Структура `Report`
  тегов не имеет (поля неэкспортируемые, lowercase) — поэтому маршалить надо **новый** тип `Verdict`
  с экспортируемыми полями и тегами, а НЕ `Report` напрямую. [Source: code-map agent — Report без json-тегов, поля unexported]
- **Имя источника** для поля `data_source`: `src.Name()` (`source.go` — `ows` / `file:<dir>` /
  `scrape:goszakup-public`), печатается в `main.go:87`. [Source: stage0-audit/source.go:23,48,99; main.go:87]
- **Дата окна/режим запуска** уже в `cfg` (`WindowMonths`, `GeocoderKind`) — переиспользовать для
  контекстных полей артефакта.

### Откуда берутся числа (Report → Verdict)

Маппинг полей `Report` (`main.go:184–201`) на ключи вердикта — **не вычислять заново, читать готовое**:

| Ключ вердикта | Поле(я) `Report` | Как считается сейчас | Citation |
|---|---|---|---|
| `oq1_volume.contracts_in_window` | `contractsInWindow` | Шаг A: инкремент если `sign_date ≥ cutoff` | main.go:185, 244–246 |
| `oq1_volume.sum_completeness` | `sumPresent / contractsTotal` | Шаг A, `pct()` | main.go:404 |
| `oq1_volume.supplier_bin_completeness` | `supplierPresent / contractsTotal` | Шаг A | main.go:405 |
| `oq4_geo_coverage.coverage` | `geocodeSuccess / geocodeAttempted` | Шаг B+, реальный геокодер | main.go:419 |
| `oq4_geo_coverage.sample_size` | `geocodeAttempted` | Шаг B+ счётчик попыток | main.go:188, 299 |
| `oq4_geo_coverage.successes/errors` | `geocodeSuccess`, `geocodeErrors` | Шаг B+ | main.go:305, 307 |
| `oq4_geo_coverage.ci_lower/ci_upper` | **новое:** `wilsonInterval(success, attempted, 1.96)` | — | новый код |
| `oq6_sample.groups_total/sufficient` | `medianGroups` + порог `cfg.MinSample` | Шаг D: групп с `n ≥ MinSample` | main.go:195, 437–451 |
| `oq6_sample.anno_matched` | `annoMatched` | Шаг D join контракт↔лот | main.go:194, 327 |

> ⚠️ **Гео = Шаг B+ (реальный геокодер), НЕ Шаг B (прокси).** `oq4_geo_coverage` строится из
> `geocode*`-полей (Шаг B+), а **не** из `geoWithMarker/geoEligible` (прокси по адресному маркеру в
> названии). Прокси (`main.go:412`, `geoProxy` в таблице `main.go:474,478`) — намеренно НЕ финальный
> гейт; не использовать его для вердикта. [Source: architecture.md:488 «гео-метрика = Шаг B+, НЕ Шаг B»]

### Контракт JSON (рекомендуемая форма — обязательны 5 ключей AC1)

Обязательны (AC1): `oq1_volume`, `oq4_geo_coverage` (с CI + размером выборки), `oq6_sample`,
`verdict`, `methodology_version`. Остальные поля — рекомендуемый контекст воспроизводимости
(опциональны, но полезны для baseline-диффа). Все имена — **snake_case**; enum/состояния —
**lower_snake строки**. [Source: architecture.md:523–524, 569]

```json
{
  "methodology_version": "stage0-1.0",
  "generated_at": "2026-06-20T11:30:00+05:00",
  "data_source": "file:./data",
  "window_months": 24,
  "geo_gate_threshold": 0.70,
  "oq1_volume": {
    "contracts_in_window": 312,
    "contracts_total": 1000,
    "sum_completeness": 0.97,
    "supplier_bin_completeness": 0.99
  },
  "oq4_geo_coverage": {
    "coverage": 0.62,
    "sample_size": 200,
    "successes": 124,
    "errors": 3,
    "ci_lower": 0.552,
    "ci_upper": 0.684,
    "ci_method": "wilson_0.95",
    "state": "ok"
  },
  "oq6_sample": {
    "comparability_key": "direction × kato × 24mo",
    "min_sample": 5,
    "groups_total": 8,
    "groups_sufficient": 3,
    "anno_matched": 140,
    "state": "ok"
  },
  "verdict": "no_go",
  "verdict_reason": "oq4_geo_coverage 0.62 < gate 0.70"
}
```

- Поля состояний `state ∈ {ok, insufficient_sample, no_data}` — отражают честные состояния
  (двухосевой enum `value_state`, AR-16). [Source: epics.md:239–242 AR-16]
- `coverage` как доля `0.0–1.0` (не «62.0»); при недоступном замере — `null` + `state:
  insufficient_sample` (Go-указатель `*float64` или `omitempty`-обёртка — на усмотрение дева).
- Деньги/суммы в этой истории НЕ выводим (вердикт оперирует долями/счётчиками), так что правило
  «деньги — строкой» (architecture.md:557) здесь не применяется.

### Логика вердикта (deriveVerdict)

Детерминированная, читается как таблица; **гео — несущий гейт** (AC2):

| Условие (в порядке проверки) | verdict | exit |
|---|---|---|
| `oq4.state != ok` (геокодер не прогонялся / `attempted==0` / ниже минимума выборки) | `no_go` | 1 |
| `coverage ≥ 0.70` | `go` | 0 |
| `coverage < 0.70` **и** НЕ задан `-allow-fallback` | `no_go` | 1 |
| `coverage < 0.70` **и** задан `-allow-fallback` (см. Q1) | `go_with_fallback` | 0 |

- **Граница:** `≥ 0.70 → go` (строго: `coverage >= cfg.GeoGate`). [Source: runbook:34–49 «≥70% → Go»]
- **`go_with_fallback`** — это **«частичный Go»**: осознанное решение владельца идти с ручной
  гео-очередью Directus как *известной стоимостью*. Это **человеческое** решение (Story 0.4 фиксирует
  владельца + дату), поэтому в инструменте оно доступно **только через явный флаг-override**
  `-allow-fallback`, а НЕ выдаётся автоматически. **По умолчанию `geo < 0.70 → no_go + exit 1`** —
  это и есть контракт CI-хука «блокирует merge». [Source: epics.md:544 «прописанный исход частичный
  Go… с владельцем и датой»; epics.md:714 Story 0.4 AC]
- **Объём (oq1) и выборка (oq6)** — **репортятся**, но автоматический exit-гейт на них **не вешаем**:
  порог объёма — «≥ согласованного с PM» (не фиксированное число), а достаточность медиан — решение
  Story 0.3/0.4. Не хардкодить порог объёма. [Source: architecture.md:170 «N ≥ согласованного с PM»;
  runbook:18–31]
- `verdict_reason` — короткая человекочитаемая причина (для отладки CI-фейла).

### Соблюдение архитектуры (guardrails)

- **Честность над домыслом.** Недостаточная выборка → `insufficient_sample`/`недостаточно
  сопоставимых данных`; отсутствие данных → `no_data`/`нет данных`. Никогда не выдавать `0%` или
  выдуманное число за измерение. Это ограждение равноценно нейтральности формулировок.
  [Source: prd.md:328–330; prd.md:311–315]
- **Нейтральность.** Вердикт — технический сигнал гейта; никаких оценочных формулировок. (В этой
  истории нет пользовательской прозы — только машинный контракт, но `verdict_reason` держать
  фактологичным.)
- **methodology_version — иммутабельный пин формулы.** Менять при изменении порогов/логики; внешний
  пересчёт по тем же входам обязан дать тот же вердикт. [Source: architecture.md:108, 124–127]
- **Только stdlib.** Никаких внешних зависимостей: `go.mod` остаётся однострочным (`go 1.25`).
  Wilson CI — на `math`; JSON — `encoding/json`; дата — `time`. [Source: stage0-audit/go.mod; CLAUDE.md «dependency-free»]
- **Схемная устойчивость не затрагивается** этой историей (мы не добавляем новых полей API) — но
  не ломать `fieldCandidates`-логику `models.go`. [Source: stage0-audit/models.go; CLAUDE.md «VERIFY»]
- **Детерминированный артефакт.** Топ-уровень — struct (порядок полей фиксирован определением);
  map-поля (oq6 по группам, если разворачивать) — выводить через отсортированные ключи. Чистый
  git-diff между прогонами — требование к baseline. [Source: architecture.md:348 «хеш по key-sorted
  схеме, иначе мигает» — тот же принцип стабильности]

### Файлы и куда писать результат

- **Меняем (UPDATE):**
  - `stage0-audit/main.go` — флаги `-format`/`-verdict-out`/`-verdict-date`/`-allow-fallback`; ветка
    вывода и `os.Exit(code)` после `main.go:101–102`. [Source: stage0-audit/main.go:13–37, 101–103]
  - `stage0-audit/config.go` **или** новый `stage0-audit/verdict.go` — `const MethodologyVersion`.
  - `stage0-audit/README.md` — раздел про вердикт + CI-контракт.
- **Создаём (NEW):**
  - `stage0-audit/verdict.go` — тип `Verdict`, `deriveVerdict`, `wilsonInterval` (рекомендуется
    вынести из `main.go` для тестируемости).
  - `stage0-audit/verdict_test.go` — **первые юнит-тесты модуля.**
  - (опц.) `docs/ops/stage0-verdict-YYYYMMDD.json` — пример-артефакт на фикстурах (честный
    `data_source: file`). Авторитетный живой baseline — Story 0.3.
- **НЕ трогать:** `runAudit`/Шаги A–D-логику (кроме чтения полей); `source.go`/`client.go`/
  `geocoder.go`/`models.go`/`fixtures.go`/`scrape.go` (вердикт их только читает через `Report`);
  прикладной MVP-код (`server/`, `web/` — ещё не существует, появится в Epic 1).
- **Каталог `docs/ops/`** уже существует (создан Story 0.1, там `stage0-access.md`; Story 0.3
  допишет `stage0-gate.md`). [Source: 0-1-…access.md File List; architecture.md:708]

### Тестирование / проверка готовности

- **Проверки сборки (единственные в модуле + новые тесты):**
  `cd stage0-audit && go build ./... && go vet ./... && go test ./...` — все зелёные.
- **Воспроизводимый прогон на фикстурах (без токена!):**
  ```bash
  cd stage0-audit
  go run . -gen-fixtures -data-dir ./data -fixtures-n 300
  go run . -source file -format json                       # вердикт; gate без гео → no_go/exit1 (честно)
  echo "exit=$?"                                            # ожидаем 1 (гео не прогонялся)
  go run . -source file -format text                        # текст по-прежнему работает (регресс-проверка)
  ```
- **Путь `verdict=go` детерминированно** проверяется **юнит-тестом** `deriveVerdict` (подставить
  `Report{geocodeAttempted:200, geocodeSuccess:170}` → `go/exit0`), а НЕ через живой Nominatim
  (сеть/недетерминизм). Реальный гео-прогон ≥70% — Story 0.3.
- **DoD истории:** AC1–AC3 закрыты; `-format json` даёт валидный JSON с 5 обязательными ключами;
  `geo<0.70 → verdict≠go ∧ exit1`, `geo≥0.70 → go ∧ exit0`; артефакт пишется с детерминированным
  порядком ключей; текстовый режим не сломан; `go build/vet/test` зелёные; CI-контракт задокументирован.

### Внешние знания / web-research (осознанно ограничено)

История **не вводит новых библиотек** — только stdlib (`encoding/json`, `math`, `time`, `flag`,
`os`). Wilson score interval — стандартная формула для доли (биномиальная пропорция), считается
вручную, не требует пакета:

> Для `p̂ = s/n`, `z = 1.96` (95%):
> `center = (p̂ + z²/2n) / (1 + z²/n)`,
> `half = z·√( (p̂(1−p̂) + z²/4n) / n ) / (1 + z²/n)`,
> `CI = [center − half, center + half]`, клампить в `[0,1]`. При `n=0` → честное состояние.

Выбран Wilson (а не нормальное приближение) — устойчив при малых `n` и долях у границ 0/1 (а
выборка гео — это ~200 и доли могут быть близки к 0.7). Метод PRD/архитектурой **не предписан** →
это латитюд дева; зафиксировать выбор в `ci_method: "wilson_0.95"`. [Source: PRD/runbook — метод CI
не специфицирован (агент-анализ); architecture.md:160 — граничные n→«недостаточно»]

### Previous Story Intelligence (Story 0.1 — прямой предшественник)

- **Главный урок:** 0.1 ушла в HALT, потому что её AC требовали **живого токена** (200 OK,
  реальные КАТО-коды). **У 0.2 этого ограничения НЕТ** — машинерия вердикта строится и тестируется
  на фикстурах. Не блокируйся на токене. [Source: 0-1-…access.md Dev Agent Record «🚧 ЗАБЛОКИРОВАНО»]
- **Инструмент готов:** `go build ./...` + `go vet ./...` зелёные; `-probe` и
  `resolveParticipantField` работают на фикстурах (`trd-buy → "count"`). Коммит инструмента —
  `796bd54`. [Source: 0-1-…access.md Debug Log; git log 796bd54]
- **Конвенции из 0.1:** в `stage0-audit` **нет тестов и Makefile** — `go build ./...` была
  единственной проверкой. Эта история **меняет это**: добавляет первые `*_test.go` (логика вердикта
  тестируема, в отличие от операционных AC 0.1). [Source: 0-1-…access.md Dev Notes «Тестирование»]
- **Гардрейл честности** соблюдён в 0.1 (неподтверждённое → «нет данных»). Наследовать тот же
  стиль в полях `state`. [Source: 0-1-…access.md «Гардрейл соблюдён»]
- **Решение владельца (2026-06-19):** временно источник = парсер портала до получения токена
  (override §6.1/§6.4). Для 0.2 это **не помеха**: вердикт-машинерия проверяется на `-source file`
  (фикстуры). НЕ использовать `-source scrape` для вердикта (keyword-bias, lots-only исказят гео и
  объём). [Source: memory data-source-interim-parser-first; 0-1-…access.md раздел 6]

### Git-контекст

Релевантные коммиты: `796bd54` (план спринта + Story 0.1; правка `stage0-audit` — journal в probe,
`resolveParticipantField`), `acee72d` (fix: покрыть `/v2/journal` в probe, источник числа
участников). Прикладного MVP-кода/наследуемых паттернов нет — опора на `stage0-audit/` и
планировочные артефакты. Эта история — вторая в первом эпике; первый код **с тестами** в модуле.

### Project Structure Notes

- Изменения ограничены модулем `stage0-audit/` + один пример-артефакт под `docs/ops/`. Целевое
  монорепо (`server/`, `web/`, `cmd/stage0`, `.github/workflows/`) — предмет Epic 1+ (Story 1.1).
  Здесь его НЕ создавать. [Source: architecture.md:289–294, 688–696]
- Конфликтов с целевой структурой нет: в Epic 1 `stage0-audit` вливается в монорепо «поверх
  `internal/goszakup`» (`cmd/stage0`); машинный вердикт, сделанный здесь, едет туда без переписывания.
  [Source: architecture.md:293]
- Конвенция CI (`красный = блок merge`, exit-код) задаётся здесь как **контракт**; физический
  `.github/workflows/ci-server.yml` подключит его, когда появится монорепо-CI. [Source: architecture.md:475–477, 688–696]

### References

- [Source: epics.md:659–678 — Story 0.2 (полный текст AC)]
- [Source: epics.md:539–548 — Epic 0 (формат вердикта, CI-хук, baseline-артефакт)]
- [Source: epics.md:421–427 — Party Mode реш. №1 (гейт №0 как quality-gate с `-format json`)]
- [Source: epics.md:700–722 — Story 0.4 (частичный Go: владелец+дата; ключ идемпотентности; схема)]
- [Source: epics.md:679–698 — Story 0.3 (живой прогон вердикта, числа в docs/ops/stage0-gate.md)]
- [Source: architecture.md:162–177 — Гейт №0, таблица Go/No-Go, пороги OQ-1/4/6]
- [Source: architecture.md:108, 124–127 — methodology_version иммутабельный; воспроизводимость]
- [Source: architecture.md:475–477, 688–696, 659–661 — CI-гейты, GitHub Actions, exit-код]
- [Source: architecture.md:523–524, 557–558, 569 — JSON snake_case, enum lower_snake]
- [Source: architecture.md:160, 488 — n=0/1/2→«недостаточно»; гео=Шаг B+ не Шаг B]
- [Source: prd.md:128–133 (FR-6), 150–154 (FR-9), 311–315/328–330 (гардрейлы нейтральности/честности), 404 (OQ-1)]
- [Source: docs/AshyqQala_stage0_data_audit_runbook_v1.md:18–76 — Шаги A/B/B+/C/D, гейт ≥70%]
- [Source: docs/AshyqQala_MVP_data_model_and_flags_v1.md:68–81 — methodology_params дефолты (min_sample=5, ×1.5, 0.5)]
- [Source: stage0-audit/main.go:13–37 (флаги), :54 (GeoGate), :101–103 (точка вызова), :184–201 (Report), :401–489 (print+таблица)]
- [Source: stage0-audit/config.go:16 (GeoGate=0.70), :7–27 (Config)]
- [Source: stage0-audit/source.go:23,48,99 (Name()); go.mod (Go 1.25, stdlib-only)]
- [Source: 0-1-…токен-ows_v2…md — Previous Story Intelligence, гардрейлы, конвенции]
- [Source: CLAUDE.md — «-probe first», VERIFY, схемная устойчивость, гардрейлы; стек]

## Открытые вопросы / решения владельцу (зафиксировать до/во время dev)

1. **`go_with_fallback` — триггер.** Рекомендация: по умолчанию `geo<0.70 → no_go + exit 1`
   (контракт CI-хука); `go_with_fallback + exit 0` — только через явный `-allow-fallback` (owner
   sign-off, Story 0.4). Подтвердить, что это удовлетворяет AC2 «geo<0.70 → verdict≠go ∧ exit 1»
   (override — осознанное отклонение, а не дефолт). **Рекомендуется принять как есть.**
2. **Коммитимая локация baseline** — `docs/ops/stage0-verdict-YYYYMMDD.json` (рекоменд., рядом с
   `stage0-access.md`/`stage0-gate.md`) vs `stage0-audit/`. **Рекоменд.: `docs/ops/`.**
3. **Метод CI** — Wilson score 95% (`z=1.96`). Метод не предписан PRD → латитюд. **Рекоменд.: принять.**
4. **`methodology_version` стартовое значение** — `"stage0-1.0"`. Подтвердить формат.
5. **Exit-код в text-режиме** — применять exit-гейт всегда или только в `-format json`/`-gate`?
   **Рекоменд.:** по умолчанию text не меняет exit (обратная совместимость с 0.1-прогонами);
   гейт включается `-format json` или явным `-gate`. Зафиксировать выбор в Dev Agent Record.
6. **Минимальный GH Actions workflow сейчас?** Рекоменд.: **отложить** до монорепо-CI (Epic 1/2);
   здесь — exit-код + артефакт + документированный контракт.

## Dev Agent Record

### Agent Model Used

claude-opus-4-8[1m] (Claude Opus 4.8, 1M context) — dev-story workflow.

### Debug Log References

- Baseline (start): `git rev-parse HEAD` → `796bd548…`; `go build ./...` + `go vet ./...` зелёные; фикстуры (300) присутствуют в `data/`.
- TDD: `verdict_test.go` написан первым → `go test` падает (red, `undefined: deriveVerdict/…`) → реализован `verdict.go` → green.
- `go test -count=1 -v ./...` → **11/11 PASS** (`TestDeriveVerdict` 7 кейсов, `honestyNoFabricated0`, `requiredKeys`, `TestWilsonInterval`, `deterministicKeys`).
- AC1 (JSON): `go run . -source file -format json` → валидный JSON-объект, 5 обязательных ключей, диагностика в stderr; `oq4.coverage=null`, `state=insufficient_sample` (гео не прогонялся).
- AC2 (exit): json без гео → **exit 1**; `text` → exit 0; `text -gate` → exit 1.
- AC3 (артефакт): `-verdict-out <dir> -verdict-date 20260620` → `stage0-verdict-20260620.json`; `diff` со stdout пустой (байт-в-байт); `.json`-путь пишется как есть.
- Детерминизм: два прогона (фикс. `-verdict-date`, без `generated_at`) → `diff` пустой; порядок ключей верхнего уровня стабилен.
- `gofmt -l *.go` → пусто (чисто).

### Completion Notes List

- **Реализовано (AC1–AC3 закрыты на фикстурах, без токена — как и требует история):**
  - `verdict.go` — тип `Verdict` (+ вложенные `OQ1Volume`/`OQ4GeoCoverage`/`OQ6Sample`), чистая `deriveVerdict(Report,Config,verdictOpts)→(Verdict,int)`, `wilsonInterval` (Wilson 95%, stdlib `math`), `const MethodologyVersion="stage0-1.0"`, helpers `round4`/`frac`. Без `os.Exit` внутри — тестируемость.
  - `main.go` — флаги `-format`/`-verdict-out`/`-verdict-date`/`-allow-fallback`/`-gate`; ветка JSON-вывода + `os.Exit(code)`; запись артефакта (`writeVerdictArtifact`/`resolveVerdictPath`); `[warn]` и баннер/probe → stderr в JSON-режиме (чистый stdout).
  - `verdict_test.go` — **первые юнит-тесты модуля** (11 кейсов).
  - `README.md` — раздел вердикта + таблица exit-кодов + CI-хук; обновлена «Структура».
  - `docs/ops/stage0-verdict-20260620.json` — пример-артефакт на фикстурах (честный `data_source: file:./data`).
- **Числа НЕ пересчитываются:** `deriveVerdict` только читает готовые поля `Report` из Шагов A–D; текстовый вывод (`Report.print`) не сломан (регресс-проверка зелёная).
- **Гардрейлы соблюдены:** (1) честность — «нет замера» → `coverage:null`+`insufficient_sample`, не `0%`; n∈{0,1,2}→insufficient (`minGeoSample=3`, [architecture.md:160]); (2) только stdlib — `go.mod` остаётся однострочным; (3) гео-метрика = Шаг B+ (`geocode*`), не прокси Шага B; (4) `methodology_version` пинит формулу+пороги.
- **Решения по открытым вопросам (приняты по рекомендациям истории):**
  - Q1 — `go_with_fallback` только через явный `-allow-fallback` (sign-off владельца, Story 0.4); по умолчанию `geo<0.70 → no_go+exit1` (контракт CI-хука). Удовлетворяет AC2.
  - Q2 — baseline в `docs/ops/`. Q3 — Wilson 95% (`ci_method:"wilson_0.95"`). Q4 — `methodology_version="stage0-1.0"`.
  - **Q5 — exit-код: `json` гейтит всегда; `text` по умолчанию НЕ меняет exit (обратная совместимость), включается `-gate`.**
  - Q6 — GH Actions workflow отложен до монорепо-CI (Epic 1+); здесь только exit-код + артефакт + документированный контракт.
- **Не заблокировано на токене** (в отличие от Story 0.1): вся машинерия построена/проверена на `-source file`. Живой прогон вердикта с настоящими числами — Story 0.3.

### File List

- `stage0-audit/verdict.go` — **новый.** Тип `Verdict`, чистая `deriveVerdict`, `wilsonInterval`, `MethodologyVersion`, helpers.
- `stage0-audit/verdict_test.go` — **новый.** Первые юнит-тесты модуля (11 кейсов: граница гейта, честность, Wilson, детерминизм, обязательные ключи).
- `stage0-audit/main.go` — изменён: флаги `-format`/`-verdict-out`/`-verdict-date`/`-allow-fallback`/`-gate`; ветка JSON-вывода + exit-код + запись артефакта; диагностика/`[warn]` → stderr в JSON-режиме.
- `stage0-audit/README.md` — изменён: раздел «Машиночитаемый вердикт + CI-гейт», обновлена «Структура».
- `docs/ops/stage0-verdict-20260620.json` — **новый.** Пример baseline-артефакта на фикстурах (`data_source: file`).
- `_bmad-output/implementation-artifacts/0-2-машиночитаемый-go-no-go-вердикт-stage0-audit.md` — изменён: frontmatter `baseline_commit`, чекбоксы задач, Dev Agent Record, File List, Change Log, Status.
- `_bmad-output/implementation-artifacts/sprint-status.yaml` — изменён: статус истории `ready-for-dev` → `in-progress` → `review`.

## Change Log

| Дата | Изменение |
|---|---|
| 2026-06-20 | Старт dev-story: история in-progress, зафиксирован `baseline_commit` (`796bd54`). TDD: написан `verdict_test.go` (red) → реализован `verdict.go` (green). Добавлен JSON-вердикт (`-format json`), exit-код для CI (`-gate`/json-авто), baseline-артефакт (`-verdict-out`/`-verdict-date`). 11/11 тестов PASS; `go build/vet/test/gofmt` зелёные. AC1–AC3 закрыты на фикстурах. README дополнен CI-контрактом; закоммичен пример-артефакт `docs/ops/stage0-verdict-20260620.json`. Открытые вопросы Q1–Q6 приняты по рекомендациям. Статус → review. |
| 2026-06-23 | Code review (адверсариальный, 3 слоя: Blind / Edge Case / Acceptance). AC1–AC3 и гардрейлы подтверждены **MET**; `go build/vet/test/gofmt` перепроверены — зелёные; `GeoGate=0.70` дефолт подтверждён. Найдено: 1 decision-needed (`generated_at` vs «байт-в-байт»), 5 patch, 1 defer (провенанс fallback → Story 0.4), ~11 dismissed (false-positive / by-design / out-of-scope). См. «Review Findings». |
| 2026-06-23 | Применены все 6 патчей ревью: D1 → пин `generated_at` через `-verdict-date` (вариант b, решение владельца); валидация `-verdict-date` (anti-traversal); warning при `-verdict-out` в text; защитный клэмп `0≤successes≤n`; `writeVerdictArtifact` возвращает путь; `const DefaultGeoGate` + 2 новых теста (`TestDefaultGeoGate`, `TestDeriveVerdict_geoMeasuredCompound`). README синхронизирован. 7 тест-функций PASS; build/vet/gofmt зелёные; поведение проверено e2e. Defer (провенанс fallback) → `deferred-work.md` / Story 0.4. **Статус → done.** |

## Review Findings (Code Review — 2026-06-23)

> Источник диффа: `796bd54..HEAD`, scoped к File List (изменения 0.2 закоммичены в `cedd1ce`). Слои: Blind Hunter (только дифф), Edge Case Hunter (дифф + проект), Acceptance Auditor (дифф + спека). Все AC (AC1–AC3) и несущие гардрейлы — **MET**; `go build/vet/test/gofmt` — зелёные.

### Decision-needed

- [x] [Review][Decision] `generated_at` ломает «байт-в-байт» baseline (`stage0-audit/verdict.go:45`, `stage0-audit/main.go:181`). **РЕШЕНО владельцем 2026-06-23: вариант (b)** — пинить `generated_at` через `-verdict-date` (override управляет и именем, и временем; без override = `time.Now()`). → переведено в Patch (P6).

### Patch (все применены 2026-06-23 — `go build/vet/test/gofmt` зелёные, поведение проверено e2e)

- [x] [Review][Patch] (из D1) `generated_at` пинится через `-verdict-date` — добавлена `generatedAtFor()` (полночь Астаны +05:00 для заданной даты); проверено: два прогона `-verdict-date 20260620` байт-идентичны, пример-артефакт обновлён на `2026-06-20T00:00:00+05:00` ✅ [stage0-audit/main.go]
- [x] [Review][Patch] `-verdict-out` в text-режиме → предупреждение в stderr, артефакт не пишется (проверено e2e) ✅ [stage0-audit/main.go]
- [x] [Review][Patch] `-verdict-date` валидируется строго `time.Parse("20060102")` — режет `../` traversal и битые даты (`20261332`) → exit 2 (проверено e2e) ✅ [stage0-audit/main.go]
- [x] [Review][Patch] защитный клэмп `0≤successes≤n` в `wilsonInterval` + ветке coverage `deriveVerdict` (нет NaN→exit 2, нет `cov>1`-ложного-go); тест `wilsonInterval(10,5)` без NaN ✅ [stage0-audit/verdict.go]
- [x] [Review][Patch] `writeVerdictArtifact` теперь возвращает итоговый путь — убран второй `resolveVerdictPath`/`os.Stat` в лог-строке ✅ [stage0-audit/main.go]
- [x] [Review][Patch] тест-хардненинг: `const DefaultGeoGate=0.70` (единый источник, `main()`+тесты ссылаются на него); `TestDefaultGeoGate` (дефолт + связь с фикстурами); `TestDeriveVerdict_geoMeasuredCompound` (обе ветки `geoMeasured` + граница n=3) ✅ [stage0-audit/verdict.go, verdict_test.go]

### Defer

- [x] [Review][Defer] `-allow-fallback`: артефакт `go_with_fallback` без поля провенанса владельца/sign-off [stage0-audit/verdict.go:175-180] — deferred, по спеке (Q1) владелец+дата фиксируются в Story 0.4
