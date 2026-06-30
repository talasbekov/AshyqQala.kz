import { test, expect } from '@playwright/test';

// Замоканные ответы /api/districts/{kato} (форма DistrictAggregates из schema.gen.ts; CI без docker).
// Smoke закрепляет: маршрут → Query → страница района (FR-17, Story 6.3): сводка, container_state отдельной
// строкой, список объектов со ссылками, нейтральный сигнал-блок, честная ошибка на 400.
// notComparableMedian — медиана направления в текущем токен-независимом состоянии (нет length_km, Epic 3):
// район/город/дельта = not_comparable (честный skip, НЕ число).
function notComparableMedian(direction: string) {
  return {
    direction,
    district_median_tng: { value: null, state: 'not_comparable' },
    city_median_tng: { value: null, state: 'not_comparable' },
    comparison_pct: { value: null, state: 'not_comparable' },
    district_sample_size: '0',
    city_sample_size: '0',
    district_comparability_key: `direction=${direction}|kato=710000000`,
    city_comparability_key: `direction=${direction}|kato=71`,
  };
}

function base(kato: string) {
  return {
    kato,
    // Имя района — no_data (КАТО-код не подтверждён, Story 0.1) → заголовок по коду КАТО.
    name_ru: { value: null, state: 'no_data' },
    name_kk: { value: null, state: 'no_data' },
    contract_count: '0',
    total_amount_tng: { value: null, state: 'no_data' },
    active_flags_count: '0',
    flags_by_type: [] as unknown[],
    container_state: ['no_contracts'] as string[],
    objects: [] as unknown[],
    // Медиана ₸/км район vs город (FR-18): сейчас not_comparable по обоим направлениям (нет length_km).
    medians: [notComparableMedian('road'), notComparableMedian('water')] as unknown[],
    median_methodology_version: 'v1.0',
  };
}

test('район: населённый — сводка, сигналы района, список объектов со ссылкой, not_geocoded строкой', async ({
  page,
}) => {
  const dto = base('710000000');
  dto.contract_count = '18';
  dto.total_amount_tng = { value: '2400000000', state: 'ok' };
  dto.active_flags_count = '6';
  dto.flags_by_type = [
    { flag_type: 'single_participant', count: '5' },
    { flag_type: 'monopoly', count: '1' },
  ];
  dto.container_state = ['not_geocoded'];
  dto.objects = [
    {
      goszakup_contract_id: 'DEMO-0002',
      subject_ru: { value: 'Строительство автодороги', state: 'ok' },
      subject_kk: { value: 'Автожол салу', state: 'ok' },
      amount_tng: { value: '240000000', state: 'ok' },
      kato_code: { value: '710000000', state: 'ok' },
      direction: { value: 'road', state: 'ok' },
      has_active_flag: true,
    },
  ];
  await page.route('**/api/districts/710000000', (route) => route.fulfill({ json: dto }));
  await page.goto('/districts/710000000');

  await expect(page.getByTestId('district-kato')).toHaveText('710000000');
  await expect(page.getByTestId('district-objects-count')).toHaveText('18');
  await expect(page.getByTestId('district-active-flags')).toHaveText('6');
  // Сигнал-блок: единственный участник с количеством (нейтральная рамка).
  await expect(page.getByTestId('district-flag-single_participant')).toBeVisible();
  await expect(page.getByTestId('district-flag-count-single_participant')).toHaveText('5');
  // Список объектов: ссылка на карточку контракта (lang-агностично по href).
  await expect(page.locator('a[href="/contracts/DEMO-0002"]')).toBeVisible();
  // container_state «not_geocoded» — отдельной строкой (AR-17).
  await expect(page.getByTestId('district-state-not_geocoded')).toBeVisible();
});

test('район: медиана ₸/км — дорога с полосами район/город и % дельтой; вода — честное состояние', async ({
  page,
}) => {
  const dto = base('710000000');
  dto.contract_count = '9';
  dto.container_state = ['not_geocoded'];
  dto.objects = [
    {
      goszakup_contract_id: 'DEMO-0002',
      subject_ru: { value: 'Строительство автодороги', state: 'ok' },
      subject_kk: { value: 'Автожол салу', state: 'ok' },
      amount_tng: { value: '240000000', state: 'ok' },
      kato_code: { value: '710000000', state: 'ok' },
      direction: { value: 'road', state: 'ok' },
      has_active_flag: false,
    },
  ];
  // Дорога: достаточная выборка → медиана район 384, город 331, +16%. Вода остаётся not_comparable.
  dto.medians = [
    {
      direction: 'road',
      district_median_tng: { value: '38400000', state: 'ok' },
      city_median_tng: { value: '33100000', state: 'ok' },
      comparison_pct: { value: '16', state: 'ok' },
      district_sample_size: '9',
      city_sample_size: '40',
      district_comparability_key: 'direction=road|kato=710000000',
      city_comparability_key: 'direction=road|kato=71',
    },
    notComparableMedian('water'),
  ];
  await page.route('**/api/districts/710000000', (route) => route.fulfill({ json: dto }));
  await page.goto('/districts/710000000');

  // Дорога: блок медианы виден, обе полосы, % дельта +16% (знак/число — lang-агностично).
  await expect(page.getByTestId('district-median-road')).toBeVisible();
  await expect(page.getByTestId('district-median-road-district')).toBeVisible();
  await expect(page.getByTestId('district-median-road-city')).toBeVisible();
  await expect(page.getByTestId('district-median-road-delta')).toContainText('+16%');
  // Вода: честное состояние «недостаточно сопоставимых данных» — НЕ число, НЕ 0.
  await expect(page.getByTestId('district-median-water')).toBeVisible();
  await expect(page.getByTestId('district-median-water-district')).not.toContainText('₸');
});

test('район: пустой — container_state «no_contracts» отдельной строкой, count 0', async ({
  page,
}) => {
  const dto = base('799999999');
  await page.route('**/api/districts/799999999', (route) => route.fulfill({ json: dto }));
  await page.goto('/districts/799999999');
  await expect(page.getByTestId('district-objects-count')).toHaveText('0');
  // Пустой район НЕ «всё чисто» — честное состояние отдельной строкой.
  await expect(page.getByTestId('district-state-no_contracts')).toBeVisible();
});

test('район: невалидный КАТО (400) → честная ошибка', async ({ page }) => {
  await page.route('**/api/districts/abc', (route) =>
    route.fulfill({
      status: 400,
      json: { error: { code: 'VALIDATION_FAILED', message: 'invalid kato code' } },
    }),
  );
  await page.goto('/districts/abc');
  await expect(page.getByRole('alert')).toBeVisible();
});
