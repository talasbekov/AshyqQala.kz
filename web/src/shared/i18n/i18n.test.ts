import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { describe, it, expect } from 'vitest';
import { resources, LANGS } from './index';

const here = dirname(fileURLToPath(import.meta.url));
// honest_states.json — источник истины honest-state (Story 1.4), repo-root registry/.
const honest = JSON.parse(
  readFileSync(join(here, '../../../../registry/values/honest_states.json'), 'utf8'),
) as { value_state: string[]; flag_state: string[] };

describe('i18n chrome локали', () => {
  it('бейдж фолбэка verbatim в обеих локалях', () => {
    expect(resources.ru.chrome.lang.ru_source).toBe('рус. источник');
    expect(resources.kk.chrome.lang.ru_source).toBe('орыс дереккөзі');
  });

  it('i18n-ось: обе локали покрывают КАЖДЫЙ value_state (для DataState/меток)', () => {
    for (const lang of LANGS) {
      const vs = resources[lang].chrome.value_state as Record<string, string>;
      for (const s of honest.value_state) {
        expect(vs[s], `${lang}.value_state.${s}`).toBeTruthy();
      }
    }
  });

  it('i18n-ось: обе локали покрывают КАЖДЫЙ flag_state', () => {
    for (const lang of LANGS) {
      const fs = resources[lang].chrome.flag_state as Record<string, string>;
      for (const s of honest.flag_state) {
        expect(fs[s], `${lang}.flag_state.${s}`).toBeTruthy();
      }
    }
  });

  it('нейтральная рамка присутствует (не taboo)', () => {
    expect(resources.ru.chrome.frame.signal).toContain('требующий проверки');
    expect(resources.kk.chrome.frame.signal).toContain('тексеру');
  });
});
