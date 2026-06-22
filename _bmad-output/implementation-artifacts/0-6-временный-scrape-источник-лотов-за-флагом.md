# Story 0.6: ⏳ Временный scrape-источник лотов за флагом

Status: ready-for-dev

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

- [ ] **Task 0 — Предусловие: подтвердить фундамент монорепо (БЛОКЕР, см. «Зависимости»)** *(AC1)*
  - [ ] Убедиться, что существуют (созданы предыдущими историями): монорепо `server/` (модуль `ashyqqala/server`, Story 1.1), каталог `server/tools/` и `cmd/{api,importer}` (1.1), миграция с **проекцией `lots`** + `internal/store` (Story 1.2 / `migrations/0002_projection`), и пакет **`internal/ingest/decode`** (живая граница декодирования — Story 2.1)
  - [ ] **Если любого из них нет — HALT** и эскалировать владельцу решение о последовательности (эту историю нельзя реализовать поверх несуществующего монорепо; см. раздел «Зависимости и предусловия»). Не создавать монорепо в рамках этой истории — это скоуп 1.1.
- [ ] **Task 1 — Перенести парсер в `server/tools/scrape/` за build-tag (AC1, AC2)**
  - [ ] Перенести логику `stage0-audit/scrape.go` (`scrapeSource`, `parseRows`, `parseMoney`, `astanaKatos`, round-robin по `terms`) в `server/tools/scrape/` как пакет вне продового бинаря
  - [ ] Поставить **build-tag** (например `//go:build scrape`) + `doc.go` с пометкой «one-off, НЕ прод; §6.1/§6.4» — по образцу целевого дерева (`server/tools/scrape/`)
  - [ ] Реализовать/переиспользовать `Source` для `scrape` так, чтобы интерфейс совпадал с боевым `internal/goszakup` (`Name()`, `Fetch(resource, scopeBINs, max, handle)`) — это и есть точка swap `scrape → ows`
- [ ] **Task 2 — Интерим-команда `tools/` с флагом и громким warning (AC1)**
  - [ ] Создать `tools/`-команду интерим-импорта (отдельный `main` за тем же build-tag, **вне** `cmd/api`/`cmd/importer`)
  - [ ] Гейт по `ASHYQQALA_INTERIM_SCRAPE=1`: **без флага команда отказывает** (ненулевой exit + сообщение); с флагом — печатает **громкий warning** о временном отклонении §6.1/§6.4 и критерии удаления
  - [ ] Параметры запуска (КАТО Астаны, `max`, delay для вежливости Nominatim/портала) — переиспользовать дефолты `stage0-audit` (delay вежливости, viewbox)
- [ ] **Task 3 — Запись scraped-лотов в проекцию `lots` через `ingest/decode` (AC1)**
  - [ ] Прогнать scraped-записи через `internal/ingest/decode` (тот же вход, что у будущего `ows`), а не писать в БД напрямую — это сохраняет downstream при swap
  - [ ] Маппинг полей scrape → `lots` (модель данных): `name_ru → title_ru`, `amount → amount`, `ref_kato → kato_code`, `trd_buy_number_anno` → связь с объявлением (объявления нет в парсере → `announcement_id = null`); **отсутствующие поля (`title_kk`, `quantity`, `unit`) = NULL, НЕ выдумывать**
  - [ ] Идемпотентность записи (UPSERT по натуральному ключу лота) — повторный прогон не плодит дубли
- [ ] **Task 4 — CI-страж изоляции `tools/scrape` от прода (AC2)**
  - [ ] Добавить архитектурный go-list-тест (по образцу `ci-server.yml` «АРХИТЕКТУРНЫЙ go-list-тест» / границы `internal/goszakup` импортируется только из `ingest/decode`): проверить, что `cmd/api` и `cmd/importer` НЕ имеют `tools/scrape` в транзитивном графе импортов
  - [ ] Страж должен быть **red при нарушении** (тест/CI-шаг падает, если импорт появится) — это исполняемая граница, не комментарий
