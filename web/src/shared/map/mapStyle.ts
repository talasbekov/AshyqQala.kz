// Стиль карты MapLibre (Story 1.8). По умолчанию — НЕЙТРАЛЬНЫЙ ФОН (один background-слой), что
// допускает AC1 («PMTiles-подложка ИЛИ нейтральный фон»). Когда статический Astana-PMTiles-ассет
// (готовая Protomaps-сборка через Caddy) появится — передать basemapPmtilesUrl, и подложка
// подключится через зарегистрированный pmtiles-протокол (см. shared/map/pmtilesProtocol).
// Цвет фона приходит из JS-моста к semantic-токену (карта — единственный UI вне CSS, architecture.md).
import type { StyleSpecification } from 'maplibre-gl';

export interface MapStyleOpts {
  backgroundColor: string;
  // Опц.: URL статического .pmtiles (напр. '/tiles/astana.pmtiles' — раздаётся Caddy). S-0.5/ops.
  basemapPmtilesUrl?: string;
  attribution?: string;
}

export function buildMapStyle(opts: MapStyleOpts): StyleSpecification {
  const style: StyleSpecification = {
    version: 8,
    sources: {},
    layers: [
      {
        id: 'aq-neutral-bg',
        type: 'background',
        paint: { 'background-color': opts.backgroundColor },
      },
    ],
  };

  if (opts.basemapPmtilesUrl) {
    style.sources['aq-basemap'] = {
      type: 'raster',
      url: `pmtiles://${opts.basemapPmtilesUrl}`,
      attribution: opts.attribution ?? '',
    };
    style.layers.push({ id: 'aq-basemap-raster', type: 'raster', source: 'aq-basemap' });
  }

  return style;
}
