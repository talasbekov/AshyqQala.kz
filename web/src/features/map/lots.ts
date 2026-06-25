import { useQuery } from '@tanstack/react-query';
import type { components } from '../../shared/api/schema.gen';
import { formatMoney } from '../../shared/i18n/format';
import type { Lang } from '../../shared/i18n';

// ⏳ ИНТЕРИМ (Story 0.8, трек «Парсер-мост»): загрузка реальных scraped-лотов Астаны для ранней карты.
// Заменяет хардкод 1.8 (3 демо-точки). ТОЛЬКО лоты — карточки/флаги/медианы ждут токен ows_v2.
export type MapLot = components['schemas']['MapLot'];
export type ApiError = components['schemas']['Error'];

export interface LotsFetchError extends Error {
  status: number;
  code: string;
}

// fetchLots — голый fetch к /api/lots (паттерн fetchContract, Story 1.3). На !ok — честный код.
export async function fetchLots(): Promise<MapLot[]> {
  const res = await fetch('/api/lots');
  if (!res.ok) {
    let code = 'INTERNAL';
    try {
      const body = (await res.json()) as ApiError;
      code = body.error?.code ?? code;
    } catch {
      // тело не JSON — оставляем INTERNAL
    }
    const err = new Error(`lots fetch failed: ${res.status} ${code}`) as LotsFetchError;
    err.status = res.status;
    err.code = code;
    throw err;
  }
  return (await res.json()) as MapLot[];
}

// useLots — Query владеет загрузкой (loading/error/success); Router-loader НЕ фетчит (конвенция 1.7).
export function useLots() {
  return useQuery({ queryKey: ['lots'], queryFn: fetchLots });
}

// GeoLot — лот с подтверждённой координатой → точка на карте.
export interface GeoLot {
  lot: MapLot;
  lon: number;
  lat: number;
}

// splitLots — гео-объекты (точка) ⊥ негеокодированные (честно вне карты, AC3). Гардрейл честности:
// точка ТОЛЬКО при geocode_state='ok' И обеих координатах непустых — координату из null НЕ выдумываем.
export function splitLots(lots: MapLot[]): { points: GeoLot[]; ungeocoded: MapLot[] } {
  const points: GeoLot[] = [];
  const ungeocoded: MapLot[] = [];
  for (const lot of lots) {
    if (lot.geocode_state === 'ok' && lot.lon !== null && lot.lat !== null) {
      points.push({ lot, lon: lot.lon, lat: lot.lat });
    } else {
      // P5: geocode_state='ok' без координаты — нарушение контракта источника (геокод заявлен,
      // координаты нет). Точку НЕ выдумываем (остаётся вне карты), но аномалию делаем видимой,
      // а не глотаем молча — честный сигнал в консоль для последующего разбора.
      if (lot.geocode_state === 'ok') {
        console.warn(
          `splitLots: лот ${lot.goszakup_lot_id} имеет geocode_state='ok', но lon/lat пусты — ` +
            'нарушение контракта источника; лот оставлен вне карты (координата не выдумывается)',
        );
      }
      ungeocoded.push(lot);
    }
  }
  return { points, ungeocoded };
}

// safeFormatMoney — безопасная обёртка над formatMoney (P1). formatMoney бросает на неканоничной
// строке (^-?\d+$) — без обёртки одна «грязная» сумма роняла бы весь роут карты в RouteError.
// Контракт честности: невалидная/непредставимая сумма → null (вызывающий рендерит «нет данных»),
// НИКОГДА не «0 ₸» (фабрикация). Возвращает отформатированную строку либо null.
export function safeFormatMoney(amountTng: string, lang: Lang): string | null {
  try {
    return formatMoney(amountTng, lang);
  } catch {
    // formatMoney бросил (неканоничная строка) — честно сигналим «нет данных» вместо краха роута.
    return null;
  }
}
