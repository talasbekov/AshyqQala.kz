# Deferred Work

## Deferred from: code review of 1-1-монорепо-скелет-и-compose-db (2026-06-21)

- **`/metrics` на публичном listener без гейтинга** (Story 1.3) — `cmd/api` отдаёт `/metrics` (promhttp: go-runtime-внутренности, позже SM-C1/SM-C2) на том же публичном listener, что и `/api/*`. В **Story 1.7** (Caddy reverse-proxy) ограничить путь `/metrics` (allow только внутренние IP) ИЛИ вынести на отдельный admin-listener.
- **`amount_tng` — целые тенге (решение владельца, Story 1.3)** — в **Epic 2** (живой импорт) импортёр обязан: (а) валидировать/округлять дробные суммы goszakup, если такие встречаются; (б) guard на overflow BIGINT (`>9.2e18`). Сейчас S-0/синтетика — целые тенге.
- **sqlc-регенерация зависит от распознавания goose `-- +goose Down`** (Story 1.2) — `server/sqlc.yaml` читает `../migrations` целиком; sqlc v1.31 корректно игнорирует Down-секции (проверено: генерация чистая), но это version-хрупкое неявное допущение. В **Story 1.4** добавить CI-страж «generated==regenerated» (уже в плане `ci-registry.yml`), чтобы дрейф sqlc/миграций ловился автоматически.
- **Скомпилированный бинарь `stage0-audit/stage0-audit` отслеживается git** — предсуществующая проблема (не вызвана Story 1.1). Бинарь ~9.6 МБ закоммичен в ранних коммитах; меняется при каждой сборке `stage0-audit`. Вычистить в рамках Story 0.2: `git rm --cached stage0-audit/stage0-audit` + добавить правило в `.gitignore` (например `stage0-audit/stage0-audit`).

## Deferred from: code review of 1-4-registry-двухосевой-enum-честных-состояний-render-каркас (2026-06-22)

- **Матчер нейтральности однонаправленный** (`server/internal/registry/neutrality.go`) — `FindTaboo` проверяет лишь ОТСУТСТВИЕ taboo-строк, но не ПРИСУТСТВИЕ обязательной нейтральной рамки в выводе. Архитектура (doc-нейтральность) предполагает двунаправленный матчер. Сейчас рамка `frame.signal` структурно гарантирована в `render.Render`, поэтому риск низкий. Активировать в **Epic 4**, когда появятся per-flag шаблоны прозы (текст уже не сводится к одной рамке).
