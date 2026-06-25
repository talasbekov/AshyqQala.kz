// Адаптер API-флага (Story 5.1/5.3) → view-модели. Карточка берёт флаги из реального compute Эпика 4 через
// /api/contracts (НЕ захардкожено). ЗАЩИТНЫЙ маппинг сырого snake_case evidence (FR-23): только присутствующие
// поля, НЕ фабрикуем (honesty §7.4). Числа → каноничные строки целых для formatMoney (строгий парсер).
import type { components } from '../../shared/api/schema.gen';
import type { ContractFlag, FlagId, FlagEvidence, ManualFlagState } from './contractStories';

export type ApiContractFlag = components['schemas']['ContractFlag'];

// MethodologyTarget — вход экрана методики (Story 5.3) для ЛЮБОГО состояния флага (raised → с evidence;
// not_raised/insufficient → пустой evidence, формула/пороги берутся из /api/methodology). Делает методику
// достижимой и для не-raised меток (AC-2).
export interface MethodologyTarget {
  flagId: FlagId;
  state: ApiContractFlag['state'];
  evidence: FlagEvidence; // {} для не-raised (нет данных — не выдумываем)
  methodologyVersion: string; // '' если no_data
  detectedAt: string; // '' если no_data
}

// intString — число/целочисленная строка → каноничная строка целых (^-?\d+$); иначе undefined (не выдумываем).
// Число ВНЕ безопасного целого диапазона (|v| > 2^53) уже потеряло точность при JSON.parse → честно
// СКРЫВАЕМ (undefined), а НЕ публикуем кривое значение (FR-23: пересчитываемость по точным числам).
function intString(v: unknown): string | undefined {
  if (typeof v === 'number' && Number.isSafeInteger(v)) return String(v);
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

// mapEvidence — сырой jsonb evidence → FlagEvidence (защитно). evidence ДОЛЖЕН быть простым объектом;
// массив/скаляр/null → пустой объект (поля честно отсутствуют). Общий для бейджа и экрана методики.
function mapEvidence(f: ApiContractFlag): FlagEvidence {
  const raw = f.evidence;
  const ev: Record<string, unknown> =
    raw !== null && typeof raw === 'object' && !Array.isArray(raw) ? (raw as Record<string, unknown>) : {};
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
  return out;
}

// apiToViewFlag — RAISED API-флаг → ContractFlag (для бейджа). null для НЕ-raised (бейдж не публикуется).
export function apiToViewFlag(f: ApiContractFlag): ContractFlag | null {
  if (f.state !== 'raised') return null;
  return {
    flagId: f.flag_id as FlagId,
    flagState: 'raised' as ManualFlagState,
    detectedAt: f.detected_at.value ?? '',
    methodologyVersion: f.methodology_version.value ?? '',
    evidence: mapEvidence(f),
  };
}

// apiToMethodologyTarget — API-флаг ЛЮБОГО состояния → вход экрана методики (Story 5.3). evidence только для
// raised; для не-raised пустой (формула/пороги придут из /api/methodology, AC-2).
export function apiToMethodologyTarget(f: ApiContractFlag): MethodologyTarget {
  return {
    flagId: f.flag_id as FlagId,
    state: f.state,
    evidence: f.state === 'raised' ? mapEvidence(f) : {},
    methodologyVersion: f.methodology_version.value ?? '',
    detectedAt: f.detected_at.value ?? '',
  };
}
