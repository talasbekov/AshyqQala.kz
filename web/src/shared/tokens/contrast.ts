// WCAG 2.1 контраст-ядро (чистые функции). Используется контраст-тестом (AC4) — каркас готов,
// расширяется парами с первым цветом флага. Без зависимостей.

function channel(v: number): number {
  const s = v / 255;
  return s <= 0.03928 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4);
}

// relativeLuminance — относительная яркость по WCAG для hex-цвета (#RGB или #RRGGBB).
export function relativeLuminance(hex: string): number {
  const c = hex.replace('#', '');
  const full =
    c.length === 3
      ? c
          .split('')
          .map((ch) => ch + ch)
          .join('')
      : c;
  const r = parseInt(full.slice(0, 2), 16);
  const g = parseInt(full.slice(2, 4), 16);
  const b = parseInt(full.slice(4, 6), 16);
  return 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b);
}

// contrastRatio — отношение контраста (1..21) между двумя hex-цветами.
export function contrastRatio(a: string, b: string): number {
  const la = relativeLuminance(a);
  const lb = relativeLuminance(b);
  const hi = Math.max(la, lb);
  const lo = Math.min(la, lb);
  return (hi + 0.05) / (lo + 0.05);
}
