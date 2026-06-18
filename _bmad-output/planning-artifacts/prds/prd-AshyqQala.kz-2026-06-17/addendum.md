# Addendum — AshyqQala.kz MVP PRD

Технический «как», вынесенный из PRD (capability-level). Питает архитектуру (`bmad-create-architecture`), не дублируется в `prd.md`.

## 1. Технологический стек (стартовый, §3.3 конкурсного документа)

- **Backend:** Go 1.25 + chi; pgx + sqlc.
- **БД:** PostgreSQL 16 + PostGIS (геометрия `geom`, геопроверка/расстояния).
- **Frontend:** React + Vite + MapLibre GL (mobile-first веб).
- **Админка:** Directus поверх Postgres — очереди геопривязки (FR-5) и нормализации (FR-2), без отдельной разработки бэк-офиса.
- **Инфра:** Caddy, Docker Compose на одном VPS (РК); CI/CD — GitHub Actions.
- **Наблюдаемость:** structured logging (slog) + `/metrics` (NFR-4).

**Отложено до реальной нагрузки/команды эксплуатации:** Kafka/Redpanda, ClickHouse, Redis, Kubernetes, gRPC, полный Prometheus/Grafana/OTel, Terraform.

## 2. Модель данных MVP (ссылка)

Логическая модель MVP зафиксирована отдельно: `docs/AshyqQala_MVP_data_model_and_flags_v1.md`. Ключевое:
- Центральный объект — `contracts`; организации в одной таблице `organizations` (БИН — ключ, `is_customer`/`is_supplier`).
- Геопривязка — `geo_objects` (POINT|LINESTRING, `length_km` для цены/км, `district_id`).
- Флаги — `risk_flags` (`evidence` jsonb + `methodology_version` → пересчитываемость), `rnu_entries`, кэш медиан `price_benchmarks`.
- Импорт/подписки — `import_runs`, `import_journal`, `subscriptions`, `notifications_outbox`.
- Двуязычие — поля `name_kz`/`name_ru`, `subject_kz`/`subject_ru` с первого дня (решение по языку).

## 3. Дефолты методики флагов (ссылка)

Полная таблица — в `docs/AshyqQala_MVP_data_model_and_flags_v1.md` §2. Все значения — конфиг (`methodology_params`), калибруются на этапе 0:
- `median.comparability_key = direction × kato × период(24 мес, скользящее)`, `median.min_sample = 5`.
- `flag.price_per_km.deviation_factor = 1.5` (простая объяснимая формула; альтернатива — median + 3×MAD).
- `flag.monopoly.concentration_share = 0.5`, `flag.monopoly.min_group_contracts = 5`.
- `flag.single_participant` и `flag.rnu` — бинарные, с исключением законных способов из одного источника.

## 4. Заложенные решения и рассмотренные альтернативы

- **Язык MVP:** RU + i18n-каркас (вместо «KZ+RU сразу» — дороже по времени на старте; вместо «только RU без задела» — дорогая переделка под KZ потом).
- **Формула отклонения цены:** простой множитель ×1.5 ради публичной объяснимости (§4.1) вместо статистически более точного MAD.
- **Окно медианы:** скользящие 24 мес — согласовано с горизонтом «контракты за 2 года» (§2).
- **Геопривязка дорог:** полилиния (`length_km`) как основа цены/км; без длины флаг цены не строится (а не выдумывается).

## 5. Привязка к спринтам (§3.4, ориентир)

- Нед. 1–2: схема БД, пайплайн импорта (FR-1), нормализация (FR-2); каркас фронта и карта (FR-7).
- Нед. 3–4: Карточки (FR-10…FR-14), медианы (FR-18), геопривязка через Directus (FR-5). **Нед. 3 — Go/No-Go-гейт (FR-6).**
- Нед. 5–6: 4 флага (FR-19…FR-23), страницы районов (FR-17), Telegram-бот (FR-24…FR-26), SEO/шаринг (FR-27).
- Нед. 7–8: нагрузочное/приёмочное тестирование (NFR-1), бэкапы, публичный запуск + медиапакет.
