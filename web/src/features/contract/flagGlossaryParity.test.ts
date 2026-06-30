import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { describe, it, expect } from 'vitest';
import { resources, LANGS } from '../../shared/i18n';

// Story 5.5 (вариант A — единый источник прозы флага): name/summary флагов в web-i18n ДОЛЖНЫ совпадать
// со строками в registry/values/glossary-*.json. Так OG-рендер (Go, из glossary через render.FlagLine) и
// web-бейдж говорят ОДИН текст → cross-surface golden (render.TestFlagLine_CrossSurfaceGolden) осмыслен.
// Этот тест — «страж единого источника»: краснеет, если копии разойдутся (две расходящиеся правды запрещены).
const here = dirname(fileURLToPath(import.meta.url));

function glossary(lang: string): Record<string, string> {
  return JSON.parse(
    readFileSync(join(here, `../../../../registry/values/glossary-${lang}.json`), 'utf8'),
  ) as Record<string, string>;
}

function getKey(dict: Record<string, unknown>, path: string): unknown {
  return path.split('.').reduce<unknown>((o, p) => (o as Record<string, unknown>)?.[p], dict);
}

// Флаги MVP (синхронно с server contractFlagTypes / registry requiredGlossaryKeys).
const FLAGS = ['single_participant', 'price_per_km'];
const FIELDS = ['name', 'summary'];

// «Хвосты», которые web FlagBadge склеивает ПОМИМО name/summary (FlagBadge.tsx:29-31):
//   raised → `${summary} — ${t('frame.signal')}`;  иначе → `${summary}: ${t('flag.insufficient')}`.
// Cross-surface golden (render.TestFlagLine_CrossSurfaceGolden) пинит вывод render.FlagLine, который
// собирает рамку из glossary frame.signal и хвост из value_state.insufficient_sample. Без сверки ниже
// web-only правка `frame.signal`/`flag.insufficient` молча развела бы web↔OG, а golden остался бы зелёным
// ([[guards-must-prove-red]]: страж обязан покрывать ВСЁ смысловое ядро, что он же заявляет).
const TAILS: { web: string; reg: string }[] = [
  { web: 'frame.signal', reg: 'frame.signal' },
  { web: 'flag.insufficient', reg: 'value_state.insufficient_sample' },
];

describe('Проза флага: единый источник registry↔web (Story 5.5, вариант A)', () => {
  it('flag.<id>.name/summary в web-i18n РАВНЫ registry/values/glossary в обеих локалях', () => {
    for (const lang of LANGS) {
      const g = glossary(lang);
      const chrome = resources[lang].chrome as Record<string, unknown>;
      for (const id of FLAGS) {
        for (const field of FIELDS) {
          const regVal = g[`flag.${id}.${field}`];
          const webVal = getKey(chrome, `flag.${id}.${field}`);
          expect(typeof regVal, `registry glossary-${lang}: flag.${id}.${field}`).toBe('string');
          expect(webVal, `web ${lang}: flag.${id}.${field} должен совпадать с registry`).toBe(
            regVal,
          );
        }
      }
    }
  });

  it('frame.signal и flag.insufficient (хвосты бейджа) в web-i18n РАВНЫ registry в обеих локалях', () => {
    for (const lang of LANGS) {
      const g = glossary(lang);
      const chrome = resources[lang].chrome as Record<string, unknown>;
      for (const { web, reg } of TAILS) {
        const regVal = g[reg];
        const webVal = getKey(chrome, web);
        expect(typeof regVal, `registry glossary-${lang}: ${reg}`).toBe('string');
        expect(webVal, `web ${lang}: ${web} должен совпадать с registry ${reg}`).toBe(regVal);
      }
    }
  });
});
