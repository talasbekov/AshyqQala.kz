import { test, expect } from '@playwright/test';

// Smoke скелет-карты (Story 1.8): маршрут /map → карта (MapLibre) → маркер → превью → переход в
// карточку. Ответ /api замокан (route.fulfill) — CI без docker. Полный байт-стек — ручная DoD.
const DEMO = {
  goszakup_contract_id: 'DEMO-0001',
  subject_ru: {
    value: 'Демонстрационный контракт: ремонт автодороги (синтетика S-0)',
    state: 'ok',
  },
  subject_kk: { value: 'Демонстрациялық келісімшарт: автожол жөндеу (синтетика S-0)', state: 'ok' },
  amount_tng: { value: '123456789', state: 'ok' },
  sign_date: { value: '2026-03-15', state: 'ok' },
  plan_start: { value: '2026-04-01', state: 'ok' },
  plan_end: { value: '2026-09-30', state: 'ok' },
  status: { value: 'active', state: 'ok' },
  direction: { value: 'road', state: 'ok' },
  kato_code: { value: '710000000', state: 'ok' },
  source_url: { value: 'https://goszakup.gov.kz/ru/contract/DEMO-0001', state: 'ok' },
  imported_at: { value: '2026-03-15T10:00:00Z', state: 'ok' },
  updated_at: { value: '2026-03-15T10:00:00Z', state: 'ok' },
};

test('скелет-карта: маршрут → маркер → превью → переход в карточку', async ({ page }) => {
  await page.route('**/api/contracts/DEMO-0001', (route) => route.fulfill({ json: DEMO }));

  await page.goto('/map');

  // Карта-контейнер присутствует (role=application, AC1).
  await expect(page.getByRole('application')).toBeVisible();

  // Негеокодированный контракт честно в списке вне карты (AC3): отдельный пункт-ссылка.
  await expect(page.getByRole('link', { name: 'DEMO-0004' })).toBeVisible();

  // Маркер появляется после загрузки карты (DOM-кнопка с aria-label, AC1).
  const marker = page.getByRole('button', { name: /DEMO-0001/ });
  await expect(marker).toBeVisible({ timeout: 30_000 });

  // Тап по маркеру → нижний лист превью (AC2).
  await marker.click();
  await expect(page.getByRole('dialog')).toBeVisible();

  // Переход «Подробнее» → карточка контракта.
  await page.getByRole('link', { name: /Толығырақ|Подробнее/ }).click();
  await expect(page).toHaveURL(/\/contracts\/DEMO-0001/);
  // Заголовок именно карточки (уникальный id) — не лист превью (у обоих одинаковый subject).
  await expect(page.locator('#contract-subject')).toBeVisible();
});
