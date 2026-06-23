---
baseline_commit: b5258b5bd7f8d6e1bccbb16b89e0255ef8b7d513
---

# Story 1.8: Скелет-карта с PMTiles-подложкой и тремя точками

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a **гражданин**,
I want **видеть объекты на карте Астаны**,
so that **я нахожу контракт географически**.

> **Тип истории:** фронтенд-история Epic 1 (React + Vite + MapLibre GL). Несёт **скелет-срез FR-7**
> (маркеры) и тонко затрагивает **FR-8** (превью→карточка). В терминах архитектуры это этап **S-0.5**
> («PMTiles-подложка к первому демо»), идущий поверх walking skeleton S-0 (Story 1.7, done).
> **Гео-независимая, на синтетике** — НЕ ждёт токен ows_v2, Nominatim или вердикт гейта №0:
> три точки захардкожены вручную. [Source: epics.md:830, 979–997; architecture.md:389,497,765]

## Acceptance Criteria

**AC1 — Карта с подложкой и тремя точками отрисовывается прогрессивно** [epics.md:987–989]
**Given** MapLibre GL 5 + PMTiles-подложка (статика через Caddy) или нейтральный фон
**When** карта загружается
**Then** 3 захардкоженные точки-маркера Астаны отрисованы прогрессивно (канвас → тайлы → маркеры).

**AC2 — Тап по маркеру → превью → переход в карточку** [epics.md:991–993]
**Given** маркер
**When** тап
**Then** открывается нижний лист превью `{preview-sheet}` → переход в карточку контракта (`/contracts/:goszakupId`).

**AC3 — Негеокодированный контракт честно показан вне карты** [epics.md:995–997]
**Given** негеокодированный контракт
**When** он показан
**Then** он НЕ на карте, но виден в списке с меткой «без точки на карте» `{data-state-ungeocoded}`
(честное состояние, не дефект).

## Tasks / Subtasks

- [x] **Task 1 — Установить и закрепить зависимости карты (AC1)**
  - [x] `npm install maplibre-gl@5.24.0 pmtiles@4.4.1` в `web/` — **pin exact, без `^`** (конвенция 1.5/1.7); закоммитить `web/package-lock.json`
  - [x] **НЕ ставить maplibre-gl v6** — архитектура явно запрещает (`v6 — pre-release, НЕ берём`) [architecture.md:247,263,446]
  - [x] При необходимости донастроить `web/vite.config.ts` под воркеры/ассеты MapLibre (worker bundling) — минимально; не ломать существующий alias `@fixtures`
  - [x] Проверить `npm run typecheck` зелёный после добавления типов `maplibre-gl`/`pmtiles`
- [x] **Task 2 — Каркас фичи карты + регистрация маршрута `/map` (AC1)**
  - [x] Наполнить `web/src/features/map/` (компонент маршрута карты) и `web/src/shared/map/` (map-lib: регистрация PMTiles-протокола, стиль, JS-мост к токенам) — каркасы существуют как `.gitkeep`, **наполнять, не пересоздавать**
  - [x] Зарегистрировать маршрут `/map` в `createBrowserRouter` (`web/src/router.tsx`) — путь уже объявлен в `routePaths.map`/`RouteId`, но **НЕ добавлен в children** → добавить child-route. Навигационная ссылка в шапке (`App.tsx`), заголовок KZ «Картада» / RU «Карта» через `t()` [EXPERIENCE.md:168]
  - [x] **Router-loader НЕ фетчит** — данные грузит TanStack Query в компоненте маршрута (конвенция AC1 Story 1.7) [epics.md:896; 1-7 Dev Notes]
- [x] **Task 3 — Инициализация MapLibre с `useRef`-guard под StrictMode (AC1)**
  - [x] `new maplibregl.Map()` **ровно один раз** через `useRef`-guard — иначе React StrictMode в dev смонтирует карту дважды. Это **обязательный** инвариант, заранее заложенный в Story 1.7 именно под эту историю [architecture.md:362–363,449–450,612; epics.md:974–976; 1-7 AC3]
  - [x] Контейнер карты: `role="application"` + `aria-label` «Карта Астаны» / KZ «Астана картасы» [EXPERIENCE.md:69,351; mock key-map.html:283]
  - [x] Корректный teardown карты на unmount; уважать `prefers-reduced-motion` (мгновенный зум при включённом) [EXPERIENCE.md:318,365]
