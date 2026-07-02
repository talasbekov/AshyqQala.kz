#!/usr/bin/env bash
# Bootstrap Directus-коллекций для кураторской очереди геопривязки (Story 3.2, FR-5).
#
# Идемпотентен: повторный запуск безопасен (существующие коллекции/поля/закладки не дублируются).
# Регистрирует в Directus ТОЛЬКО КУРАТОРСКИЕ таблицы (AR-4/D5): geo_objects (правка) + districts
# (read-only справочник). Проекционные таблицы (contracts, organizations, …) в админку НЕ заводятся.
# Схему таблиц НЕ трогает (schema принадлежит goose-миграциям) — только метаданные Directus
# (какие коллекции видны, какими интерфейсами редактировать, закладки очереди).
#
# Запуск (Directus уже поднят, см. docs/ops/geocoding.md → «Ручная курация через Directus»):
#   deploy/directus/bootstrap.sh
# Переменные (дефолты = dev из docker-compose.cold.yml):
#   DIRECTUS_URL, DIRECTUS_ADMIN_EMAIL, DIRECTUS_ADMIN_PASSWORD
set -euo pipefail

URL="${DIRECTUS_URL:-http://127.0.0.1:8055}"
EMAIL="${DIRECTUS_ADMIN_EMAIL:-admin@example.com}"
PASSWORD="${DIRECTUS_ADMIN_PASSWORD:-change-me}"

say() { printf '%s\n' "$*"; }

command -v curl >/dev/null 2>&1 && command -v jq >/dev/null 2>&1 \
  || { say "ОШИБКА: нужны curl и jq (не найдены в PATH)"; exit 1; }

# Тело логина — через jq --arg (кавычка/бэкслеш в пароле не ломают JSON) и на stdin (пароль не в argv/ps).
# Сбой curl обрабатывается явно (`|| login_resp=""`): под set -e падение подстановки убило бы скрипт ДО
# дружелюбной диагностики ниже.
login_body=$(jq -n --arg email "$EMAIL" --arg password "$PASSWORD" '{email: $email, password: $password}')
login_resp=$(curl -sf -X POST "$URL/auth/login" -H 'Content-Type: application/json' \
  --data @- <<<"$login_body") || login_resp=""
[ -n "$login_resp" ] || { say "ОШИБКА: Directus недоступен ($URL) — поднят ли профиль directus?"; exit 1; }
TOKEN=$(jq -r '.data.access_token' <<<"$login_resp")
[ -n "$TOKEN" ] && [ "$TOKEN" != "null" ] || { say "ОШИБКА: логин в Directus не удался ($URL)"; exit 1; }

api() { # api METHOD PATH [JSON] — curl -f: HTTP-ошибка ⇒ пустой вывод и ненулевой статус (не error-JSON,
        # который jq-проверки вызывающих могли бы принять за «0 найдено» и наплодить дубликатов).
  local method="$1" path="$2" body="${3:-}"
  if [ -n "$body" ]; then
    curl -sf -X "$method" "$URL$path" -H "Authorization: Bearer $TOKEN" \
      -H 'Content-Type: application/json' -d "$body"
  else
    curl -sf -X "$method" "$URL$path" -H "Authorization: Bearer $TOKEN"
  fi
}

# --- 1) Коллекции поверх СУЩЕСТВУЮЩИХ таблиц (schema не передаём — таблицы уже созданы goose) ---
ensure_collection() { # ensure_collection NAME JSON_META
  local name="$1" meta="$2"
  if api GET "/collections/$name" | jq -e '.data.collection' >/dev/null 2>&1; then
    say "коллекция $name — уже зарегистрирована"
  else
    api POST "/collections" "{\"collection\":\"$name\",\"meta\":$meta}" | jq -e '.data.collection' >/dev/null \
      && say "коллекция $name — зарегистрирована" \
      || { say "ОШИБКА регистрации коллекции $name"; exit 1; }
  fi
}

ensure_collection geo_objects '{"icon":"place","note":"КУРАТОРСКАЯ: очередь геопривязки (FR-5, Story 3.2). Правила — docs/ops/geocoding.md","display_template":"{{geocode_status}} · contract {{contract_id}}"}'
ensure_collection districts '{"icon":"map","note":"КУРАТОРСКАЯ (read-only справочник районов; наполнение — seed/Story 0.1)"}'

