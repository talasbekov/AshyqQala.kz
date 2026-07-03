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

test('каноническая карта: кластер с амбер-кольцом → раскрытие; глифы; счётчик «без точки»', async ({
  page,
}) => {
  await page.route('**/api/map/objects*', (route) => route.fulfill({ json: OBJECTS }));

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

  // Выбор маркера (D7): тап → двойное кольцо (--selected), глиф сохраняется; z-priority через класс.
  await flagged.click();
  await expect(flagged).toHaveClass(/aq-map-marker--selected/);
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
