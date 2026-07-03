import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  fetchMapObjects,
  splitObjects,
  markerKind,
  boundsToBBox,
  debounce,
  type MapObject,
} from './objects';

const obj = (over: Omit<Partial<MapObject>, 'geom'> & { geom: unknown }): MapObject =>
  ({
    public_id: 'p-1',
    goszakup_contract_id: 'DEMO-GEO-01',
    geocode_status: 'auto',
    has_active_flag: false,
    length_km: null,
    ...over,
  }) as MapObject;

afterEach(() => {
  vi.restoreAllMocks();
  vi.useRealTimers();
});

describe('fetchMapObjects', () => {
  it('200 → возвращает тело как есть', async () => {
    const body = { items: [], truncated: false, ungeocoded_count: 3 };
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(body),
    } as unknown as Response);
    await expect(fetchMapObjects('71.2,51.0,71.78,51.3')).resolves.toEqual(body);
    expect(globalThis.fetch).toHaveBeenCalledWith(
      `/api/map/objects?bbox=${encodeURIComponent('71.2,51.0,71.78,51.3')}`,
      { signal: undefined },
    );
  });

  it('пробрасывает AbortSignal в fetch (отмена in-flight при смене окна)', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ items: [], truncated: false, ungeocoded_count: 0 }),
    } as unknown as Response);
    const ctrl = new AbortController();
    await fetchMapObjects('71.2,51.0,71.78,51.3', ctrl.signal);
    expect(globalThis.fetch).toHaveBeenCalledWith(expect.any(String), { signal: ctrl.signal });
  });

  it('!ok → бросает с кодом из Error-конверта', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      json: () => Promise.resolve({ error: { code: 'VALIDATION_FAILED', message: 'bbox' } }),
    } as unknown as Response);
    await expect(fetchMapObjects('мусор')).rejects.toMatchObject({
      status: 400,
      code: 'VALIDATION_FAILED',
    });
  });

  it('!ok с не-JSON телом → код INTERNAL (честный дефолт)', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 502,
      json: () => Promise.reject(new Error('not json')),
    } as unknown as Response);
    await expect(fetchMapObjects('71.2,51.0,71.78,51.3')).rejects.toMatchObject({
      status: 502,
      code: 'INTERNAL',
    });
  });
});

describe('splitObjects', () => {
  it('Point → points [lon,lat], LineString → lines', () => {
    const { points, lines } = splitObjects([
      obj({ geom: { type: 'Point', coordinates: [71.43, 51.13] } }),
      obj({
        public_id: 'l-1',
        geom: {
          type: 'LineString',
          coordinates: [
            [71.4, 51.1],
            [71.45, 51.14],
          ],
        },
      }),
    ]);
    expect(points).toHaveLength(1);
    expect(points[0].lon).toBe(71.43); // порядок [lon,lat] — несущий инвариант
    expect(points[0].lat).toBe(51.13);
    expect(lines).toHaveLength(1);
    expect(lines[0].coordinates).toHaveLength(2);
  });

  it('RFC7946-позиция [lon,lat,alt] (Z-геометрия куратора, CHECK 0022 её пропускает) → рисуется по lon/lat', () => {
    const { points, lines } = splitObjects([
      obj({ geom: { type: 'Point', coordinates: [71.43, 51.13, 350.0] } }),
      obj({
        public_id: 'l-3d',
        geom: {
          type: 'LineString',
          coordinates: [
            [71.4, 51.1, 350.0],
            [71.45, 51.14, 351.0],
          ],
        },
      }),
    ]);
    expect(points).toHaveLength(1);
    expect(points[0].lon).toBe(71.43);
    expect(points[0].lat).toBe(51.13);
    expect(lines).toHaveLength(1);
  });

  it('неизвестный тип геометрии → пропуск с warn (default-ветка AR-16), не выдуманная точка', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
    const { points, lines } = splitObjects([
      obj({ geom: { type: 'Polygon', coordinates: [] } }),
      obj({ geom: null }),
    ]);
    expect(points).toHaveLength(0);
    expect(lines).toHaveLength(0);
    expect(warn).toHaveBeenCalledTimes(2);
  });

  it('нефинитные/битые координаты → пропуск (координату не выдумываем)', () => {
    vi.spyOn(console, 'warn').mockImplementation(() => {});
    const { points, lines } = splitObjects([
      obj({ geom: { type: 'Point', coordinates: [Infinity, 51.13] } }),
      obj({ geom: { type: 'Point', coordinates: [71.43] } }),
      obj({
        geom: {
          type: 'LineString',
          coordinates: [[71.4, 51.1]], // одна вершина — не линия
        },
      }),
    ]);
    expect(points).toHaveLength(0);
    expect(lines).toHaveLength(0);
  });
});

describe('markerKind (D3: глиф статуса)', () => {
  it('активный флаг первичен — даже поверх verified', () => {
    expect(markerKind({ geocode_status: 'verified', has_active_flag: true })).toBe('flagged');
  });
  it('verified без флага → confirmed («✓»)', () => {
    expect(markerKind({ geocode_status: 'verified', has_active_flag: false })).toBe('confirmed');
  });
  it('auto/manual/wrong_reported/unmatched → plain (все 5 статусов enum через маппер; unmatched структурно не приходит, но enum его несёт)', () => {
    for (const s of ['auto', 'manual', 'wrong_reported', 'unmatched'] as const) {
      expect(markerKind({ geocode_status: s, has_active_flag: false })).toBe('plain');
    }
  });
  it('НЕИЗВЕСТНЫЙ статус → plain (default-ветка обязана существовать, negative-control)', () => {
    expect(
      markerKind({
        geocode_status: 'bogus-future-status' as MapObject['geocode_status'],
        has_active_flag: false,
      }),
    ).toBe('plain');
  });
});

describe('boundsToBBox', () => {
  const bounds = (w: number, s: number, e: number, n: number) => ({
    getWest: () => w,
    getSouth: () => s,
    getEast: () => e,
    getNorth: () => n,
  });

  it('обычное окно → строка minLon,minLat,maxLon,maxLat', () => {
    expect(boundsToBBox(bounds(71.2, 51.0, 71.78, 51.3))).toBe('71.2,51,71.78,51.3');
  });

  it('за пределами WGS84 (обёртка мира на малом зуме) → зажим, а не 400 от сервера', () => {
    expect(boundsToBBox(bounds(-250, -95, 250, 95))).toBe('-180,-90,180,90');
  });

  it('вырожденное после зажима окно → null (запрос не уходит)', () => {
    expect(boundsToBBox(bounds(200, 51.0, 250, 51.3))).toBeNull();
  });
});

describe('debounce (D6)', () => {
  it('схлопывает серию вызовов в один по истечении интервала (продвигающиеся часы — урок 7-1)', () => {
    vi.useFakeTimers();
    const fn = vi.fn();
    const d = debounce(fn, 300);
    d('a');
    vi.advanceTimersByTime(100);
    d('b');
    vi.advanceTimersByTime(100);
    d('c');
    expect(fn).not.toHaveBeenCalled();
    vi.advanceTimersByTime(300);
    expect(fn).toHaveBeenCalledTimes(1);
    expect(fn).toHaveBeenCalledWith('c');
  });

  it('cancel отменяет отложенный вызов (unmount не стреляет по снесённой карте)', () => {
    vi.useFakeTimers();
    const fn = vi.fn();
    const d = debounce(fn, 300);
    d('a');
    d.cancel();
    vi.advanceTimersByTime(1000);
    expect(fn).not.toHaveBeenCalled();
  });
});
