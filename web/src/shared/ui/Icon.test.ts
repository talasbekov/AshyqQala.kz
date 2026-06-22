import { describe, it, expect } from 'vitest';
import { ICON_GLYPH } from './Icon';
import type { IconName } from './Icon';

describe('Icon (закрытый набор имён)', () => {
  it('каждое имя имеет непустой глиф', () => {
    const names: IconName[] = [
      'flag',
      'verified',
      'no-data',
      'insufficient',
      'ungeocoded',
      'error',
    ];
    for (const n of names) {
      expect(ICON_GLYPH[n], n).toBeTruthy();
    }
  });

  it('карта глифов не имеет лишних/пустых значений', () => {
    for (const [name, glyph] of Object.entries(ICON_GLYPH)) {
      expect(glyph, name).toBeTruthy();
    }
  });
});
