import { test, expect } from '@playwright/test';

// Каноническая карта (Story 3.4, FR-7): /map → geo_objects из /api/map/objects (замокан) →
// кластер с амбер-кольцом (флаг не теряется, AC2) → тап → раскрытие; глиф-статусы маркеров (AC1);
// счётчик «Ещё N без точки» (AC3); честные состояния (пусто/усечение/ошибка). CI без docker.
// Фикстуры строго по schema.gen.ts (урок AI-1b: дрейф фикстур ↔ схемы = красный e2e).

// Тройка точек в ~800 м друг от друга (кластер на стартовом зуме 11; раскрытие одним тапом),
// одна из них с активным флагом → кластер несёт амбер-кольцо. Отдельно: verified-точка («✓»)
// поодаль и LINESTRING дороги (canvas-слой, DOM-маркера не имеет).
const OBJECTS = {
  items: [
    {
      // РАВНОСТОРОННЯЯ тройка (~800 м попарно): на z11 (50px ≈ 2.4 км ground) — один кластер;
      // на expansion-зуме ВСЕ ТРИ пары превышают радиус одновременно → полный распад ОДНИМ тапом.
      // Неравные расстояния дают ступенчатое раскрытие (кластер 2 + точка) и недетерминированный тест.
      public_id: '0198a3b0-0000-7000-8000-00000000c001',
      goszakup_contract_id: 'DEMO-GEO-01',
      geocode_status: 'auto',
      has_active_flag: true,
      length_km: null,
      geom: { type: 'Point', coordinates: [71.43, 51.128] },
    },
    {
      public_id: '0198a3b0-0000-7000-8000-00000000c002',
      goszakup_contract_id: 'DEMO-GEO-02',
      geocode_status: 'manual',
      has_active_flag: false,
      length_km: null,
      geom: { type: 'Point', coordinates: [71.4415, 51.128] },
    },
    {
      public_id: '0198a3b0-0000-7000-8000-00000000c003',
      goszakup_contract_id: 'DEMO-GEO-03',
      geocode_status: 'auto',
      has_active_flag: false,
      length_km: null,
      geom: { type: 'Point', coordinates: [71.43575, 51.13426] },
    },
    {
      // ~4 км от кластера (≈87px на z11 > clusterRadius 50) — гарантированно одиночный маркер,
      // и близко к центру: фактическое окно канваса уже layout-контейнера (60vh × ширина колонки).
      public_id: '0198a3b0-0000-7000-8000-00000000c004',
      goszakup_contract_id: 'DEMO-GEO-04',
      geocode_status: 'verified',
      has_active_flag: false,
      length_km: null,
      geom: { type: 'Point', coordinates: [71.49, 51.14] },
    },
    {
      public_id: '0198a3b0-0000-7000-8000-00000000c005',
      goszakup_contract_id: 'DEMO-GEO-05',
      geocode_status: 'manual',
      has_active_flag: false,
      length_km: 5.0,
      geom: {
        type: 'LineString',
        coordinates: [
          [71.44, 51.1],
          [71.48, 51.12],
        ],
      },
    },
  ],
  truncated: false,
  ungeocoded_count: 4,
};

// --- Story 3.5 (FR-8): фикстуры контракта для превью (форма Contract из schema.gen.ts,
// зеркало e2e/contract.spec.ts — StringField-конверты, НЕ голые строки; урок AI-1b). ---

const FLAGS_QUIET = [
  {
    flag_id: 'single_participant',
    state: 'insufficient_data',
    methodology_version: { value: null, state: 'no_data' },
    detected_at: { value: null, state: 'no_data' },
    evidence: null,
  },
  {
    flag_id: 'price_per_km',
    state: 'insufficient_data',
    methodology_version: { value: null, state: 'no_data' },
    detected_at: { value: null, state: 'no_data' },
    evidence: null,
  },
];

