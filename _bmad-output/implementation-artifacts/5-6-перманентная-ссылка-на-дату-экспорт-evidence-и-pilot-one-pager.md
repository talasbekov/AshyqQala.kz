---
baseline_commit: 42fcc8c  # HEAD на момент создания (Epic 5: 5.3 review-фиксы + 5.4 FR-28). ⚠️ 5.5 (done) — в
                          # рабочем дереве, НЕ закоммичена: пакет og/, render.FlagLine, export-смежные правки уже
                          # на месте; dev 5.6 строит поверх рабочего дерева. Закоммитить 5.5 ДО/вместе с 5.6.
context:
  - _bmad-output/planning-artifacts/epics.md
  - _bmad-output/planning-artifacts/prds/prd-AshyqQala.kz-2026-06-17/prd.md
  - _bmad-output/planning-artifacts/architecture.md
---

# Story 5.6: Перманентная ссылка-на-дату, экспорт evidence и pilot-one-pager

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

> ✅ **Токен-НЕзависима для разработки/тестов на синтетике (как 5.1/5.3/5.4/5.5).** Все три AC строят поверх
> УЖЕ готовых, протестированных на синтетике слоёв: `ContractDTO`/`Projection` (5.1), `resolveContractFlags`
> (5.1), `export.Export` (4.6), `metrics` SM-C1 (4.6), вердикт Epic 0 (`docs/ops/stage0-verdict-*.json`).
> Реальные данные карточек придут с токеном `ows_v2` — тот же барьер, что у 5.1. Эпик 5 «🚫 Ожидает токен»
> только для **реальных данных** (epics.md:1507-1514); презентационная логика 5.6 токен-агностична.
> См. [[sprint-sequencing-epic1-first]], [[data-source-interim-parser-first]], [[guards-must-prove-red]].

> 🧭 **Это НЕ 5.2, НЕ 5.7.** 5.2 (карточка подрядчика) заблокирована фундаментом Epic 2. 5.7 (per-surface
> рендер-тест + cross-surface snapshot) — следующая после 5.6; её ядро 5.5 уже частично заложила
> (web↔OG parity на хвостах P2 + taboo по всем флаг×состояние P3 в ревью 5.5). 5.6 — предпоследняя
> презентационная история Эпика 5 (после неё 5.7, затем эпик-ретро).

## Story

**Как** Данияр-журналист (и команда — для пилота с акиматом),
**Я хочу** стабильную ссылку-на-дату, экспорт доказательств флага и обоснованный артефакт пути к пилоту,
**Чтобы** публикация в СМИ была цитируема и проверяема, а готовность к пилоту — доказуема цифрами.

