import { describe, it, expect } from 'vitest';
import { formatMoney, formatDate, formatKm } from './format';

// formatKm (код-ревью 3.5): double из БД с float-шумом → локализованное число ≤2 знаков.
describe('formatKm', () => {
  it('срезает float-шум double до ≤2 знаков', () => {
    expect(formatKm(3.0600000000000005, 'ru')).toBe('3,06');
    expect(formatKm(5.000000000000001, 'ru')).toBe('5');
  });

  it('локальный десятичный разделитель (ru/kk — запятая), не точка', () => {
    expect(formatKm(3.06, 'ru')).toContain(',');
    expect(formatKm(3.06, 'kk')).toContain(',');
    expect(formatKm(3.06, 'ru')).not.toContain('.');
  });

  it('целое — без дробной части', () => {
    expect(formatKm(5, 'ru')).toBe('5');
    expect(formatKm(5, 'kk')).toBe('5');
  });
});

describe('formatMoney', () => {
  it('группирует разряды и добавляет ₸ (ru)', () => {
    const s = formatMoney('1234567', 'ru');
    expect(s).toMatch(/₸$/);
    expect(s.replace(/[^0-9]/g, '')).toBe('1234567');
  });

  it('сохраняет точность BigInt (>2^53)', () => {
    const s = formatMoney('9007199254740993', 'kk');
    expect(s.replace(/[^0-9]/g, '')).toBe('9007199254740993');
  });

  it('бросает на нечисловом входе (не молчит)', () => {
    expect(() => formatMoney('abc', 'ru')).toThrow();
    expect(() => formatMoney('12.5', 'ru')).toThrow(); // дробное — не целые тенге
  });

  it('честность: пустое/пробел/hex НЕ становятся «0 ₸» (review P2)', () => {
    expect(() => formatMoney('', 'ru')).toThrow(); // НЕ «0 ₸»
    expect(() => formatMoney('   ', 'ru')).toThrow();
    expect(() => formatMoney('0x10', 'ru')).toThrow(); // не reinterpret в 16
    expect(formatMoney('0', 'ru')).toMatch(/0 ₸$/); // легитимный ноль — ок
  });
});

describe('formatDate', () => {
  it('форматирует ISO по локали', () => {
    expect(formatDate('2026-03-15', 'ru')).toMatch(/2026/);
    expect(formatDate('2026-03-15', 'kk')).toMatch(/2026/);
  });

  it('бросает на невалидной дате', () => {
    expect(() => formatDate('not-a-date', 'ru')).toThrow();
  });

  it('честность: частичный ISO не фабрикует день/месяц (review P3)', () => {
    expect(() => formatDate('2026', 'ru')).toThrow(); // НЕ «1 января 2026»
    expect(() => formatDate('2026-03', 'ru')).toThrow(); // НЕ «1 марта 2026»
  });

  it('TZ-стабильность: дата без off-by-one (UTC)', () => {
    // 2026-03-15 должно остаться 15-м числом независимо от локальной TZ раннера
    expect(formatDate('2026-03-15', 'ru')).toMatch(/15/);
  });
});
