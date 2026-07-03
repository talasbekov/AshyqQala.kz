// JS-мост к semantic-токенам для карты (Story 1.8; architecture.md: карта — единственный UI вне CSS).
// MapLibre рисует фон на canvas и НЕ читает CSS-переменные → цвет берём из computed-стиля
// documentElement (следует активной теме на момент чтения). Закрытый набор имён токенов.
// Story 3.4: +accent (выбранная линия — золотой stroke) — canvas line-слои дорог; DOM-маркеры
// красятся CSS-переменными напрямую и в мост НЕ ходят.
export type MapTokenName = 'surface-sunken' | 'surface' | 'primary' | 'accent';

export function mapToken(name: MapTokenName, fallback: string): string {
  if (typeof document === 'undefined') return fallback;
  const value = getComputedStyle(document.documentElement)
    .getPropertyValue(`--color-${name}`)
    .trim();
  return value || fallback;
}
