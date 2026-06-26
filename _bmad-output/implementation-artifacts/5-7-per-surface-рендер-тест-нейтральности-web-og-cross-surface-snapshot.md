---
baseline_commit: 42fcc8c  # HEAD на момент создания. ⚠️ 5.5 (done) + 5.6 (done) — в рабочем дереве, НЕ
                          # закоммичены: пакет og/, render.FlagLine, export/, pilot-onepager/, permalink уже на
                          # месте; dev 5.7 строит поверх рабочего дерева. Закоммитить 5.5+5.6 ДО/вместе с 5.7.
context:
  - _bmad-output/planning-artifacts/epics.md
  - _bmad-output/planning-artifacts/architecture.md
  - _bmad-output/planning-artifacts/prds/prd-AshyqQala.kz-2026-06-17/prd.md
---

# Story 5.7: Per-surface рендер-тест нейтральности (web/OG) + cross-surface snapshot

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

> 🧭 **ЧЕСТНАЯ ОЦЕНКА ОБЪЁМА (ключевое — читать первым): 5.7 уже ~90% реализована** стражами Epic 4 (render-
> нейтральность + линтер чисел) + Story 5.5 (OG per-surface + cross-surface golden + web parity, усилены
> review-фиксами P2/P3). Эта история — **тонкая КОНСОЛИДАЦИЯ + закрытие явных пробелов**, НЕ повторная сборка.
> Раздел «УЖЕ СДЕЛАНО» ниже — анти-дублирование: НЕ переписывать перечисленное. См. Open Question #1
> (возможно, владелец захочет закрыть 5.7 как «покрыто» вместо тонкого прохода).
> [[guards-must-prove-red]], [[epic5-presentation-track-sequencing]].

> ✅ **Токен-НЕзависима** (чистые тесты нейтральности/golden на синтетике; рендер токен-агностичен).

## Story

**Как** команда,
**Я хочу** доказать нейтральность на КАЖДОЙ публичной поверхности и что смысловое ядро флага совпадает между
поверхностями,
**Чтобы** флаг нигде (web/OG) не превращался в обвинение и цифры/рамка не расходились между каналами.

