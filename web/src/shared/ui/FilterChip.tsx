// FilterChip (Story 6.1, AC2): обобщённый интерактивный чип-пилюля для фасетных фильтров. БЕЗ доменного
// знания (architecture.md:537 — shared/ui без домена; фичи компонуют, передавая label/selected/onToggle).
// Выбранное состояние несёт НЕ-цветовой признак — глиф «✓» (Icon verified, WCAG 1.4.1) + насыщённая
// заливка + bold (см. filter-chip.css). Хитбокс ≥ tap-target, видимый focus-ring. Тап переключает.
import { Icon } from './Icon';
import './filter-chip.css';

export interface FilterChipProps {
  label: string;
  selected: boolean;
  onToggle: () => void;
  testId?: string;
}

export function FilterChip({ label, selected, onToggle, testId }: FilterChipProps) {
  return (
    <button
      type="button"
      className={selected ? 'filter-chip filter-chip--selected' : 'filter-chip'}
      aria-pressed={selected}
      data-testid={testId}
      onClick={onToggle}
    >
      {selected ? <Icon name="verified" /> : null}
      <span className="filter-chip__label">{label}</span>
    </button>
  );
}