const contractFixture = (id: string, over: Record<string, unknown> = {}) => ({
  goszakup_contract_id: id,
  subject_ru: { value: `Ремонт автодороги (${id})`, state: 'ok' },
  subject_kk: { value: `Автожол жөндеу (${id})`, state: 'ok' },
  amount_tng: { value: '90000000', state: 'ok' },
  sign_date: { value: '2026-03-15', state: 'ok' },
  plan_start: { value: null, state: 'no_data' },
  plan_end: { value: null, state: 'no_data' },
  status: { value: 'active', state: 'ok' },
  direction: { value: 'road', state: 'ok' },
  kato_code: { value: '710000000', state: 'ok' },
  customer: { value: null, state: 'no_data' },
  supplier: { value: null, state: 'no_data' }, // организации ждут Epic 2 → честное состояние в превью
  source_url: { value: `https://goszakup.gov.kz/ru/contract/${id}`, state: 'ok' },
  act: {
    present: false,
    act_date: { value: null, state: 'no_data' },
    signer: { value: null, state: 'no_data' },
    source_url: { value: null, state: 'no_data' },
  },
  flags: FLAGS_QUIET,
  imported_at: { value: '2026-03-15T10:00:00Z', state: 'ok' },
  updated_at: { value: '2026-03-15T10:00:00Z', state: 'ok' },
  methodology_version: { value: 'v1.0', state: 'ok' },
  as_of: { value: '2026-03-15T10:00:00Z', state: 'ok' },
  methodology_drift: { present: false, requested_version: '', current_version: '' },
  ...over,
});

const FLAG_RAISED = [
  {
    flag_id: 'single_participant',
    state: 'raised',
    methodology_version: { value: 'v1.0', state: 'ok' },
    detected_at: { value: '2026-05-12T10:00:00Z', state: 'ok' },
    evidence: { participant_count: 1 },
  },
  FLAGS_QUIET[1],
];

// Мок /api/contracts/{id}: известный id → фикстура; неизвестный → честный 404-конверт.
const routeContracts = (page: import('@playwright/test').Page, byId: Record<string, unknown>) =>
  page.route('**/api/contracts/*', (route) => {
    const id = decodeURIComponent(new URL(route.request().url()).pathname.split('/').pop() ?? '');
    const fx = byId[id];
    if (fx !== undefined) return route.fulfill({ json: fx });
    return route.fulfill({
      status: 404,
      json: { error: { code: 'NOT_FOUND', message: 'contract not found' } },
    });
  });

test('каноническая карта: кластер с амбер-кольцом → раскрытие; глифы; счётчик «без точки»', async ({
  page,
}) => {
  await page.route('**/api/map/objects*', (route) => route.fulfill({ json: OBJECTS }));
  await routeContracts(page, { 'DEMO-GEO-01': contractFixture('DEMO-GEO-01') });

  await page.goto('/map');
  await expect(page.getByRole('application')).toBeVisible();

  // Кластер трёх близких точек: бейдж-счётчик «3» + амбер-кольцо с «!»-точкой (внутри есть флаг, AC2).
  const cluster = page.locator('.aq-map-cluster');
  await expect(cluster).toHaveCount(1, { timeout: 30_000 });
  await expect(cluster).toHaveClass(/aq-map-cluster--flag/);
  await expect(cluster.locator('.aq-map-cluster__count')).toHaveText('3');
  await expect(cluster.locator('.aq-map-cluster__dot')).toHaveText('!');

  // Verified-точка поодаль — одиночный маркер с глифом «✓» (статус несёт форма/глиф, AC1).
  const confirmed = page.locator('.aq-map-marker--confirmed');
  await expect(confirmed).toHaveCount(1);
  await expect(confirmed.locator('.aq-map-marker__glyph')).toHaveText('✓');

  // AC3: аффорданса-счётчик «объектов без точки» — ссылка на список (/search).
  const more = page.locator('.aq-map-ungeocoded-more a');
  await expect(more).toBeVisible();
  await expect(more).toHaveAttribute('href', '/search');
  await expect(more).toContainText('4');

  // Линия отрисована (Task 6, AC1): canvas-слой недоступен DOM-локаторам — через тест-шов
  // __aqMapTest (MapView) спрашиваем сам MapLibre: слой существует и рендерит фичи LINESTRING.
  await expect
    .poll(
      () =>
        page.evaluate(() => {
          const m = (
            window as unknown as {
              __aqMapTest?: {
                getLayer(id: string): unknown;
                queryRenderedFeatures(opts: { layers: string[] }): unknown[];
              };
            }
          ).__aqMapTest;
          if (!m || !m.getLayer('aq-geo-line')) return -1;
          return m.queryRenderedFeatures({ layers: ['aq-geo-line'] }).length;
        }),
      { timeout: 30_000 },
    )
    .toBeGreaterThan(0);

  // Тап по кластеру → плавный зум к раскрытию (AC2): кластер исчезает, тройка распадается на
  // отдельные маркеры (+ verified-точка остаётся в окне на expansion-зуме ~13), среди них
  // флаг-пин с глифом «!» (сигнал не потерялся).
  await cluster.click();
  await expect(page.locator('.aq-map-cluster')).toHaveCount(0, { timeout: 30_000 });
  await expect(page.locator('.aq-map-marker')).toHaveCount(4, { timeout: 30_000 });
  const flagged = page.locator('.aq-map-marker--flagged');
  await expect(flagged).toHaveCount(1);
  await expect(flagged.locator('.aq-map-marker__glyph')).toHaveText('!');

  // Выбор маркера: тап → двойное кольцо (--selected) + превью-лист (Story 3.5, AC1).
  await flagged.click();
  await expect(flagged).toHaveClass(/aq-map-marker--selected/);
  await expect(page.getByTestId('preview-sheet')).toBeVisible();
});

