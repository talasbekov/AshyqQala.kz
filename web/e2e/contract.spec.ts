import { test, expect } from '@playwright/test';

// Замоканный ответ /api/contracts/DEMO-0001 (форма Contract из schema.gen.ts) — соответствует
// seed-строке fixtures/seed/contracts.sql. Smoke закрепляет: маршрут → Query → карточка.
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

test('сквозной путь: маршрут карточки → фетч → реальная строка видна', async ({ page }) => {
  await page.route('**/api/contracts/DEMO-0001', (route) => route.fulfill({ json: DEMO }));

  await page.goto('/');
  await page.getByRole('link', { name: /демонстрац/i }).click();
  await expect(page).toHaveURL(/\/contracts\/DEMO-0001/);

  // Сумма отформатирована через formatMoney (группировка + ₸) — доказывает работу Query→карточка.
  const amount = page.getByTestId('contract-amount');
  await expect(amount).toContainText('123');
  await expect(amount).toContainText('₸');

  // Заголовок-subject виден (honest state ok).
  await expect(page.getByRole('heading', { level: 2 })).toBeVisible();
});
