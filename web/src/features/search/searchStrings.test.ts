import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { describe, it, expect } from 'vitest';
import { resources, LANGS } from '../../shared/i18n';

// Story 6.1 (AC6): per-surface нейтральность + парность kk↔ru для строк фасетного поиска.
// taboo_lexicon.json — единый источник нейтральности (как contractCardStrings.test 5.1).
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
function keyPaths(obj: unknown, prefix = '', out: string[] = []): string[] {
  if (obj && typeof obj === 'object')
    for (const [k, v] of Object.entries(obj)) {
      const p = prefix ? `${prefix}.${k}` : k;
      if (v && typeof v === 'object') keyPaths(v, p, out);
      else out.push(p);
    }
  return out;
}

describe('search строки (Story 6.1)', () => {
  it('per-surface нейтральность: search.* свободны от taboo-корней в обеих локалях', () => {
    for (const lang of LANGS) {
      const dict = (resources[lang].chrome as Record<string, unknown>).search;
      for (const s of collectStrings(dict).map((x) => x.toLowerCase())) {
        for (const root of TABOO_ROOTS) {
          expect(s.includes(root), `taboo "${root}" в ${lang}.search: "${s}"`).toBe(false);
        }
      }
    }
  });

  it('парность kk↔ru: одинаковый набор ключей search.* (нет тихого ru-фолбэка для kk)', () => {
    const ru = keyPaths((resources.ru.chrome as Record<string, unknown>).search).sort();
    const kk = keyPaths((resources.kk.chrome as Record<string, unknown>).search).sort();
    expect(kk).toEqual(ru);
  });

  it('ключевые строки присутствуют (пустой результат, has_flag, период-фасет)', () => {
    for (const lang of LANGS) {
      const s = (resources[lang].chrome as Record<string, Record<string, unknown>>).search;
      for (const k of ['empty_title', 'empty_hint', 'has_flag', 'period_note', 'signal_present']) {
        expect(typeof s[k], `${lang}.search.${k}`).toBe('string');
        expect((s[k] as string).trim().length).toBeGreaterThan(0);
      }
    }
  });

  // Story 6.2 (AC7): строки текстового поиска присутствуют и нейтральны (общий taboo/парность-страж выше уже
  // покрывает их generically; здесь — явная фиксация ключей 6.2, чтобы их удаление/пропуск падал тестом).
  it('строки текстового поиска (Story 6.2) присутствуют в обеих локалях', () => {
    for (const lang of LANGS) {
      const s = (resources[lang].chrome as Record<string, Record<string, unknown>>).search;
      for (const k of [
        'query_placeholder',
        'query_min_hint',
        'query_results_heading',
        'query_empty_title',
        'result_kind_org',
        'result_kind_contract',
        'result_bin',
      ]) {
        expect(typeof s[k], `${lang}.search.${k}`).toBe('string');
        expect((s[k] as string).trim().length).toBeGreaterThan(0);
      }
    }
  });
});
