import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { describe, it, expect } from 'vitest';
import { resources, LANGS } from '../../shared/i18n';

// Story 3.4 (AC1/AC2): per-surface нейтральность + парность kk↔ru для строк карты.
// taboo_lexicon.json — единый источник нейтральности (паттерн districtStrings.test 6.3).
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

// findTaboo — ЕДИНЫЙ матчер стража и negative-control (guards-must-prove-red: контроль обязан гонять
// ТУ ЖЕ функцию, что и страж, — проверка «String.includes работает» ничего не доказывает).
// toLowerCase с ОБЕИХ сторон: капитализированный корень в лексиконе иначе молча перестал бы матчиться.
function findTaboo(strings: string[]): { s: string; root: string }[] {
  const hits: { s: string; root: string }[] = [];
  for (const s of strings.map((x) => x.toLowerCase())) {
    for (const root of TABOO_ROOTS.map((r) => r.toLowerCase())) {
      if (s.includes(root)) hits.push({ s, root });
    }
  }
  return hits;
}

describe('map строки (Story 3.4)', () => {
  it('per-surface нейтральность: map.* свободны от taboo-корней в обеих локалях', () => {
    for (const lang of LANGS) {
      const dict = (resources[lang].chrome as Record<string, unknown>).map;
      expect(findTaboo(collectStrings(dict))).toEqual([]);
    }
  });

  // Negative-control (memory guards-must-prove-red): сам страж (findTaboo, не String.includes)
  // краснеет на подмешанном словаре с taboo-корнем — включая капитализированный вариант.
  it('negative-control: страж ловит подмешанный taboo-корень (и в верхнем регистре)', () => {
    const root = taboo.ru[0];
    expect(root.length).toBeGreaterThan(0);
    const ruMap = (resources.ru.chrome as Record<string, unknown>).map as Record<string, unknown>;
    const poisoned = collectStrings({
      ...ruMap,
      injected: `объект с ${root.toUpperCase()}ем на карте`,
    });
    expect(findTaboo(poisoned).length).toBeGreaterThan(0);
  });

  it('парность kk↔ru: одинаковый набор ключей map.* (нет тихого ru-фолбэка для kk)', () => {
    const ru = keyPaths((resources.ru.chrome as Record<string, unknown>).map).sort();
    const kk = keyPaths((resources.kk.chrome as Record<string, unknown>).map).sort();
    expect(kk).toEqual(ru);
  });

  it('несущие ключи 3.4 присутствуют в обеих локалях (маркеры/кластер/счётчик/честные состояния)', () => {
    for (const lang of LANGS) {
      const m = (resources[lang].chrome as Record<string, unknown>).map as Record<string, string>;
      for (const k of [
        'marker_label',
        'marker_label_flag',
        'marker_label_confirmed',
        'cluster_label',
        'cluster_label_flag',
        'objects_error',
        'objects_empty',
        'truncated_notice',
        'ungeocoded_more',
      ]) {
        expect(typeof m[k], `${lang}.map.${k}`).toBe('string');
        expect(m[k].trim().length, `${lang}.map.${k} пуст`).toBeGreaterThan(0);
      }
    }
  });

  // Флаг-проза карты строго в нейтральной рамке: «сигнал(ы), требующ…» — не «нарушение»/оценка.
  it('флаг-метки маркера/кластера несут рамку «сигнал…» (ru)', () => {
    const m = (resources.ru.chrome as Record<string, unknown>).map as Record<string, string>;
    expect(m.marker_label_flag.toLowerCase()).toContain('сигнал');
    expect(m.cluster_label_flag.toLowerCase()).toContain('сигнал');
  });

  // i18next-ловушка (урок 6.4 P2): 'count' — reserved-имя (плюрализация). Интерполяция числа в
  // map.* обязана использовать другое имя ({{n}}); появление {{count}} — регресс.
  it('интерполяция числа — {{n}}, не reserved {{count}}', () => {
    for (const lang of LANGS) {
      const m = (resources[lang].chrome as Record<string, unknown>).map as Record<string, string>;
      for (const k of ['cluster_label', 'cluster_label_flag', 'ungeocoded_more']) {
        expect(m[k].includes('{{count}}'), `${lang}.map.${k} использует reserved count`).toBe(
          false,
        );
        expect(m[k].includes('{{n}}'), `${lang}.map.${k} без {{n}}`).toBe(true);
      }
    }
  });
});