# --- 2) Метаданные полей geo_objects (интерфейсы/read-only; сами колонки НЕ меняем) ---
field_meta() { # field_meta COLLECTION FIELD JSON_META
  api PATCH "/fields/$1/$2" "{\"meta\":$3}" | jq -e '.data.field' >/dev/null \
    && say "поле $1.$2 — настроено" || { say "ОШИБКА настройки поля $1.$2"; exit 1; }
}

# Геометрия: map-интерфейс (рисование POINT — объект; LINESTRING — дорога). SRID 4326 задан колонкой.
field_meta geo_objects geom '{"interface":"map","special":null,"options":{"geometryType":null},"note":"POINT — объект; LINESTRING — дорога. Не ставить точку «примерно» — честный unmatched лучше (§7.4)."}'
# Статус: закрытый enum 0022 (см. чек-лист переходов в docs/ops/geocoding.md).
field_meta geo_objects geocode_status '{"interface":"select-dropdown","options":{"choices":[{"text":"auto — авто-кандидат (не проверен)","value":"auto"},{"text":"manual — ручная разметка куратора","value":"manual"},{"text":"unmatched — честно без точки","value":"unmatched"},{"text":"verified — авто-точка подтверждена","value":"verified"},{"text":"wrong_reported — «точка не там»","value":"wrong_reported"}]},"note":"Переходы — чек-лист оператора (docs/ops/geocoding.md); БД отвергает ложь (CHECK 0020/0022)."}'
# Служебные/выведенные поля — read-only в UI (владелец — БД/пайплайн, не оператор).
for f in id public_id contract_id district_id geocoded_at; do
  field_meta geo_objects "$f" '{"readonly":true}'
done
field_meta geo_objects length_km '{"readonly":true,"note":"Выводится триггером 0022 из LINESTRING — руками не заполнять."}'
field_meta geo_objects confidence '{"readonly":true,"note":"Nominatim importance (только авто-путь); при ручной правке обнуляется."}'
field_meta geo_objects geocoded_by '{"note":"Провенанс: при ручной правке ставить directus (чек-лист)."}'

# districts — справочник: всё read-only в UI.
for f in id kato_code name_ru name_kk geom imported_at; do
  field_meta districts "$f" '{"readonly":true}'
done

# --- 3) Закладки очереди (глобальные пресеты: role=null, user=null → видны всем) ---
ensure_preset() { # ensure_preset BOOKMARK JSON_BODY
  local mark="$1" body="$2"
  local found
  # Сбой GET (сеть/не-2xx) обязан остановить скрипт, а НЕ провалиться в POST: иначе транзиентная ошибка
  # проверки плодила бы дубликаты закладок при повторном прогоне (у presets нет уникальности по имени).
  found=$(api GET "/presets?filter%5Bbookmark%5D%5B_eq%5D=$(jq -rn --arg s "$mark" '$s|@uri')" | jq -e '.data | length') \
    || { say "ОШИБКА: не удалось проверить существование закладки «$mark»"; exit 1; }
  if [ "$found" -gt 0 ]; then
    say "закладка «$mark» — уже есть"
  else
    api POST "/presets" "$body" | jq -e '.data.id' >/dev/null \
      && say "закладка «$mark» — создана" || { say "ОШИБКА создания закладки «$mark»"; exit 1; }
  fi
}

# user/role заданы null ЯВНО (глобальность закладки — не полагаемся на дефолт версии Directus).
ensure_preset "Очередь геопривязки" '{"bookmark":"Очередь геопривязки","collection":"geo_objects","user":null,"role":null,"filter":{"geocode_status":{"_in":["unmatched","wrong_reported"]}},"layout":"tabular"}'
ensure_preset "Верификация (auto)" '{"bookmark":"Верификация (auto)","collection":"geo_objects","user":null,"role":null,"filter":{"geocode_status":{"_eq":"auto"}},"layout":"tabular","layout_query":{"tabular":{"sort":["confidence"]}}}'

say "bootstrap завершён: кураторские коллекции geo_objects/districts + закладки очереди готовы."