// Два сценария РАЗДЕЛЬНО (код-ревью 3.4): сервер не может выдать {items:[], truncated:true}
// (truncated ⇒ выдача = полный cap) — прежний единый мок закреплял взаимоисключающие плашки.
test('честное пустое окно: строка «нет объектов», плашки усечения НЕТ', async ({ page }) => {
  await page.route('**/api/map/objects*', (route) =>
    route.fulfill({ json: { items: [], truncated: false, ungeocoded_count: 0 } }),
  );

  await page.goto('/map');
  // Пустая выдача — честная строка (kk-дефолт или ru), НЕ пустой экран.
  await expect(
    page.getByRole('status').filter({ hasText: /нысандар жоқ|нет объектов/i }),
  ).toBeVisible({ timeout: 30_000 });
  // Усечения нет — и плашки нет (не «всё сразу»).
  await expect(
    page.getByRole('status').filter({ hasText: /көрсетілмеген|не все объекты/i }),
  ).toHaveCount(0);
  // Счётчика «без точки» нет при N=0.
  await expect(page.locator('.aq-map-ungeocoded-more')).toHaveCount(0);
});

test('честное усечение: truncated=true при непустой выдаче — видимая плашка, не тихое обрезание', async ({
  page,
}) => {
  await page.route('**/api/map/objects*', (route) =>
    route.fulfill({ json: { ...OBJECTS, truncated: true } }),
  );

  await page.goto('/map');
  await expect(
    page.getByRole('status').filter({ hasText: /көрсетілмеген|не все объекты/i }),
  ).toBeVisible({ timeout: 30_000 });
  // Объекты при этом отрисованы (усечение — не пустота): кластер тройки виден.
  await expect(page.locator('.aq-map-cluster')).toHaveCount(1, { timeout: 30_000 });
});

test('ошибка загрузки объектов — role=alert, карта не притворяется пустой-чистой', async ({
  page,
}) => {
  await page.route('**/api/map/objects*', (route) =>
    route.fulfill({
      status: 500,
      json: { error: { code: 'INTERNAL', message: 'internal error' } },
    }),
  );

  await page.goto('/map');
  await expect(page.getByRole('alert')).toBeVisible({ timeout: 30_000 });
});

// --- Story 3.5 (FR-8): превью объекта и переход в карточку ---

// Один одиночный verified-маркер близко к центру — без кластера, тап детерминирован.
const SINGLE_POINT = {
  items: [
    {
      public_id: '0198a3b0-0000-7000-8000-00000000c004',
      goszakup_contract_id: 'DEMO-GEO-04',
      geocode_status: 'verified',
      has_active_flag: false,
      length_km: null,
      geom: { type: 'Point', coordinates: [71.45, 51.12] },
    },
  ],
  truncated: false,
  ungeocoded_count: 0,
};