Контекст: реализует обязательство Эпика 5 «per-surface рендер-тест нейтральности (web/OG)» + cross-surface
golden (epics.md:531, 1636-1650; architecture.md All-Surfaces 625-632). Замыкает «один render → три адаптера,
источник прозы — один registry». Telegram-плечо — Epic 7 (поверхности нет; 5.7 = web↔OG, задел под тройное).
[Source: epics.md#Epic 5 → Story 5.7 (lines 1636-1650); architecture.md:625-632]

## Acceptance Criteria (дословно из эпика, lines 1644-1650)

### AC-1 — Per-surface рендер-тест нейтральности (web/OG): ноль taboo + рамка-токен
**Given** общий lexicon-контракт (Epic 4)
**When** web и OG рендерят золотой текст флага
**Then** каждый проходит per-surface рендер-тест (ноль taboo, рамка-токен «сигнал, требующий проверки»).

Тестируемо (что ДОЛЖНО быть верно — большей частью УЖЕ верно, см. ниже):
- На КАЖДОЙ поверхности (web, OG) для всех генерируемых флаг-строк × обе локали: `FindTaboo` = 0.
- Рамка-токен `frame.signal` («сигнал, требующий проверки» / «тексеруді талап ететін сигнал») **присутствует**
  (позитивная проверка, не только отсутствие taboo) в raised-выводе каждой поверхности.

### AC-2 — Cross-surface golden snapshot: смысловое ядро совпадает web↔OG
**Given** cross-surface golden snapshot
**When** прогон CI
**Then** смысловое ядро (рамка, число, `flag_id`, `methodology_version`, locale) совпадает между web и OG.

Тестируемо:
- Ядро `{рамка, число, flag_id, methodology_version, locale}` идентично web↔OG, запинено golden-литералом,
  прогоняется в CI. «Число» — N/A в текущей прозе (FlagLine без чисел; линтер запрещает литералы, числа из
  `methodology_params` плейсхолдерами — Epic 4); зафиксировать как явный N/A, не молча.
- Стражи доказывают, что краснеют (negative-control + `-count=1` + golden-литерал).

## УЖЕ СДЕЛАНО (НЕ переписывать — анти-дублирование; источник истины для «что осталось»)

**AC-1 покрыто:**
- **render-слой** (Epic 4 / 1.4): `server/internal/render/render_property_test.go:TestNeutrality_AllFlags_SurfaceAgnostic`
  — ВСЕ flag_type × ВСЕ FlagState × ОБЕ локали: `FindTaboo`=0 **И** вывод НАЧИНАЕТСЯ с рамки (позитивная проверка
  присутствия рамки, `:29-31`). `render_test.go:TestRender_Frame` (`:20,115`) — то же для `Render`/`RenderFlagState`.
- **OG-поверхность** (Story 5.5): `server/internal/og/meta_test.go` — `TestMetaHandler_PerSurfaceNeutrality`
  (build-строки taboo-free), `TestOGProza_AllFlagStates_NoTaboo` (P3: `FlagLine` все флаги×состояния×локали + сводки
  taboo-free + negative-control), `TestMetaHandler_RaisedFlag_OGTags`/`_LocaleRu` (рамка ПРИСУТСТВУЕТ в OG kk/ru).
- **web-поверхность** (Story 5.1/5.5): `web/src/features/contract/contractCardStrings.test.ts` (contract.*+methodology.*
  taboo-free обе локали + parity), `flagGlossaryParity.test.ts` (P2: web↔registry parity name/summary +
  `frame.signal`/`flag.insufficient`).
- **«Число»-страж** (AC-2 «число»): `render_property_test.go:TestNoNumericLiterals_InGlossary` +
  `TestThresholdLiteralLinter_Discriminates` (negative+positive control) — ни одна glossary-строка не несёт литерал-порог.

**AC-2 покрыто:**
- `server/internal/render/flagline_golden_test.go:TestFlagLine_CrossSurfaceGolden` — golden-литерал {summary, рамка,
  flag_id, locale} для raised + not_raised + insufficient × обе локали (== текст web-бейджа, замкнуто parity-тестом).
- `flagGlossaryParity.test.ts` — web-строки == registry (транзитивное web↔OG равенство).
- `meta_test.go:TestMetaHandler_MethodologyVersion_Pinned` — methodology_version запинен на OG-поверхности.

## ЧТО РЕАЛЬНО ОСТАЛОСЬ (тонкая консолидация)

### Tasks / Subtasks

- [x] **T1 — Консолидированный cross-surface контракт-тест (единый источник истины)** (AC: 2)
  - [x] `server/internal/render/cross_surface_test.go:TestCrossSurfaceContract_SemanticCore` — ОДНА явная карта ядра
        `{рамка, число, flag_id, methodology_version, locale}` с коммент-картой «элемент → enforcing-гард»: рамка/flag_id/locale
        → golden-литерал (raised, обе локали); web-равенство → parity; methodology_version → meta-пин; **число → явный N/A**
        (`hasThresholdLiteral` гарантирует отсутствие литерала; числа — Epic 4 плейсхолдерами).
  - [x] Negative-controls: golden-литерал краснеет при расхождении web↔OG; `FindTaboo` краснеет на «нарушение»;
        golden-сравнение не тавтологично (≠ заведомо неверный литерал). `-count=1`.
- [x] **T2 — Явная per-surface проверка присутствия рамки для WEB-бейджа** (AC: 1)
  - [x] `web/src/features/contract/badgeFramePresence.test.ts` — реконструирует raised-бейдж из web-i18n
        (`${summary} — ${frame.signal}`, как `FlagBadge.tsx:29`), утверждает присутствие рамки обе локали + negative-control
        (бейдж без рамки её не содержит). Без jsdom (component-render — defer). Равенство web==registry держит flagGlossaryParity.
- [x] **T3 — Явные N/A + задел Telegram (честность охвата)** (AC: 1, 2)
  - [x] Зафиксировано в коммент-карте `cross_surface_test.go`: «число» — N/A до Epic 4; Telegram-плечо — Epic 7 (5.7 = web↔OG,
        тройное замкнётся на 7.1). Не выдаём частичный охват за полный.
- [x] **T4 — CI-явность cross-surface гейта** (AC: 2 «прогон CI»)
  - [x] Подтверждено: Go-тест гоняется `go test ./...` (`ci-server.yml:38`) + `make check-registry` (render); web-тест —
        `npm run test`/vitest (`ci-web.yml:45`). Обе стороны cross-surface контракта в CI. Задокументировано в комменте теста
        (отдельный CI-шаг не добавлял — избыточно, тест уже в общих гейтах).

## Dev Notes

### Что строить (one-liner ориентир)
НЕ строить заново нейтральность/golden (готово). Сделать ОДИН консолидированный cross-surface контракт-тест,
который явно картографирует ядро `{рамка, число(N/A), flag_id, methodology_version, locale}` на enforcing-гарды;
добавить явную позитивную проверку рамки для web-бейджа (string-тест без jsdom); честно зафиксировать N/A (число —
Epic 4) и Telegram (Epic 7); убедиться в CI-прогоне. Стражи доказанно краснеют.

### REUSE (переиспользовать дословно)
- `server/internal/render/flagline_golden_test.go` (`TestFlagLine_CrossSurfaceGolden` — golden-литерал ядра).
- `server/internal/render/render_property_test.go` (`TestNeutrality_AllFlags_SurfaceAgnostic` — позитив рамки + taboo;
  `TestNoNumericLiterals_InGlossary` — «число»-линтер).
- `server/internal/og/meta_test.go` (`TestOGProza_AllFlagStates_NoTaboo`, `TestMetaHandler_MethodologyVersion_Pinned`).
- `web/src/features/contract/{flagGlossaryParity.test.ts, contractCardStrings.test.ts}` (web parity + neutrality).
- `web/src/features/flag/FlagBadge.tsx:29-31` (сборка текста бейджа — эталон для T2 string-теста).
- `registry/values/glossary-{ru,kk}.json` (`frame.signal`, `flag.*.summary`, `value_state.insufficient_sample`).
- `render.Renderer.{FlagLine, Text}` (`server/internal/render/render.go`), `registry.FindTaboo` (`neutrality.go`),
  `registry.AllLocales()/AllFlagStates()`.
- CI: `.github/workflows/{ci-server.yml, ci-web.yml, ci-registry.yml}`.

### NEW (создать — тонко)
1. Консолидированный cross-surface контракт-тест (Go) — единая карта ядра → enforcing-гарды (T1). Может быть тонким:
   golden-литерал ядра + утверждения, ссылающиеся на parity/version/линтер; либо doc-комментарий-контракт + assert'ы.
2. Web string-тест присутствия рамки в собранном тексте бейджа (T2) — без jsdom.
3. (Опц.) Явный CI-шаг/коммент cross-surface (T4), если не выделен.

### Уроки прошлых историй (применить)
- **Страж обязан краснеть** ([[guards-must-prove-red]]): консолидированный тест краснеет при расхождении web↔OG; web-рамка-тест краснеет, если рамка пропала; `-count=1`; golden-литерал, не из кода-под-тестом.
- **Не завышать охват** (5.3/5.5): «число» и Telegram — честные N/A, не молча. Component-render web НЕ покрывается (jsdom/@testing-library не установлены — defer 5.1/5.3); T2 — чистый string-тест, не render.
- **Один источник прозы** (5.5 вариант A): проза только из registry/render; тест НЕ хардкодит лексикон мимо registry (кроме golden-литерала-эталона, который ОБЯЗАН быть литералом).

### Project Structure Notes
- **Backend:** консолидированный тест — `server/internal/render/` (рядом с `flagline_golden_test.go`) ИЛИ `server/internal/og/`.
  Граница импортов (`arch/boundaries_test.go`) не меняется (только тесты). Без новой схемы/миграций.
- **Web:** T2 string-тест — `web/src/features/contract/` (паттерн `flagGlossaryParity.test.ts`).
- **CI:** `.github/workflows/` — при необходимости явный cross-surface шаг.
- ⚠️ **Токен-граница:** чистые тесты на синтетике/registry; реальные данные не нужны.

### Definition of Done
- Backend: `go build ./...` + `go vet` + `go test -count=1 ./...` + `make check-registry`/`check-core`/`lint` зелёные.
- Frontend: `npm run typecheck` + `vitest run` + `eslint` + `lint:css` зелёные.
- Коммит: RU, `Story 5.7: …` + Verification + Отложено; концовка `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`.

## References

- [Source: epics.md#Epic 5 → Story 5.7, lines 1636-1650] — user story + AC дословно.
- [Source: epics.md, line 531] — обязательство Эпика 5 «per-surface рендер-тест нейтральности (web/OG)».
- [Source: architecture.md, lines 625-632] — All-Surfaces: один render → три адаптера; cross-surface golden ядро
  {рамка, число, flag_id, methodology_version, locale} web/OG/Telegram.
- Существующие стражи (источник «что готово»): `render/flagline_golden_test.go`, `render/render_property_test.go`,
  `render/render_test.go`, `og/meta_test.go`, `web/.../flagGlossaryParity.test.ts`, `web/.../contractCardStrings.test.ts`.
- Прошлые истории: 5.5 (OG + cross-surface golden + web parity, review P2/P3); 4.x (render-нейтральность + линтер чисел).
- Defer (Telegram-плечо → 7.1; component-render web → jsdom): `deferred-work.md` (dev 5.5, code review 5.1/5.3).
- Память: [[guards-must-prove-red]], [[epic5-presentation-track-sequencing]], [[sprint-sequencing-epic1-first]].

## Open Questions — ✅ РЕШЕНЫ владельцем (Bratan, 2026-06-26)

1. **Объём 5.7 → ✅ ТОНКИЙ DEV-ПРОХОД (вариант а).** Сделать консолидированный cross-surface контракт-тест +
   web-рамка-string-тест + явные N/A (число/Telegram) + CI-явность (T1–T4). НЕ переписывать готовое (раздел «УЖЕ
   СДЕЛАНО»). Ценность: single-source-of-truth карта контракта для будущего Telegram-плеча (7.1).
2. **Форма консолидированного теста (T1) → ✅ Go-ТЕСТ С GOLDEN-ЛИТЕРАЛОМ ядра + assert'ы на enforcing-гарды**
   (исполняемый, краснеет при дрейфе web↔OG). Не doc-комментарий.

## Review Findings (code review 2026-06-26)

> Три adversarial-слоя (свежий контекст). Багов продукта нет (продукт не менялся), но найдены РЕАЛЬНЫЕ слабости
> самих стражей: web-тест тавтологичен, «cross-surface» не сравнивает поверхности напрямую, CI без `-count=1` для
> glossary-читающих golden (кэш-маскировка), KK-taboo без negative-control. Раздел «УЖЕ СДЕЛАНО» и N/A подтверждены честными.

### patch

- [x] [Review][Patch] (P1) `badgeFramePresence.test.ts` тавтологичен: собирает `${summary} — ${frame}` и проверяет вхождение `frame` (всегда true); FlagBadge не читается → не ловит реальную регрессию. Заменить на сверку собранного бейджа с ТЕМ ЖЕ golden-литералом, что Go-тест → реальный web↔OG пин (закрывает и «нет прямого cross-surface сравнения») [web/src/features/contract/badgeFramePresence.test.ts]
- [x] [Review][Patch] (P2) CI гоняет golden/нейтральность-тесты, читающие glossary с ДИСКА (`os.ReadFile`, не embed), БЕЗ `-count=1` → PR, меняющий только `glossary-*.json`, может пройти из кэша (`ok cached`) с битым golden/taboo. Добавить `-count=1` в `check-registry` (несущий урок [[guards-must-prove-red]]); T4 ошибочно счёл шаг избыточным [Makefile check-registry / .github/workflows/ci-registry.yml]
- [x] [Review][Patch] (P3) KK-ветка taboo без negative-control: `FindTaboo(KK, got)` гоняется, но краснота доказана только для RU («нарушение»). Добавить KK negative-control («бұзушылық») [server/internal/render/cross_surface_test.go]
- [x] [Review][Patch] (P4) Консолидированный golden — только `raised`; не-raised ядро (`summary: недостаточно...`) живёт лишь в `flagline_golden_test.go`. Добавить not_raised/insufficient кейсы (golden+taboo; рамка-позитив только для raised) — «единая карта» честно покрывает состояния [server/internal/render/cross_surface_test.go]
- [x] [Review][Patch] (P5) Go negative-control 2 слаб (`FlagLine(...) == "Зафиксировано нарушение"` — тривиально false, не задействует golden-цикл). Усилить: мутировать golden-литерал → доказать, что `!=` ловит расхождение [server/internal/render/cross_surface_test.go]
- [x] [Review][Patch] (P6) Завышенная формулировка «single-source-of-truth: для КАЖДОГО элемента ЭТОТ тест» — версия/web-равенство лишь делегированы коммент-картой. Смягчить до честного «часть энфорсится здесь, часть — ссылки»; заодно фикс цитаты `FlagBadge.tsx:29`→`:30` [server/internal/render/cross_surface_test.go, badgeFramePresence.test.ts]

### defer

- [x] [Review][Defer] Полноценный component-render web (FlagBadge) — jsdom/@testing-library не установлены [web/.../badgeFramePresence.test.ts] — deferred, повтор defer 5.1/5.3; P1 (badge==golden) закрывает бóльшую часть разрыва без jsdom
- [x] [Review][Defer] Path-filter CI: glossary-only PR не со-триггерит делегированные og/web-стражи (`ci-server` только `server/**`, `ci-web` только `web/**`) [.github/workflows/] — deferred, ПРЕД-СУЩЕСТВУЮЩИЙ инфра-долг (не введён 5.7); P2 (-count=1) частично закрывает кэш-риск

### dismissed (noise)

- `hasThresholdLiteral(got)` избыточен на exact-match golden — оставлен намеренно как «число»-страж; его краснота доказана в `render_property_test.go:TestThresholdLiteralLinter_Discriminates` (тот же пакет, гоняется вместе). Belt-and-suspenders, не дефект.

## Dev Agent Record

### Agent Model Used

claude-opus-4-8[1m] (Claude Opus 4.8, 1M context) — dev-story 2026-06-26.

### Debug Log References

- gofmt переформатировал godoc-комментарий-карту в `cross_surface_test.go` (табы) — применён `gofmt -w`.

### Completion Notes List

**Сделано (T1–T4) — тонкая консолидация (НЕ переписывал готовое, OQ#1 ✅ вариант а):**
- **T1:** `render/cross_surface_test.go:TestCrossSurfaceContract_SemanticCore` — единая исполняемая карта cross-surface
  ядра {рамка, число, flag_id, methodology_version, locale} с коммент-картой «элемент → enforcing-гард». Golden-литерал
  raised обе локали + позитив рамки + ноль taboo + «число» N/A через `hasThresholdLiteral`. 2 negative-control'а
  (taboo краснеет; golden не тавтологичен).
- **T2:** `web/.../badgeFramePresence.test.ts` — позитив присутствия рамки в собранном web-бейдже (из web-i18n, как
  FlagBadge.tsx:29), обе локали + negative-control. Без jsdom (component-render — defer).
- **T3:** явные N/A в коммент-карте: «число» → Epic 4 (плейсхолдеры); Telegram → Epic 7 (5.7 = web↔OG, тройное на 7.1).
- **T4:** CI-прогон подтверждён (go test ./... ci-server + vitest ci-web); задокументировано в комменте, отдельный шаг
  избыточен.

**Что НЕ делал (готово ранее — анти-дублирование):** per-surface taboo OG (`og/meta_test.go` P3), render-нейтральность +
позитив рамки (`render_property_test.go`/`render_test.go`), линтер чисел, cross-surface golden FlagLine
(`flagline_golden_test.go`), web parity (`flagGlossaryParity.test.ts` P2), web neutrality (`contractCardStrings.test.ts`),
methodology_version пин (`meta_test.go`). Эти стражи и есть «90% покрытия» — 5.7 их КОНСОЛИДИРУЕТ, а не повторяет.

**Проверка (всё зелёное):** `go build ./...`; `go test -count=1 ./...`; `make check-registry`/`check-core`/`lint`;
web `typecheck`, `vitest` (93/93, +badgeFramePresence), `eslint`, `stylelint`. Только тесты/доки — кода продукта не
менялось, регрессий нет.

### File List

**Новые (тесты):**
- `server/internal/render/cross_surface_test.go` — консолидированный cross-surface контракт-тест.
- `web/src/features/contract/badgeFramePresence.test.ts` — web позитив присутствия рамки.

**BMAD-артефакты:**
- `_bmad-output/implementation-artifacts/sprint-status.yaml` (5-7 → review).

### Change Log

- 2026-06-26 — Story 5.7: тонкая консолидация (OQ ✅ вариант а). T1 единый cross-surface контракт-тест (Go, карта ядра
  → enforcing-гарды), T2 web позитив рамки (без jsdom), T3 явные N/A (число→Epic 4, Telegram→7.1), T4 CI-прогон подтверждён.
  Готовые стражи 5.5+Epic4 НЕ переписаны. Всё зелёное (go test ./... -count=1; check-registry/check-core/lint; web
  typecheck/vitest 93/eslint/stylelint). Status → review.
- 2026-06-26 — Code review (3 adversarial-слоя) + применены 6 patch-фиксов: (P1) web-тест из тавтологии → сверка
  собранного бейджа с golden-литералом OG (реальный web↔OG пин); (P2) `-count=1` в `check-registry` (кэш-маскировка
  glossary-читающих golden); (P3) kk-ветка taboo negative-control; (P4) консолидированный golden + not_raised/insufficient;
  (P5) усилен golden negative-control (мутация); (P6) честная формулировка карты + цитата `:30`. 2 defer + 1 dismiss.
  Всё зелёное (go test ./... -count=1; check-registry/check-core/lint; web typecheck/vitest 94/eslint/stylelint). Status → done.
