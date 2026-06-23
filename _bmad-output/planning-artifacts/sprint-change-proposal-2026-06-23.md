# Sprint Change Proposal — Токен-независимый фундамент Epic 2 (2026-06-23)

**Тип изменения:** Direct Adjustment (Option 1) · **Scope:** Moderate (backlog-реорганизация) · **Статус:** одобрено владельцем (Bratan).
**Прецедент:** трек «Парсер-мост» (Sprint Change Proposal 2026-06-20) — аналогичное вырезание токен-независимого среза.

## §1. Issue Summary (триггер)

**Выявлено** на ретроспективе Epic 1 (2026-06-23) как «значимое открытие»: **Epic 2 нельзя стартовать в текущем состоянии.**
- Story 2.1 «Живая граница декодирования ows_v2 + schema_hash» требует `GOSZAKUP_TOKEN` (живой ows_v2) И боевого `Source` + проекции `lots`.
- Токен-независимый фундамент «размазан»: проекция `lots` помечена «1.2-расширение / B-3»; `Source`-интерфейс зашит в 2.1; `internal/{goszakup,store/projection}` — пустые заглушки.
- Тот же фундамент — **предусловие (Task 0) заблокированной Story 0.6** «Парсер-мост» (она HALT-ила дважды именно на отсутствии проекции `lots` + `Source`).

**Тип:** техническая последовательность, выявленная при реализации (НЕ смена требований, НЕ провал подхода).

**Evidence:** dev-story HALT 0.6 (2026-06-22/23) на Task 0; ретро Epic 1 §«Значимое открытие»; `decode`+`schema_hash` уже реализованы в Story 1.10 (Story 2.1 ссылается «из 1.10»), т.е. недостаёт именно `Source` + `lots`.

## §2. Impact Analysis

- **PRD / MVP:** не затронут. Это пере-секвенирование; FR-1/FR-2/FR-3 остаются за Epic 2, скоуп MVP не меняется.
- **Epic 2:** завершается как планировалось; добавляется одна предшествующая история (2.0); Story 2.1 слегка сужается (становится «ows-реализация Source поверх 2.0»).
- **Epic 0:** Story 0.6 (Парсер-мост) разблокируется — её Task 0 (проекция `lots` + `Source`) закрывается Story 2.0.
- **Архитектура:** согласуется без изменений — `Source` как точка swap (AR-27: `goszakup` импортируется только из `ingest/decode`); проекционные ⊥ кураторские таблицы (AR-4); `decode`+`schema_hash` готовы (Story 1.10).
- **UX:** N/A (бэкенд).
- **Вторичные артефакты:** миграция `lots` → sqlc-генерация требует Docker (memory: локально `POSTGRES_PORT=55432`, Docker Hub иногда требует ретрая). go-list-страж ядра/границ (Story 1.10) расширяется на новые пакеты.

## §3. Recommended Approach — Option 1: Direct Adjustment

Добавить **одну** новую историю в Epic 2 (Story 2.0), разблокирующую и 2.1, и 0.6. Риск **Low**, эффорт **Medium**. НЕ rollback (нечего откатывать), НЕ MVP-review (скоуп не меняется). Токен-независимо (нужен только Docker для sqlc/integration).

## §4. Detailed Change Proposal

### Новая история — Story 2.0

**Story 2.0: Токен-независимый фундамент — Source-интерфейс, file-источник, проекция `lots`**

_As a команда, I want зафиксировать `Source`-интерфейс (точка swap scrape→ows|file) + file-реализацию + проекцию `lots`, so that живой импорт (Story 2.1) и трек «Парсер-мост» (Story 0.6) разблокированы без `GOSZAKUP_TOKEN`._

- **AC1 — `Source`-интерфейс:** `internal/goszakup` определяет `Source` (`Name() string`, `Fetch(resource string, scopeBINs []string, max int, handle func([]map[string]any) error) error`) — единая точка swap `scrape→ows|file`. Сигнатура совпадает с `stage0-audit/source.go` (переиспользовать концепт, не изобретать; модуль не импортируется). go-list-граница: `goszakup` импортируется ТОЛЬКО из `ingest/decode` (AR-27).
- **AC2 — file-реализация `Source`:** читает локальные JSON-дампы `<data-dir>/<resource>.json` (как `stage0-audit -source file`); токен НЕ нужен. ows-реализация (живая) — Story 2.1.
- **AC3 — проекция `lots`:** миграция `migrations/0003_*.sql` — проекционная таблица `lots` (минимум колонок: `id`, `announcement_id` FK null, `goszakup_lot_id`, `title_ru`, `title_kk`, `amount`, `quantity`, `unit`, `kato_code`; **БЕЗ flags/geo**), + доступ `internal/store/projection` + sqlc-генерат. Проекционная ⊥ кураторская (AR-4).
- **AC4 — границы/тесты:** go-list-страж (проекция/goszakup не нарушают границ); golden/юнит на file-`Source` и декод `lots` через `ingest/decode`; `go test ./...` зелёный; честные состояния (нет данных vs пусто).

**Слот:** Epic 2, перед Story 2.1 (предшествующий фундамент). Ключ sprint-status: `2-0-фундамент-source-file-источник-и-проекция-lots`.

### Правка существующей — Story 2.1 (сужение)

Story 2.1 строит **ows-реализацию** `Source` (живую, нужен `GOSZAKUP_TOKEN`) ПОВЕРХ интерфейса из Story 2.0 + `decode`/`schema_hash` из Story 1.10. Интерфейс `Source` и проекция `lots` в 2.1 НЕ определяются заново (они из 2.0).

### Правка sprint-status.yaml

- Добавить `2-0-фундамент-source-file-источник-и-проекция-lots: backlog` в Epic 2 (перед 2-1).

## §5. Implementation Handoff

**Scope: Moderate** (backlog-реорганизация). Маршрут: **PO/DEV**.
1. `create-story` для `2-0` (контекст-движок — выявит точные зависимости/Docker-предусловия).
2. `dev-story` для `2-0` (токен-независимо; нужен Docker для sqlc-генерации миграции `lots` + integration-тестов).
3. После `2-0` → разблокируется Story 0.6 (Task 0 закрыт) и Story 2.1 (остаётся блокер только на токене для ЖИВОГО импорта).

**Success criteria:** `Source`-интерфейс + file-impl + проекция `lots` существуют; 0.6 проходит Task 0; 2.1 имеет интерфейс для ows-реализации; гейты `ci-server` зелёные.

**Остаточный блокер (не входит в 2.0):** живой импорт (Story 2.1+) по-прежнему требует `GOSZAKUP_TOKEN` (человеко-операционный, как 0-1).
