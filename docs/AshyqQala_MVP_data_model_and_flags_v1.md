# AshyqQala.KZ — MVP: модель данных, флаги и зафиксированные решения (v1)

> Вспомогательный документ к use cases v4 и конкурсному документу v2. Закрывает три «блокера быстрого старта MVP», выявленные на сессии анализа полноты: (1) модель данных самого MVP, (2) дефолты флагов/медиан, (3) язык интерфейса. Служит входом для PRD (скоуп = MVP).
>
> Все пороги — **сигнал, требующий проверки**, не «нарушение» (соответствует §4.1 конкурсного документа). Числовые значения — **стартовые, конфигурируемые, калибруются на реальных данных этапа 0**.

---

## 0. Зафиксированные решения

| # | Решение | Значение | Последствие |
|---|---|---|---|
| Язык MVP | Язык гражданского интерфейса | **RU-интерфейс + i18n-каркас и поля `name_kz`/`name_ru` в схеме с первого дня** | KZ включается позже дёшево; правильная оптика для грантов/государства; без дорогой переделки |
| Флаги | Способ фиксации порогов | **Конфигурируемые параметры (`methodology_params`) со стартовыми дефолтами** | UC-11 можно кодить сразу; значения уточняются на этапе 0 |
| Рейтинги | Веса рейтинга (§4.2) | **Не фиксируются на MVP** — это этап 5 | Не блокер MVP; фиксируется в PRD этапа 5 |

---

## 1. Логическая модель данных MVP

Центральный объект карточки проекта = **договор (`contracts`)**. Аккаунтов и ПД-контура нет (соответствует §1.9 конкурсного документа). Двуязычные поля имён — `*_ru` / `*_kz`. PostGIS — для геометрии (`geom`) и геопроверки.

> Допущение: конкретные типы полей, индексы и точные имена полей goszakup ows_v2 уточняются на этапе проектирования БД. Ниже — логическая модель.

### A. Справочники
| Таблица | Ключевые поля | Назначение |
|---|---|---|
| **districts** | `id`, `kato_code`, `name_ru`, `name_kz`, `geom`(polygon) | Районы Астаны (Алматы, Сарыарка, Есиль, Байконыр, Нура). Привязка и агрегаты |
| **methodology_params** | `key`, `value`, `description`, `version`, `effective_from` | Конфиг порогов флагов/медиан (раздел 2) |

### B. Организации (юрлица)
| Таблица | Ключевые поля | Назначение |
|---|---|---|
| **organizations** | `id`, `bin`(uniq), `name_ru`, `name_kz`, `reg_kato`, `is_customer`, `is_supplier`, `first_seen_at`, `source_url` | Заказчики и подрядчики в одной таблице (БИН — ключ) |
| **org_name_aliases** | `id`, `organization_id`(FK), `raw_name`, `source`, `resolve_status`(auto/manual/conflict) | Нормализация наименований (UC-09): варианты → канонический БИН |

### C. Закупки и договоры
| Таблица | Ключевые поля | Назначение |
|---|---|---|
| **announcements** (`trd_buy`) | `id`, `goszakup_id`, `title_ru`, `title_kz`, `customer_org_id`(FK), `method`, `publish_date`, `status`, `source_url` | Объявления — нужны для флага «единственный участник» |
| **participants** | `id`, `announcement_id`(FK), `organization_id`(FK), `is_winner` | M:N объявление↔участник; счётчик участников |
| **lots** | `id`, `announcement_id`(FK,null), `goszakup_lot_id`, `title_ru`, `title_kz`, `amount`, `quantity`, `unit`, `kato_code` | Лоты |
| **contracts** ⭐ | `id`, `goszakup_contract_id`, `lot_id`(FK,null), `customer_org_id`(FK), `supplier_org_id`(FK), `subject_ru`, `subject_kz`, `amount`, `sign_date`, `plan_start`, `plan_end`, `status`, `direction`(road/water/other), `kato_code`, `source_url`, `is_deleted`, `imported_at`, `updated_at` | Центральный объект карточки проекта |
| **acts** | `id`, `contract_id`(FK), `goszakup_act_id`, `act_date`, `signer_info`, `source_url` | Акты приёмки (UC-02 A1) |

