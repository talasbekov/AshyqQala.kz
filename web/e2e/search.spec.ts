import { test, expect, type Page } from '@playwright/test';

// Story 6.1 (FR-15): фасетный поиск. API замокан (route.fulfill) с СИМУЛЯЦИЕЙ фасетной фильтрации на стороне
// мока — закрепляет поведение фронта: тап чипа → немедленная пересборка списка (AC1), повторный тап снимает,
// выбранный чип несёт «✓» (AC2), узкий фильтр → «Ничего не найдено» (AC3).

type Field = { value: string | null; state: string };
const f = (v: string | null): Field => (v === null ? { value: null, state: 'no_data' } : { value: v, state: 'ok' });

interface Item {
  goszakup_contract_id: string;
  subject_ru: Field;
  subject_kk: Field;
  amount_tng: Field;
  sign_date: Field;
  status: Field;
  direction: Field;
  kato_code: Field;
  has_active_flag: boolean;
}
const item = (id: string, dir: string, flag: boolean, sd: string): Item => ({
  goszakup_contract_id: id,
  subject_ru: f(`Контракт ${id}`),
  subject_kk: f(`Келісімшарт ${id}`),
  amount_tng: f('1000000'),
  sign_date: f(sd),
  status: f('active'),
  direction: f(dir),
  kato_code: f('710000000'),
  has_active_flag: flag,
});

const ITEMS: Item[] = [
  item('E2E-RD-FLAG', 'road', true, '2026-05-01'),
  item('E2E-WATER', 'water', false, '2026-04-01'),
  item('E2E-RD2', 'road', false, '2026-03-01'),
];

async function mockList(page: Page): Promise<void> {
  await page.route('**/api/contracts*', (route) => {
    const url = new URL(route.request().url());
    // карточка контракта (/api/contracts/{id}) имеет сегмент пути — её не перехватываем (на всякий случай).
    if (/\/api\/contracts\/[^/?]+/.test(url.pathname)) return route.fallback();
    const dir = url.searchParams.get('direction');
    const hasFlag = url.searchParams.get('has_flag') === 'true';
    let items = ITEMS.slice();
    if (dir) {
      const set = dir.split(',');
      items = items.filter((i) => set.includes(i.direction.value ?? ''));
    }
    if (hasFlag) items = items.filter((i) => i.has_active_flag);
    return route.fulfill({ json: { items, next_cursor: null } });
  });
}

test.describe('Поиск/фильтры (Story 6.1)', () => {
  test.beforeEach(async ({ page }) => {
    await mockList(page);
  });

  test('тап чипа немедленно сужает список; повторный тап снимает (AC1)', async ({ page }) => {
    await page.goto('/search');
    await expect(page.getByTestId('search-item')).toHaveCount(3);

    await page.getByTestId('chip-dir-road').click();
    await expect(page.getByTestId('search-item')).toHaveCount(2); // только road
    await expect(page).toHaveURL(/direction=road/);

    await page.getByTestId('chip-dir-road').click(); // снять тем же тапом
    await expect(page.getByTestId('search-item')).toHaveCount(3);
    await expect(page).not.toHaveURL(/direction=/);
  });

  test('выбранный чип несёт «✓» и aria-pressed (AC2)', async ({ page }) => {
    await page.goto('/search');
    const chip = page.getByTestId('chip-has-flag');
    await expect(chip).toHaveAttribute('aria-pressed', 'false');

    await chip.click();
    await expect(chip).toHaveAttribute('aria-pressed', 'true');
    await expect(chip).toContainText('✓'); // не-цветовой признак выбора (WCAG 1.4.1)
    await expect(page.getByTestId('search-item')).toHaveCount(1); // только с сигналом
  });

  test('узкий фильтр → «Ничего не найдено», без выдуманных строк (AC3)', async ({ page }) => {
    await page.goto('/search');
    await page.getByTestId('chip-dir-water').click();
    await page.getByTestId('chip-has-flag').click(); // water + сигнал → таких нет
    await expect(page.getByTestId('search-item')).toHaveCount(0);
    await expect(page.getByTestId('search-empty')).toBeVisible();
  });
});
