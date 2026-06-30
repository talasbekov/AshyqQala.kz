import { test, expect, type Page } from '@playwright/test';

// Story 6.2 (FR-16): текстовый поиск по БИН/наименованию. API /api/search замокан (route.fulfill) с симуляцией
// ветвления (БИН → орг; имя → орг+контракт). Закрепляет поведение фронта: ввод имени → результаты (AC2),
// ввод БИН → точная орг (AC3), короткий запрос → подсказка без скана (AC4), орг → карточка подрядчика (AC6),
// негеопривязанные помечены «без точки на карте» (AC5).

type Field = { value: string | null; state: string };
const f = (v: string | null): Field =>
  v === null ? { value: null, state: 'no_data' } : { value: v, state: 'ok' };

const orgItem = (bin: string, nameRu: string) => ({
  kind: 'organization',
  bin,
  name_ru: f(nameRu),
  name_kk: f(null),
  reg_kato: f('710000000'),
  has_geo: false, // до Epic 3 гео нет — честно false
});
const contractItem = (id: string) => ({
  kind: 'contract',
  goszakup_contract_id: id,
  subject_ru: f(`Ремонт автодороги ${id}`),
  subject_kk: f(null),
  amount_tng: f('1000000'),
  sign_date: f('2026-05-01'),
  status: f('active'),
  direction: f('road'),
  kato_code: f('710000000'),
  has_active_flag: false,
  has_geo: false,
});

async function mockApi(page: Page): Promise<void> {
  // browse-режим (пустой q) дергает /api/contracts — отдаём пусто, чтобы не шуметь.
  await page.route('**/api/contracts*', (route) => {
    const url = new URL(route.request().url());
    if (/\/api\/contracts\/[^/?]+/.test(url.pathname)) return route.fallback();
    return route.fulfill({ json: { items: [], next_cursor: null } });
  });
  await page.route('**/api/search*', (route) => {
    const url = new URL(route.request().url());
    const q = (url.searchParams.get('q') ?? '').trim();
    const digits = q.replace(/\D/g, '');
    if (digits.length === 12) {
      // БИН-ветка: только организация.
      return route.fulfill({
        json: { items: [orgItem('123456789012', 'ТОО Жол Курылыс')], next_cursor: null },
      });
    }
    if (q.toLowerCase().includes('жол')) {
      // имя-ветка: организация + контракт.
      return route.fulfill({
        json: {
          items: [orgItem('123456789012', 'ТОО Жол Курылыс'), contractItem('E2E-C1')],
          next_cursor: null,
        },
      });
    }
    return route.fulfill({ json: { items: [], next_cursor: null } });
  });
}

test.describe('Текстовый поиск (Story 6.2)', () => {
  test.beforeEach(async ({ page }) => {
    await mockApi(page);
  });

  test('ввод имени → результаты орг+контракт; негеопривязанные помечены (AC2/AC5/AC6)', async ({
    page,
  }) => {
    await page.goto('/search');
    await page.getByTestId('search-query').fill('жол');
    await expect(page).toHaveURL(/q=/);
    await expect(page.getByTestId('search-result-item')).toHaveCount(2);
    // AC5: маркер «без точки на карте» (has_geo=false) присутствует.
    await expect(page.getByTestId('search-ungeocoded').first()).toBeVisible();
  });

  test('ввод 12-значного БИН → точная организация (AC3)', async ({ page }) => {
    await page.goto('/search');
    await page.getByTestId('search-query').fill('123456789012');
    await expect(page.getByTestId('search-result-item')).toHaveCount(1);
  });

  test('короткий запрос → подсказка мин-длины, без выдачи и без ложного «ничего не найдено» (AC4 / review P1)', async ({
    page,
  }) => {
    await page.goto('/search');
    await page.getByTestId('search-query').fill('жо'); // 2 символа — ниже минимума
    await expect(page.getByTestId('search-query-hint')).toBeVisible();
    await expect(page.getByTestId('search-results-list')).toHaveCount(0);
    // review P1: панель результатов НЕ монтируется → нет ложного «Ничего не найдено по запросу».
    await expect(page.getByTestId('search-results')).toHaveCount(0);
    await expect(page.getByTestId('search-empty-query')).toHaveCount(0);
  });

  test('результат-организация → переход в карточку подрядчика (AC6)', async ({ page }) => {
    await page.goto('/search');
    await page.getByTestId('search-query').fill('жол');
    await page.getByTestId('search-result-item').first().getByRole('link').click();
    await expect(page).toHaveURL(/\/contractors\/123456789012/);
  });
});
