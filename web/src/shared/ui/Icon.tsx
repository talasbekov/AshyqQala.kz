// <Icon name> (AC3): типизированный ЗАКРЫТЫЙ name. Только статус-глифы (WCAG 1.4.1: статус несёт
// форму/глиф, не только цвет). Цвет — через семантические токены (CSS-класс), без хардкода.
export type IconName = 'flag' | 'verified' | 'no-data' | 'insufficient' | 'ungeocoded' | 'error';

// Закрытая карта глиф для каждого имени (DESIGN.md: ! флаг, ✓ подтверждено, — нет данных,
// ≈ недостаточно, перечёркнутый пин — нет гео).
export const ICON_GLYPH: Record<IconName, string> = {
  flag: '!',
  verified: '✓',
  'no-data': '—',
  insufficient: '≈',
  ungeocoded: '⦸',
  error: '!',
};

export function Icon({ name, ariaHidden = true }: { name: IconName; ariaHidden?: boolean }) {
  // aria-hidden по умолчанию: глиф декоративен, смысл несёт соседний текст (DataState/бейдж).
  return (
    <span className={`icon icon--${name}`} aria-hidden={ariaHidden}>
      {ICON_GLYPH[name]}
    </span>
  );
}
