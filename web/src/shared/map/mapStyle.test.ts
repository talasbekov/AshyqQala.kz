import { describe, it, expect } from 'vitest';
import { buildMapStyle } from './mapStyle';

describe('buildMapStyle', () => {
  it('по умолчанию — валидный стиль с одним нейтральным background-слоем (AC1)', () => {
    const s = buildMapStyle({ backgroundColor: 'rgb(238, 246, 249)' });
    expect(s.version).toBe(8);
    expect(s.layers).toHaveLength(1);
    expect(s.layers[0].type).toBe('background');
    const bg = s.layers[0] as { paint?: Record<string, unknown> };
    expect(bg.paint?.['background-color']).toBe('rgb(238, 246, 249)');
    expect(Object.keys(s.sources)).toHaveLength(0);
  });

  it('с basemapPmtilesUrl добавляет pmtiles-источник и raster-слой подложки', () => {
    const s = buildMapStyle({
      backgroundColor: 'rgb(0, 0, 0)',
      basemapPmtilesUrl: '/tiles/astana.pmtiles',
      attribution: '© источник',
    });
    expect(s.layers).toHaveLength(2);
    const src = s.sources['aq-basemap'] as { type: string; url: string; attribution?: string };
    expect(src.type).toBe('raster');
    expect(src.url).toBe('pmtiles:///tiles/astana.pmtiles');
    expect(src.attribution).toBe('© источник');
    expect(s.layers.some((l) => l.id === 'aq-basemap-raster')).toBe(true);
  });
});