test('превью маркера (AC1/AC2): мини-карточка → «Подробнее» → карточка; Escape возвращает фокус; Tab заперт', async ({
  page,
}) => {
  await page.route('**/api/map/objects*', (route) => route.fulfill({ json: SINGLE_POINT }));
  await routeContracts(page, { 'DEMO-GEO-04': contractFixture('DEMO-GEO-04') });

  await page.goto('/map');
  const marker = page.locator('.aq-map-marker--confirmed');
  await expect(marker).toHaveCount(1, { timeout: 30_000 });
  await marker.click();

  // Лист виден: сумма отформатирована (₸), подрядчик — честное состояние (нет данных, не «0»),
  // ссылка-первоисточник (UX-DR18) ведёт на goszakup.
  const sheet = page.getByTestId('preview-sheet');
  await expect(sheet).toBeVisible();
  await expect(page.getByTestId('preview-amount')).toContainText('₸');
  await expect(page.getByTestId('preview-amount')).toContainText('90');
  await expect(page.getByTestId('preview-supplier')).toContainText(/жоқ|нет данных/i);
  await expect(sheet.locator('.aq-sheet__source')).toHaveAttribute(
    'href',
    'https://goszakup.gov.kz/ru/contract/DEMO-GEO-04',
  );

  // Focus-trap (AC2): десяток Tab — фокус не покидает лист.
  for (let i = 0; i < 10; i++) {
    await page.keyboard.press('Tab');
    const inside = await page.evaluate(
      () => document.activeElement?.closest('[data-testid="preview-sheet"]') !== null,
    );
    expect(inside, `Tab №${i + 1} увёл фокус из листа`).toBe(true);
  }
  // Реверс-цикл (код-ревью 3.5): Shift+Tab тоже заперт (самая хитрая ветка условий трапа).
  for (let i = 0; i < 5; i++) {
    await page.keyboard.press('Shift+Tab');
    const inside = await page.evaluate(
      () => document.activeElement?.closest('[data-testid="preview-sheet"]') !== null,
    );
    expect(inside, `Shift+Tab №${i + 1} увёл фокус из листа`).toBe(true);
  }

  // Escape: лист закрыт, выбор снят, фокус ВЕРНУЛСЯ на триггер-маркер (AC2).
  await page.keyboard.press('Escape');
  await expect(sheet).toHaveCount(0);
  await expect(marker).not.toHaveClass(/aq-map-marker--selected/);
  await expect(marker).toBeFocused();

  // «Подробнее» → Карточка контракта (FR-10), один уровень глубины.
  await marker.click();
  await page.getByTestId('preview-more').click();
  await expect(page).toHaveURL(/\/contracts\/DEMO-GEO-04/);
});

