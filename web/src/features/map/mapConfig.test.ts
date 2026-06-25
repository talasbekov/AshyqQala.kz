import { describe, it, expect } from 'vitest';
import { ASTANA_CENTER, toLngLat } from './mapConfig';

// Viewbox Астаны (stage0-audit/config.go): lon 71.20–71.78, lat 51.00–51.30.
const LON_MIN = 71.2;
const LON_MAX = 71.78;
const LAT_MIN = 51.0;
const LAT_MAX = 51.3;

describe('mapConfig', () => {
  it('toLngLat возвращает порядок [lon, lat] (НЕ [lat,lon])', () => {
    expect(toLngLat({ lon: 71.4, lat: 51.1 })).toEqual([71.4, 51.1]);
    const [lon, lat] = toLngLat({ lon: 71.4, lat: 51.1 });
    expect(lon).toBe(71.4);
    expect(lat).toBe(51.1);
  });

  it('центр Астаны в пределах viewbox', () => {
    const [lon, lat] = ASTANA_CENTER;
    expect(lon).toBeGreaterThanOrEqual(LON_MIN);
    expect(lon).toBeLessThanOrEqual(LON_MAX);
    expect(lat).toBeGreaterThanOrEqual(LAT_MIN);
    expect(lat).toBeLessThanOrEqual(LAT_MAX);
  });
});
