import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { describe, it, expect } from 'vitest';
import { contrastRatio } from './contrast';

const here = dirname(fileURLToPath(import.meta.url));
const tokens = JSON.parse(readFileSync(join(here, '../../..', 'tokens.json'), 'utf8'));

type Theme = 'light' | 'dark';
function c(role: string, theme: Theme): string {
  const t = tokens.color[role];
  return theme === 'light' ? t.$value : t.$extensions['aq.dark'];
}

// Ключевые пары из DESIGN.md (WCAG 2.1 AA). Каркас — расширяется парами с первым цветом флага.
const cases: Array<{ name: string; fg: string; bg: string; theme: Theme; min: number }> = [
  { name: 'text-primary / surface', fg: 'text-primary', bg: 'surface', theme: 'light', min: 4.5 },
  {
    name: 'text-secondary / surface',
    fg: 'text-secondary',
    bg: 'surface',
    theme: 'light',
    min: 4.5,
  },
  { name: 'muted-text / surface', fg: 'muted-text', bg: 'surface', theme: 'light', min: 4.5 },
  {
    name: 'signal-attention-fg / signal-attention-bg',
    fg: 'signal-attention-fg',
    bg: 'signal-attention-bg',
    theme: 'light',
    min: 4.5,
  },
  {
    name: 'link-on-sunken / surface',
    fg: 'link-on-sunken',
    bg: 'surface',
    theme: 'light',
    min: 4.5,
  },
  {
    name: 'link-on-sunken / surface-sunken',
    fg: 'link-on-sunken',
    bg: 'surface-sunken',
    theme: 'light',
    min: 4.5,
  },
  // focus-ring — графический контраст ≥3:1 к ОБЕИМ фонам (WCAG 1.4.11)
  { name: 'focus-ring / surface', fg: 'focus-ring', bg: 'surface', theme: 'light', min: 3 },
  {
    name: 'focus-ring / surface-sunken',
    fg: 'focus-ring',
    bg: 'surface-sunken',
    theme: 'light',
    min: 3,
  },
  // тёмная тема
  {
    name: 'text-primary / surface (dark)',
    fg: 'text-primary',
    bg: 'surface',
    theme: 'dark',
    min: 4.5,
  },
  {
    name: 'signal-attention-fg / bg (dark)',
    fg: 'signal-attention-fg',
    bg: 'signal-attention-bg',
    theme: 'dark',
    min: 4.5,
  },
  { name: 'focus-ring / surface (dark)', fg: 'focus-ring', bg: 'surface', theme: 'dark', min: 3 },
];

describe('WCAG контраст ключевых пар токенов', () => {
  for (const tc of cases) {
    it(`${tc.name} ≥ ${tc.min}:1`, () => {
      const ratio = contrastRatio(c(tc.fg, tc.theme), c(tc.bg, tc.theme));
      expect(ratio).toBeGreaterThanOrEqual(tc.min);
    });
  }

  it('contrastRatio: белый/чёрный = 21', () => {
    expect(contrastRatio('#FFFFFF', '#000000')).toBeCloseTo(21, 0);
  });
});
