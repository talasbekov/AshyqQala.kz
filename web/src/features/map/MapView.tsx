import { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';
import maplibregl from 'maplibre-gl';
import 'maplibre-gl/dist/maplibre-gl.css';
import Supercluster from 'supercluster';
import i18n from '../../shared/i18n';
import { Icon } from '../../shared/ui/Icon';
import { registerPmtilesProtocol } from '../../shared/map/pmtilesProtocol';
import { buildMapStyle } from '../../shared/map/mapStyle';
import { mapToken } from '../../shared/map/tokenBridge';
import { ASTANA_CENTER, ASTANA_ZOOM } from './mapConfig';
import {
  useMapObjects,
  splitObjects,
  markerKind,
  boundsToBBox,
  debounce,
  type MarkerKind,
} from './objects';
import './map.css';

// prefers-reduced-motion (UX): мгновенный зум/recenter/раскрытие кластера без анимации.
const PREFERS_REDUCED_MOTION =
  typeof window !== 'undefined' &&
  typeof window.matchMedia === 'function' &&
  window.matchMedia('(prefers-reduced-motion: reduce)').matches;

// Опц. хук подложки: URL статического Astana-PMTiles (готовая Protomaps-сборка через Caddy, S-0.5).
// Пока ассета нет — undefined → нейтральный фон (AC1 это допускает). Подложка включается без правок
// downstream: выставить URL, и зарегистрированный pmtiles-протокол отдаст тайлы.
const BASEMAP_PMTILES_URL: string | undefined = undefined;

const ASTANA_CENTER_LNGLAT: [number, number] = [ASTANA_CENTER[0], ASTANA_CENTER[1]];

// Supercluster-параметры (AC1, epics.md:1288): radius≈50px, кластеризация до зума 15.
const CLUSTER_RADIUS = 50;
const CLUSTER_MAX_ZOOM = 15;
// Хит-толеранс тапа по линии ≈ tap-target/2 (EXPERIENCE.md {map-line}).
const LINE_HIT_PX = 22;
// Debounce рефетча bbox на панораме (D6).
const BBOX_DEBOUNCE_MS = 300;

// Пропсы точки в supercluster-индексе и агрегат кластера: флаг НЕ теряется при свёртке (AC2).
interface PointProps {
  idx: number;
  hasFlag: boolean;
  [key: string]: unknown;
}
interface ClusterProps {
  hasFlag: boolean;
  [key: string]: unknown;
}

// Вид окна для локальной рекластеризации (мгновенно на moveend, без debounce — сеть не трогает).
interface ViewState {
  zoom: number;
  bounds: [number, number, number, number]; // [west, south, east, north]
}

// aria-метка маркера: контрактный id, если есть; иначе public_id (честно, не выдумываем).
const markerAriaKey: Record<MarkerKind, string> = {
  plain: 'map.marker_label',
  flagged: 'map.marker_label_flag',
  confirmed: 'map.marker_label_confirmed',
};

interface MarkerRef {
  el: HTMLButtonElement;
  publicId: string | null; // null у кластера — selected-класс не применяется
  labelKey: string;
  labelParams: Record<string, string | number>;
}

export function MapView() {
  const { t } = useTranslation('chrome');

  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<maplibregl.Map | null>(null);
  const markerRefs = useRef<MarkerRef[]>([]);
  const prevSelectedLine = useRef<string | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [failed, setFailed] = useState(false);
  const [selected, setSelected] = useState<string | null>(null); // public_id объекта (маркер или линия)
  const [bbox, setBbox] = useState<string | null>(null); // окно для API (debounced)
  const [view, setView] = useState<ViewState | null>(null); // окно для рекластеризации (мгновенно)

  // Канонические geo_objects в bbox (Story 3.4; заменили интерим-лоты 0.8 — D5).
  const objectsQuery = useMapObjects(bbox);
  const { points, lines } = useMemo(
    () => splitObjects(objectsQuery.data?.items ?? []),
    [objectsQuery.data],
  );

  // Supercluster-индекс пересобирается на каждый ответ API (D6). map/reduce протаскивают
  // «есть флаг» в свёрнутый кластер — амбер-кольцо не теряет сигнал (AC2).
  const clusterIndex = useMemo(() => {
    const sc = new Supercluster<PointProps, ClusterProps>({
      radius: CLUSTER_RADIUS,
      maxZoom: CLUSTER_MAX_ZOOM,
      map: (props) => ({ hasFlag: props.hasFlag }),
      reduce: (acc, props) => {
        acc.hasFlag = acc.hasFlag || props.hasFlag;
      },
    });
    sc.load(
      points.map((p, idx) => ({
        type: 'Feature' as const,
        geometry: { type: 'Point' as const, coordinates: [p.lon, p.lat] },
        properties: { idx, hasFlag: p.obj.has_active_flag },
      })),
    );
    return sc;
  }, [points]);

  // Инициализация карты ОДИН раз. useRef-guard от двойного new Map() под React StrictMode.
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
      // Карта не инициализировалась (напр. нет WebGL) → ЧЕСТНОЕ состояние, не вечная «загрузка».
      console.error('MapLibre init failed', err);
      setFailed(true);
      return;
    }
    mapRef.current = map;
    // Тест-шов e2e (Task 6: «линия отрисована» — canvas-слой недоступен DOM-локаторам Playwright):
    // инстанс карты наружу; убирается в teardown. Продукт-код его не читает.
    (window as { __aqMapTest?: maplibregl.Map }).__aqMapTest = map;

    const applyView = () => {
      const b = map.getBounds();
      setView({
        zoom: map.getZoom(),
        bounds: [b.getWest(), b.getSouth(), b.getEast(), b.getNorth()],
      });
    };
    // bbox для API — debounced (панорама не бомбит сеть); рекластеризация — мгновенная.
    // null (вырожденное после clamp окно, напр. пан к антимеридиану) НЕ затирает рабочее окно:
    // запрос по старому bbox остаётся, его объекты geографически вне вида — карта не пустеет молча.
    const applyBbox = () => {
      const s = boundsToBBox(map.getBounds());
      if (s !== null) setBbox(s);
    };
    const debouncedBbox = debounce(applyBbox, BBOX_DEBOUNCE_MS);

    let loadedOk = false;
    map.on('load', () => {
      loadedOk = true;
      setLoaded(true);
      applyView();
      applyBbox(); // первое окно — сразу, без debounce (NFR-1)
    });
    map.on('moveend', () => {
      applyView();
      debouncedBbox();
    });
    // Тап по линии (canvas): queryRenderedFeatures с падом ≈ tap-target/2. Тап по пустому месту —
    // снятие выбора. Маркеры (DOM) сами останавливают всплытие — сюда не долетают.
    map.on('click', (e) => {
      if (!map.getLayer('aq-geo-line')) {
        setSelected(null);
        return;
      }
      const feats = map.queryRenderedFeatures(
        [
          [e.point.x - LINE_HIT_PX, e.point.y - LINE_HIT_PX],
          [e.point.x + LINE_HIT_PX, e.point.y + LINE_HIT_PX],
        ],
        { layers: ['aq-geo-line'] },
      );
      setSelected(feats.length > 0 && feats[0].id != null ? String(feats[0].id) : null);
    });
    // Карта упала до 'load' (нет WebGL и т.п.) → честное состояние вместо вечной «загрузки».
    map.on('error', (ev) => {
      console.error('MapLibre error', ev);
      if (!loadedOk) setFailed(true);
    });

    return () => {
      debouncedBbox.cancel();
      delete (window as { __aqMapTest?: maplibregl.Map }).__aqMapTest;
      map.remove();
      mapRef.current = null;
    };
  }, []);

  // Слой линий дорог (LINESTRING — DOM полилинии не умеет): GeoJSON source + casing/line.
  // Цвета — через tokenBridge (canvas не читает CSS-переменные); выбранная линия — accent (feature-state).
  useEffect(() => {
    const map = mapRef.current;
    if (!map || !loaded) return;
    const data = {
      type: 'FeatureCollection' as const,
      features: lines.map((l) => ({
        type: 'Feature' as const,
        geometry: { type: 'LineString' as const, coordinates: l.coordinates },
        properties: { public_id: l.obj.public_id },
      })),
    };
    const src = map.getSource('aq-geo-lines') as maplibregl.GeoJSONSource | undefined;
    if (src) {
      src.setData(data);
      return;
    }
    map.addSource('aq-geo-lines', { type: 'geojson', data, promoteId: 'public_id' });
    map.addLayer({
      id: 'aq-geo-line-casing',
      type: 'line',
      source: 'aq-geo-lines',
      layout: { 'line-cap': 'round', 'line-join': 'round' },
      paint: { 'line-color': mapToken('surface', '#FFFFFF'), 'line-width': 9 },
    });
    map.addLayer({
      id: 'aq-geo-line',
      type: 'line',
      source: 'aq-geo-lines',
      layout: { 'line-cap': 'round', 'line-join': 'round' },
      paint: {
        'line-color': [
          'case',
          ['boolean', ['feature-state', 'selected'], false],
          mapToken('accent', '#F0B429'),
          mapToken('primary', '#1496C0'),
        ],
        'line-width': 6,
        'line-dasharray': [2, 1.5],
      },
    });
  }, [loaded, lines]);

  // Маркеры и кластеры (DOM поверх canvas — прямой задел a11y 3.8): пересобираются на смену данных,
  // индекса или окна (рекластеризация на каждом уровне зума — UX {map-cluster}).
  useEffect(() => {
    const map = mapRef.current;
    if (!map || !loaded || !view) return;
    const markers: maplibregl.Marker[] = [];
    const refs: MarkerRef[] = [];
    const clusters = clusterIndex.getClusters(view.bounds, Math.floor(view.zoom));

    for (const f of clusters) {
      const [lon, lat] = f.geometry.coordinates;
      if (f.properties.cluster) {
        // --- Кластер: круг со счётчиком; при флаг-объектах внутри — амбер-кольцо + «!»-точка (AC2).
        const clusterId = f.properties.cluster_id as number;
        const count = f.properties.point_count as number;
        const hasFlag = (f.properties as unknown as ClusterProps).hasFlag === true;
        const el = document.createElement('button');
        el.type = 'button';
        el.className = hasFlag ? 'aq-map-cluster aq-map-cluster--flag' : 'aq-map-cluster';
        const labelKey = hasFlag ? 'map.cluster_label_flag' : 'map.cluster_label';
        el.setAttribute('aria-label', t(labelKey, { n: count }));
        const cnt = document.createElement('span');
        cnt.className = 'aq-map-cluster__count';
        cnt.textContent = String(count);
        el.appendChild(cnt);
        if (hasFlag) {
          const dot = document.createElement('span');
          dot.className = 'aq-map-cluster__dot';
          dot.textContent = '!';
          dot.setAttribute('aria-hidden', 'true');
          el.appendChild(dot);
        }
        el.addEventListener('click', (ev) => {
          ev.stopPropagation();
          // Плавный зум к раскрытию (AC2); мгновенно при prefers-reduced-motion.
          const zoom = Math.min(clusterIndex.getClusterExpansionZoom(clusterId), 20);
          const target = { center: [lon, lat] as [number, number], zoom };
          if (PREFERS_REDUCED_MOTION) map.jumpTo(target);
          else map.easeTo(target);
        });
        markers.push(
          new maplibregl.Marker({ element: el, anchor: 'center' }).setLngLat([lon, lat]).addTo(map),
        );
        refs.push({ el, publicId: null, labelKey, labelParams: { n: count } });
      } else {
        // --- Одиночный маркер: статус несёт ФОРМА/ГЛИФ (пустой/«!»/«✓»), цвет вторичен (WCAG 1.4.1).
        const p = points[(f.properties as PointProps).idx];
        if (!p) continue;
        const kind = markerKind(p.obj);
        const labelId = p.obj.goszakup_contract_id ?? p.obj.public_id;
        const el = document.createElement('button');
        el.type = 'button';
        el.className = [
          'aq-map-marker',
          kind === 'flagged' ? 'aq-map-marker--flagged' : '',
          kind === 'confirmed' ? 'aq-map-marker--confirmed' : '',
        ]
          .filter(Boolean)
          .join(' ');
        el.setAttribute('aria-label', t(markerAriaKey[kind], { id: labelId }));
        const pin = document.createElement('span');
        pin.className = 'aq-map-marker__pin';
        el.appendChild(pin);
        if (kind !== 'plain') {
          const glyph = document.createElement('span');
          glyph.className = 'aq-map-marker__glyph';
          glyph.textContent = kind === 'flagged' ? '!' : '✓';
          glyph.setAttribute('aria-hidden', 'true');
          el.appendChild(glyph);
        }
        el.addEventListener('click', (ev) => {
          ev.stopPropagation(); // не долетает до map click (снятие выбора)
          setSelected(p.obj.public_id);
        });
        markers.push(
          new maplibregl.Marker({ element: el, anchor: 'bottom' })
            .setLngLat([p.lon, p.lat])
            .addTo(map),
        );
        refs.push({
          el,
          publicId: p.obj.public_id,
          labelKey: markerAriaKey[kind],
          labelParams: { id: labelId },
        });
      }
    }
    markerRefs.current = refs;
    return () => {
      markers.forEach((m) => m.remove());
      markerRefs.current = [];
    };
    // t намеренно НЕ в deps — смена языка переписывает aria-label существующих маркеров ниже.
  }, [loaded, view, points, clusterIndex]);

  // Висячий selected (код-ревью 3.4): объект ушёл из СВЕЖЕЙ выдачи (пан/refetch) — сброс, иначе
  // выделение «воскресает» при возврате объекта в окно. Placeholder-данные не считаются
  // (keepPreviousData: во время панорамы points/lines стары — сбрасывать по ним рано).
  useEffect(() => {
    if (selected === null || objectsQuery.data == null || objectsQuery.isPlaceholderData) return;
    const alive =
      points.some((p) => p.obj.public_id === selected) ||
      lines.some((l) => l.obj.public_id === selected);
    if (!alive) setSelected(null);
  }, [selected, points, lines, objectsQuery.data, objectsQuery.isPlaceholderData]);

  // Выбранный маркер: класс --selected (двойное кольцо, z 3) БЕЗ пересоздания маркеров.
  // Объявлен ПОСЛЕ эффекта маркеров: на общую смену deps рефы уже пересобраны.
  useEffect(() => {
    markerRefs.current.forEach(({ el, publicId }) => {
      if (publicId === null) return; // кластер не бывает «выбран»
      el.classList.toggle('aq-map-marker--selected', publicId === selected);
    });
  }, [selected, loaded, view, points, clusterIndex]);

  // Выбранная линия: feature-state → золотой stroke (canvas-аналог --selected).
  useEffect(() => {
    const map = mapRef.current;
    if (!map || !loaded || !map.getSource('aq-geo-lines')) return;
    if (prevSelectedLine.current !== null) {
      map.setFeatureState(
        { source: 'aq-geo-lines', id: prevSelectedLine.current },
        { selected: false },
      );
      prevSelectedLine.current = null;
    }
    if (selected !== null && lines.some((l) => l.obj.public_id === selected)) {
      map.setFeatureState({ source: 'aq-geo-lines', id: selected }, { selected: true });
      prevSelectedLine.current = selected;
    }
  }, [loaded, selected, lines]);

  // Смена языка: переписать aria-label СУЩЕСТВУЮЩИХ маркеров/кластеров без пересоздания (паттерн P3/0.8).
  useEffect(() => {
    markerRefs.current.forEach(({ el, labelKey, labelParams }) => {
      el.setAttribute('aria-label', t(labelKey, labelParams));
    });
  }, [t]);

  const zoomIn = () => mapRef.current?.zoomIn();
  const zoomOut = () => mapRef.current?.zoomOut();
  const recenter = () => {
    const target = { center: ASTANA_CENTER_LNGLAT, zoom: ASTANA_ZOOM };
    if (PREFERS_REDUCED_MOTION) mapRef.current?.jumpTo(target);
    else mapRef.current?.flyTo(target);
  };

  const data = objectsQuery.data;

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

      {/* Честные состояния данных (AR-16/17): ошибка/пустота/усечение — видимой строкой, не «всё чисто».
          !isPlaceholderData (код-ревью 3.4): во время панорамы data — keepPreviousData СТАРОГО окна;
          выдавать его пустоту/усечение за состояние текущего окна — нечестно. */}
      {objectsQuery.isError && (
        <p className="aq-map-note" role="alert">
          {t('map.objects_error')}
        </p>
      )}
      {!objectsQuery.isPending &&
        !objectsQuery.isError &&
        !objectsQuery.isPlaceholderData &&
        data &&
        data.items.length === 0 && (
          <p className="aq-map-note" role="status">
            {t('map.objects_empty')}
          </p>
        )}
      {!objectsQuery.isPlaceholderData && data?.truncated && (
        <p className="aq-map-note" role="status">
          {t('map.truncated_notice')}
        </p>
      )}

      {/* AC3: аффорданса-счётчик «Ещё N объектов без точки на карте → открыть списком» (вся Астана).
          Их видимость на карте — этот счётчик: маркера нет структурно (geom NULL ⟺ unmatched). */}
      {data != null && data.ungeocoded_count > 0 && (
        <p className="aq-map-note aq-map-ungeocoded-more">
          <Icon name="ungeocoded" />
          <Link className="aq-ungeocoded__link" to="/search">
            {t('map.ungeocoded_more', { n: data.ungeocoded_count })}
          </Link>
        </p>
      )}
    </section>
  );
}
