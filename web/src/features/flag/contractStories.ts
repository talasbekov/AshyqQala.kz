// Ручные evidence трёх контрактов-историй (Story 1.9, демо-фундамент). В API флагов/гео нет —
// данные ВРУЧНУЮ (гео-независимо, без compute; реальный расчёт — Epic 4). Числа демонстрационные.
// Нейтральная рамка («сигнал, требующий проверки») и метки берутся из i18n (frame.signal/flag.*),
// числа/пороги — данные ниже (НЕ в прозе флага). [Source: epics.md:1007,1021; data-model flags_v1]

export type FlagId = 'single_participant' | 'price_per_km';
export type ManualFlagState = 'raised' | 'insufficient_data';

// Числа для экрана методики (ручные демо-данные). Суммы — строки целых тенге (→ formatMoney).
export interface FlagEvidence {
  direction?: string; // 'road'
  katoCode?: string;
  windowMonths?: number; // 24
  minSample?: number; // 5
  sampleSize?: number; // фактическая выборка
  deviationFactor?: string; // '1.5' (порог = медиана × 1.5)
  medianPerKm?: string; // ₸/км
  thresholdPerKm?: string; // ₸/км (медиана × 1.5)
  thisPerKm?: string; // ₸/км этого контракта
  participants?: number; // число участников (флаг «единственный участник»)
}

export interface ContractFlag {
  flagId: FlagId;
  flagState: ManualFlagState;
  detectedAt: string; // ISO YYYY-MM-DD
  methodologyVersion: string; // напр. '1.0'
  evidence: FlagEvidence;
}

const STORIES: Record<string, readonly ContractFlag[]> = {
  // История 1: единственный участник (сигнал выставлен).
  'DEMO-0001': [
    {
      flagId: 'single_participant',
      flagState: 'raised',
      detectedAt: '2026-05-12',
      methodologyVersion: '1.0',
      evidence: { direction: 'road', katoCode: '710000000', participants: 1 },
    },
  ],
  // История 2: аномальная цена за км (сигнал выставлен, полные числа evidence).
  'DEMO-0002': [
    {
      flagId: 'price_per_km',
      flagState: 'raised',
      detectedAt: '2026-05-12',
      methodologyVersion: '1.0',
      evidence: {
        direction: 'road',
        katoCode: '710000000',
        windowMonths: 24,
        minSample: 5,
        sampleSize: 9,
        deviationFactor: '1.5',
        medianPerKm: '38400000',
        thresholdPerKm: '57600000',
        thisPerKm: '71000000',
      },
    },
  ],
  // История 3: цена за км, но НЕДОСТАТОЧНО сопоставимых данных (<5) — честное состояние, не «чисто».
  'DEMO-0003': [
    {
      flagId: 'price_per_km',
      flagState: 'insufficient_data',
      detectedAt: '2026-05-12',
      methodologyVersion: '1.0',
      evidence: {
        direction: 'road',
        katoCode: '710000000',
        windowMonths: 24,
        minSample: 5,
        sampleSize: 3,
        deviationFactor: '1.5',
      },
    },
  ],
};

// flagsFor — ручные флаги контракта по natural goszakup_id; пусто, если историй нет (не выдумывать).
export function flagsFor(goszakupId: string): readonly ContractFlag[] {
  return STORIES[goszakupId] ?? [];
}

// Все flagId, у которых должна быть нейтральная i18n-метка (используется тестом нейтральности).
export const ALL_FLAG_IDS: readonly FlagId[] = ['single_participant', 'price_per_km'];
