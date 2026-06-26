import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { describe, it, expect } from 'vitest';
import { resources, LANGS } from '../../shared/i18n';

// Story 5.1 (AC6/AC8): per-surface нейтральность + парность kk↔ru для НОВЫХ строк карточки (акт, Заказчик/
// Подрядчик, статус сигналов, имена флагов). taboo_lexicon.json — единый источник нейтральности (Story 1.4).
const here = dirname(fileURLToPath(import.meta.url));
const taboo = JSON.parse(
  readFileSync(join(here, '../../../../registry/values/taboo_lexicon.json'), 'utf8'),
) as { ru: string[]; kk: string[] };
const TABOO_ROOTS = [...taboo.ru, ...taboo.kk];

function collectStrings(obj: unknown, out: string[] = []): string[] {
  if (typeof obj === 'string') out.push(obj);
  else if (obj && typeof obj === 'object') for (const v of Object.values(obj)) collectStrings(v, out);
  return out;
}

function getKey(dict: Record<string, unknown>, path: string): unknown {
  return path.split('.').reduce<unknown>((o, p) => (o as Record<string, unknown>)?.[p], dict);
}

// Все строки карточки 5.1, добавленные/затронутые этой историей (для парности kk↔ru).
const CARD_KEYS = [
  'contract.act.heading',
  'contract.act.date',
  'contract.act.signer',
  'contract.act.source',
  'contract.field.customer',
  'contract.field.supplier',
  'contract.signals_none_heading',
  'contract.signals_status_heading',
  'contract.contractor_signals_note',
  'flag.single_participant.name',
  'flag.price_per_km.name',
  // Story 5.3 — экран методики:
  'methodology.not_raised_title',
  'methodology.insufficient_price',
  'methodology.insufficient_monopoly',
  'methodology.similarity_criterion',
  'methodology.worksheet_heading',
  'methodology.as_of',
  'methodology.as_of_no_date',
  // Story 5.3 review-фиксы (2026-06-26): честный статус порогов + условие single_participant.
  'methodology.thresholds_loading',
  'methodology.thresholds_unavailable',
  'methodology.insufficient_single_participant',
  // Story 5.6 (AR-29): перманентная ссылка-на-дату + дрейф методики + экспорт evidence.
  'methodology.permalink',
  'methodology.permalink_copied',
  'methodology.permalink_failed',
  'methodology.drift',
  'contract.export_evidence',
];

describe('ContractCard строки (Story 5.1)', () => {
  it('per-surface нейтральность: весь contract.* и methodology.* свободны от taboo-корней в обеих локалях', () => {
    for (const lang of LANGS) {
      // Story 5.6: methodology.* несёт строки перманентной ссылки/дрейфа — тоже под нейтральность-стражем.
      for (const ns of ['contract', 'methodology'] as const) {
        const dict = (resources[lang].chrome as Record<string, unknown>)[ns];
        const strings = collectStrings(dict).map((s) => s.toLowerCase());
        for (const s of strings) {
          for (const root of TABOO_ROOTS) {
            expect(s.includes(root), `taboo "${root}" в ${lang}.${ns}: "${s}"`).toBe(false);
          }
        }
      }
    }
  });

  it('новые ключи присутствуют и парны kk↔ru (нет тихого ru-фолбэка для kk)', () => {
    for (const lang of LANGS) {
      const dict = resources[lang].chrome as Record<string, unknown>;
      for (const k of CARD_KEYS) {
        const v = getKey(dict, k);
        expect(typeof v, `${lang}:${k}`).toBe('string');
        expect((v as string).trim().length, `${lang}:${k} непусто`).toBeGreaterThan(0);
      }
    }
  });

  it('флаг-состояния (строка статуса) имеют нейтральные метки в обеих локалях', () => {
    for (const lang of LANGS) {
      const fs = (resources[lang].chrome as Record<string, unknown>).flag_state as Record<string, string>;
      for (const st of ['raised', 'not_raised', 'insufficient_data', 'not_published']) {
        expect(fs[st], `${lang}:flag_state.${st}`).toBeTruthy();
      }
    }
  });
});