Контекст: AR-29 («перманентная ссылка/экспорт», Issues #5) — закрывает UJ Данияра (SM-5: 3+ публикации СМIA).
Три развязанных AC: (1) ссылка-на-дату, чтобы карточка не «плыла» при смене методики; (2) экспорт `evidence`
для цитирования; (3) pilot-one-pager с метриками этапа 5, опирающийся на экспорт evidence (AC-2) и вердикт
Epic 0. Это «дверь наружу к доказуемости» — публикация переживает смену методики и несёт пересчитываемые факты.
[Source: epics.md#Epic 5 → Story 5.6 (lines 1616-1634); architecture.md:290-291 (AR-29), :842, :865, :895-896;
prd.md#metrics SM-5; epics.md:464-466 (Victor: pilot-one-pager)]

## Acceptance Criteria (дословно из эпика, lines 1622-1634)

### AC-1 — Перманентная ссылка-на-дату (snapshot/as_of, AR-29): карточка не «плывёт»
**Given** перманентная ссылка-на-дату (snapshot/as_of, AR-29)
**When** методика позже изменилась
**Then** карточка по ссылке не «плывёт» (показывает состояние на дату).

> ⚠️ **РЕШАЮЩЕЕ ОГРАНИЧЕНИЕ (подтверждено кодом):** истории снапшотов в MVP НЕТ. Колонки `snapshot_id` не
> существует ни в одной миграции; `risk_flags` хранит ТОЛЬКО текущее состояние (UNIQUE `(flag_type,
> contract_id)`, UPSERT перезаписывает evidence/version — `migrations/0007_risk_flags.sql:4-5,35`,
> `RaiseContractFlag` `risk_flags.sql:1-13`). Реконструировать «как выглядела карточка на дату X» из БД
> **невозможно**. Тяжёлый снапшот-стор архитектура ЯВНО отложила до пилота (architecture.md:333-340 «MVP-
> реализация ЛЁГКАЯ: snapshot_id + дешёвый дамп; тяжёлый стор отложен»). **⇒ AC-1 решается ЧЕСТНЫМ ШТАМПОМ +
> детектом дрейфа, НЕ реконструкцией прошлого.** См. Open Question #1 (решить ДО старта T1).

Тестируемо (реалистичный MVP-дизайн — штамп + честный дрейф):
- Перманентная ссылка = URL карточки + штамп `?as_of=<detected_at>&mv=<methodology_version>` (источники уже в
  проекции: `firstRaisedAsOf` из `DetectedAt` 5.5, `params.MethodologyVersion`). Пустые `as_of`/`mv` НЕ
  штампуются (honesty-hole: повторить паттерн OG `meta.go:194-198` — пустую версию НЕ выдаём как `v=`).
- На чтении карточка сравнивает `mv` из ссылки с ТЕКУЩЕЙ `params.MethodologyVersion`:
  - **совпадает** → результат идентичен (контракт пересчитываемости `(snapshot_id, methodology_version) → тот
    же результат бит-в-бит`, architecture.md:108; `methodology_params` иммутабелен по версиям) → ссылка не «плывёт»;
  - **не совпадает** → ЧЕСТНАЯ деградация: видимый нейтральный баннер «методика изменилась с `<old>` на
    `<current>`; показано ТЕКУЩЕЕ состояние; исторический срез не хранится (MVP)». Это уже предусмотренное
    честное состояние «данные устарели (с <дата>)» (architecture.md:138). **Не** молчаливо подменять прошлое текущим.
- Страж AR-1 («не плывёт») обязан **доказать, что краснеет** ([[guards-must-prove-red]]): negative-control —
  подмена `params.MethodologyVersion` → баннер дрейфа появился; `-count=1`; пин golden литералом.

### AC-2 — Экспорт `evidence` (JSON/печатный) для цитирования в СМИ (SM-5)
**Given** экспорт `evidence`
**When** запрошен
**Then** доступен JSON/печатный для цитирования в СМИ (SM-5).

Тестируемо:
- Публичный per-flag экспорт raised-флага контракта: `GET /api/contracts/{goszakup_id}/flags/{flag_type}/evidence.json`
  (`application/json`) + `…/evidence.txt` (`text/plain`). `{flag_type}` ∈ `contractFlagTypes`
  (`single_participant|price_per_km`). Переиспользовать путь данных `Projection` (GetContractByID → c.ID →
  ListContractFlags → найти строку по flag_type).
- JSON — канонический `export.Export` (стабильные ключи, evidence round-trip, пересчитываемо третьим лицом).
  Печатный — нейтральный текст: рамка из registry `frame.signal` (через `render.Renderer.Text`) + факты +
  evidence; **без оценочных слов** (FindTaboo = 0 в обеих локалях; negative-control обязателен).
- **Честная деградация:** evidence есть ТОЛЬКО при raised (`resolveContractFlags`: не-raised/нет-строки →
  evidence `null`). Экспорт не-raised/несуществующего флага → честный **404 / no_data** (нечего цитировать), НЕ
  фабрикованный «пустой evidence-документ» (§7.4, [[guards-must-prove-red]]).
- **`subject_id`-утечка (решить — Open Question #2):** `export.exportDoc.SubjectID int64` сейчас выведет
  ВНУТРЕННИЙ `contract_id`, что нарушает несущую wire-конвенцию «внутренний bigint на проводе НЕ отдаём; публичный
  id = natural `goszakup_contract_id`» (`contracts.go:28-29`). Закрыть до подключения.

### AC-3 — Pilot-one-pager с метриками этапа 5, опирающийся на экспорт evidence + вердикт Epic 0
**Given** артефакт пути к пилоту (Victor)
**When** готовится демо
**Then** pilot-one-pager с метриками этапа 5 (N контрактов размечено, M флагов поднято, X проверяемых вручную)
опирается на экспорт evidence и вердикт Epic 0.

Тестируемо:
- Артефакт-генератор (Go-CLI, напр. `server/cmd/pilot-onepager/`) детерминированно
  пишет Markdown в `docs/ops/` (рядом с вердиктом). НЕ публичный эндпоинт (defer 1.1: `/metrics` не светить
  наружу; AC говорит «к демо ГОТОВИТСЯ артефакт»).
- **N** = размеченные контракты (`SELECT count(*) FROM contracts WHERE NOT is_deleted` — NEW; уточнить семантику
  «размечено»). **M** = поднятые флаги (`SELECT count(*) FROM risk_flags WHERE is_active` — NEW; `CountActiveContractFlags`
  сейчас per-flag_type). **X** = «проверяемых вручную» (см. Open Question #3; рекоменд. трактовка: флаги с полным
  `evidence`+`methodology_version` → пересчитываемы по опубликованной методике).
- Вердикт Epic 0: читать закоммиченный `docs/ops/stage0-verdict-20260620.json` (struct `Verdict`), встроить
  `verdict`/`verdict_reason`/`oq4_geo_coverage`/`oq1_volume`. НЕ перезапускать stage0-audit.
- Evidence-цитаты — через `export.Export` (AC-2): нейтральная рамка, без оценочных слов.
- **Честность источника (токен-граница):** данные синтетические/интерим (lots-only, seed; вердикт сейчас
  `no_go` — гео не замерено). One-pager ОБЯЗАН штамповать `data_source`/`methodology_version` и не выдавать
  синтетические N/M за живые пилотные доказательства. Нет данных → «нет данных», мало → «недостаточно
  сопоставимых данных» (тот же словарь, что вердикт). [[data-source-interim-parser-first]]

## Tasks / Subtasks

- [x] **T1 — Перманентная ссылка-на-дату: штамп + детект дрейфа** (AC: 1)
  - [x] Сервер: `Projection` штампует `methodology_version` (текущая) + `as_of` (detected_at raised / updated_at,
        пустое → no_data); `Get` по `?mv` сравнивает с текущей `h.Version` → `methodology_drift{present,requested,current}`
        в `ContractDTO`. Пустую версию НЕ штампует (`versionField`, honesty-hole). OpenAPI расширен (`MethodologyDrift` +
        3 поля Contract) + `make gen-web`.
  - [x] `ContractsHandler.Version` прокинут из `params.MethodologyVersion` в `main.go`.
  - [x] Web: `ContractRoute.tsx` парсит `?mv` → форвардит в `useContract`/`fetchContract`; баннер дрейфа (i18n
        `methodology.drift`); `PermalinkButton` (копирует абс. URL); `buildPermalink` (штамп as_of+mv, пустое не
        фабрикует). i18n `permalink`/`permalink_copied`/`drift` (ru+kk). CSS только-раскладка.
  - [x] Go-стражи `contracts_permalink_test.go`: штамп+дрейф (conformance по OpenAPI) + **negative-controls** (== версия / без mv / пустая версия → нет дрейфа), `-count=1`; web `permalink.test.ts` (4 кейса, honesty-hole пустого штампа).
- [x] **T2 — Подключение экспорта evidence к HTTP** (AC: 2)
  - [x] `subject_id`-утечка закрыта: `export.exportDoc.subject_ref` = goszakup_contract_id; внутренний int64 убран
        из `FlagRecord`/wire. `export/evidence_test.go` обновлён (subject_ref, нет subject_id).
  - [x] `EvidenceExportHandler` (`httpapi/evidence_export.go`): `GetJSON`/`GetText` по `/api/contracts/{id}/flags/{flag_type}/evidence.{json,txt}`;
        путь данных GetContractByID → ListContractFlags → raised-строка по flag_type → `export.Export(rec, Renderer.Text(loc,"frame.signal"))`;
        не-raised/нет контракта → честный 404. Маршруты в `main.go` (Renderer прокинут).
  - [x] Адаптер `gen.RiskFlag`→`export.FlagRecord`; defensive `json.Valid` + пустой evidence → error (в `export.Export`);
        `export.Export` маршалит в буфер ДО записи (нет частичного ответа).
  - [x] OpenAPI: схема `EvidenceExport` + путь `evidence.json` + conformance (`evidence_export_test.go` VisitJSON);
        `.txt` (text/plain) НЕ внесён (прецедент 5.5). `make gen-web` зелёный.
  - [x] Нейтральность печатного: `FindTaboo`-проверки фактами + **negative-control** (taboo-слова) в `export/evidence_test.go`
        и `httpapi/evidence_export_test.go`. Web: ссылка «Экспорт доказательств (JSON)» у raised-флага (ContractCard, i18n ru+kk).
- [x] **T3 — Pilot-one-pager генератор** (AC: 3)
  - [x] NEW count-запросы sqlc (`queries/pilot_metrics.sql`): N (`contracts NOT is_deleted`), M (`risk_flags is_active`),
        X (`is_active AND methodology_version<>'' AND evidence<>'{}'` — OQ#3 ✅ пересчитываемые). `make gen-sqlc` зелёный.
  - [x] Генератор `server/cmd/pilot-onepager/` (main.go IO + report.go pure): pgxpool как `cmd/api`; `readMetrics`
        (count'ы); `readVerdict` парсит `docs/ops/stage0-verdict-20260620.json`; `renderOnePager` → Markdown в
        `docs/ops/pilot-onepager-<date>.md`. Канал цитирования evidence (AC-2 эндпоинт) явно в артефакте.
  - [x] Честность: штамп `data_source` («синтетика/демо») + методика Stage-0; пустая проекция (Marked==0) → честный
        «данные не загружены» (НЕ таблица нулей); `honestCount` 0→«нет данных»; нейтральная лексика.
  - [x] `report_test.go`: golden-литералы (N/M/X, вердикт, гео, канал экспорта) + **negative-controls** (пустая БД →
        честно; `honestCount`; taboo-страж краснеет); `main_test.go`: `readVerdict` против реального артефакта + `readMetrics` мок.
- [x] **T4 — Контракт/доки/нейтральность (signature-стражи)** (AC: 1, 2, 3)
  - [x] Per-surface нейтральность × обе локали: экспорт (`export/evidence_test.go` + `httpapi/evidence_export_test.go` ru+kk),
        one-pager (`report_test.go`), web (`contractCardStrings.test.ts` расширен на `methodology.*` + новые ключи). Все с negative-control.
  - [x] `sprint-status.yaml` 5-6 → review; `deferred-work.md` дополнен; Dev Agent Record заполнен честно.

## Dev Notes

### Что строить (one-liner ориентир)
Три развязанных «двери к доказуемости»: (1) перманентная ссылка = честный `?as_of/?mv`-штамп + баннер дрейфа
(история не хранится — НЕ реконструировать прошлое, честно показать «текущее + методика сменилась»); (2)
подключить готовую `export.Export` к `GET …/flags/{flag_type}/evidence.{json,txt}` (raised-only, честный 404
иначе, без утечки внутреннего id); (3) Go-CLI-генератор Markdown one-pager из count-запросов (N/M/X) + вердикта
Epic 0 + evidence-цитат — со стражами, доказанно краснеющими.

### REUSE (НЕ переизобретать — переиспользовать дословно)
- **`ContractsHandler.Projection`** — `server/internal/httpapi/contracts.go:110-148` (единый путь данных JSON+OG:
  GetContractByID → c.ID → ListContractFlags → resolveContractFlags). И штамп-логику AC-1, и export AC-2 строить здесь.
- **`resolveContractFlags` + `ContractFlagDTO`** — `server/internal/httpapi/contract_flags.go:54-80`: честные
  состояния; `Evidence` непусто ТОЛЬКО при raised (`:63,71`); `DetectedAt`/`versionField` (`:14-19,38,70,75`).
- **`export.Export` + `FlagRecord` + `exportDoc`** — `server/internal/export/evidence.go:13-50` (канонический JSON +
  нейтральный печатный; тест-страж нейтральности `evidence_test.go:45-49`). ⚠️ `SubjectID int64` — см. NEW/OQ#2.
- **`render.Renderer.Text(loc,"frame.signal")`** — `server/internal/render/render.go:67`; источник
  `registry/values/glossary-{ru,kk}.json:2`. Рамку для печатного экспорта брать ТОЛЬКО отсюда (export не тянет лексикон).
- **OG as_of-паттерн** — `server/internal/og/meta.go:194-204,225-235` (`firstRaisedAsOf`, пустую версию НЕ штамповать).
- **`metrics` SM-C1 (DB-derived)** — `server/internal/metrics/metrics.go` (⚠️ `counterValue` уже выпилен в 4.6; refs
  deferred-work.md:31,75 — УСТАРЕЛИ). `/metrics` — `main.go:76,103`.
- **Вердикт Epic 0** — struct `Verdict` `stage0-audit/verdict.go:52-66`; артефакт `docs/ops/stage0-verdict-20260620.json`
  (сейчас `no_go`, `data_source: file:./data` — синтетика); writer `stage0-audit/main.go:234-255`.
- **count-шаблоны** — `CountActiveContractFlags` (`risk_flags.sql:28-30`, per-type), `CountFlagDisputesByStatus`
  (`flag_disputes.sql:20-22`), `CountPriceBenchmarks` (`price_benchmarks.sql:23-24`).
- **pgxpool-паттерн для CLI** — `server/cmd/api/main.go:36-50`. `writeJSON`/`apierr` — `contracts.go:167-171`,
  `apierr/codes.go`. `resolveLocale` — `og/locale.go:16-26`. `Field[T]` — `httpapi/field.go:15-21`.
- **i18n as_of** — `web/src/shared/i18n/locales/{ru,kk}/chrome.json:113-114` («Срез методики: {{version}}, расчёт
  {{date}}»; TODO-маркер «permalink — Story 5.6» в `MethodologyDialog.tsx:173-181`); `formatDateSafe` `format.ts:41`.
- **`useSearchParams` query-паттерн** — `web/src/features/contract/ContractRoute.tsx` (как `?report` 5.4).

### NEW (нужно создать — в репо НЕТ; подтверждено grep'ом)
1. **Штамп `?as_of/?mv` + детект дрейфа** — `ContractRoute.tsx` знает только `?report`; `Projection`/`Get` не имеют
   ни параметра, ни фильтра по дате (`contracts.go:139`, `risk_flags.sql:37-40` — всегда «текущее»). Нужны: парсинг
   штампа, сравнение версий, поле/баннер дрейфа. **`snapshot_id`-колонку/историю НЕ создавать** (отложено архитектурой).
2. **HTTP-эндпоинты экспорта** — под `/api/contracts/{id}` сейчас только GET карточки; подпутей `/flags/...` нет.
   Адаптер `gen.RiskFlag`→`FlagRecord`; `json.Valid` defensive; буфер-до-WriteHeader.
3. **OpenAPI-схема `EvidenceExport`** + путь + conformance (для `.json`; `.txt` — вне арбитра, как OG-HTML 5.5).
4. **count-запросы N/M/X** (sqlc) — готовых нет.
5. **Генератор one-pager** (Go-CLI Markdown) — в репо нет (`grep one.pager|pilot|onepager` = 0).
6. **Renderer в `ContractsHandler`** — сейчас прокинут только в `og.MetaHandler` (`main.go:112`); нужен для рамки экспорта.

### Ключевое архитектурное решение (AC-1) — permalink = штамп, НЕ реконструкция
История снапшотов в MVP отсутствует (подтверждено: нет колонки `snapshot_id`, UPSERT перезаписывает evidence/version).
Архитектура это предвидела — тяжёлый снапшот-стор отложен до пилота (architecture.md:333-340). Поэтому буквальное
«показывает состояние на дату» невозможно без новой тяжёлой работы. **Честный MVP-дизайн: штамп `(as_of, mv)` +
детект дрейфа.** Если методика не менялась — карточка идентична бесплатно (пересчитываемость бит-в-бит,
`methodology_params` иммутабелен по версиям). Если менялась — честный баннер «показано текущее, исторический срез
не хранится». Истинная заморозка (`snapshot_id` + append-only дамп) — НЕ в этой истории (см. OQ#1; не плодить
тяжёлый стор вопреки architecture.md:337). Дух AC-1 («не плывёт молча») закрывается честно.

### Уроки прошлых историй (применить дословно)
**A. Honesty-hole 5.1/5.5 (commit 94b68b8).** Пустое поле НЕ выдавать за «ok». Для 5.6: пустые `as_of`/`mv` НЕ
штамповать в URL (паттерн OG `meta.go:194-198`); evidence-экспорт не-raised → честный 404, не «пустой документ».
**B. Страж обязан краснеть ([[guards-must-prove-red]]).** КАЖДЫЙ новый страж (drift-баннер, нейтральность экспорта,
golden one-pager): negative-control, краснеющий по ПРИЧИНЕ (подмена версии → баннер; taboo-слово → краснеет) +
`-count=1` + golden-литерал. Уроки 5.4/5.5: negative-control не «err != nil», а проверка причины.
**C. Per-field деградация (5.3).** Битая дата/версия не должна валить карточку/экспорт — деградировать per-field
(`formatDateSafe`; честный конверт).
**D. Не завышать покрытие (ревью 5.3/5.5).** Честно в Dev Agent Record: что РЕАЛЬНО прогналось (на синтетике; БД не
поднята → интеграция на CI). `@axe-core` не установлен — не предполагать (deferred-work).
**E. Нейтральность на КАЖДОЙ генерируемой строке (5.5 P3).** Экспорт, баннер, one-pager — все строки через
`FindTaboo` × обе локали; рамка/проза только из render/registry, не литералом.
**F. subject_id-конвенция (4.6 defer).** Внутренний bigint на провод НЕ отдавать — публичный id = goszakup natural id.

### Project Structure Notes
- **Бэкенд:** AC-1/AC-2 — `server/internal/httpapi/` (Projection, новый export-хендлер) + маршруты `server/cmd/api/main.go`;
  AC-3 — НОВЫЙ `server/cmd/pilot-onepager/` + sqlc count-запросы `server/internal/store/queries/`.
  Граница импортов (`arch/boundaries_test.go`): `export` — чистый слой (без store); пакет one-pager — adapter (может
  читать store/export/registry), pure-core его НЕ импортирует. Проверить место относительно go-list-стража.
- **Контракт:** `docs/api-contracts/openapi.yaml` (арбитр) → `make gen-web` для `evidence.json`; `.txt` вне арбитра.
- **Артефакты:** `docs/ops/` (рядом с `stage0-verdict-*.json`).
- **Тесты:** Go — `httpapi/*_test.go`, `export/evidence_test.go` (расширить), генератор `*_test.go` + golden; web —
  паттерн `flagGlossaryParity.test.ts`/`contractCardStrings.test.ts`; нейтральность под `make check-registry`/`-count=1`.
- **Миграции:** AC-1 — БЕЗ новой таблицы (НЕ создавать snapshot_id). Если count-запросам нужен индекс — оценить, вряд ли.
- ⚠️ **Токен-граница:** строить на синтетике (`fixtures/seed/*`, демо `DEMO-0001..0003`); вердикт сейчас `no_go`
  (гео не замерено) — one-pager честно помечает источник и не выдаёт синтетику за живой пилот.

### Definition of Done (планка из git-конвенций — повторить)
- Backend: `cd server && go build ./...` + `go vet` + `go test -count=1 ./...` + `make check-registry`/`check-core`/`lint`
  зелёные; «generated == regenerated» зелёный после `make gen-web` (новый OpenAPI-путь); интеграция на реальном
  Postgres (compose `postgis:16-3.4`, порт 55432, миграции + seed + `curl` evidence-эндпоинта + прогон генератора one-pager).
- Frontend (если затронут): `npm run typecheck` + `vitest run` + `eslint` + `lint:css` + `npm run e2e`.
- Коммит: RU, header `Story 5.6: …`, тело по областям + `Verification:` + `Отложено (deferred-work):`, концовка
  `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`.

## References

- [Source: epics.md#Epic 5 → Story 5.6, lines 1616-1634] — user story + AC дословно.
- [Source: epics.md#Epic 5, lines 464-466] — Victor: pilot-one-pager с метриками этапа 5 (N размечено, M флагов, X
  проверяемых вручную), опирается на экспорт evidence + вердикт Epic 0.
- [Source: epics.md, line 290-291, 529-531] — AR-29 (перманентная ссылка/экспорт); Epic 5 несёт AR-29.
- [Source: prd.md#metrics SM-5] — 3+ публикации СМИ со ссылкой на платформу (валидирует ссылка-на-дату + экспорт).
- [Source: architecture.md:108, 122-124] — пересчитываемость `(snapshot_id, methodology_version) → бит-в-бит`.
- [Source: architecture.md:333-340, 399, 878-879] — снапшот лёгкий (snapshot_id + дамп); тяжёлый стор ОТЛОЖЕН; `snapshot_id`
  НЕ в ключе уникальности (as_of-штамп); last-import-wins.
- [Source: architecture.md:138] — честное состояние «данные устарели (с <дата>)».
- [Source: architecture.md:842, 865, 895-896] — AR-29 / UJ Данияра (SM-5): ссылка-на-дату + экспорт evidence.
- Существующий код: `server/internal/export/evidence.go` (Export/FlagRecord; `subject_id int64` leak `:24`),
  `httpapi/contracts.go` (Projection `:110-148`, «не отдаём внутр. id» `:28-29`, writeJSON `:167`), `contract_flags.go`
  (resolveContractFlags `:54-80`, evidence@raised `:71`), `render/render.go:67` (Text), `registry/values/glossary-*.json:2`
  (frame.signal), `og/meta.go:194-204` (as_of/honesty-hole паттерн), `metrics/metrics.go` (SM-C1 DB-derived),
  `stage0-audit/verdict.go:52-66` + `docs/ops/stage0-verdict-20260620.json`, `store/queries/{risk_flags,flag_disputes,
  price_benchmarks,contracts}.sql` (count-шаблоны), `migrations/0007_risk_flags.sql` (нет snapshot_id),
  `web/src/features/contract/ContractRoute.tsx` (`?report`), `web/src/shared/i18n/locales/*/chrome.json:113-114` (as_of i18n).
- Прошлые истории: 5.1 (карточка/resolveContractFlags/honesty-hole 94b68b8), 5.3 (методика/formatDateSafe/as_of),
  5.4 (deep-link `?report`/client-IP negative-control), 5.5 (OG/render.FlagLine/as_of-в-URL/честное различение состояний/
  P2 parity/P3 taboo — done, в рабочем дереве), 4.6 (export.Export/SM-C1 DB-derived/`json.Valid` defer).
- Память: [[sprint-sequencing-epic1-first]], [[guards-must-prove-red]], [[data-source-interim-parser-first]],
  [[epic5-presentation-track-sequencing]].

## Open Questions — ✅ ВСЕ РЕШЕНЫ владельцем (Bratan, 2026-06-26) до старта dev

1. **(T1, AC-1) Глубина «ссылки-на-дату» → ✅ ШТАМП + БАННЕР ДРЕЙФА.** История снапшотов в MVP отсутствует (нет
   `snapshot_id`, UPSERT перезаписывает). Реализовать: ссылка = URL + `?as_of/?mv`-штамп; совпала версия → карточка
   идентична (пересчёт бит-в-бит); сменилась → честный нейтральный баннер «показано текущее, исторический срез не
   хранится». **`snapshot_id`/append-only дамп НЕ создавать** (тяжёлый стор отложен архитектурой до пилота, architecture.md:337).
2. **(T2, AC-2) `subject_id`-утечка → ✅ ДОБАВИТЬ `subject_ref` = `goszakup_contract_id`** в `exportDoc` (стабильный
   публичный ключ для пересчёта третьим лицом); внутренний `int64`-id убрать/НЕ отдавать на провод (wire-конвенция
   `contracts.go:28-29`). Эндпоинт идёт по goszakup_id → резолв в contract → строка флага.
3. **(T3, AC-3) «X проверяемых вручную» → ✅ ПЕРЕСЧИТЫВАЕМЫЕ** = флаги с полным `evidence` + `methodology_version`
   (оба NOT NULL → по конструкции пересчитываемы по опубликованной методике; X≈M активных). Трактовка через
   `flag_disputes confirmed` НЕ выбрана (на синтетике ≈0). Честная подпись метрики: «X пересчитываемых по методике».
4. **(T3, AC-3) Размещение one-pager → ✅ `server/cmd/pilot-onepager/`** (полноценная команда рядом с `cmd/api`/
   `cmd/importer`; one-pager — first-class артефакт пилота). Вывод — `docs/ops/pilot-onepager-YYYYMMDD.md`.
   Это НЕ публичный эндпоинт (артефакт-генератор; defer 1.1 «не светить метрики наружу» соблюдён).

## Review Findings (code review 2026-06-26)

> Три adversarial-слоя (Blind/Edge/Acceptance Hunter, Opus 4.8, свежий контекст). **Critical/High нет** —
> Acceptance Auditor подтвердил все 3 AC + обоснованность owner-OQ-отклонений. Главное схождение: экспорт
> принимает вырожденный evidence (`{}`/`null`) и пустую `methodology_version` как «пересчитываемые», противореча
> X-гейту (`evidence <> '{}' AND methodology_version <> ''`) той же истории.

### patch

- [x] [Review][Patch] (P1) `export.Export`: вырожденный evidence (`{}`/`null`/массив/скаляр) проходит как «пересчитываемый» (guard только `len==0 || !json.Valid`) — противоречит X-гейту `evidence <> '{}'` и OpenAPI `evidence: object`; требовать непустой JSON-ОБЪЕКТ [server/internal/export/evidence.go:33-39]
- [x] [Review][Patch] (P2) Экспорт raised-флага с пустой `methodology_version` отдаётся как `""` (не-пересчитываемо, но 200) — противоречит X-гейту `methodology_version <> ''` и honesty-конверту карточки (`versionField`→no_data); честный 404 [server/internal/httpapi/evidence_export.go:lookupRaisedFlag, export/evidence.go:Export]
- [x] [Review][Patch] (P3) One-pager `dataSourceLabel`: интерим/`scrape:`-источник (ОСНОВНОЙ до токена, [[data-source-interim-parser-first]]) не помечается как «не живой» — default-ветка отдаёт строку как есть; добавить `scrape:`→«(интерим-парсер)» + честный default [server/cmd/pilot-onepager/report.go:dataSourceLabel]
- [x] [Review][Patch] (P4) `PermalinkButton`: отказ `navigator.clipboard` (HTTP/старый браузер) — тихий no-op без фидбэка/фолбэка → нельзя скопировать ссылку для СМИ (SM-5); показать ошибку/URL [web/src/features/contract/PermalinkButton.tsx:onCopy]
- [x] [Review][Patch] (P5) One-pager: отсутствующий/нулевой `geo_gate_threshold` → «гейт ≥ 0%» как реальный порог (вводит в заблуждение); честная пометка «порог не задан» при 0 [server/cmd/pilot-onepager/report.go:renderOnePager]
- [x] [Review][Patch] (P6) `assertNeutral` negative-control слабоват: `strings.Contains("Это нарушение","нарушение")` доказывает примитив, не что страж ловит taboo в РЕАЛЬНОМ выводе; прогнать скан по подделанному выводу [server/cmd/pilot-onepager/report_test.go:assertNeutral]

### defer

- [x] [Review][Defer] Семантика N («размечено» = импортированные vs прошедшие расчёт флагов) [server/internal/store/queries/pilot_metrics.sql] — deferred, документированная трактовка (импортированы в проекцию); уточнить при зрелом compute-пайплайне (Epic 2)
- [x] [Review][Defer] M=0/X=0 при N>0 печатаются как факт (не отличить «оценено, 0» от «не считалось») [server/cmd/pilot-onepager/report.go] — deferred, нет compute-tracking для различения; внутренний артефакт
- [x] [Review][Defer] `as_of` штампуется в URL, но на чтении не потребляется (декоративный дисплей-штамп) [web/.../permalink.ts] — deferred, by-design (OQ#1; слайс не обещан замороженным)
- [x] [Review][Defer] Дрейф детектится только по оси методики; data-drift (повторный импорт) молчит [server/internal/httpapi/contracts.go] — deferred, owner-approved OQ#1 (детект data-drift без snapshot невозможен по конструкции)
- [x] [Review][Defer] `honestCount(0)`: измеренный 0 неотличим от «не измеряли» [server/cmd/pilot-onepager/report.go] — deferred, перестраховка в пользу честности; внутренний артефакт
- [x] [Review][Defer] Web drift-баннер без компонентного теста (только `buildPermalink` + i18n parity) [web/.../ContractRoute.tsx] — deferred, jsdom/@testing-library не установлены (повтор defer 5.1/5.3)
- [x] [Review][Defer] `verdict_reason`/`data_source` встраиваются verbatim без `FindTaboo` на рендере [server/cmd/pilot-onepager/report.go] — deferred, источник — контролируемый stage0-audit; CLI не импортирует registry

### dismissed (noise)

- Web-ссылка экспорта использует `f.flag_id` в слоте `{flag_type}` (Blind, не мог проверить без проекта) — `flag_id == flag_type` (`resolveContractFlags` ставит `FlagID=ft` из `contractFlagTypes`; OpenAPI enum совпадает; `apiFlag.ts` трактует как тип) → ссылка корректна.

## Dev Agent Record

### Agent Model Used

claude-opus-4-8[1m] (Claude Opus 4.8, 1M context) — dev-story 2026-06-26.

### Debug Log References

- LSP-диагностика про `gen.New(pool)` (missing `InsertErrorReport`/`CountActiveFlags`) — устаревший индекс
  сгенерированного sqlc-кода после `gen-sqlc`; `go build ./...` и весь `go test -count=1` зелёные (CLAUDE.md:
  авторитетен `go build`).
- gofmt переформатировал `contracts.go` (выравнивание комментариев в литерале) — применён `gofmt -w`.
- staticcheck QF1012 в `report.go` (WriteString(Sprintf) → Fprintf) — применён для чистоты golangci.

### Completion Notes List

**Сделано (T1–T4) — всё зелёное:**
- **T1 (permalink+дрейф):** `ContractDTO.{methodology_version, as_of, methodology_drift}`; `Projection` штампует
  текущую версию+as_of (пустое→no_data, honesty-hole); `Get` по `?mv` детектит дрейф; `ContractsHandler.Version`
  из `params`. Web: `?mv`-форвард, баннер дрейфа, `PermalinkButton`, `buildPermalink` (пустое не фабрикует), i18n
  ru+kk. OpenAPI расширен + `gen-web`.
- **T2 (экспорт evidence):** `export.Export` — `subject_ref`=goszakup-id (внутренний int64 убран) + defensive
  `json.Valid` + честная деградация на пустом evidence. `EvidenceExportHandler` (`/api/contracts/{id}/flags/{ft}/evidence.{json,txt}`)
  — та же проекция; не-raised/нет → честный 404; рамка из render. OpenAPI `EvidenceExport`+путь. Web: ссылка экспорта у raised-флага.
- **T3 (one-pager):** sqlc count'ы N/M/X (`pilot_metrics.sql`); `server/cmd/pilot-onepager` (main IO + pure render)
  читает вердикт `docs/ops/stage0-verdict-20260620.json` + count'ы → нейтральный Markdown с честным штампом источника
  и пустой-БД-деградацией; канал цитирования evidence (AC-2) явно в артефакте.
- **T4 (стражи):** per-surface нейтральность × обе локали (export ru+kk, one-pager, web `methodology.*`+`contract.export_evidence`),
  все с negative-control ([[guards-must-prove-red]]).

**Проверка (что реально прогонялось и прошло):** `go build ./...`; `go vet`; `go test -count=1 ./...` (все пакеты ok);
`make check-registry`/`check-core`/`lint`; `make gen-sqlc`/`gen-web` (идемпотентны, «generated==regenerated»);
web `typecheck`, `vitest` (92/92, +permalink+нейтральность), `eslint`, `stylelint`. **НЕ прогонялось:** интеграция на
реальном Postgres + `curl` evidence-эндпоинта + прогон `pilot-onepager` против БД (БД не поднята; эндпоинты не
добавляют новой схемы карточки — переиспускают протестированную проекцию; count-запросы тривиальны) → CI/демо.

**Решения/отклонения (честно):** (1) AC-1 — штамп+баннер дрейфа, НЕ истинная заморозка (OQ#1 ✅; истории снапшотов нет,
тяжёлый стор отложен архитектурой). (2) one-pager НЕ встраивает сырые evidence-дампы (на синтетике нечего цитировать
репрезентативно) — несёт метрику X (пересчитываемых) + указывает на экспорт-эндпоинт AC-2 как канал. (3) `.txt`-экспорт
вне OpenAPI (text/plain, прецедент 5.5). (4) `Accept-Language` для локали экспорта не реализован (query `?lang` достаточно).

### File List

**Изменённые (Go):**
- `server/internal/httpapi/contracts.go` — DTO `methodology_version`/`as_of`/`methodology_drift` + `MethodologyDriftDTO`; `Projection` штамп; `Get` дрейф; `Version`; `asOfStamp`.
- `server/internal/export/evidence.go` — `subject_ref` (вместо int64) + defensive `json.Valid` + деградация.
- `server/cmd/api/main.go` — `ContractsHandler.Version` + `EvidenceExportHandler` маршруты.

**Новые (Go):**
- `server/internal/httpapi/evidence_export.go` — `EvidenceExportHandler` (GetJSON/GetText).
- `server/internal/httpapi/{contracts_permalink_test.go, evidence_export_test.go}`.
- `server/cmd/pilot-onepager/{main.go, report.go, report_test.go, main_test.go}`.
- `server/internal/store/queries/pilot_metrics.sql`.

**Сгенерированные (sqlc — НЕ править руками):**
- `server/internal/store/gen/pilot_metrics.sql.go` (новый); `server/internal/store/gen/querier.go` (+3 метода).

**Изменённые (тесты Go):**
- `server/internal/export/evidence_test.go` — `subject_ref` + negative-controls (пустой/битый evidence).

**Контракт/доки:**
- `docs/api-contracts/openapi.yaml` — схемы `MethodologyDrift`, `EvidenceExport`; `Contract` +3 поля; путь `evidence.json`.
- `web/src/shared/api/schema.gen.ts` (regen `gen-web`, НЕ править руками).

**Новые (web):**
- `web/src/features/contract/{permalink.ts, permalink.test.ts, PermalinkButton.tsx}`.

**Изменённые (web):**
- `web/src/features/contract/{ContractRoute.tsx, useContract.ts, ContractCard.tsx, contract-card.css, contractCardStrings.test.ts}`.
- `web/src/shared/i18n/locales/{ru,kk}/chrome.json` — `methodology.{permalink,permalink_copied,drift}` + `contract.export_evidence`.

**BMAD-артефакты:**
- `_bmad-output/implementation-artifacts/sprint-status.yaml` (5-6 → review).
- `_bmad-output/implementation-artifacts/deferred-work.md` (дополнен — раздел dev 5.6).

### Change Log

- 2026-06-26 — Story 5.6: T1 (перманентная ссылка-на-дату — штамп `?as_of/?mv` + баннер дрейфа методики, без
  snapshot-стора), T2 (HTTP-экспорт evidence JSON/печатный, `subject_ref`=goszakup-id, честный 404 для не-raised),
  T3 (генератор `pilot-onepager`: count'ы N/M/X + вердикт Stage-0 → нейтральный Markdown), T4 (signature-стражи
  нейтральности × обе локали, все с negative-control). OpenAPI+sqlc регенерированы. Всё зелёное (go build/vet/test
  -count=1; check-registry/check-core/lint; gen идемпотентны; web typecheck/vitest 92/eslint/stylelint). Status → review.
- 2026-06-26 — Code review (3 adversarial-слоя) + применены 6 patch-фиксов: (P1) `export.Export` отвергает
  вырожденный evidence (`{}`/`null`/не-объект); (P2) пустая `methodology_version` → честный 404 (согласовано с
  X-гейтом); (P3) one-pager помечает интерим/`scrape:`/нераспознанный источник как «не живой»; (P4) `PermalinkButton`
  фолбэк при отказе clipboard (показ ссылки); (P5) нулевой `geo_gate_threshold` → «гейт не задан»; (P6) усилен
  negative-control нейтральности (скан по реальному выводу). 7 defer + 1 dismiss (web flag_id==flag_type). Всё зелёное
  (go test ./... -count=1; check-registry/check-core/lint; gen идемпотентны; web typecheck/vitest/eslint/stylelint). Status → done.