- [ ] **Task 5 — Обратимость и критерий удаления (AC3)**
  - [ ] Зафиксировать в `doc.go`/доке: swap `scrape → ows` = один флаг `Source`; downstream неизменен; **критерий удаления** интерим-команды = «получен `GOSZAKUP_TOKEN`»; путь восстановления §6.1/§6.4 — ссылка на Sprint Change Proposal 2026-06-20
  - [ ] Обновить `docs/ops/` (раздел про интерим-источник) — согласовать с `docs/ops/stage0-access.md` §6
- [ ] **Task 6 — Тесты и финализация**
  - [ ] Юнит-тест парсера на **записанной фикстуре HTML** (НЕ живой запрос в тесте): `parseRows`/`parseMoney`/маппинг → `lots` детерминированы
  - [ ] Тест гейта флага: без `ASHYQQALA_INTERIM_SCRAPE` команда отказывает; с флагом — warning
  - [ ] Тест идемпотентности декода в `lots` (повторный прогон = без дублей)
  - [ ] `go build ./...` + go-list-страж зелёные; обновить File List, Change Log, Completion Notes

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

### Completion Notes List

- **⛔ HALT на Task 0 (предусловие).** Скелет монорепо (Story 1.1, done) есть, но три из четырёх несущих компонентов AC1–AC3 — пустые заглушки/отсутствуют:
  - проекция `lots` (миграция + `internal/store`) — НЕ создана (Story 1.2 сделала только `contracts`; `lots` отложена в Epic 2 / B-3);
  - `internal/ingest/decode` — пустой `doc.go` (Story 2.1, backlog);
  - `internal/goszakup` `Source` (точка swap `scrape → ows`) — пустой `doc.go` (Story 2.1, backlog).
- Реализовать перенос парсера + интерим-импорт + маппинг в `lots` через `ingest/decode` НЕВОЗМОЖНО, пока нет проекции `lots`, decode-границы и интерфейса `Source`. История подтверждённо сидит downstream фундамента Epic 1 (1.2) и Epic 2 (2.1).
- **Гардрейл соблюдён:** проекцию `lots`/миграции (скоуп 1.2/Epic 2) и `ingest/decode`+`Source` (скоуп 2.1) в рамках этой истории НЕ создаю — это прямой запрет Task 0 и раздела «Зависимости» (дублирование сломало бы анти-churn структуру). Честность над домыслом: не имитирую несуществующие пакеты.
- **Эскалация владельцу — решение о последовательности.** Реалистичный путь (как и предсказывала сама история): сначала проекция `lots` (расширение Story 1.2 / Epic 2 B-3) + минимальные `internal/ingest/decode` и интерфейс `Source` (Story 2.1), затем 0.6.

### File List

- (изменений кода нет) `_bmad-output/implementation-artifacts/0-6-временный-scrape-источник-лотов-за-флагом.md` — заполнены Dev Agent Record (Task 0 HALT) и Change Log.

## Change Log

| Дата | Изменение |
|---|---|
| 2026-06-21 | Создан context engine для Story 0.6 (трек «Парсер-мост»). Исчерпывающий анализ: epics 0.6–0.9, architecture (отклонение + целевое дерево + границы), модель данных `lots`, код `stage0-audit/scrape.go`+`source.go`, предыдущие истории 0.1/0.2 + Sprint Change Proposal. **Зафиксирован критичный блокер:** AC1–AC3 зависят от ещё не созданного монорепо (Story 1.1 скелет, 1.2 проекция `lots`+sqlc, 2.1 `ingest/decode`) — Task 0 = предусловие/HALT + эскалация владельцу по последовательности. Статус → ready-for-dev. |
| 2026-06-22 | Запуск dev-story. **Task 0 (предусловие) — HALT.** Проверено фактическое состояние репо: скелет монорепо (Story 1.1, done) есть, но проекция `lots` (миграция + store), `internal/ingest/decode` и `internal/goszakup` `Source` — пустые заглушки/отсутствуют (1.2 сделала только `contracts`; `lots` → TODO Epic 2/B-3; `ingest/decode`+`Source` → скоуп Story 2.1, backlog). Историю нельзя реализовать поверх несуществующего фундамента; создавать его здесь запрещено (чужой скоуп 1.2/2.1, анти-churn). Эскалировано владельцу: решить последовательность (сначала проекция `lots` + Story 2.1, затем 0.6). Статус остаётся ready-for-dev (реализация не начата). |
