import { describe, it, expect } from 'vitest';
import {
  buildQuery,
  periodToSignedFrom,
  filtersFromParams,
  filtersToParams,
  EMPTY_FILTERS,
  type SearchFilters,
} from './useContracts';

// фикс «сейчас» — детерминизм периода (фасет по дате, AC4).
const NOW = new Date(Date.UTC(2026, 5, 29)); // 2026-06-29

describe('useContracts фасет-маппинг (Story 6.1)', () => {
  it('periodToSignedFrom: пресет → дата от now; all → ""', () => {
    expect(periodToSignedFrom('all', NOW)).toBe('');
    expect(periodToSignedFrom('12m', NOW)).toBe('2025-06-29');
    expect(periodToSignedFrom('24m', NOW)).toBe('2024-06-29');
    expect(periodToSignedFrom('36m', NOW)).toBe('2023-06-29');
  });

  // negative-control [code review 6.1]: клэмп дня на граничном месяце. 29 фев високосного 2024 − 12 мес = 28 фев
  // 2023 (не «1 мар» из-за переполнения Date.UTC). На старой арифметике вернулось бы '2023-03-01' → тест краснеет.
  it('periodToSignedFrom: клэмп 29 фев високосного (нет off-by-one переполнения в март)', () => {
    expect(periodToSignedFrom('12m', new Date(Date.UTC(2024, 1, 29)))).toBe('2023-02-28');
  });

  it('buildQuery: пустые фасеты → пустая строка (без «?»)', () => {
    expect(buildQuery(EMPTY_FILTERS, undefined, NOW)).toBe('');
  });

  it('buildQuery: фасеты → snake_case backend-query; period разворачивается в signed_from (НЕ пресет)', () => {
    const f: SearchFilters = {
      directions: ['road', 'water'],
      period: '12m',
      hasFlag: true,
      amountMin: '100',
      amountMax: '999',
      supplierBin: '123456789012',
    };
    const p = new URLSearchParams(buildQuery(f, 'CUR', NOW).slice(1));
    expect(p.get('direction')).toBe('road,water');
    expect(p.get('signed_from')).toBe('2025-06-29');
    expect(p.get('signed_to')).toBe('2026-06-29'); // верхняя граница периода = now ([now−N, now], P9 [code review 6.1])
    expect(p.get('has_flag')).toBe('true');
    expect(p.get('amount_min')).toBe('100');
    expect(p.get('amount_max')).toBe('999');
    expect(p.get('supplier_bin')).toBe('123456789012');
    expect(p.get('cursor')).toBe('CUR');
    expect(p.has('period')).toBe(false); // AC4: бэкенд получает signed_from, не пресет периода
  });

  it('URL round-trip: filtersToParams → filtersFromParams сохраняет фасеты (period как пресет)', () => {
    const f: SearchFilters = {
      directions: ['other'],
      period: '24m',
      hasFlag: true,
      amountMin: '5',
      amountMax: '',
      supplierBin: '',
    };
    expect(filtersFromParams(filtersToParams(f))).toEqual(f);
  });

  it('filtersFromParams: мусорные значения отброшены (закрытые списки direction/period)', () => {
    const f = filtersFromParams(new URLSearchParams('direction=road,plane&period=99y&has_flag=maybe'));
    expect(f.directions).toEqual(['road']); // 'plane' вне enum → отброшен
    expect(f.period).toBe('all'); // '99y' → дефолт
    expect(f.hasFlag).toBe(false); // не 'true'
  });

  it('filtersToParams: дефолтные фасеты не засоряют URL', () => {
    expect(filtersToParams(EMPTY_FILTERS).toString()).toBe('');
  });
});
