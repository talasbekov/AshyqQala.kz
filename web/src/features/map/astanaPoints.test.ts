import { describe, it, expect } from 'vitest';
import {
  ASTANA_POINTS,
  UNGEOCODED_CONTRACTS,
  ASTANA_CENTER,
  toLngLat,
  type GeoPoint,
} from './astanaPoints';

// Viewbox Астаны (stage0-audit/config.go): lon 71.20–71.78, lat 51.00–51.30.
const LON_MIN = 71.2;
const LON_MAX = 71.78;
const LAT_MIN = 51.0;
const LAT_MAX = 51.3;

describe('astanaPoints', () => {
  it('toLngLat возвращает порядок [lon, lat] (НЕ [lat,lon])', () => {
    const p: GeoPoint = { goszakupId: 'X', lon: 71.4, lat: 51.1 };
    expect(toLngLat(p)).toEqual([71.4, 51.1]);
    const [lon, lat] = toLngLat(p);
    expect(lon).toBe(71.4);
    expect(lat).toBe(51.1);
  });

  it('ровно три геокодированные точки (AC1)', () => {
    expect(ASTANA_POINTS).toHaveLength(3);
  });

  it('все точки в пределах viewbox Астаны', () => {
    for (const p of ASTANA_POINTS) {
      expect(p.lon).toBeGreaterThanOrEqual(LON_MIN);
      expect(p.lon).toBeLessThanOrEqual(LON_MAX);
      expect(p.lat).toBeGreaterThanOrEqual(LAT_MIN);
      expect(p.lat).toBeLessThanOrEqual(LAT_MAX);
    }
  });

  it('центр Астаны в пределах viewbox', () => {
    const [lon, lat] = ASTANA_CENTER;
    expect(lon).toBeGreaterThanOrEqual(LON_MIN);
    expect(lon).toBeLessThanOrEqual(LON_MAX);
    expect(lat).toBeGreaterThanOrEqual(LAT_MIN);
    expect(lat).toBeLessThanOrEqual(LAT_MAX);
  });

  it('есть хотя бы один негеокодированный контракт (AC3)', () => {
    expect(UNGEOCODED_CONTRACTS.length).toBeGreaterThanOrEqual(1);
  });

  it('id уникальны среди гео- и негео-контрактов', () => {
    const ids = [
      ...ASTANA_POINTS.map((p) => p.goszakupId),
      ...UNGEOCODED_CONTRACTS.map((u) => u.goszakupId),
    ];
    expect(new Set(ids).size).toBe(ids.length);
  });
});
