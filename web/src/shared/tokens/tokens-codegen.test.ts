import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { describe, it, expect } from 'vitest';
import { generate } from '../../../scripts/tokens-codegen.mjs';

const here = dirname(fileURLToPath(import.meta.url));
const webRoot = join(here, '../../..');
const tokens = JSON.parse(readFileSync(join(webRoot, 'tokens.json'), 'utf8'));
const out = generate(tokens);

describe('tokens codegen (golden / generated==regenerated)', () => {
  it('закоммиченный tokens.css == перегенерированный', () => {
    const committed = readFileSync(join(here, 'tokens.css'), 'utf8');
    expect(committed).toBe(out.css);
  });

  it('закоммиченный tokens.gen.ts == перегенерированный', () => {
    const committed = readFileSync(join(here, 'tokens.gen.ts'), 'utf8');
    expect(committed).toBe(out.ts);
  });

  it('детерминизм: повторная генерация идентична', () => {
    expect(generate(tokens)).toEqual(out);
  });

  it('css несёт обе темы и нейтральный янтарь флага (не алый)', () => {
    expect(out.css).toContain(':root {');
    expect(out.css).toContain('[data-theme="dark"] {');
    expect(out.css).toContain('--color-signal-attention: #E09915;'); // янтарь
    expect(out.css).toContain('--color-surface: #FFFFFF;');
  });
});
