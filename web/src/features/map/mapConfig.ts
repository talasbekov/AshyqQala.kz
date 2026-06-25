// Константы вида Астаны для ранней карты (Story 0.8). Точки приходят из /api/lots (НЕ хардкод —
// Story 1.8 рисовала 3 захардкоженные демо-точки; 0.8 заменила их реальными scraped-лотами).
// Порядок координат — [lon, lat] (GeoJSON RFC7946; НЕ [lat,lon]) — несущий инвариант проекта.

// Центр/зум Астаны для recenter (в спеках численно не задан — выставлен под viewbox Астаны).
export const ASTANA_CENTER: readonly [number, number] = [71.43, 51.13];
export const ASTANA_ZOOM = 11;

// toLngLat — координаты в порядке [lon, lat] (для MapLibre setLngLat / GeoJSON).
export function toLngLat(p: { lon: number; lat: number }): [number, number] {
  return [p.lon, p.lat];
}
