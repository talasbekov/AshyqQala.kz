import { test, expect } from '@playwright/test';

// Story 5.3: пороги методики из /api/methodology (единый источник; формула/пороги ВСЕГДА).
const METHODOLOGY = {
  version: 'v1.0',
  thresholds: {
    min_sample: 5,
    comparability_window_months: 24,
    price_per_km_deviation_factor: 1.5,
    monopoly_concentration_share: 0.5,
    monopoly_min_group_contracts: 5,
    single_participant_exclude_methods: ['из_одного_источника'],
  },
};

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
  customer: { value: null, state: 'no_data' },
  supplier: { value: null, state: 'no_data' },
  source_url: { value: 'https://goszakup.gov.kz/ru/contract/DEMO-0002', state: 'ok' },
  act: {
    present: false,
    act_date: { value: null, state: 'no_data' },
    signer: { value: null, state: 'no_data' },
    source_url: { value: null, state: 'no_data' },
  },
  // Story 5.1: price_per_km RAISED из API (evidence пересчитываемости) → бейдж + методика ×1.5.
  flags: [
    {
      flag_id: 'single_participant',
      state: 'not_raised',
      methodology_version: { value: 'v1.0', state: 'ok' },
      detected_at: { value: '2026-05-12T10:00:00Z', state: 'ok' },
      evidence: null,
    },
    {
      flag_id: 'price_per_km',
      state: 'raised',
      methodology_version: { value: 'v1.0', state: 'ok' },
      detected_at: { value: '2026-05-12T10:00:00Z', state: 'ok' },
      evidence: {
        price_per_km: 71000000,
        median: 38400000,
        sample_size: 9,
        deviation_factor: 1.5,
        comparability_key: 'road|710000000',
        methodology_version: 'v1.0',
      },
    },
  ],
  imported_at: { value: '2026-02-10T10:00:00Z', state: 'ok' },
  updated_at: { value: '2026-02-10T10:00:00Z', state: 'ok' },
};

test('карточка 1.9: нейтральный флаг → методика → дверь + первоисточник', async ({ page }) => {
  await page.route('**/api/contracts/DEMO-0002', (route) => route.fulfill({ json: DEMO2 }));
  await page.route('**/api/methodology', (route) => route.fulfill({ json: METHODOLOGY }));
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
  customer: { value: null, state: 'no_data' },
  supplier: { value: null, state: 'no_data' },
  source_url: { value: 'https://goszakup.gov.kz/ru/contract/DEMO-0003', state: 'ok' },
  act: {
    present: false,
    act_date: { value: null, state: 'no_data' },
    signer: { value: null, state: 'no_data' },
    source_url: { value: null, state: 'no_data' },
  },
  // Story 5.1: НИ ОДНОГО raised. Честная реконструкция (AC4): single_participant проверен (not_raised),
  // price_per_km без строки → insufficient_data. Оба видны строкой статуса — «всё чисто» не подразумевается.
  flags: [
    {
      flag_id: 'single_participant',
      state: 'not_raised',
      methodology_version: { value: 'v1.0', state: 'ok' },
      detected_at: { value: '2026-05-12T10:00:00Z', state: 'ok' },
      evidence: null,
    },
    {
      flag_id: 'price_per_km',
      state: 'insufficient_data',
      methodology_version: { value: null, state: 'no_data' },
      detected_at: { value: null, state: 'no_data' },
      evidence: null,
    },
  ],
  imported_at: { value: '2026-04-05T10:00:00Z', state: 'ok' },
  updated_at: { value: '2026-04-05T10:00:00Z', state: 'ok' },
};

test('карточка 5.1: честное «нет данных» + видимая реконструкция (insufficient ≠ «всё чисто»)', async ({
  page,
}) => {
  await page.route('**/api/contracts/DEMO-0003', (route) => route.fulfill({ json: DEMO3 }));
  await page.route('**/api/methodology', (route) => route.fulfill({ json: METHODOLOGY }));
  await page.goto('/contracts/DEMO-0003');

  // Поле plan_end = NULL → честное «нет данных» (AC1), не пустота.
  await expect(page.getByText(/нет данных|деректер жоқ/i).first()).toBeVisible();

  // Активных сигналов нет — заголовок честный (не «Сигналы (0)»).
  await expect(
    page.getByRole('heading', { name: /Активных сигналов нет|Белсенді сигнал жоқ/ }),
  ).toBeVisible();

  // AC4: реконструкция видна строкой статуса — «недостаточно данных для оценки» (НЕ молчание = «чисто»).
  await expect(
    page.getByText(/недостаточно данных для оценки|бағалауға дерек жеткіліксіз/i).first(),
  ).toBeVisible();

  // Story 5.3 (AC-2): строка статуса кликабельна → методика для НЕ-raised: формула/пороги ВСЕГДА +
  // «сигнал не выставлен» + какого порога не хватило (без выдуманных чисел).
  await page
    .getByRole('button', { name: /Шақырым құны|Цена за км/ })
    .first()
    .click();
  const dialog = page.getByRole('dialog');
  await expect(dialog).toBeVisible();
  await expect(dialog).toContainText('1.5'); // формула из /api/methodology, не из evidence
  await expect(
    dialog.getByText(/Сигнал не выставлен|Сигнал қойылмады/).first(),
  ).toBeVisible();
});
