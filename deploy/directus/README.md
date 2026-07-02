# Directus — внутренняя админка курации (Story 3.2)

Кураторская очередь геопривязки (FR-5) поверх **той же** Postgres. Directus видит ТОЛЬКО
кураторские коллекции (`geo_objects`, `districts` — read-only справочник) — граница AR-4;
проекционные таблицы в админку не заводятся.

Полный операционный порядок и чек-лист оператора: **`docs/ops/geocoding.md` →
«Ручная курация через Directus (Story 3.2)»**.

## Быстрый старт

```bash
# 1) Поднять (db уже жив; POSTGRES_PORT — как у вашего db):
POSTGRES_PORT=55432 docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.cold.yml \
  --profile directus up -d directus

# 2) Один раз настроить коллекции/закладки (идемпотентно):
deploy/directus/bootstrap.sh

# 3) Работать: http://127.0.0.1:8055 (на VPS — ssh -L 8055:127.0.0.1:8055 <vps>)
#    Закладки: «Очередь геопривязки» (unmatched/wrong_reported) и «Верификация (auto)».

# 4) Погасить после сессии (AR-21 — не 24/7):
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.cold.yml --profile directus stop directus
```

## Что здесь лежит

- `bootstrap.sh` — идемпотентная настройка метаданных Directus через API: регистрация коллекций
  поверх СУЩЕСТВУЮЩИХ таблиц (схему не трогает — она принадлежит goose-миграциям), map-интерфейс
  для `geom`, dropdown статусов (enum `0022`), read-only служебных полей, закладки очереди.
  Вместо `directus schema snapshot/apply` сознательно: snapshot несёт и schema-часть таблиц,
  а владелец схемы — goose; скрипт применяет ТОЛЬКО метаданные админки.

## Заметки

- Первый старт Directus создаёт свои системные таблицы `directus_*` в той же БД — это вне goose
  и НЕ дрейф миграций (админка поверх Postgres, architecture.md).
- Секреты: `DIRECTUS_SECRET` / `DIRECTUS_ADMIN_EMAIL` / `DIRECTUS_ADMIN_PASSWORD` в `deploy/.env`
  (см. `.env.example`); dev-дефолты зашиты в `docker-compose.cold.yml`. `ADMIN_EMAIL` должен быть
  валидным адресом с реальным TLD (`.local` не проходит валидацию Directus).
- Порт 8055 — только loopback (решение D4): не за Caddy, доступ SSH-туннелем.
- Гейты честности живут в БД и переживают любой UI: CHECK-констрейнты статусов/геометрии (0020/0022),
  триггер length_km, batch-гейт «перезаписывать только auto|unmatched» (`UpsertGeoObject`).
