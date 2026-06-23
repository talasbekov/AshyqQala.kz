// Три захардкоженные демо-точки Астаны (Story 1.8, AC1). Гео-независимый скелет: координаты заданы
// ВРУЧНУЮ (в API гео-полей нет — Story 1.3). Полные «3 читаемых контракта-истории» — Story 1.9.
// Порядок координат — [lon, lat] (GeoJSON RFC7946; НЕ [lat,lon]) — несущий инвариант проекта.

export interface GeoPoint {
  goszakupId: string;
  lon: number;
  lat: number;
}

export interface UngeocodedItem {
  goszakupId: string;
  // Честное value_state (Story 1.4): геопривязка выполняется/не удалась — НЕ выдуманная координата.
  geocodeState: 'geocode_pending' | 'geocode_failed';
}

// Демо-координаты в пределах viewbox Астаны (lon 71.20–71.78, lat 51.00–51.30 — stage0-audit/config.go).
export const ASTANA_POINTS: readonly GeoPoint[] = [
  { goszakupId: 'DEMO-0001', lon: 71.4306, lat: 51.1281 },
  { goszakupId: 'DEMO-0002', lon: 71.4044, lat: 51.0905 },
  { goszakupId: 'DEMO-0003', lon: 71.455, lat: 51.169 },
];

// Негеокодированный контракт (AC3): честно ВНЕ карты, виден в списке с меткой «без точки на карте».
export const UNGEOCODED_CONTRACTS: readonly UngeocodedItem[] = [
  { goszakupId: 'DEMO-0004', geocodeState: 'geocode_failed' },
];

// Центр/зум Астаны для recenter (в спеках численно не задан — выставлен под три точки).
export const ASTANA_CENTER: readonly [number, number] = [71.43, 51.13];
export const ASTANA_ZOOM = 11;

// toLngLat — координаты маркера в порядке [lon, lat] (для MapLibre setLngLat / GeoJSON).
export function toLngLat(p: GeoPoint): [number, number] {
  return [p.lon, p.lat];
}
