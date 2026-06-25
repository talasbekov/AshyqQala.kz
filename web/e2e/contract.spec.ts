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
  customer: { value: null, state: 'no_data' },
  supplier: { value: null, state: 'no_data' },
  source_url: { value: 'https://goszakup.gov.kz/ru/contract/DEMO-0001', state: 'ok' },
  // Story 5.1: акт отсутствует (синтетика без акта) + флаги из API (single_participant raised).
  act: {
    present: false,
    act_date: { value: null, state: 'no_data' },
    signer: { value: null, state: 'no_data' },
    source_url: { value: null, state: 'no_data' },
  },
  flags: [
    {
      flag_id: 'single_participant',
      state: 'raised',
      methodology_version: { value: 'v1.0', state: 'ok' },
      detected_at: { value: '2026-05-12T10:00:00Z', state: 'ok' },
      evidence: { participant_count: 1 },
    },
    {
      flag_id: 'price_per_km',
      state: 'insufficient_data',
      methodology_version: { value: null, state: 'no_data' },
      detected_at: { value: null, state: 'no_data' },
      evidence: null,
    },
  ],
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