- [x] **Task 4 — PMTiles-подложка / нейтральный фон + прогрессивная загрузка (AC1)**
  - [x] Зарегистрировать PMTiles-протокол в `web/src/shared/map/` (`maplibregl.addProtocol('pmtiles', …)` через пакет `pmtiles`)
  - [x] **Использовать ГОТОВУЮ Protomaps-базу**, НЕ собственный `astana.pmtiles` экстракт (planetiler отложен в S-0.5/после демо) [architecture.md:447–450,389]. Артефакт класть в `web/public/tiles/astana.pmtiles`, раздавать статикой через Caddy [architecture.md:746,463,752]
  - [x] Атрибуция подложки (© источник тайлов) — в моке как контрол карты отсутствует, **добавить** (лицензионно обязательна)
  - [x] **Допустимый фолбэк (по AC1 «или нейтральный фон»):** если pmtiles-ассет ещё не получен — нейтральный фон (ориентир — токен `--color-surface-sunken`); 3 точки всё равно отрисовываются. История остаётся проходимой без подложки [architecture.md:267,497]
  - [x] Прогрессивная отрисовка: **серый канвас + плашка-скелетон карты (не пустой экран, не спиннер-стена)** → базовые тайлы → маркеры; бюджет NFR-1 <3с [EXPERIENCE.md:261,458; epics.md:989]
- [x] **Task 5 — Три захардкоженные точки как DOM-маркеры (AC1)**
  - [x] Три захардкоженные координаты Астаны (3 контракта-истории из Story 0.4) — **в API гео-полей НЕТ** (`schema.gen.ts`: у `Contract` нет `lat`/`lon`), координаты задаются вручную в фикстуре/константе фичи. **Не выдумывать гео в API** [epics.md:989,897–899; Часть A: schema.gen.ts]
  - [x] Порядок координат на проводе/в GeoJSON — **`[lon, lat]` (RFC7946, НЕ `[lat,lon]`)** [architecture.md:562,670; AR-19]
  - [x] Маркеры — **DOM-кнопки `maplibregl.Marker` с `<button aria-label>`**, НЕ canvas-символы (закладывает a11y) [review-accessibility.md:45–56; EXPERIENCE.md:348,356]
  - [x] Стиль `{map-marker-point}`: каплевидный голубой пин (`fill:--color-primary`, `stroke:--color-surface` 2px, `radius 50% 50% 50% 2px`, тень, **glyph: none**), читаем поверх любой подложки. **Цвет/глиф маркера — через JS-мост к semantic-токенам (закрытый enum), НЕ хардкод hex** [DESIGN.md:208–215; architecture.md:622–623]
  - [x] Хитбокс ≥44px (`--space-tap-target`) вокруг точки; видимый focus-ring токеном (`--color-focus-ring`) на каждом маркере [DESIGN.md:178,309–314; review-accessibility.md:94–102]
  - [x] Маркеры появляются **после** тайлов (порядок прогрессивной загрузки); базовая клавиатура карты (стрелки=pan, +/−=zoom — из коробки MapLibre)
  - [x] **Нейтральность:** обычные точки — голубые, никакого алого/«горячих точек»/обвинительной семантики; если точка несёт флаг — глиф «!» и **равная визуальная весомость** с голубым (не «громче») [review-neutrality.md:23–24,41; DESIGN.md:394–395]
  - [x] Зум-контролы (+/−) в нижней зоне (зона большого пальца, mobile-first) + recenter «к Астане» [mock:172–181,337–340; review-map-ux.md:67–73]
- [x] **Task 6 — Превью-лист и переход в карточку (AC2)**
  - [x] Тап по маркеру → минимальный нижний лист `{preview-sheet}` (`role="dialog"`); из листа — переход в карточку контракта `/contracts/:goszakupId`
  - [x] **Переиспользовать `useContract`/`fetchContract`** (`web/src/features/contract/index.ts`), НЕ писать свой fetch [1-7 Dev Notes]
  - [x] Честный рендер полей превью — **копировать паттерн `Value` из `ContractCard.tsx`** (показ только при `state==='ok' && value!==null`, иначе `<DataState>`; дыра `ok+null` уже закрыта) [1-7 review-fix P1]
  - [x] **Попап/лист карты вне React-дерева → `i18n.t` императивно + подписка на `languageChanged`** (обновлять текст при смене языка вручную) [architecture.md:284–285,457]
  - [x] Объём листа — минимальный (тап→лист→карточка). **Полный `{preview-sheet}` с детентами peek→half, focus-trap, превью-список при совпадающих координатах — Epic 3 (Story 3.5), НЕ здесь** [epics.md:527,1271–1289]
- [x] **Task 7 — Честное состояние «без точки на карте» (AC3)**
  - [x] Один негеокодированный контракт: **НЕ рисуется маркером**, но показан в минимальном списке с меткой «без точки на карте» и глифом `<Icon name="ungeocoded">` (`⦸` уже есть в `Icon.tsx`) [epics.md:995–997; Часть A: Icon.tsx]
  - [x] Подпись честная (KZ «картадағы нүкте әзірге жоқ» / RU «без точки на карте»), это **честное состояние, не дефект** — гардрейл честности [EXPERIENCE.md:255; CLAUDE.md гардрейл]
  - [x] **Полноценный фолбэк-список района, баннер «карта недоступна», агрегат «N объектов без точки», `geo_coverage`-гейт — Epic 3 (Story 3.3/3.6), НЕ здесь.** В 1.8 — минимальный список с меткой [epics.md:1235–1309]
