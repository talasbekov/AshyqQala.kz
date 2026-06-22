// formatMoney/formatDate (AC3): следуют языку chrome через Intl. Сырой toLocaleString запрещён
// eslint-правилом — это единственный санкционированный путь форматирования.
import type { Lang } from './index';

const LOCALE: Record<Lang, string> = { kk: 'kk-KZ', ru: 'ru-KZ' };

// formatMoney — целые тенге приходят СТРОКОЙ (точность >2^53 не теряем: группируем через BigInt).
// Честный fail: только каноничная целая строка (^-?\d+$). Иначе throw — НЕ молчаливый «0 ₸»
// (BigInt('')→0n, BigInt('0x10')→16n — фабрикация/reinterpret недопустимы).
export function formatMoney(amountTng: string, lang: Lang): string {
  if (!/^-?\d+$/.test(amountTng)) {
    throw new Error(
      `formatMoney: ожидалась строка целых тенге (^-?\\d+$), получено "${amountTng}"`,
    );
  }
  const n = BigInt(amountTng);
  return `${new Intl.NumberFormat(LOCALE[lang]).format(n)} ₸`;
}

// formatDate — ISO8601 → локализованная дата (язык chrome) через Intl. Требуем ПОЛНУЮ дату
// (YYYY-MM-DD…): частичный ISO ('2026' → 1 янв) фабрикует день/месяц. timeZone:UTC убирает
// off-by-one (дата-без-времени парсится как UTC-полночь, а Intl иначе рендерит в локальной TZ).
export function formatDate(iso: string, lang: Lang): string {
  if (!/^\d{4}-\d{2}-\d{2}/.test(iso)) {
    throw new Error(`formatDate: ожидалась полная дата YYYY-MM-DD, получено "${iso}"`);
  }
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) {
    throw new Error(`formatDate: невалидная дата "${iso}"`);
  }
  return new Intl.DateTimeFormat(LOCALE[lang], {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    timeZone: 'UTC',
  }).format(d);
}
