import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  fetchMapObjects,
  splitObjects,
  markerKind,
  boundsToBBox,
  debounce,
  coordKey,
  coincidentPoints,
  allCoincident,
  pricePerKm,
  contractPath,
  type MapObject,
  type PointObject,
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

// --- Story 3.5 (AC3): множественное попадание — двойники координат ---

const pt = (id: string, lon: number, lat: number): PointObject => ({
  obj: obj({ public_id: id, geom: { type: 'Point', coordinates: [lon, lat] } }),
  lon,
  lat,
});

describe('coordKey / coincidentPoints (AC3, D5)', () => {
  it('одна координата → один ключ; расхождение за ε (6 знаков) → разные ключи', () => {
    expect(coordKey(71.43, 51.13)).toBe(coordKey(71.43, 51.13));
    // 1e-7 схлопывается округлением до 6 знаков (двойники «в одном адресе»)
    expect(coordKey(71.43, 51.13)).toBe(coordKey(71.43 + 1e-7, 51.13));
    // 1e-5 — уже разные точки (≈1 м): не сливаем
    expect(coordKey(71.43, 51.13)).not.toBe(coordKey(71.43 + 1e-5, 51.13));
  });

  it('группа двойников: все точки той же координаты, включая саму цель', () => {
    const a = pt('a', 71.43, 51.13);
    const b = pt('b', 71.43, 51.13);
    const c = pt('c', 71.44, 51.13);
    expect(coincidentPoints([a, b, c], a).map((p) => p.obj.public_id)).toEqual(['a', 'b']);
  });

  it('negative-control: без двойников группа = только сама точка (лист-список НЕ открывается)', () => {
    const a = pt('a', 71.43, 51.13);
    const c = pt('c', 71.44, 51.13);
    expect(coincidentPoints([a, c], a)).toHaveLength(1);
  });
});

describe('allCoincident (кластер «не разваливается зумом»)', () => {
  it('все позиции совпадают (ε) → true', () => {
    expect(
      allCoincident([
        [71.43, 51.13],
        [71.43 + 1e-7, 51.13],
        [71.43, 51.13],
      ]),
    ).toBe(true);
  });

  it('negative-control: хоть одна позиция врозь → false (обычный зум, не превью-список)', () => {
    expect(
      allCoincident([
        [71.43, 51.13],
        [71.4315, 51.13],
      ]),
    ).toBe(false);
  });

  it('пустой/одиночный набор → true (вырожденно совпадают)', () => {
    expect(allCoincident([])).toBe(true);
    expect(allCoincident([[71.43, 51.13]])).toBe(true);
  });
});

// --- Story 3.5 (AC1, D6): цена/км линии — деривация из двух ВИДИМЫХ фактов, не выдумка ---

describe('pricePerKm (D6)', () => {
  it('каноничная сумма и длина > 0 → целые ₸/км (усечение)', () => {
    expect(pricePerKm('90000000', 5)).toBe('18000000');
    expect(pricePerKm('90000000', 3.06)).toBe('29411764'); // 90e6/3.06 = 29411764.7…
  });

  it('точность > 2^53 не теряется (BigInt-путь): 2^54 ₸ / 2 км', () => {
    expect(pricePerKm('18014398509481984', 2)).toBe('9007199254740992');
  });

  it('честные null: нет суммы / неканоничная / отрицательная — не выдумываем', () => {
    expect(pricePerKm(null, 5)).toBeNull();
    expect(pricePerKm(undefined, 5)).toBeNull();
    expect(pricePerKm('', 5)).toBeNull();
    expect(pricePerKm('12 000', 5)).toBeNull();
    expect(pricePerKm('-90000000', 5)).toBeNull();
  });

  it('честные null: длина отсутствует / 0 / отрицательная / нефинитная', () => {
    expect(pricePerKm('90000000', null)).toBeNull();
    expect(pricePerKm('90000000', undefined)).toBeNull();
    expect(pricePerKm('90000000', 0)).toBeNull();
    expect(pricePerKm('90000000', -5)).toBeNull();
    expect(pricePerKm('90000000', Number.NaN)).toBeNull();
    // вырожденно малая длина: scale=round(0.0000004*1000)=0 → null, не деление на ноль
    expect(pricePerKm('90000000', 0.0000004)).toBeNull();
  });

  it('монструозная длина: lengthKm*1000 → Infinity — null, не RangeError из BigInt (код-ревью 3.5)', () => {
    expect(pricePerKm('90000000', 9e305)).toBeNull();
  });
});

// contractPath — единственная точка сборки пути карточки (код-ревью 3.5: энкодинг внешнего id).
describe('contractPath', () => {
  it('безопасный id — как есть', () => {
    expect(contractPath('DEMO-GEO-04')).toBe('/contracts/DEMO-GEO-04');
  });

  it('спецсимволы natural-id энкодятся (слэш/пробел/кириллица/юникод не ломают роут)', () => {
    expect(contractPath('A/B 01')).toBe('/contracts/A%2FB%2001');
    expect(contractPath('№44-2026')).toBe(`/contracts/${encodeURIComponent('№44-2026')}`);
    expect(contractPath('a?b#c')).toBe('/contracts/a%3Fb%23c');
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