- [x] **Task 8 — i18n-строки карты (kk + ru синхронно)**
  - [x] Добавить строки карты (заголовок, кнопки зума/recenter, «без точки на карте», подписи превью) в `web/src/shared/i18n/locales/kk/chrome.json` **И** `.../ru/chrome.json` — **синхронно**, иначе i18n-cross тест красный. KZ — дефолт [1-6 Dev Notes; architecture.md:282–283]
  - [x] UI-строки через `useTranslation('chrome')` + `t()`; числа/даты — только `formatMoney`/`formatDate` (сырой `toLocale*` = eslint-red) [1-6 eslint-граница]
- [x] **Task 9 — Тесты и финализация**
  - [x] Vitest unit на **чистую логику** (маппинг координат `[lon,lat]`, выбор стиля маркера/состояния, разделение гео/негео контрактов) — детерминированно, без живых тайлов [architecture.md:350; ci-web.yml]
  - [x] Playwright smoke (`web/e2e/`): маршрут `/map` грузится, маркеры присутствуют (DOM-кнопки), тап → лист → переход в карточку; `/api` мокается `route.fulfill` (CI без docker) [1-7 e2e паттерн; playwright.config.ts]
  - [x] **Dependency-light:** RTL/jsdom НЕ установлены — без явного одобрения владельца не добавлять; рендер-логику покрывать Playwright [Часть A: тесты web]
  - [x] Зелёные: `typecheck`, `eslint`, `lint:css` (stylelint — **0 hex вне `tokens.css`**), `prettier --check`, `vitest`, `vite build`, Playwright smoke (`ci-web.yml`) [Часть A: CI]
  - [x] Обновить File List, Change Log, Completion Notes

### Review Findings

_Code review 2026-06-23 — адверсариальные слои (Blind Hunter, Edge Case Hunter, Acceptance Auditor). Итог: 2 patch, 4 defer, 6 dismissed. Единственная «High» (Blind: `i18n.t` без namespace) — ложная тревога: `defaultNS: 'chrome'` + e2e находит маркер по интерполированному `map.marker_label`._

- [x] [Review][Patch] `aria-modal="true"` обещает изоляцию без focus-trap → выставить `aria-modal="false"` (как в UX-моке key-map.html), честнее [web/src/features/map/MapPreviewSheet.tsx] — **fixed:** `aria-modal="false"`
- [x] [Review][Patch] Нет обработки ошибки карты: при недоступности WebGL событие `load` не наступит → вечная плашка «Карта загружается…» (нарушает гардрейл честности) [web/src/features/map/MapView.tsx] — **fixed:** try/catch вокруг `new Map()` + `map.on('error')` (до `load`) → состояние `failed` → честная плашка `map.unavailable` («карта недоступна»), контролы скрыты; i18n-ключ добавлен kk+ru
- [x] [Review][Defer] Фокус не возвращается на маркер-триггер при закрытии листа — focus-return; полное управление фокусом отложено в Epic 3 (Story 3.5) [web/src/features/map/MapPreviewSheet.tsx] — deferred
- [x] [Review][Defer] Фон canvas и `prefers-reduced-motion` читаются один раз — не реагируют на рантайм-смену темы/настройки (рантайм-переключателя тем пока нет) [web/src/features/map/{MapView.tsx,../shared/map/tokenBridge.ts}] — deferred
- [x] [Review][Defer] Декоративные `rgba()` в map.css (тень пина — из DESIGN.md; scrim/тень листа) вне токен-системы и не следуют тёмной теме; проходят stylelint (`color-no-hex` разрешает rgba). Токенизация scrim/elevation — задача дизайн-токенов (1.5) [web/src/features/map/map.css] — deferred
- [x] [Review][Defer] 404 по несидированным DEMO-0002/0003 показывается как общая ошибка, не «нет данных по объекту» — унаследовано от ContractRoute; реальные данные/seed — Story 1.9 [web/src/features/map/MapPreviewSheet.tsx] — deferred

## Dev Notes

### Контекст истории (зачем именно сейчас и где границы)

Epic 1 строит сквозной «позвонок» на синтетике и сразу показывает читаемую единицу ценности. Story 1.7
(walking skeleton, done) уже провела байт Postgres→sqlc→chi→Caddy→React и показала карточку контракта;
**в 1.7 заранее заложен `useRef`-guard карты под StrictMode именно для этой истории**. Story 1.8 добавляет
географическую поверхность: минимальную карту (подложка/фон + 3 точки + тап-в-карточку + честное «без
точки»). В терминах архитектуры — этап **S-0.5**. [Source: epics.md:550–565,958–977,979–997; architecture.md:389,497,765]

