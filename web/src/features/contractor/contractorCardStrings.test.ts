import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { describe, it, expect } from 'vitest';
import { resources, LANGS } from '../../shared/i18n';

// Story 5.2 (AC-5): per-surface нейтральность + парность kk↔ru для строк карточки подрядчика.
// taboo_lexicon.json — единый источник нейтральности (Story 1.4). РНУ «недобросовестный/жосықсыз» допустим
// как ЦИТАТА названия гос-реестра (атрибуция государству) — и не входит в taboo-корни, так что страж зелёный.
const here = dirname(fileURLToPath(import.meta.url));
const taboo = JSON.parse(
  readFileSync(join(here, '../../../../registry/values/taboo_lexicon.json'), 'utf8'),
) as { ru: string[]; kk: string[] };
const TABOO_ROOTS = [...taboo.ru, ...taboo.kk];

function collectStrings(obj: unknown, out: string[] = []): string[] {
  if (typeof obj === 'string') out.push(obj);
  else if (obj && typeof obj === 'object')
    for (const v of Object.values(obj)) collectStrings(v, out);
  return out;
}

function getKey(dict: Record<string, unknown>, path: string): unknown {
  return path.split('.').reduce<unknown>((o, p) => (o as Record<string, unknown>)?.[p], dict);
}

const CARD_KEYS = [
  'contractor.overlabel',
  'contractor.not_found',
  'contractor.error',
  'contractor.profile.incomplete',
  'contractor.profile.partial',
  'contractor.profile.unverified',
  'contractor.contracts.heading',
  'contractor.contracts.empty',
  'contractor.field.bin',
  'contractor.field.contract_count',
  'contractor.field.total_amount',
  'contractor.field.regions',
  'contractor.signals_heading',
  'contractor.signals_none_heading',
  'contractor.flag.monopoly.name',
  'contractor.flag.rnu.name',
  'contractor.report_subject',
  'contractor.rnu.heading',
  'contractor.rnu.registry_name',
  'contractor.rnu.since',
  'contractor.rnu.source',
  'contractor.rnu.cleared',
];

describe('ContractorCard строки (Story 5.2)', () => {
  it('per-surface нейтральность: весь contractor.* свободен от taboo-корней в обеих локалях', () => {
    for (const lang of LANGS) {
      const dict = (resources[lang].chrome as Record<string, unknown>).contractor;
      const strings = collectStrings(dict).map((s) => s.toLowerCase());
      for (const s of strings) {
        for (const root of TABOO_ROOTS) {
          expect(s.includes(root), `taboo "${root}" в ${lang}.contractor: "${s}"`).toBe(false);
        }
      }
    }
  });

  it('ключи присутствуют и парны kk↔ru (нет тихого ru-фолбэка для kk)', () => {
    for (const lang of LANGS) {
      const dict = resources[lang].chrome as Record<string, unknown>;
      for (const k of CARD_KEYS) {
        const v = getKey(dict, k);
        expect(typeof v, `${lang}:${k}`).toBe('string');
        expect((v as string).trim().length, `${lang}:${k} непусто`).toBeGreaterThan(0);
      }
    }
  });
});
