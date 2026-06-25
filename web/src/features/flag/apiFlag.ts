// Адаптер API-флага (Story 5.1) → view-модель карточки (FlagBadge/MethodologyDialog). Карточка больше НЕ
// берёт флаги из захардкоженного contractStories.ts — источник реальный compute Epic 4 через /api/contracts.
// Честная реконструкция состояний УЖЕ сделана на бэке (resolveContractFlags): каждый contract-флаг приходит
// со state ∈ {raised, not_raised, insufficient_data}. Здесь — ЗАЩИТНЫЙ маппинг сырого snake_case evidence
// (FR-23) в FlagEvidence: ставим ТОЛЬКО присутствующие поля, НЕ фабрикуем (honesty §7.4). Числа → каноничные
// строки целых для formatMoney (строгий парсер; кормим валидным входом, а не ослабляем его).
import type { components } from '../../shared/api/schema.gen';
import type { ContractFlag, FlagId, FlagEvidence, ManualFlagState } from './contractStories';

export type ApiContractFlag = components['schemas']['ContractFlag'];

// intString — число/целочисленная строка → каноничная строка целых (^-?\d+$); иначе undefined (не выдумываем).
function intString(v: unknown): string | undefined {
  if (typeof v === 'number' && Number.isFinite(v)) return String(Math.trunc(v));
  if (typeof v === 'string' && /^-?\d+$/.test(v)) return v;
  return undefined;
}

// intNum — конечное число → number; иначе undefined.
function intNum(v: unknown): number | undefined {
  return typeof v === 'number' && Number.isFinite(v) ? v : undefined;
}

// thresholdPerKm = медиана × deviation_factor, ТОЧНО через BigInt (1.5 = 1500/1000) — публичная
// пересчитываемость (FR-23). Каноничная строка целых для formatMoney. undefined, если входов нет.
function thresholdPerKm(medianStr: string | undefined, factor: number | undefined): string | undefined {
  if (medianStr === undefined || factor === undefined || !Number.isFinite(factor)) return undefined;
  const scaled = Math.round(factor * 1000);
  return ((BigInt(medianStr) * BigInt(scaled)) / 1000n).toString();
}

// apiToViewFlag — RAISED API-флаг → ContractFlag (для бейджа + методики). Возвращает null для НЕ-raised
// (not_raised/insufficient_data/not_published не публикуются бейджем — их рисует честная строка статуса).
export function apiToViewFlag(f: ApiContractFlag): ContractFlag | null {
  if (f.state !== 'raised') return null;
  const ev = (f.evidence ?? {}) as Record<string, unknown>;
  const out: FlagEvidence = {};

  if (f.flag_id === 'single_participant') {
    const pc = intNum(ev.participant_count);
    if (pc !== undefined) out.participants = pc;
  } else {
    const sample = intNum(ev.sample_size);
    if (sample !== undefined) out.sampleSize = sample;
    const factor = intNum(ev.deviation_factor);
    if (factor !== undefined) out.deviationFactor = String(factor);
    const median = intString(ev.median);
    if (median !== undefined) out.medianPerKm = median;
    const price = intString(ev.price_per_km);
    if (price !== undefined) out.thisPerKm = price;
    const thr = thresholdPerKm(median, factor);
    if (thr !== undefined) out.thresholdPerKm = thr;
  }

  return {
    flagId: f.flag_id as FlagId,
    flagState: 'raised' as ManualFlagState,
    detectedAt: f.detected_at.value ?? '', // ISO8601; пусто → FlagBadge/диалог честно скрывают дату
    methodologyVersion: f.methodology_version.value ?? '',
    evidence: out,
  };
}
