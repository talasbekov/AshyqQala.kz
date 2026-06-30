import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { describe, it, expect } from 'vitest';
import { resources, LANGS } from '../../shared/i18n';

// Story 6.3 (AC4): per-surface нейтральность + парность kk↔ru для строк страницы района.
// taboo_lexicon.json — единый источник нейтральности (как searchStrings.test 6.1/6.2).
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

describe('district строки (Story 6.3)', () => {
  it('per-surface нейтральность: district.* свободны от taboo-корней в обеих локалях', () => {
    for (const lang of LANGS) {
      const dict = (resources[lang].chrome as Record<string, unknown>).district;
      for (const s of collectStrings(dict).map((x) => x.toLowerCase())) {
        for (const root of TABOO_ROOTS) {
          expect(s.includes(root), `taboo "${root}" в ${lang}.district: "${s}"`).toBe(false);
        }
      }
    }
  });

  it('парность kk↔ru: одинаковый набор ключей district.* (нет тихого ru-фолбэка для kk)', () => {
    const ru = keyPaths((resources.ru.chrome as Record<string, unknown>).district).sort();
    const kk = keyPaths((resources.kk.chrome as Record<string, unknown>).district).sort();
    expect(kk).toEqual(ru);
  });

  it('ключевые строки присутствуют (имя-фолбэк, container_state, поля сводки)', () => {
    for (const lang of LANGS) {
      const d = (resources[lang].chrome as Record<string, Record<string, unknown>>).district;
      for (const k of [
        'overlabel',
        'name_pending',
        'name_pending_note',
        'signals_heading',
        'objects_heading',
      ]) {
        expect(typeof d[k], `${lang}.district.${k}`).toBe('string');
        expect((d[k] as string).trim().length).toBeGreaterThan(0);
      }
      const container = d.container as Record<string, string>;
      for (const cs of ['no_contracts', 'no_flags_raised', 'not_geocoded']) {
        expect(
          (container[cs] ?? '').trim().length,
          `${lang}.district.container.${cs}`,
        ).toBeGreaterThan(0);
      }
      const field = d.field as Record<string, string>;
      for (const f of ['objects', 'total_amount', 'active_flags']) {
        expect((field[f] ?? '').trim().length, `${lang}.district.field.${f}`).toBeGreaterThan(0);
      }
    }
  });

  // Story 6.4 (FR-18): подблок median.* присутствует в ОБЕИХ локалях (числа методики — из API, здесь только
  // проза). Парность уже покрыта общим тестом keyPaths; здесь — явное присутствие несущих median-ключей.
  it('median.* (FR-18): ключи блока медианы района присутствуют в обеих локалях', () => {
    for (const lang of LANGS) {
      const d = (resources[lang].chrome as Record<string, Record<string, unknown>>).district;
      const median = d.median as Record<string, string>;
      expect(median, `${lang}.district.median отсутствует`).toBeTruthy();
      for (const k of ['section', 'heading', 'city_label', 'delta', 'computed']) {
        expect((median[k] ?? '').trim().length, `${lang}.district.median.${k}`).toBeGreaterThan(0);
      }
    }
  });

  // Гардрейл нейтральности района (UX-макет): агрегат флага монополии назван «Преобладание подрядчика в
  // районе», НЕ «монополия». Discriminating-страж: краснеет, если кто-то впишет «монопол…» в ru-имя.
  it('монополия на поверхности района названа нейтрально (не «монополия»)', () => {
    const district = (resources.ru.chrome as Record<string, unknown>).district as {
      flag: { monopoly: { name: string } };
    };
    const ruName = district.flag.monopoly.name.toLowerCase();
    expect(ruName.includes('монопол'), `district.flag.monopoly.name (ru) = "${ruName}"`).toBe(
      false,
    );
    expect(ruName).toContain('преобладание');
  });
});
