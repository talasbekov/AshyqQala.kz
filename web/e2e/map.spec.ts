import { test, expect } from '@playwright/test';

// Smoke ранней карты лотов (Story 0.8): /map → реальные лоты из /api/lots (замокан) → маркер
// геокодированного лота → превью ЛОТА с честной плашкой «ожидает официального источника»;
// негеокодированный лот честно в списке вне карты. Ответ /api замокан (route.fulfill) — CI без docker.
const LOTS = [
  {
    goszakup_lot_id: 'scrape-0001',
    subject_ru: { value: 'Реконструкция автодороги по ул. Абая', state: 'ok' },
    subject_kk: { value: 'Абай к. бойынша автожол реконструкциясы', state: 'ok' },
    amount_tng: { value: '240000000', state: 'ok' },
    lon: 71.4306,
    lat: 51.1281,
    geocode_state: 'ok',
  },
  {
    goszakup_lot_id: 'scrape-0002',
    subject_ru: { value: 'Ремонт сетей водоснабжения', state: 'ok' },
    subject_kk: { value: 'Сумен жабдықтау желілерін жөндеу', state: 'ok' },
    amount_tng: { value: '5000000', state: 'ok' },
    lon: null,
    lat: null,
    geocode_state: 'geocode_failed',
  },
];

test('ранняя карта лотов: /api/lots → маркер → превью лота + честная плашка', async ({ page }) => {
  await page.route('**/api/lots', (route) => route.fulfill({ json: LOTS }));

  await page.goto('/map');

  // Карта-контейнер присутствует (role=application, AC1).
  await expect(page.getByRole('application')).toBeVisible();

  // Честная плашка состояния контейнера видна (AC1): временные/частичные данные (kk-дефолт или ru).
  await expect(page.getByText(/Временные|Уақытша/i)).toBeVisible();
  // AC1: направления помечены «предв.» (keyword-bias) — вторая строка плашки.
  await expect(page.getByText(/предв\.|алдын ала/i)).toBeVisible();

  // Негеокодированный лот честно в списке вне карты (AC3): отдельный пункт-кнопка (имя = голый id).
  await expect(page.getByRole('button', { name: 'scrape-0002' })).toBeVisible();

  // Маркер геокодированного лота появляется после загрузки карты (DOM-кнопка с aria-label, AC1).
  const marker = page.getByRole('button', { name: /scrape-0001/ });
  await expect(marker).toBeVisible({ timeout: 30_000 });

  // P9 НЕГАТИВ (защита гардрейла честности): негеокодированный scrape-0002 НЕ получает маркер на
  // карте — он только в списке вне карты. Маркер опознаём по подписи map.marker_label
  // («Объект на карте {{id}}» / «Картадағы нысан {{id}}»), а не по голому id (так пункт списка не
  // спутать с маркером). Регрессия, выдумавшая точку/маркер для негеокода, упала бы здесь.
  await expect(
    page.getByRole('button', { name: /(Объект на карте|Картадағы нысан) scrape-0002/i }),
  ).toHaveCount(0);
  // И всего ровно один маркер на карте (только геокодированный scrape-0001).
  await expect(page.locator('.aq-map-marker')).toHaveCount(1);

  // Тап по маркеру → нижний лист превью ЛОТА (AC2).
  await marker.click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toBeVisible();

  // AC2: блок контракта/сигналов — честная плашка «ожидает официального источника» (не пустота/ошибка).
  await expect(dialog.getByText(/ожидает официального|ресми дереккөзді/i)).toBeVisible();
});
