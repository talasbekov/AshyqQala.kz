// <DataState> (AC3): автомат состояний данных. Честность структурой — пустота НИКОГДА не рендерится
// как «0»/«—»/выдуманное, а как явное состояние с нейтральным текстом (chrome) + глифом (WCAG 1.4.1).
// empty = «нет данных» (no_data) ≠ partial = «недостаточно сопоставимых» (insufficient_sample).
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Icon } from '../ui/Icon';

export type DataStateKind = 'idle' | 'loading' | 'empty' | 'partial' | 'error' | 'success';

// dataStateFromValueState — проекция честного value_state (Story 1.4) на UI-автомат.
// Правило архитектуры: no_data→empty, insufficient_sample→partial.
export function dataStateFromValueState(state: string): DataStateKind {
  switch (state) {
    case 'ok':
      return 'success';
    case 'no_data':
    case 'not_applicable':
    case 'redacted':
      return 'empty';
    case 'insufficient_sample':
    case 'not_comparable':
    case 'stale':
    case 'geocode_pending':
    case 'geocode_failed':
    case 'source_conflict':
      return 'partial';
    case 'error':
      return 'error';
    default:
      // Неизвестное состояние → честный error (не пусто) — закрытый union на стороне данных.
      return 'error';
  }
}

// state (опц.) — исходный honest value_state: рендерим ЕГО конкретную метку, чтобы redacted≠«нет
// данных», geocode_pending≠«недостаточно», error=«ошибка расчёта». Без state — обобщённая метка kind.
export function DataState({
  kind,
  state,
  children,
}: {
  kind: DataStateKind;
  state?: string;
  children?: ReactNode;
}) {
  const { t } = useTranslation('chrome');
  // Конкретная метка value_state с честным фолбэком на обобщённый ключ kind.
  const label = (fallbackKey: string): string =>
    state ? t(`value_state.${state}`, { defaultValue: t(fallbackKey) }) : t(fallbackKey);
  switch (kind) {
    case 'success':
      return <>{children}</>;
    case 'idle':
      return null;
    case 'loading':
      return (
        <span className="data-state data-state--loading" role="status" aria-busy="true">
          …
        </span>
      );
    case 'empty':
      return (
        <span className="data-state data-state--empty">
          <Icon name="no-data" /> {label('value_state.no_data')}
        </span>
      );
    case 'partial':
      return (
        <span className="data-state data-state--partial">
          <Icon name="insufficient" /> {label('value_state.insufficient_sample')}
        </span>
      );
    case 'error':
      return (
        <span className="data-state data-state--error" role="alert">
          <Icon name="error" /> {label('state.unknown')}
        </span>
      );
    default: {
      // Закрытый union: новый член без ветки — ошибка компиляции (never).
      const exhaustive: never = kind;
      return <>{String(exhaustive)}</>;
    }
  }
}
