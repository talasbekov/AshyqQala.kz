# Deferred Work

## Deferred from: code review of 1-1-монорепо-скелет-и-compose-db (2026-06-21)

- **sqlc-регенерация зависит от распознавания goose `-- +goose Down`** (Story 1.2) — `server/sqlc.yaml` читает `../migrations` целиком; sqlc v1.31 корректно игнорирует Down-секции (проверено: генерация чистая), но это version-хрупкое неявное допущение. В **Story 1.4** добавить CI-страж «generated==regenerated» (уже в плане `ci-registry.yml`), чтобы дрейф sqlc/миграций ловился автоматически.
- **Скомпилированный бинарь `stage0-audit/stage0-audit` отслеживается git** — предсуществующая проблема (не вызвана Story 1.1). Бинарь ~9.6 МБ закоммичен в ранних коммитах; меняется при каждой сборке `stage0-audit`. Вычистить в рамках Story 0.2: `git rm --cached stage0-audit/stage0-audit` + добавить правило в `.gitignore` (например `stage0-audit/stage0-audit`).
