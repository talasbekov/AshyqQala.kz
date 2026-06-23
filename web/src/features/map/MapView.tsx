import { useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import maplibregl from 'maplibre-gl';
import 'maplibre-gl/dist/maplibre-gl.css';
import i18n from '../../shared/i18n';
import type { Lang } from '../../shared/i18n';
import { Icon } from '../../shared/ui/Icon';
import { registerPmtilesProtocol } from '../../shared/map/pmtilesProtocol';
import { buildMapStyle } from '../../shared/map/mapStyle';
import { mapToken } from '../../shared/map/tokenBridge';
import {
  ASTANA_POINTS,
  UNGEOCODED_CONTRACTS,
  ASTANA_CENTER,
  ASTANA_ZOOM,
  toLngLat,
} from './astanaPoints';
import { MapPreviewSheet } from './MapPreviewSheet';
import './map.css';

// prefers-reduced-motion (UX): мгновенный зум/recenter без анимации.
const PREFERS_REDUCED_MOTION =
  typeof window !== 'undefined' &&
  typeof window.matchMedia === 'function' &&
  window.matchMedia('(prefers-reduced-motion: reduce)').matches;

// Опц. хук подложки: URL статического Astana-PMTiles (готовая Protomaps-сборка через Caddy, S-0.5).
// Пока ассета нет — undefined → нейтральный фон (AC1 это допускает). Подложка включается без правок
// downstream: выставить URL, и зарегистрированный pmtiles-протокол отдаст тайлы.
const BASEMAP_PMTILES_URL: string | undefined = undefined;

const ASTANA_CENTER_LNGLAT: [number, number] = [ASTANA_CENTER[0], ASTANA_CENTER[1]];

export function MapView() {
  const { t } = useTranslation('chrome');
  const lang: Lang = i18n.language.startsWith('ru') ? 'ru' : 'kk';

  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<maplibregl.Map | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [selected, setSelected] = useState<string | null>(null);
  const [failed, setFailed] = useState(false);

  // Инициализация карты ОДИН раз. useRef-guard от двойного new Map() под React StrictMode
  // (architecture.md; заложено в Story 1.7 именно под эту историю). Пустые deps — эффект на маунт;
  // i18n внутри — через imperative i18n.t + подписку languageChanged (маркеры вне React-дерева).
  useEffect(() => {
    if (mapRef.current || !containerRef.current) return;
    registerPmtilesProtocol();

    let map: maplibregl.Map;
    try {
      map = new maplibregl.Map({
        container: containerRef.current,
        style: buildMapStyle({
          backgroundColor: mapToken('surface-sunken', '#EEF6F9'),
          basemapPmtilesUrl: BASEMAP_PMTILES_URL,
          attribution: BASEMAP_PMTILES_URL ? i18n.t('map.attribution') : undefined,
        }),
        center: ASTANA_CENTER_LNGLAT,
        zoom: ASTANA_ZOOM,
        attributionControl: BASEMAP_PMTILES_URL ? undefined : false,
        fadeDuration: PREFERS_REDUCED_MOTION ? 0 : 300,
      });
    } catch (err) {
      // Карта не инициализировалась (напр. нет WebGL) → ЧЕСТНОЕ состояние, не вечная плашка «загружается».
      console.error('MapLibre init failed', err);
      setFailed(true);
      return;
    }
    mapRef.current = map;

    let loadedOk = false;
    const markers: maplibregl.Marker[] = [];

    // Маркеры/попапы вне React-дерева → i18n императивно + подписка на languageChanged (architecture.md).
    const applyMarkerLabels = () => {
      markers.forEach((m, idx) => {
        m.getElement().setAttribute(
          'aria-label',
          i18n.t('map.marker_label', { id: ASTANA_POINTS[idx].goszakupId }),
        );
      });
    };

    map.on('load', () => {
      loadedOk = true;
      setLoaded(true);
      // Маркеры появляются ПОСЛЕ тайлов/канваса (прогрессивная отрисовка, AC1).
      ASTANA_POINTS.forEach((p) => {
        const el = document.createElement('button');
        el.type = 'button';
        el.className = 'aq-map-marker';
        const pin = document.createElement('span');
        pin.className = 'aq-map-marker__pin';
        el.appendChild(pin);
        el.addEventListener('click', () => setSelected(p.goszakupId));
        markers.push(
          new maplibregl.Marker({ element: el, anchor: 'bottom' })
            .setLngLat(toLngLat(p))
            .addTo(map),
        );
      });
      applyMarkerLabels();
    });

    // Карта упала до 'load' (нет WebGL и т.п.) → честное состояние вместо вечной «загрузки».
    map.on('error', (ev) => {
      console.error('MapLibre error', ev);
      if (!loadedOk) setFailed(true);
    });

    i18n.on('languageChanged', applyMarkerLabels);

    return () => {
      i18n.off('languageChanged', applyMarkerLabels);
      markers.forEach((m) => m.remove());
      map.remove();
      mapRef.current = null;
    };
  }, []);

  const zoomIn = () => mapRef.current?.zoomIn();
  const zoomOut = () => mapRef.current?.zoomOut();
  const recenter = () => {
    const target = { center: ASTANA_CENTER_LNGLAT, zoom: ASTANA_ZOOM };
    if (PREFERS_REDUCED_MOTION) mapRef.current?.jumpTo(target);
    else mapRef.current?.flyTo(target);
  };

  return (
    <section className="aq-map-route">
      <h1 className="aq-map-route__title">{t('map.title')}</h1>

      <div className="aq-map-wrap">
        <div
          ref={containerRef}
          className="aq-map"
          role="application"
          aria-label={t('map.aria_label')}
        />
        {failed ? (
          <div className="aq-map__unavailable" role="status">
            {t('map.unavailable')}
          </div>
        ) : (
          !loaded && (
            <div className="aq-map__skeleton" aria-hidden="true">
              {t('map.loading')}
            </div>
          )
        )}
        {!failed && (
          <div className="aq-map__controls">
            <button
              type="button"
              className="aq-map__control"
              onClick={zoomIn}
              aria-label={t('map.zoom_in')}
            >
              +
            </button>
            <button
              type="button"
              className="aq-map__control"
              onClick={zoomOut}
              aria-label={t('map.zoom_out')}
            >
              −
            </button>
            <button
              type="button"
              className="aq-map__control"
              onClick={recenter}
              aria-label={t('map.recenter')}
            >
              ⌖
            </button>
          </div>
        )}
      </div>

      {UNGEOCODED_CONTRACTS.length > 0 && (
        <section aria-labelledby="aq-ungeo-heading">
          <h2 id="aq-ungeo-heading" className="aq-ungeocoded__heading">
            {t('map.ungeocoded_heading')}
          </h2>
          <ul className="aq-ungeocoded">
            {UNGEOCODED_CONTRACTS.map((u) => (
              <li key={u.goszakupId} className="aq-ungeocoded__item">
                <Icon name="ungeocoded" />
                <span>{t('map.ungeocoded_label')}</span>
                <Link
                  className="aq-ungeocoded__link"
                  to={`/contracts/${encodeURIComponent(u.goszakupId)}`}
                >
                  {u.goszakupId}
                </Link>
              </li>
            ))}
          </ul>
        </section>
      )}

      {selected && (
        <MapPreviewSheet goszakupId={selected} lang={lang} onClose={() => setSelected(null)} />
      )}
    </section>
  );
}