**Эта история гео-независима и идёт на синтетике** — не требует токена, Nominatim или вердикта гейта №0
(в отличие от заблокированных 0-1/0-6). Координаты трёх точек захардкожены. [Source: epics.md:830,897–899]

### Что переиспользовать (НЕ изобретать заново)

Веб-каркас Epic 1 уже построен (истории 1.3–1.7). **Не дублировать**:

- **API/Query:** `web/src/features/contract/useContract.ts` (`useContract(id)`, `fetchContract(id)`, честный
  `ContractFetchError {status, code}`), единый `web/src/app/queryClient.ts`. Превью точки **обязано**
  переиспользовать `useContract`/`fetchContract`, а не писать свой `fetch`.
- **Честный рендер:** паттерн `Value` в `web/src/features/contract/ContractCard.tsx` (показ только при
  `ok && value!==null`, иначе `<DataState>`); `web/src/shared/state/DataState.tsx` (`dataStateFromValueState`);
  `web/src/shared/ui/Icon.tsx` — **готовый глиф `ungeocoded: '⦸'`** под AC3.
- **Токены/темы:** `web/src/shared/tokens/{tokens.css,tokens.gen.ts,theme.ts}` — auto-generated из
  `web/tokens.json` через `npm run gen-tokens`, **руками не править** (страж `generated==regenerated`).
- **i18n:** `web/src/shared/i18n/{index.ts,format.ts}` (kk-дефолт, ru-fallback, namespace `chrome`),
  словари `locales/{kk,ru}/chrome.json`.
- **Каркасы карты:** `web/src/features/map/.gitkeep` и `web/src/shared/map/.gitkeep` — наполнять.
- **Контейнер MapLibre на canvas** рисует под React → императивные эффекты в `useEffect` с `useRef`-guard.

### Новые зависимости (закрепить точно)

| Пакет | Версия | Зачем | Примечание |
|---|---|---|---|
| `maplibre-gl` | **5.24.0** | движок карты | major **5**; **v6 запрещён** архитектурой (pre-release) [architecture.md:247,263,446] |
| `pmtiles` | **4.4.1** | `addProtocol('pmtiles', …)` для статической PMTiles-подложки | в architecture-списке зависимостей НЕ перечислен — добавить [architecture.md:259–268] |

Pin exact (без `^`), закоммитить `package-lock.json` (конвенция 1.5/1.7). Актуальные версии подтверждены
на npm (июнь 2026): maplibre-gl 5.24.0 — последняя в ветке 5.x; pmtiles 4.4.1.

### Соблюдение архитектуры (guardrails)

- **Карта — единственный UI вне CSS:** палитра/глифы/состояния маркеров — закрытый enum **через JS-мост к
  semantic-токенам**, НЕ хардкод hex в canvas. `z-index` карты — токен `--z-map`. [architecture.md:622–624]
- **MapLibre-попапы/лист вне React-дерева:** прокидывать `i18n.t` императивно + подписка на `languageChanged`.
  [architecture.md:284–285,457]
- **Маркеры — DOM (`maplibregl.Marker`), не canvas-символы** — обязательное требование a11y-ревью.
  [review-accessibility.md:45–56]
- **`useRef`-guard от двойного `new maplibregl.Map()`** под StrictMode — обязателен с первого коммита.
  [architecture.md:362–363,449–450,612]
- **Router-loader НЕ фетчит;** загрузку владеет TanStack Query. SSR не использовать. [architecture.md:458]
- **GeoJSON `[lon, lat]`** (RFC7946), НИКОГДА `[lat,lon]`. [architecture.md:562,670]
- **Фичи компонуют из `shared/ui`** (никаких сырых интерактивных элементов); `shared` без домена; lint —
  граница (запрет hex в CSS, `outline:none` без замены, сырого `toLocale*`). [architecture.md:615–620,782–783]
- **Список объектов — равноправное карте представление** (a11y-эквивалент), обязателен для негеокодированных.
  В 1.8 это операционализирует AC3. [architecture.md:72–74,451–452]

### Гардрейлы нейтральности и честности (несущие)

- **Нейтральность:** обычные точки — нейтрально-голубые; **алый под маркеры запрещён**; амбер-флаг (если
  появится) — равная весомость с голубым, не «горячая точка», глиф «!» (не только цвет). Любая надпись на
  карте — нейтральна (CI-страж taboo: запрещены корни `нарушен/коррупц/виновн`, `бұзушылық/...`).
  [Source: review-neutrality.md:23–24,41; DESIGN.md:394–395; 1-4 taboo-страж]
