import { describe, it, expect, vi, afterEach } from 'vitest';
import { fetchLots, splitLots, safeFormatMoney } from './lots';
import type { MapLot } from './lots';

const g = globalThis as unknown as { fetch?: unknown };
afterEach(() => {
  delete g.fetch;
});

// lot — фабрика честного лота (по умолчанию негеокодированный, поля no_data).
function lot(partial: Partial<MapLot> & { goszakup_lot_id: string }): MapLot {
  return {
    subject_ru: { value: null, state: 'no_data' },
    subject_kk: { value: null, state: 'no_data' },
    amount_tng: { value: null, state: 'no_data' },
    lon: null,
    lat: null,
    geocode_state: 'geocode_pending',
    ...partial,
  };
}

describe('fetchLots', () => {
  it('возвращает массив лотов при 200', async () => {
    g.fetch = vi.fn().mockResolvedValue({
      ok: true,
      json: () => Promise.resolve([{ goszakup_lot_id: 'scrape-1' }]),
    });
    const lots = await fetchLots();
    expect(lots).toHaveLength(1);
    expect(lots[0].goszakup_lot_id).toBe('scrape-1');
  });

  it('бросает с честным кодом при !ok', async () => {
    g.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      json: () => Promise.resolve({ error: { code: 'INTERNAL', message: 'x' } }),
    });
    await expect(fetchLots()).rejects.toMatchObject({ status: 500, code: 'INTERNAL' });
  });

  it('INTERNAL при не-JSON теле ошибки', async () => {
    g.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 503,
      json: () => Promise.reject(new Error('not json')),
    });
    await expect(fetchLots()).rejects.toMatchObject({ status: 503, code: 'INTERNAL' });
  });
});

describe('splitLots', () => {
  it('matched (ok + координаты) → точка в порядке [lon,lat]; остальные → негео', () => {
    const lots: MapLot[] = [
      lot({ goszakup_lot_id: 'g', geocode_state: 'ok', lon: 71.43, lat: 51.13 }),
      lot({ goszakup_lot_id: 'f', geocode_state: 'geocode_failed' }),
      lot({ goszakup_lot_id: 'p', geocode_state: 'geocode_pending' }),
    ];
    const { points, ungeocoded } = splitLots(lots);
    expect(points).toHaveLength(1);
    expect(points[0].lon).toBe(71.43); // [lon,lat] — несущий инвариант
    expect(points[0].lat).toBe(51.13);
    expect(points[0].lot.goszakup_lot_id).toBe('g');
    expect(ungeocoded.map((l) => l.goszakup_lot_id)).toEqual(['f', 'p']);
  });

  it('ok без координат (защита от выдумки) → НЕ точка, честно вне карты', () => {
    const lots: MapLot[] = [
      lot({ goszakup_lot_id: 'x', geocode_state: 'ok', lon: null, lat: null }),
    ];
    const { points, ungeocoded } = splitLots(lots);
    expect(points).toHaveLength(0);
    expect(ungeocoded).toHaveLength(1);
  });

  it('пустой вход → пустые группы (честная деградация контейнера)', () => {
    const { points, ungeocoded } = splitLots([]);
    expect(points).toHaveLength(0);
    expect(ungeocoded).toHaveLength(0);
  });

  // P5: geocode_state='ok' без координат — нарушение контракта источника; точку не выдумываем,
  // но аномалию делаем видимой (console.warn), а не глотаем молча.
  it('ok без координат → console.warn (видимый сигнал нарушения контракта), а не молчаливый дроп', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
    try {
      splitLots([lot({ goszakup_lot_id: 'broken', geocode_state: 'ok', lon: null, lat: null })]);
      expect(warn).toHaveBeenCalledTimes(1);
      expect(warn.mock.calls[0][0]).toContain('broken');
    } finally {
      warn.mockRestore();
    }
  });

  it('честный (geocode_failed/pending) лот без координат НЕ шумит в console.warn', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
    try {
      splitLots([
        lot({ goszakup_lot_id: 'f', geocode_state: 'geocode_failed' }),
        lot({ goszakup_lot_id: 'p', geocode_state: 'geocode_pending' }),
      ]);
      expect(warn).not.toHaveBeenCalled();
    } finally {
      warn.mockRestore();
    }
  });
});

// P1: safeFormatMoney — обёртка, на которой держится защита роута карты. Неканоничная/бросающая сумма
// НЕ должна ронять весь роут (RouteError); вместо этого → null, который превью-лист рендерит
// честным «нет данных» (НИКОГДА «0 ₸»). Тест доказывает: throw из formatMoney не пробрасывается.
describe('safeFormatMoney (P1 — защита роута от падения formatMoney)', () => {
  it('каноничная целая строка → форматирует, не бросает', () => {
    expect(safeFormatMoney('240000000', 'ru')).toContain('₸');
  });

  it('неканоничная строка (formatMoney бросил бы) → null, БЕЗ throw', () => {
    // Эти входы у formatMoney бросают (^-?\d+$ не матчится). safeFormatMoney обязан их проглотить → null.
    expect(safeFormatMoney('1 200,50', 'ru')).toBeNull();
    expect(safeFormatMoney('0x10', 'ru')).toBeNull();
    expect(safeFormatMoney('', 'ru')).toBeNull();
    expect(safeFormatMoney('нет данных', 'ru')).toBeNull();
  });

  it('не бросает наружу ни при каком входе (инвариант устойчивости роута)', () => {
    expect(() => safeFormatMoney('???', 'kk')).not.toThrow();
  });
});