test('детенты (AC1): peek без флагов → ручка → half с флаг-бейджем в нейтральной рамке', async ({
  page,
}) => {
  const FLAGGED_POINT = {
    ...SINGLE_POINT,
    items: [
      {
        ...SINGLE_POINT.items[0],
        public_id: '0198a3b0-0000-7000-8000-00000000c001',
        goszakup_contract_id: 'DEMO-GEO-01',
        geocode_status: 'auto',
        has_active_flag: true,
      },
    ],
  };
  await page.route('**/api/map/objects*', (route) => route.fulfill({ json: FLAGGED_POINT }));
  await routeContracts(page, {
    'DEMO-GEO-01': contractFixture('DEMO-GEO-01', { flags: FLAG_RAISED }),
  });

  await page.goto('/map');
  const marker = page.locator('.aq-map-marker--flagged');
  await expect(marker).toHaveCount(1, { timeout: 30_000 });
  await marker.click();

  const sheet = page.getByTestId('preview-sheet');
  await expect(sheet).toBeVisible();
  await expect(page.getByTestId('preview-amount')).toContainText('₸');
  // peek: флагов НЕТ (half = мини-карточка + флаги, EXPERIENCE.md:222).
  await expect(sheet.locator('.aq-flag')).toHaveCount(0);

  // Ручка — не-жестовый путь к детенту (UX-DR31): Enter/клик → half → бейдж с рамкой «сигнал…».
  const detent = page.getByTestId('preview-detent');
  await expect(detent).toHaveAttribute('aria-expanded', 'false');
  await detent.click();
  await expect(detent).toHaveAttribute('aria-expanded', 'true');
  await expect(sheet.locator('.aq-flag__text')).toContainText(/сигнал/i);

  // Свайпы на ручке (код-ревью 3.5, High-фикс: capture — жест не «протекает» кликом в бэкдроп).
  // hover() ПЕРЕД замером: детент анимирует max-height (200мс) и ручка едет вверх — замер до
  // стабилизации промахивался мимо ручки в контент (actionability-wait Playwright это снимает).
  const dragHandle = async (dy: number) => {
    const handle = page.getByTestId('preview-detent');
    await handle.hover();
    const box = await handle.boundingBox();
    expect(box).not.toBeNull();
    if (box === null) return;
    const cx = box.x + box.width / 2;
    const cy = box.y + box.height / 2;
    await page.mouse.move(cx, cy);
    await page.mouse.down();
    await page.mouse.move(cx, cy + dy, { steps: 4 });
    await page.mouse.up();
  };
  // вниз из half → peek (НЕ закрытие — click по бэкдропу подавлен capture'ом).
  await dragHandle(100);
  await expect(detent).toHaveAttribute('aria-expanded', 'false');
  await expect(sheet).toBeVisible();
  // вверх из peek → half (основной жест раскрытия — до фикса он ЗАКРЫВАЛ лист).
  await dragHandle(-100);
  await expect(detent).toHaveAttribute('aria-expanded', 'true');
  // вниз, вниз: half → peek → закрытие (свайп вниз за нижним детентом, UX-DR15).
  await dragHandle(100);
  await dragHandle(100);
  await expect(sheet).toHaveCount(0);
});

test('совпадающие координаты (AC3): кластер-без-разворота → авто-зум → превью-список → карточка', async ({
  page,
}) => {
  const COINCIDENT = {
    items: [
      {
        public_id: '0198a3b0-0000-7000-8000-00000000c001',
        goszakup_contract_id: 'DEMO-GEO-01',
        geocode_status: 'auto',
        has_active_flag: false,
        length_km: null,
        geom: { type: 'Point', coordinates: [71.45, 51.12] },
      },
      {
        public_id: '0198a3b0-0000-7000-8000-00000000c002',
        goszakup_contract_id: 'DEMO-GEO-02',
        geocode_status: 'manual',
        has_active_flag: false,
        length_km: null,
        geom: { type: 'Point', coordinates: [71.45, 51.12] }, // ИДЕНТИЧНАЯ координата
      },
    ],
    truncated: false,
    ungeocoded_count: 0,
  };
  await page.route('**/api/map/objects*', (route) => route.fulfill({ json: COINCIDENT }));
  await routeContracts(page, {
    'DEMO-GEO-01': contractFixture('DEMO-GEO-01'),
    'DEMO-GEO-02': contractFixture('DEMO-GEO-02', {
      subject_ru: { value: 'Водопровод, тот же адрес (DEMO-GEO-02)', state: 'ok' },
      subject_kk: { value: 'Су құбыры, сол мекенжай (DEMO-GEO-02)', state: 'ok' },
    }),
  });

  await page.goto('/map');
  const cluster = page.locator('.aq-map-cluster');
  await expect(cluster).toHaveCount(1, { timeout: 30_000 });
  await expect(cluster.locator('.aq-map-cluster__count')).toHaveText('2');

  // Тап: зум не разваливает точки → авто-зум до предела + лист-список (не бесконечный ре-зум).
  await cluster.click();
  const list = page.getByTestId('preview-list');
  await expect(list).toBeVisible();
  await expect(list.locator('.aq-sheet__row')).toHaveCount(2);

  // Ряд ведёт в Карточку (D5).
  await list.locator('.aq-sheet__row-link').first().click();
  await expect(page).toHaveURL(/\/contracts\/DEMO-GEO-0[12]/);
});