- **Честность над домыслом:** негеокодированный контракт **не получает выдуманную координату** — он честно
  вне карты, в списке с меткой `{data-state-ungeocoded}` (AC3). Карта не интерполирует гео. [Source: epics.md:997; CLAUDE.md]

### PMTiles-подложка — как подавать

- **Готовая Protomaps-база** + наши точки векторным слоем поверх; **свой `astana.pmtiles` (planetiler) НЕ
  делать** (отложен в S-0.5/после демо). [architecture.md:447–450]
- Артефакт → `web/public/tiles/astana.pmtiles`, раздаётся **статикой через Caddy** (`deploy/Caddyfile`: SPA-статика
  `root * /srv`; web-build `dist`→`/srv`). [architecture.md:746,463,752; Часть A: Caddyfile]
- **PMTiles воспроизводимы из `tools/`, в бэкап НЕ входят** — крупный бинарь **не раздувать git**: добавить правило
  `.gitignore` (или Git LFS) и задокументировать получение в `deploy/` (а не коммитить десятки МБ). [architecture.md:482,819]
- **Если ассет ещё не получен — нейтральный фон** (AC1 допускает): 3 точки рисуются, история проходит. Это
  снимает внешнюю зависимость от источника тайлов с критического пути.

### Три точки — источник данных

- **Захардкоженные координаты Астаны** (3 контракта-истории, выбраны в Story 0.4) — в wire-форме `Contract`
  (`web/src/shared/api/schema.gen.ts`) **гео-полей нет**. Координаты — константа в фиче карты/фикстуре, НЕ
  расширение API. [Source: epics.md:989,897–899; Часть A: schema.gen.ts]
- Публичный id перехода — natural `goszakup_id` → `/contracts/:goszakupId` (как в 1.3). [Source: epics.md:896]
- **Astana center/zoom/bbox в спеках НЕ заданы** — dev задаёт под 3 выбранные точки; фон-ориентир канваса —
  `--color-surface-sunken`. [Source: architecture.md (не указано); DESIGN.md:81–82]

### Файлы и куда писать

- **Наполнить:** `web/src/features/map/` (компонент маршрута карты, превью-лист, мини-список негео),
  `web/src/shared/map/` (регистрация pmtiles-протокола, стиль карты, JS-мост к токенам). CSS — `*.module.css`/
  фиче-css **только семантические токены** (hex краснит stylelint вне `tokens.css`).
- **Изменить:** `web/src/router.tsx` (+child `/map`), `web/src/App.tsx` (нав-ссылка), `web/package.json` +
  `web/package-lock.json` (deps), `web/src/shared/i18n/locales/{kk,ru}/chrome.json` (+map-строки),
  при необходимости `web/vite.config.ts` (воркеры MapLibre). Возможно `.gitignore` (правило `*.pmtiles`).
- **Добавить:** `web/public/tiles/astana.pmtiles` (готовая база, вне git), тест(ы) в `web/e2e/` и `*.test.ts`.
- **НЕ трогать:** `web/src/shared/api/schema.gen.ts` (generated), `web/src/shared/tokens/{tokens.css,tokens.gen.ts}`
  (generated), `server/**`, `registry/`, готовые истории 1.1–1.7.

### Границы скоупа — ЧТО ОТКЛАДЫВАЕТСЯ в Epic 3 (НЕ делать в 1.8)

| Отложено | Куда | FR |
|---|---|---|
| Авто-геопривязка (batch-Nominatim), таблицы `districts`/`geo_objects` | Story 3.1 | FR-4 |
| Полуручная разметка через Directus, POINT\|LINESTRING, статусы | Story 3.2 | FR-5 |
| Серверный гейт `geo_coverage` ≥70% + фолбэк-режим района | Story 3.3 | FR-6 |
| bbox-загрузка объектов, кластеры (supercluster), линии/полилинии, z-priority | Story 3.4 | FR-7 |
| Полный `{preview-sheet}` (детенты peek→half, focus-trap, превью-список) | Story 3.5 | FR-8 |
| Фолбэк-список района, баннер «карта недоступна», агрегат «N без точки» | Story 3.6 | FR-9 |
| Глубокая a11y canvas-карты (постоянный ARIA-список, SR-объявление) | Story 3.8 | — |
| Маркеры flagged/confirmed/selected (полная спека), глифы !/✓, золото | Epic 3 | — |

[Source: epics.md:492–494,527,1190–1349] — 1.8 строит **только** статичный скелет (3 точки + подложка/фон +
минимальный тап-в-карточку + честное «без точки»). Всё динамическое — Epic 3.

### Тестирование / проверка готовности

