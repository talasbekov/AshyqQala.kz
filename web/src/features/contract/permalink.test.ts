import { describe, it, expect } from 'vitest';
import { buildPermalink } from './permalink';

// AR-29 (Story 5.6): перманентная ссылка-на-дату штампует (as_of, mv); пустые поля НЕ фабрикуются.
describe('buildPermalink', () => {
  it('штампует as_of + mv в query', () => {
    expect(buildPermalink('DEMO-0001', '2026-03-15T10:00:00Z', 'v1.0')).toBe(
      '/contracts/DEMO-0001?as_of=2026-03-15T10%3A00%3A00Z&mv=v1.0',
    );
  });

  it('honesty-hole: пустые/нулевые as_of и mv НЕ штампуются (не выдумываем штамп)', () => {
    expect(buildPermalink('DEMO-0001', null, null)).toBe('/contracts/DEMO-0001');
    expect(buildPermalink('DEMO-0001', '', '')).toBe('/contracts/DEMO-0001');
  });

  it('только mv → один параметр', () => {
    expect(buildPermalink('DEMO-0001', null, 'v1.0')).toBe('/contracts/DEMO-0001?mv=v1.0');
  });

  it('кодирует id', () => {
    expect(buildPermalink('A/B 1', null, 'v1.0')).toBe('/contracts/A%2FB%201?mv=v1.0');
  });
});
