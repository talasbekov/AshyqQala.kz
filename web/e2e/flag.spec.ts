import { test, expect } from '@playwright/test';

// Smoke вертикального среза (Story 1.9): карточка → бейдж нейтрального флага → экран методики →
// дверь «Сообщить об ошибке» + первоисточник. Ответ /api замокан (route.fulfill). Язык по умолчанию kk.
const DEMO2 = {
  goszakup_contract_id: 'DEMO-0002',
  subject_ru: {
    value: 'Строительство автомобильной дороги (демо-история: цена за км)',
    state: 'ok',
  },
  subject_kk: { value: 'Автомобиль жолын салу (демо-оқиға: шақырым құны)', state: 'ok' },
  amount_tng: { value: '240000000', state: 'ok' },
  sign_date: { value: '2026-02-10', state: 'ok' },
  plan_start: { value: '2026-03-01', state: 'ok' },
  plan_end: { value: '2026-11-30', state: 'ok' },
  status: { value: 'active', state: 'ok' },
  direction: { value: 'road', state: 'ok' },
  kato_code: { value: '710000000', state: 'ok' },
  source_url: { value: 'https://goszakup.gov.kz/ru/contract/DEMO-0002', state: 'ok' },
  imported_at: { value: '2026-02-10T10:00:00Z', state: 'ok' },
  updated_at: { value: '2026-02-10T10:00:00Z', state: 'ok' },
};

test('карточка 1.9: нейтральный флаг → методика → дверь + первоисточник', async ({ page }) => {
  await page.route('**/api/contracts/DEMO-0002', (route) => route.fulfill({ json: DEMO2 }));
  await page.goto('/contracts/DEMO-0002');

  // Бейдж нейтрального флага (role=button, текст «… — сигнал, требующий проверки»). AC1/AC2.
  const badge = page.getByRole('button', {
    name: /тексеруді талап ететін сигнал|сигнал, требующий проверки/,
  });
  await expect(badge).toBeVisible();

  // GATE: рядом дверь «Сообщить об ошибке» (mailto). AC2.
  const report = page
    .getByRole('link', { name: /Қате туралы хабарлау|Сообщить об ошибке/ })
    .first();
  await expect(report).toBeVisible();
  await expect(report).toHaveAttribute('href', /^mailto:/);

  // GATE: путь к методике → открывает экран методики с формулой ×1.5. AC1/AC2.
  await page
    .getByRole('button', { name: /Қалай есептелді|Как это посчитано/ })
    .first()
    .click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toBeVisible();
  await expect(dialog).toContainText('1.5');

  // Первоисточник доступен в методике (FR-11 демо-срез).
  await expect(dialog.getByRole('link', { name: /goszakup/i })).toBeVisible();
});

// DEMO-0003: честное «нет данных» (plan_end NULL) + методика «недостаточно сопоставимых данных» (<5). AC3.
const DEMO3 = {
  goszakup_contract_id: 'DEMO-0003',
  subject_ru: {
    value: 'Ремонт участка дороги (демо-история: мало сопоставимых данных)',
    state: 'ok',
  },
  subject_kk: { value: 'Жол учаскесін жөндеу (демо-оқиға: салыстырмалы дерек аз)', state: 'ok' },
  amount_tng: { value: '18000000', state: 'ok' },
  sign_date: { value: '2026-04-05', state: 'ok' },
  plan_start: { value: '2026-05-01', state: 'ok' },
  plan_end: { value: null, state: 'no_data' },
  status: { value: 'active', state: 'ok' },
  direction: { value: 'road', state: 'ok' },
  kato_code: { value: '710000000', state: 'ok' },
  source_url: { value: 'https://goszakup.gov.kz/ru/contract/DEMO-0003', state: 'ok' },
  imported_at: { value: '2026-04-05T10:00:00Z', state: 'ok' },
  updated_at: { value: '2026-04-05T10:00:00Z', state: 'ok' },
};

test('карточка 1.9: честное «нет данных» + методика «недостаточно сопоставимых данных»', async ({
  page,
}) => {
  await page.route('**/api/contracts/DEMO-0003', (route) => route.fulfill({ json: DEMO3 }));
  await page.goto('/contracts/DEMO-0003');

  // Поле plan_end = NULL → честное «нет данных» (AC3), не пустота.
  await expect(page.getByText(/нет данных|деректер жоқ/i).first()).toBeVisible();

  // Методика флага в состоянии «недостаточно»: формула/пороги показаны ВСЕГДА + честная плашка.
  await page
    .getByRole('button', { name: /Қалай есептелді|Как это посчитано/ })
    .first()
    .click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toBeVisible();
  await expect(dialog).toContainText('1.5');
  await expect(
    dialog.getByText(/недостаточно сопоставимых данных|салыстыруға жеткілікті дерек жоқ/i).first(),
  ).toBeVisible();
});
