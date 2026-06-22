import { describe, it, expect } from 'vitest';
import { dataStateFromValueState } from './DataState';

describe('dataStateFromValueState (проекция honest value_state → UI-автомат)', () => {
  it('правило архитектуры: no_data→empty, insufficient_sample→partial, ok→success', () => {
    expect(dataStateFromValueState('no_data')).toBe('empty');
    expect(dataStateFromValueState('insufficient_sample')).toBe('partial');
    expect(dataStateFromValueState('ok')).toBe('success');
  });

  it('error и неизвестное состояние → error (честно, не пусто)', () => {
    expect(dataStateFromValueState('error')).toBe('error');
    expect(dataStateFromValueState('totally_unknown')).toBe('error');
  });

  it('прочие честные состояния не теряются (empty/partial, не success)', () => {
    expect(dataStateFromValueState('redacted')).toBe('empty');
    expect(dataStateFromValueState('stale')).toBe('partial');
    expect(dataStateFromValueState('geocode_failed')).toBe('partial');
  });
});