- **Раннеры:** Vitest + Playwright. Vitest — чистая логика (`src/**/*.test.ts`); Playwright smoke — `web/e2e/`
  против `vite preview` :4173 с моком `/api` (`route.fulfill`). Полный байт-стек — ручная DoD `docker compose
  --profile app up` (порт БД: `POSTGRES_PORT=55432` в `deploy/.env` — локально 5432/5433 заняты). [Источник: Часть A; memory local-dev-docker-env]
- **CI `ci-web.yml`** (Node 22): typecheck → eslint → stylelint → prettier → vitest → build → Playwright smoke.
  Красный = блок merge.
- **a11y:** `@axe-core/playwright`/`vitest-axe` доступны; контраст-гейт спит до первого цвета флага. Минимум для
  скелета: `aria-label` карты, маркеры-кнопки с `aria-label`, hit-area ≥44px, focus-ring, `lang` на корне.
- **Dependency-light:** не добавлять RTL/jsdom без одобрения владельца.
- **DoD:** AC1–AC3 закрыты; карта инициализируется один раз под StrictMode; маркеры — DOM-кнопки; негео честно
  в списке; цвета через токены; i18n kk+ru синхронны; все гейты `ci-web.yml` зелёные.

### Previous Story Intelligence (1.3–1.7)

- **1.7 (walking skeleton):** `useRef`-guard карты заложен заранее под 1.8; router-loader не фетчит; скелетон под
  раскладку (не спиннер); превью переиспользует `useContract`; честный `Value`-паттерн с закрытой дырой `ok+null`.
- **1.6 (i18n):** kk-дефолт/ru-fallback, namespace `chrome`; новые ключи — синхронно kk+ru; `formatMoney`/`formatDate`
  обязательны; `<Icon name="ungeocoded">` (⦸) готов.
- **1.5 (токены):** только семантические токены; две темы через `[data-theme]`; компоненты тему не знают →
  стили маркеров/листа через `var(--color-*)`; сигнал — амбер, не red; `outline:none` запрещён.
- **1.4 (честные состояния/нейтральность):** гео-состояния `geocode_pending`/`geocode_failed` — официальные
  value_state; CI-страж taboo на надписях.
- **1.3 (wire/URL):** natural `goszakup_id`; конверт `{value,state}` (`value:null⇒no_data`); `amount_tng` строка→`formatMoney`.

### Latest tech (подтверждено на npm, июнь 2026)

