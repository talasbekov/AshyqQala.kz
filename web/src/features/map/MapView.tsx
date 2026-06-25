import { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import maplibregl from 'maplibre-gl';
import 'maplibre-gl/dist/maplibre-gl.css';
import i18n from '../../shared/i18n';
import type { Lang } from '../../shared/i18n';
import { Icon } from '../../shared/ui/Icon';
import { registerPmtilesProtocol } from '../../shared/map/pmtilesProtocol';
import { buildMapStyle } from '../../shared/map/mapStyle';
import { mapToken } from '../../shared/map/tokenBridge';
import { ASTANA_CENTER, ASTANA_ZOOM, toLngLat } from './mapConfig';
import { useLots, splitLots } from './lots';
import type { MapLot } from './lots';
import { MapStatePlaque } from './MapStatePlaque';
import { LotPreviewSheet } from './LotPreviewSheet';
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
  // P3: держим ссылки на созданные маркеры + их DOM-элемент и id лота, чтобы эффект смены языка
  // мог переписать aria-label СУЩЕСТВУЮЩИХ маркеров без их пересоздания (раньше lang в deps рушил
  // и заново строил все маркеры на каждую смену языка/refetch).
  const markerRefs = useRef<{ el: HTMLButtonElement; lotId: string }[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [selected, setSelected] = useState<string | null>(null);
  const [failed, setFailed] = useState(false);

  // ⏳ Реальные scraped-лоты Астаны (Story 0.8) — заменяют хардкод 1.8. Query владеет загрузкой.
  const lotsQuery = useLots();
  const { points, ungeocoded } = useMemo(() => splitLots(lotsQuery.data ?? []), [lotsQuery.data]);
  // P-D2: «есть загруженные лоты» = хоть одна точка ИЛИ хоть один негеокодированный. Управляет
  // показом плашки состояния (см. ниже) — при нуле лотов плашка не должна противоречить «пусто».
  const hasLoadedLots = points.length > 0 || ungeocoded.length > 0;

  // Инициализация карты ОДИН раз. useRef-guard от двойного new Map() под React StrictMode
  // (architecture.md; заложено в Story 1.7). Маркеры — в ОТДЕЛЬНОМ эффекте (данные из useLots приходят
  // асинхронно, не синхронно как хардкод 1.8).
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
    map.on('load', () => {
      loadedOk = true;
      setLoaded(true);
    });
    // Карта упала до 'load' (нет WebGL и т.п.) → честное состояние вместо вечной «загрузки».
    map.on('error', (ev) => {
      console.error('MapLibre error', ev);
      if (!loadedOk) setFailed(true);
    });

    return () => {
      map.remove();
      mapRef.current = null;
    };
  }, []);

  // P3 Эффект #1 — создаёт/обновляет набор маркеров реальных лотов: добавляется, когда карта
  // загружена И лоты получены (прогрессивно, после тайлов/канваса — AC1). Маркеры вне React-дерева.
  // deps = [loaded, points] (БЕЗ lang): смена языка/refetch с тем же points НЕ пересоздаёт маркеры.
  useEffect(() => {
    const map = mapRef.current;
    if (!map || !loaded) return;
    const markers: maplibregl.Marker[] = [];
    const refs: { el: HTMLButtonElement; lotId: string }[] = [];
    points.forEach(({ lot, lon, lat }) => {
      const el = document.createElement('button');
      el.type = 'button';
      el.className = 'aq-map-marker';
      el.setAttribute('aria-label', t('map.marker_label', { id: lot.goszakup_lot_id }));
      const pin = document.createElement('span');
      pin.className = 'aq-map-marker__pin';
      el.appendChild(pin);
      el.addEventListener('click', () => setSelected(lot.goszakup_lot_id));
      markers.push(
        new maplibregl.Marker({ element: el, anchor: 'bottom' })
          .setLngLat(toLngLat({ lon, lat }))
          .addTo(map),
      );
      refs.push({ el, lotId: lot.goszakup_lot_id });
    });
    markerRefs.current = refs;
    return () => {
      markers.forEach((m) => m.remove());
      markerRefs.current = [];
    };
    // Примечание: t намеренно НЕ в deps — смена языка не должна пересоздавать маркеры;
    // aria-label существующих маркеров переписывает эффект #2 (P3). (react-hooks-плагин не подключён.)
  }, [loaded, points]);

  // P3 Эффект #2 — переписывает aria-label СУЩЕСТВУЮЩИХ маркеров при смене языка, БЕЗ их сноса
  // (повторяет поведение убранного applyMarkerLabels). dep = t (меняется при languageChanged) —
  // зависимость от перевода явная, через useTranslation, а не singleton i18n.
  useEffect(() => {
    markerRefs.current.forEach(({ el, lotId }) => {
      el.setAttribute('aria-label', t('map.marker_label', { id: lotId }));
    });
  }, [t]);

  const zoomIn = () => mapRef.current?.zoomIn();
  const zoomOut = () => mapRef.current?.zoomOut();
  const recenter = () => {
    const target = { center: ASTANA_CENTER_LNGLAT, zoom: ASTANA_ZOOM };
    if (PREFERS_REDUCED_MOTION) mapRef.current?.jumpTo(target);
    else mapRef.current?.flyTo(target);
  };

  const selectedLot: MapLot | null = selected
    ? ((lotsQuery.data ?? []).find((l) => l.goszakup_lot_id === selected) ?? null)
    : null;

  // P4: сбрасываем «висячий» selected — если выбранный лот исчез после refetch (id больше не в
  // данных), лист превью не должен молча оставаться открытым на несуществующем лоте.
  useEffect(() => {
    if (selected && !selectedLot) setSelected(null);
  }, [selected, selectedLot]);

  return (
    <section className="aq-map-route">
      <h1 className="aq-map-route__title">{t('map.title')}</h1>

      {/* Честное состояние контейнера (AC1): временные/частичные данные, направления — предв.
          P-D2: при ошибке загрузки лотов ИЛИ нуле загруженных лотов плашку НЕ показываем — иначе
          «частичные данные» + keyword-bias противоречат честной строке lots_error/empty ниже. */}
      {!lotsQuery.isError && hasLoadedLots && <MapStatePlaque />}

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

      {/* Загрузка лотов: ошибка/пустота честно, не «всё чисто» (AR-17). */}
      {lotsQuery.isError && (
        <p className="aq-map-note" role="alert">
          {t('map.lots_error')}
        </p>
      )}
      {!lotsQuery.isPending &&
        !lotsQuery.isError &&
        points.length === 0 &&
        ungeocoded.length === 0 && (
          <p className="aq-map-note" role="status">
            {t('map.empty')}
          </p>
        )}

      {ungeocoded.length > 0 && (
        <section aria-labelledby="aq-ungeo-heading">
          <h2 id="aq-ungeo-heading" className="aq-ungeocoded__heading">
            {t('map.ungeocoded_heading')}
          </h2>
          <ul className="aq-ungeocoded">
            {ungeocoded.map((u) => (
              <li key={u.goszakup_lot_id} className="aq-ungeocoded__item">
                <Icon name="ungeocoded" />
                <span>{t('map.ungeocoded_label')}</span>
                <button
                  type="button"
                  className="aq-ungeocoded__btn"
                  onClick={() => setSelected(u.goszakup_lot_id)}
                >
                  {u.goszakup_lot_id}
                </button>
              </li>
            ))}
          </ul>
        </section>
      )}

      {selectedLot && (
        <LotPreviewSheet lot={selectedLot} lang={lang} onClose={() => setSelected(null)} />
      )}
    </section>
  );
}
