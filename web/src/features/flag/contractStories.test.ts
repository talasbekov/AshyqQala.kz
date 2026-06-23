import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { describe, it, expect } from 'vitest';
import { resources, LANGS } from '../../shared/i18n';
import { flagsFor, ALL_FLAG_IDS } from './contractStories';

const here = dirname(fileURLToPath(import.meta.url));
// taboo_lexicon.json — источник истины нейтральности (Story 1.4), repo-root registry/.
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

describe('contractStories (Story 1.9)', () => {
  it('flagsFor отдаёт флаги демо-историй и пусто для неизвестного (не выдумывать)', () => {
    expect(flagsFor('DEMO-0001').length).toBeGreaterThan(0);
    expect(flagsFor('DEMO-0002').length).toBeGreaterThan(0);
    expect(flagsFor('DEMO-0003').length).toBeGreaterThan(0);
    expect(flagsFor('UNKNOWN')).toEqual([]);
  });

  it('все flagId историй — из закрытого ALL_FLAG_IDS', () => {
    for (const id of ['DEMO-0001', 'DEMO-0002', 'DEMO-0003']) {
      for (const f of flagsFor(id)) expect(ALL_FLAG_IDS).toContain(f.flagId);
    }
  });

  it('raised цена/км несёт числа evidence; insufficient — выборка < порога', () => {
    const raised = flagsFor('DEMO-0002')[0];
    expect(raised.flagState).toBe('raised');
    expect(raised.evidence.medianPerKm).toBeTruthy();
    expect(raised.evidence.thresholdPerKm).toBeTruthy();
    expect(raised.evidence.thisPerKm).toBeTruthy();

    const insuff = flagsFor('DEMO-0003')[0];
    expect(insuff.flagState).toBe('insufficient_data');
    expect(insuff.evidence.sampleSize ?? 0).toBeLessThan(insuff.evidence.minSample ?? 5);
  });

  it('нейтральность: метки флагов есть в обеих локалях', () => {
    for (const lang of LANGS) {
      const flag = resources[lang].chrome.flag;
      for (const id of ALL_FLAG_IDS) {
        expect(flag[id].summary, `${lang}.flag.${id}.summary`).toBeTruthy();
      }
    }
  });

  it('нейтральность: проза flag/methodology/report_error свободна от taboo-корней', () => {
    for (const lang of LANGS) {
      const chrome = resources[lang].chrome as Record<string, unknown>;
      const strings = [
        ...collectStrings(chrome.flag),
        ...collectStrings(chrome.methodology),
        ...collectStrings(chrome.report_error),
        ...collectStrings((chrome.contract as Record<string, unknown>).signals_note),
      ].map((s) => s.toLowerCase());
      for (const s of strings) {
        for (const root of TABOO_ROOTS) {
          expect(s.includes(root), `taboo "${root}" в ${lang}: "${s}"`).toBe(false);
        }
      }
    }
  });
});