- `maplibre-gl` **5.24.0** — последняя в ветке 5.x (v6 — pre-release, **не брать** по решению архитектуры).
- `pmtiles` **4.4.1** — последняя; даёт `addProtocol` для статической PMTiles-подложки.
- [Source: https://www.npmjs.com/package/maplibre-gl, https://www.npmjs.com/package/pmtiles]

### Project Structure Notes

- Фундамент истории существует: монорепо `web/` (Vite+React+TS), маршрутизация, токены, i18n, карточка контракта —
  всё от историй 1.1–1.7 (done). Предусловного HALT нет (в отличие от 0-6).
- Целевое дерево для карты совпадает с существующим: `web/src/features/map/`, `web/src/shared/map/`,
  `web/public/tiles/` — каркасы на месте. [architecture.md:742–749]
- Конфликтов со структурой нет; история наполняет зарезервированные каркасы и регистрирует уже объявленный
  маршрут `/map`.

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story-1.8 (979–997); Epic 1 (550–565,825–832); FR-coverage (492–495,527); Epic 3 границы (1190–1349)]
- [Source: _bmad-output/planning-artifacts/architecture.md — веб-стек/версии (246–268); дерево web/ (742–749); PMTiles (446–450,463,746,752,482,819); S-0.5 (389,497,765); map=UI-вне-CSS/JS-мост (622–624); попапы вне React (284–285,457); useRef-guard (362–363,449–450,612); GeoJSON [lon,lat] (562,670); границы (615–620,769–783)]
- [Source: _bmad-output/planning-artifacts/prds/prd-AshyqQala.kz-2026-06-17/addendum.md — geo_objects (20); стек]
- [Source: ux-designs/.../mockups/key-map.html — layout/контролы/маркеры (108–214,294–362); role=application (283)]
- [Source: ux-designs/.../EXPERIENCE.md — карта cold-start/прогрессив (261,458); aria-label (69,351); микрокопия (168); reduced-motion (318,365); ungeocoded (255)]
- [Source: ux-designs/.../DESIGN.md — токены (28–88); map-marker-point (208–215); tap-target (178); focus-ring (309–314); нейтральность (394–395,609–611)]
- [Source: ux-designs/.../review-accessibility.md — DOM-маркеры (45–56); hit-area (94–102); focus-ring (77–84)]
- [Source: ux-designs/.../review-neutrality.md — амбер-маркер ≠ горячая точка (23–24); алый запрещён (41)]
- [Source: web/ — package.json (версии), router.tsx (/map не зарегистрирован), features/contract/* (useContract, ContractCard Value-паттерн), shared/{ui/Icon.tsx (ungeocoded ⦸),state/DataState.tsx,tokens/*,i18n/*}, e2e/contract.spec.ts, ci-web.yml, deploy/Caddyfile]
- [Source: _bmad-output/implementation-artifacts/1-7-*.md (useRef-guard, router-loader, honest Value); 1-6 (i18n); 1-5 (токены); 1-4 (честные состояния/taboo); 1-3 (wire/URL)]
- [Source: CLAUDE.md — guardrails нейтральности/честности; web-стек]

## Dev Agent Record

### Agent Model Used

claude-opus-4-8[1m] (Claude Opus 4.8, 1M context) — dev-story workflow.

### Debug Log References

- `npm install --save-exact maplibre-gl@5.24.0 pmtiles@4.4.1` → added 28 packages; версии подтверждены (`maplibre-gl 5.24.0`, `pmtiles 4.4.1`), `package-lock.json` обновлён.
- Гейты `ci-web.yml` (все зелёные): `typecheck` (tsc) OK · `eslint` OK · `stylelint` (0 hex вне tokens.css) OK · `prettier --check` OK · `vitest` 54/54 OK · `vite build` OK.
- `vite build`: code-split сработал — главный чанк 337 КБ (gzip 110), MapLibre вынесен в ленивый чанк 1.08 МБ (gzip 294), грузится только на `/map` (NFR-1 сохранён на главной/карточке).
- `playwright test` 2/2 OK: `contract.spec` (регрессия не сломана) + `map.spec` (маршрут → маркер → превью-диалог → переход в карточку). MapLibre+WebGL работает в headless chromium.

### Completion Notes List

- **AC1 — карта + прогрессивная отрисовка:** MapLibre GL 5.24.0 инициализируется один раз через `useRef`-guard под StrictMode; контейнер `role="application"` + i18n `aria-label`; плашка-скелетон до события `load`, маркеры добавляются после (канвас → фон → маркеры). `prefers-reduced-motion` → `fadeDuration:0` + `jumpTo`.
- **AC1 — подложка (ЧЕСТНОЕ РЕШЕНИЕ):** shipped **нейтральный фон** (`--color-surface-sunken` через JS-мост `tokenBridge`), что AC1 явно допускает («PMTiles-подложка ИЛИ нейтральный фон»). PMTiles-машинерия полностью смонтирована и готова: `registerPmtilesProtocol()` (идемпотентно) + `buildMapStyle({basemapPmtilesUrl})` поддерживает `pmtiles://`-источник + raster-слой подложки (покрыто unit-тестом). Реальный `astana.pmtiles`-ассет НЕ создан — генерация своего экстракта отложена архитектурой (S-0.5/после демо), готовую Protomaps-сборку в этой среде получить нельзя. **Включение подложки = одна строка** (`BASEMAP_PMTILES_URL` в `MapView.tsx` → `/tiles/astana.pmtiles`, раздаётся Caddy) + размещение ассета вне git — оформлено как ops-follow-up. Не выдумывал работающую подложку (гардрейл честности).
- **AC2 — превью → карточка:** тап по маркеру открывает нижний лист `{preview-sheet}` (`role=dialog`, базовый фокус + Escape; полный focus-trap/детенты — Epic 3 Story 3.5); лист переиспользует `useContract`/`fetchContract` и honest `Value`-паттерн из 1.7; «Подробнее →» ведёт на `/contracts/:goszakupId`. Лист рендерится в React-дереве (не `maplibregl.Popup`) → i18n штатно. Aria-label маркеров — императивно + подписка `languageChanged` (маркеры вне React-дерева, architecture.md).
- **AC3 — честное «без точки на карте»:** негеокодированный `DEMO-0004` НЕ на карте, показан в списке с `<Icon name="ungeocoded">` (⦸) + нейтральная метка; честное состояние, не дефект.
- **Гардрейлы:** маркеры — нейтрально-голубые (`--color-primary`), без алого/«горячих точек»; цвета — только семантические токены (stylelint 0 hex); i18n-строки добавлены синхронно в kk+ru; числа/даты — через `formatMoney` (eslint-граница соблюдена).
- **Перф (предотвращение регрессии):** маршрут карты лениво (`React.lazy`+`Suspense`) — MapLibre не попадает в бандл главной/карточки.
- **Три точки:** захардкоженные демо-координаты Астаны (в API гео-полей нет). Полные «3 читаемых контракта-истории» (реальные данные/seed DEMO-0002/0003) — Story 1.9; здесь скелет показывает 3 точки, превью честно отражает состояние бэкенда.
- **Не делал (граница Epic 3):** кластеры, bbox-загрузка, линии, авто-геопривязка, фолбэк-список района/баннер, глубокая a11y canvas-карты (3-8), полный preview-sheet (3-5).

### File List

**Новые:**
- `web/src/features/map/astanaPoints.ts` — 3 захардкоженные точки + негео-контракт + `toLngLat([lon,lat])` + центр/зум.
- `web/src/features/map/astanaPoints.test.ts` — unit: порядок `[lon,lat]`, 3 точки, в пределах viewbox, уникальность id, ≥1 негео.
- `web/src/features/map/MapView.tsx` — компонент маршрута карты (init, маркеры, контролы, список негео, превью).
- `web/src/features/map/MapPreviewSheet.tsx` — нижний лист превью (reuse `useContract`, honest `Value`).
- `web/src/features/map/map.css` — стили карты/маркеров/листа/списка (только semantic-токены).
- `web/src/features/map/index.ts` — экспорт `MapView`.
- `web/src/shared/map/mapStyle.ts` — `buildMapStyle()` (нейтральный фон / опц. pmtiles-подложка).
- `web/src/shared/map/mapStyle.test.ts` — unit: нейтральный стиль + pmtiles-источник.
- `web/src/shared/map/pmtilesProtocol.ts` — идемпотентная регистрация pmtiles-протокола.
- `web/src/shared/map/tokenBridge.ts` — JS-мост к semantic-токенам (фон карты).
- `web/e2e/map.spec.ts` — Playwright smoke (маршрут → маркер → превью → карточка).

**Изменённые:**
- `web/package.json`, `web/package-lock.json` — deps `maplibre-gl@5.24.0`, `pmtiles@4.4.1` (exact).
- `web/src/router.tsx` — ленивый маршрут `/map` (code-split).
- `web/src/App.tsx` — нав-ссылка на карту в шапке.
- `web/src/shared/i18n/locales/kk/chrome.json`, `.../ru/chrome.json` — группа `map` (синхронно).

**Удалённые:** `web/src/features/map/.gitkeep`, `web/src/shared/map/.gitkeep` (каркасы наполнены).

## Change Log

| Дата | Изменение |
|---|---|
| 2026-06-23 | Создан context engine для Story 1.8 (скелет-карта). Исчерпывающий параллельный анализ (4 субагента): epics.md (спека 1.8 + границы Epic 3), architecture.md (S-0.5, стек, PMTiles, guardrails), UX (key-map.html, DESIGN/EXPERIENCE, a11y/нейтральность), фактическое состояние `web/` (1.3–1.7) + версии npm. Зафиксировано: фундамент готов (HALT нет); maplibre-gl 5.24.0 + pmtiles 4.4.1 (pin exact, v6 запрещён); `useRef`-guard под StrictMode обязателен; маркеры — DOM-кнопки; источник трёх точек — хардкод (в API гео нет); чёткая граница скоупа со Epic 3. Статус → ready-for-dev. |
| 2026-06-23 | code-review (3 адверсариальных слоя): 2 patch / 4 defer / 6 dismissed; «High» (i18n namespace) — ложная (defaultNS=chrome + e2e). **Применены оба patch:** (1) `aria-modal="false"` в превью-листе (честнее, как UX-мок); (2) обработка ошибки карты — try/catch + `map.on('error')` → честная плашка «карта недоступна» вместо вечной «загрузки» (гардрейл честности), +i18n kk/ru. Defer-пункты → `deferred-work.md`. Все гейты `ci-web` зелёные после правок (typecheck/eslint/stylelint/prettier/vitest 54/build + Playwright 2/2). Статус → done. |
| 2026-06-23 | dev-story: реализован скелет-карты. Установлены `maplibre-gl@5.24.0`+`pmtiles@4.4.1` (exact). Новое: `features/map/{MapView,MapPreviewSheet,astanaPoints,map.css,index}`, `shared/map/{mapStyle,pmtilesProtocol,tokenBridge}`, unit-тесты (astanaPoints, mapStyle), Playwright smoke `e2e/map.spec.ts`. Маршрут `/map` зарегистрирован ЛЕНИВО (code-split — MapLibre вне бандла главной/карточки, NFR-1). AC1: карта + `useRef`-guard под StrictMode + прогрессивная отрисовка + нейтральный фон (AC1-допустимо; pmtiles-подложка смонтирована и включается одной строкой — ассет = ops-follow-up). AC2: тап→нижний лист→карточка (reuse `useContract`, honest Value; i18n маркеров императивно + `languageChanged`). AC3: негео-контракт честно в списке (`Icon ungeocoded ⦸`). i18n kk+ru синхронно. Все гейты `ci-web.yml` зелёные: typecheck/eslint/stylelint/prettier/vitest(54)/build + Playwright(2/2). Статус → review. |
