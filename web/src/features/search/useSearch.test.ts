import { describe, it, expect } from 'vitest';
import { isLikelyBin, queryIsSearchable, buildSearchQuery, MIN_QUERY_LEN } from './useSearch';

// Story 6.2 (FR-16): чистые функции текстового поиска. Без рендера (@testing-library не установлен) — юнитим
// детект БИН, гейт searchable и сборку query. Зеркалят бэкенд: CanonicalBIN (12 цифр) и minNameQueryLen.

describe('isLikelyBin (зеркало normalize.CanonicalBIN: ровно 12 цифр)', () => {
  it('12 цифр → БИН', () => {
    expect(isLikelyBin('123456789012')).toBe(true);
  });
  it('пробелы/дефисы не мешают (извлекаются цифры)', () => {
    expect(isLikelyBin('123 456 789 012')).toBe(true);
    expect(isLikelyBin('1234-5678-9012')).toBe(true);
  });
  it('11 или 13 цифр → НЕ БИН', () => {
    expect(isLikelyBin('12345678901')).toBe(false);
    expect(isLikelyBin('1234567890123')).toBe(false);
  });
  it('наименование → НЕ БИН', () => {
    expect(isLikelyBin('жолдары')).toBe(false);
    expect(isLikelyBin('')).toBe(false);
  });
});

describe('queryIsSearchable (БИН ИЛИ имя ≥ MIN_QUERY_LEN)', () => {
  it('пустой/пробелы → не searchable', () => {
    expect(queryIsSearchable('')).toBe(false);
    expect(queryIsSearchable('   ')).toBe(false);
  });
  it('имя короче минимума → не searchable (зеркало бэкенд-400)', () => {
    expect(queryIsSearchable('жо')).toBe(false); // 2 символа
    expect(queryIsSearchable('12')).toBe(false); // 2 цифры, не БИН
    expect(MIN_QUERY_LEN).toBe(3);
  });
  it('имя ровно минимум → searchable (граница)', () => {
    expect(queryIsSearchable('жол')).toBe(true);
    expect(queryIsSearchable('abc')).toBe(true);
  });
  it('валидный БИН (12 цифр) → searchable независимо от длины-имени', () => {
    expect(queryIsSearchable('123456789012')).toBe(true);
  });
  it('тримминг учитывается', () => {
    expect(queryIsSearchable('  жол  ')).toBe(true);
    expect(queryIsSearchable('  жо  ')).toBe(false);
  });
});

describe('buildSearchQuery', () => {
  it('собирает ?q=<term> (тримминг, корректная кодировка)', () => {
    const qs = buildSearchQuery('  жол  ');
    expect(new URLSearchParams(qs.slice(1)).get('q')).toBe('жол');
  });
  it('БИН проходит как есть', () => {
    expect(new URLSearchParams(buildSearchQuery('123456789012').slice(1)).get('q')).toBe(
      '123456789012',
    );
  });
});