test('идентичные линии (AC3, живой кейс seed): тап по оси → превью-список (2 ряда: предмет+сумма)', async ({
  page,
}) => {
  const TWIN_LINES = {
    items: [
      {
        public_id: '0198a3b0-0000-7000-8000-00000000c005',
        goszakup_contract_id: 'DEMO-GEO-05',
        geocode_status: 'auto',
        has_active_flag: false,
        length_km: 5.0,
        geom: {
          type: 'LineString',
          coordinates: [
            [71.43, 51.11],
            [71.47, 51.13],
          ],
        },
      },
      {
        public_id: '0198a3b0-0000-7000-8000-00000000c006',
        goszakup_contract_id: 'DEMO-GEO-06',
        geocode_status: 'manual',
        has_active_flag: false,
        length_km: 5.0,
        geom: {
          type: 'LineString',
          coordinates: [
            [71.43, 51.11],
            [71.47, 51.13],
          ], // идентичная ось (как 6 линий Есиль в seed)
        },
      },
    ],
    truncated: false,
    ungeocoded_count: 0,
  };
  await page.route('**/api/map/objects*', (route) => route.fulfill({ json: TWIN_LINES }));
  await routeContracts(page, {
    'DEMO-GEO-05': contractFixture('DEMO-GEO-05'),
    'DEMO-GEO-06': contractFixture('DEMO-GEO-06'),
  });

  await page.goto('/map');
  // Дождаться отрисовки линии (canvas — через тест-шов, паттерн 3.4).
  await expect
    .poll(
      () =>
        page.evaluate(() => {
          const m = (
            window as unknown as {
              __aqMapTest?: {
                getLayer(id: string): unknown;
                queryRenderedFeatures(opts: { layers: string[] }): unknown[];
              };
            }
          ).__aqMapTest;
          if (!m || !m.getLayer('aq-geo-line')) return -1;
          return m.queryRenderedFeatures({ layers: ['aq-geo-line'] }).length;
        }),
      { timeout: 30_000 },
    )
    .toBeGreaterThan(0);

  // Тап по середине оси: пиксель через project() тест-шва (canvas недоступен DOM-локаторам).
  const px = await page.evaluate(() => {
    const m = (
      window as unknown as {
        __aqMapTest?: { project(lnglat: [number, number]): { x: number; y: number } };
      }
    ).__aqMapTest;
    if (!m) return null;
    const p = m.project([71.45, 51.12]);
    return { x: p.x, y: p.y };
  });
  expect(px).not.toBeNull();
  const canvas = page.locator('.aq-map canvas');
  const box = await canvas.boundingBox();
  expect(box).not.toBeNull();
  if (px === null || box === null) return;
  await page.mouse.click(box.x + px.x, box.y + px.y);

  // Hit-test вернул обе линии → превью-список (не молчаливый первый объект).
  const list = page.getByTestId('preview-list');
  await expect(list).toBeVisible();
  await expect(list.locator('.aq-sheet__row')).toHaveCount(2);
});

test('404 контракта в превью → честное «нет данных», не общая ошибка (deferred:168)', async ({
  page,
}) => {
  await page.route('**/api/map/objects*', (route) => route.fulfill({ json: SINGLE_POINT }));
  await routeContracts(page, {}); // любой id → 404

  await page.goto('/map');
  const marker = page.locator('.aq-map-marker--confirmed');
  await expect(marker).toHaveCount(1, { timeout: 30_000 });
  await marker.click();

  const sheet = page.getByTestId('preview-sheet');
  await expect(sheet).toBeVisible();
  await expect(sheet.getByRole('alert')).toContainText(/жоқ|нет данных/i);
  // «Подробнее» на несуществующий контракт не предлагается... а закрыть можно.
  await expect(page.getByTestId('preview-more')).toHaveCount(0);
});

test('prefers-reduced-motion: превью открывается без анимаций (смоук)', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.route('**/api/map/objects*', (route) => route.fulfill({ json: SINGLE_POINT }));
  await routeContracts(page, { 'DEMO-GEO-04': contractFixture('DEMO-GEO-04') });

  await page.goto('/map');
  const marker = page.locator('.aq-map-marker--confirmed');
  await expect(marker).toHaveCount(1, { timeout: 30_000 });
  await marker.click();
  await expect(page.getByTestId('preview-sheet')).toBeVisible();
  await expect(page.getByTestId('preview-amount')).toContainText('₸');
});
