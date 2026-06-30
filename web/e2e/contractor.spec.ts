import { test, expect } from '@playwright/test';

// Замоканные ответы /api/contractors/{bin} (форма Contractor из schema.gen.ts; CI без docker).
// Smoke закрепляет: маршрут → Query → карточка + честные состояния профиля (FR-13/FR-14, Story 5.2).
function base(bin: string) {
  return {
    bin,
    name_ru: { value: 'ТОО Астана Жол', state: 'ok' },
    name_kk: { value: 'Астана Жол ЖШС', state: 'ok' },
    reg_kato: { value: '710000000', state: 'ok' },
    is_customer: false,
    is_supplier: true,
    profile: { state: 'incomplete' },
    contract_count: { value: null, state: 'no_data' },
    total_amount_tng: { value: null, state: 'no_data' },
    regions: [] as string[],
    contracts: [] as unknown[],
    flags: [
      {
        flag_id: 'monopoly',
        state: 'insufficient_data',
        methodology_version: { value: null, state: 'no_data' },
        detected_at: { value: null, state: 'no_data' },
        evidence: null,
      },
      {
        flag_id: 'rnu',
        state: 'insufficient_data',
        methodology_version: { value: null, state: 'no_data' },
        detected_at: { value: null, state: 'no_data' },
        evidence: null,
      },
    ],
    rnu_marks: [] as unknown[],
    source_url: { value: 'https://goszakup.gov.kz/ru/registry', state: 'ok' },
    imported_at: { value: '2026-06-01T10:00:00Z', state: 'ok' },
    updated_at: { value: '2026-06-01T10:00:00Z', state: 'ok' },
  };
}

test('подрядчик: «профиль неполный» (нет связанных контрактов) — count честный no_data', async ({
  page,
}) => {
  const dto = base('222222222222');
  await page.route('**/api/contractors/222222222222', (route) => route.fulfill({ json: dto }));
  await page.goto('/contractors/222222222222');
  await expect(page.getByTestId('contractor-bin')).toHaveText('222222222222');
  await expect(page.getByTestId('contractor-profile')).toBeVisible();
});

test('подрядчик: полный — монополия raised + активная РНУ-метка со своим source-link + список контрактов', async ({
  page,
}) => {
  const dto = base('222222222222');
  dto.profile = { state: 'partial' };
  dto.contract_count = { value: '2', state: 'ok' };
  dto.total_amount_tng = { value: '363456789', state: 'ok' };
  dto.regions = ['710000000'];
  dto.contracts = [
    {
      goszakup_contract_id: 'DEMO-0001',
      subject_ru: { value: 'Ремонт автодороги', state: 'ok' },
      subject_kk: { value: 'Автожол жөндеу', state: 'ok' },
      amount_tng: { value: '123456789', state: 'ok' },
      kato_code: { value: '710000000', state: 'ok' },
      direction: { value: 'road', state: 'ok' },
    },
  ];
  dto.flags = [
    {
      flag_id: 'monopoly',
      state: 'raised',
      methodology_version: { value: 'v1.0', state: 'ok' },
      detected_at: { value: '2026-06-10T10:00:00Z', state: 'ok' },
      evidence: { share: 0.648 },
    },
    {
      flag_id: 'rnu',
      state: 'insufficient_data',
      methodology_version: { value: null, state: 'no_data' },
      detected_at: { value: null, state: 'no_data' },
      evidence: null,
    },
  ];
  dto.rnu_marks = [
    {
      active: true,
      start_date: { value: '2026-01-15', state: 'ok' },
      end_date: { value: null, state: 'no_data' },
      reason_ref: { value: null, state: 'no_data' },
      source_url: { value: 'https://goszakup.gov.kz/ru/registry/rnu', state: 'ok' },
      registry_id: { value: 'RNU-A-1', state: 'ok' },
    },
  ];
  await page.route('**/api/contractors/222222222222', (route) => route.fulfill({ json: dto }));
  await page.goto('/contractors/222222222222');
  await expect(page.getByTestId('flag-monopoly')).toBeVisible();
  // AC-3: у raised-флага своя {source-link} + своя «Сообщить об ошибке» (2 ссылки)
  await expect(page.getByTestId('flag-monopoly').getByRole('link')).toHaveCount(2);
  await expect(page.getByTestId('rnu-mark')).toBeVisible();
  // у активной РНУ-метки есть своя ссылка на реестр (AC-3) + локальная «Сообщить об ошибке»
  await expect(page.getByTestId('rnu-mark').getByRole('link')).toHaveCount(2);
  // FR-13: список контрактов с ссылкой на карточку контракта (lang-агностично — по href)
  await expect(page.locator('a[href="/contracts/DEMO-0001"]')).toBeVisible();
});

test('подрядчик: «профиль уточняется» (manual/conflict) — состояние видно', async ({ page }) => {
  const dto = base('444444444444');
  dto.profile = { state: 'unverified' };
  await page.route('**/api/contractors/444444444444', (route) => route.fulfill({ json: dto }));
  await page.goto('/contractors/444444444444');
  await expect(page.getByTestId('contractor-profile')).toBeVisible();
});

test('подрядчик: 404 → честное «нет данных по подрядчику»', async ({ page }) => {
  await page.route('**/api/contractors/999999999999', (route) =>
    route.fulfill({
      status: 404,
      json: { error: { code: 'NOT_FOUND', message: 'contractor not found' } },
    }),
  );
  await page.goto('/contractors/999999999999');
  await expect(page.getByRole('alert')).toBeVisible();
});