### D. Геопривязка (UC-10)
| Таблица | Ключевые поля | Назначение |
|---|---|---|
| **geo_objects** | `id`, `contract_id`(FK), `geom`(POINT\|LINESTRING), `district_id`(FK), `address_text`, `length_km`, `geocode_status`(auto/manual/unmatched), `confidence`, `geocoded_by`, `geocoded_at` | Точка/полилиния объекта. `length_km` → цена за км |

### E. Флаги и метрики (UC-11)
| Таблица | Ключевые поля | Назначение |
|---|---|---|
| **risk_flags** | `id`, `flag_type`, `subject_type`(contract/contractor), `contract_id`(null), `organization_id`(null), `severity`, `evidence`(jsonb), `is_active`, `detected_at`, `cleared_at`, `methodology_version` | 4 автофлага. `evidence` + `methodology_version` → пересчитываемость третьим лицом (§4.1) |
| **rnu_entries** | `id`, `organization_id`(FK), `goszakup_rnu_id`, `start_date`, `end_date`(null), `reason_ref`, `source_url` | Реестр недобросовестных; авто-снятие флага по `end_date` |
| **price_benchmarks** | `id`, `comparability_key`, `median_price_per_km`, `sample_size`, `computed_at` | Кэш медиан ₸/км (флаг 2 + страница района UC-05) |

### F. Импорт и подписки
| Таблица | Ключевые поля | Назначение |
|---|---|---|
| **import_runs** | `id`, `started_at`, `finished_at`, `status`, `counts`(jsonb), `error` | Прогоны импорта (UC-08) |
| **import_journal** | `id`, `entity_type`, `entity_external_id`, `change_type`(create/update/delete), `processed_at`, `status` | Инкрементальность через `/v2/journal` |
| **subscriptions** | `id`, `telegram_chat_id`, `district_id`(FK), `is_active`, `created_at` | Подписки на район (UC-06) |
| **notifications_outbox** | `id`, `subscription_id`(FK), `event_type`, `payload`, `sent_at`, `status` | Очередь уведомлений боту + антифлуд |

---

## 2. Дефолты флагов и медианы (UC-11)

Все значения — стартовые, в `methodology_params`, калибруются на данных этапа 0.

| Параметр (`methodology_params.key`) | Дефолт | Логика |
|---|---|---|
| `flag.single_participant.enabled` | `true` | Флаг 1: участников в объявлении = 1 |
| `flag.single_participant.exclude_methods` | `[из_одного_источника, …]` | Не сигналить там, где единственный участник законен по способу закупки |
| `median.comparability_key` | `direction × kato × период(24 мес, скользящее)` | Группа сопоставимости для медианы и флага 2 |
| `median.min_sample` | `5` | Меньше выборки → медиана/флаг не строятся, показываем «недостаточно сопоставимых данных» |
| `flag.price_per_km.deviation_factor` | `1.5` | Флаг 2: цена/км > медиана × 1.5. Простая объяснимая формула (§4.1). *Альтернатива: median + 3×MAD — точнее, менее наглядно* |
| `flag.monopoly.concentration_share` | `0.5` | Флаг 3: доля одного БИН по сумме ₸ в группе `kato × direction` ≥ 50% |
| `flag.monopoly.min_group_contracts` | `5` | Не сигналить монополию на крошечной выборке |
| `flag.rnu.enabled` | `true` | Флаг 4: БИН активен в `/v2/rnu` (`start_date ≤ today`, `end_date` пуст/в будущем); авто-снятие по `end_date` |

**Заложенные решения (требуют подтверждения на данных этапа 0):**
- Медиана и флаг цены — по **скользящему окну 24 мес** (горизонт «контракты за 2 года», §2).
- Для дорог цена/км — из `geo_objects.length_km`; если длины нет → флаг 2 не строится, показываем «недостаточно данных».
- Простая формула отклонения (× 1.5) вместо статистической (MAD) — ради публичной объяснимости.

---

## 3. Открытые микровопросы (не блокеры MVP, но проговорить)

- **`telegram_chat_id`** в `subscriptions` — единственный «околоПД» элемент MVP. Верификации/ИИН нет, контур ПД на этапе 5, но хранение chat_id стоит проговорить с юристом.
- **Точность публичной геометки** дорог-полилиний (как и для этапа 5) — уровень детализации.
- **`signer_info`** в `acts` — публикуем как служебную информацию должностного лица (ФИО/должность) согласно §6.2.4, не как частные данные.

---

*Версия v1. Источник решений: сессия анализа полноты документации (фасилитатор-режим). Числовые дефолты подлежат калибровке на этапе 0 (аудит данных).*
