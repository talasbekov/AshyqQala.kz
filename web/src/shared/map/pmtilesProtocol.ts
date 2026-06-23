// Регистрация PMTiles-протокола в MapLibre (Story 1.8, AC1). Идемпотентна: повторные маунты карты
// под React StrictMode НЕ дублируют протокол. Готовит подложку «статический .pmtiles через Caddy»
// (architecture.md AR-23) — фактический ассет подключается через mapStyle(basemapPmtilesUrl).
import maplibregl from 'maplibre-gl';
import { Protocol } from 'pmtiles';

let registered = false;

export function registerPmtilesProtocol(): void {
  if (registered) return;
  const protocol = new Protocol();
  maplibregl.addProtocol('pmtiles', protocol.tile);
  registered = true;
}
