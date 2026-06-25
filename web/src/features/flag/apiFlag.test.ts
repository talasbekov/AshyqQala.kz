import { describe, it, expect } from 'vitest';
import { apiToViewFlag, apiToMethodologyTarget, type ApiContractFlag } from './apiFlag';

function apiFlag(
  partial: Partial<ApiContractFlag> & Pick<ApiContractFlag, 'flag_id' | 'state'>,
): ApiContractFlag {
  return {
    methodology_version: { value: 'v1.0', state: 'ok' },
    detected_at: { value: '2026-05-12T10:00:00Z', state: 'ok' },
    evidence: null,
    ...partial,
  } as ApiContractFlag;
}

describe('apiToViewFlag (Story 5.1: API-флаг → view-модель)', () => {
  it('НЕ-raised → null (бейдж не публикуется; рисуется строкой статуса)', () => {
    expect(apiToViewFlag(apiFlag({ flag_id: 'price_per_km', state: 'not_raised' }))).toBeNull();
    expect(apiToViewFlag(apiFlag({ flag_id: 'price_per_km', state: 'insufficient_data' }))).toBeNull();
    expect(apiToViewFlag(apiFlag({ flag_id: 'single_participant', state: 'not_published' }))).toBeNull();
  });

  it('raised single_participant → participants из evidence; версия/дата проброшены', () => {
    const f = apiToViewFlag(
      apiFlag({ flag_id: 'single_participant', state: 'raised', evidence: { participant_count: 1 } }),
    )!;
    expect(f.flagId).toBe('single_participant');
    expect(f.flagState).toBe('raised');
    expect(f.evidence.participants).toBe(1);
    expect(f.methodologyVersion).toBe('v1.0');
    expect(f.detectedAt).toBe('2026-05-12T10:00:00Z');
  });

  it('raised price_per_km → каноничные строки + ТОЧНЫЙ порог (медиана×1.5 через BigInt)', () => {
    const f = apiToViewFlag(
      apiFlag({
        flag_id: 'price_per_km',
        state: 'raised',
        evidence: { price_per_km: 71000000, median: 38400000, sample_size: 9, deviation_factor: 1.5 },
      }),
    )!;
    expect(f.evidence.medianPerKm).toBe('38400000');
    expect(f.evidence.thisPerKm).toBe('71000000');
    expect(f.evidence.sampleSize).toBe(9);
    expect(f.evidence.deviationFactor).toBe('1.5');
    expect(f.evidence.thresholdPerKm).toBe('57600000'); // 38400000 × 1.5 — точно, без float
  });

  it('пустой evidence → поля НЕ фабрикуются (honesty §7.4)', () => {
    const f = apiToViewFlag(apiFlag({ flag_id: 'price_per_km', state: 'raised', evidence: {} }))!;
    expect(f.evidence.medianPerKm).toBeUndefined();
    expect(f.evidence.thresholdPerKm).toBeUndefined();
    expect(f.evidence.sampleSize).toBeUndefined();
  });

  it('неканоничные числа отбрасываются (formatMoney строг — кормим только ^-?\\d+$)', () => {
    const f = apiToViewFlag(
      apiFlag({ flag_id: 'price_per_km', state: 'raised', evidence: { median: '38 400 000', price_per_km: 'x' } }),
    )!;
    expect(f.evidence.medianPerKm).toBeUndefined();
    expect(f.evidence.thisPerKm).toBeUndefined();
    expect(f.evidence.thresholdPerKm).toBeUndefined();
  });

  it('review-фикс: число вне безопасного диапазона (>2^53) скрывается, не публикуется кривым', () => {
    // Number.MAX_SAFE_INTEGER + N — небезопасное целое (через выражение, без неточного литерала).
    const unsafe = Number.MAX_SAFE_INTEGER + 2;
    const f = apiToViewFlag(
      apiFlag({ flag_id: 'price_per_km', state: 'raised', evidence: { median: unsafe, price_per_km: unsafe } }),
    )!;
    expect(f.evidence.medianPerKm).toBeUndefined();
    expect(f.evidence.thisPerKm).toBeUndefined();
    expect(f.evidence.thresholdPerKm).toBeUndefined();
  });

  it('review-фикс: evidence-массив (не объект) → поля честно отсутствуют, бейдж не падает', () => {
    const f = apiToViewFlag(
      apiFlag({ flag_id: 'single_participant', state: 'raised', evidence: [1, 2, 3] as unknown as ApiContractFlag['evidence'] }),
    )!;
    expect(f.flagState).toBe('raised');
    expect(f.evidence.participants).toBeUndefined();
  });

  it('raised без detected_at → пустая дата (FlagBadge/диалог честно её скрывают, не падают)', () => {
    const f = apiToViewFlag(
      apiFlag({
        flag_id: 'single_participant',
        state: 'raised',
        detected_at: { value: null, state: 'no_data' },
        evidence: { participant_count: 1 },
      }),
    )!;
    expect(f.detectedAt).toBe('');
  });
});

describe('apiToMethodologyTarget (Story 5.3: методика достижима при любом состоянии)', () => {
  it('raised → target с evidence + версия/дата', () => {
    const t = apiToMethodologyTarget(
      apiFlag({
        flag_id: 'price_per_km',
        state: 'raised',
        evidence: { median: 38400000, sample_size: 9, deviation_factor: 1.5, price_per_km: 71000000 },
      }),
    );
    expect(t.state).toBe('raised');
    expect(t.evidence.medianPerKm).toBe('38400000');
    expect(t.evidence.thresholdPerKm).toBe('57600000');
    expect(t.methodologyVersion).toBe('v1.0');
    expect(t.detectedAt).toBe('2026-05-12T10:00:00Z');
  });

  it('НЕ-raised → target БЕЗ evidence (формула/пороги придут из /api/methodology, AC-2)', () => {
    const t = apiToMethodologyTarget(
      apiFlag({
        flag_id: 'price_per_km',
        state: 'insufficient_data',
        methodology_version: { value: null, state: 'no_data' },
        detected_at: { value: null, state: 'no_data' },
        evidence: null,
      }),
    );
    expect(t.state).toBe('insufficient_data');
    expect(t.evidence).toEqual({});
    expect(t.methodologyVersion).toBe('');
    expect(t.detectedAt).toBe('');
  });
});
