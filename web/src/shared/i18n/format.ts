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

// formatPricePerKm — медиана ₸/км (FR-18): та же группировка, что formatMoney, но юнит «₸/км»
// (ставка за километр, не полная сумма). «км» locale-инвариантна, как «₸» — единственный санкционированный
// путь юнита. Заголовок секции уже несёт ₸/км, но строка-значение иначе читается как полный ₸. review-патч 6.4.
export function formatPricePerKm(amountTng: string, lang: Lang): string {
  return `${formatMoney(amountTng, lang)}/км`;
}

// formatKm — длина в километрах: double из БД несёт float-шум (3.0600000000000005) и точку вместо
// локального разделителя. Intl с ≤2 знаками — единственный санкционированный путь (запрет сырого
// toLocaleString). Юнит «км» добавляет вызывающий через i18n-строку ({{km}} км).
export function formatKm(km: number, lang: Lang): string {
  return new Intl.NumberFormat(LOCALE[lang], { maximumFractionDigits: 2 }).format(km);
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

// formatDateSafe — formatDate с честной per-field деградацией (review-фикс 5.3): prefix-валидная, но
// невалидная дата (`2026-13-45`, `0000-00-00`) НЕ роняет рендер там, где нет ErrorBoundary (бейдж флага,
// строки as_of) — возвращаем сырое значение вместо throw. Одно битое поле не сносит весь экран/карточку.
export function formatDateSafe(iso: string, lang: Lang): string {
  try {
    return formatDate(iso, lang);
  } catch {
    return iso;
  }
}
