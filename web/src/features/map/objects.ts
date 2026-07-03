import { useQuery, keepPreviousData } from '@tanstack/react-query';
import type { components } from '../../shared/api/schema.gen';

// Story 3.4 (FR-7): загрузка канонических geo_objects в bbox окна карты (/api/map/objects).
// Зеркалит паттерн lots.ts (голый fetch + честная !ok-ветка + Query владеет загрузкой).
export type MapObject = components['schemas']['MapObject'];
export type MapObjectsResponse = components['schemas']['MapObjectsResponse'];
type ApiError = components['schemas']['Error'];

export interface MapObjectsFetchError extends Error {
  status: number;
  code: string;
}

// fetchMapObjects — голый fetch к /api/map/objects. На !ok — честный код из Error-конверта.
// signal — отмена in-flight запроса при смене окна (react-query отменяет устаревший queryKey).
export async function fetchMapObjects(
  bbox: string,
  signal?: AbortSignal,
): Promise<MapObjectsResponse> {
  const res = await fetch(`/api/map/objects?bbox=${encodeURIComponent(bbox)}`, { signal });
  if (!res.ok) {
    let code = 'INTERNAL';
    try {
      const body = (await res.json()) as ApiError;
      code = body.error?.code ?? code;
    } catch {
      // тело не JSON — оставляем INTERNAL
    }
    const err = new Error(
      `map objects fetch failed: ${res.status} ${code}`,
    ) as MapObjectsFetchError;
    err.status = res.status;
    err.code = code;
    throw err;
  }
  return (await res.json()) as MapObjectsResponse;
}

// useMapObjects — Query владеет загрузкой; bbox=null ⇒ окно ещё неизвестно (карта не загрузилась) —
// запрос не уходит. keepPreviousData: при панораме старые объекты видимы до прихода новых (карта не мигает).
export function useMapObjects(bbox: string | null) {
  return useQuery({
    queryKey: ['map-objects', bbox],
    queryFn: ({ signal }) => fetchMapObjects(bbox as string, signal),
    enabled: bbox !== null,
    placeholderData: keepPreviousData,
  });
}

// --- Геометрия: честное сужение geom (schema.gen отдаёт непрозрачный object) ---

// length >= 2, не === 2: RFC7946-позиция допускает высоту третьим элементом ([lon,lat,alt]);
// CHECK 0022 (GeometryType) пропускает Z-геометрию куратора — честно рисуем по lon/lat, не отбрасываем.
const isFinitePair = (c: unknown): c is [number, number] =>
  Array.isArray(c) && c.length >= 2 && Number.isFinite(c[0]) && Number.isFinite(c[1]);

// PointObject / LineObject — объект с БЕЗОПАСНО разобранной геометрией (координаты конечны).
export interface PointObject {
  obj: MapObject;
  lon: number;
  lat: number;
}
export interface LineObject {
  obj: MapObject;
  coordinates: [number, number][];
}

// splitObjects — Point ⊥ LineString. Гардрейл честности (зеркало splitLots): геометрия ТОЛЬКО из geom
// с конечными координатами; неизвестный тип/мусорные координаты — объект пропускается С ВИДИМЫМ
// warn-сигналом (default-ветка AR-16: неизвестное не роняет карту и не рисуется наугад).
export function splitObjects(objects: MapObject[]): { points: PointObject[]; lines: LineObject[] } {
  const points: PointObject[] = [];
  const lines: LineObject[] = [];
  for (const obj of objects) {
    const geom = obj.geom as { type?: unknown; coordinates?: unknown };
    if (geom?.type === 'Point' && isFinitePair(geom.coordinates)) {
      points.push({ obj, lon: geom.coordinates[0], lat: geom.coordinates[1] });
    } else if (
      geom?.type === 'LineString' &&
      Array.isArray(geom.coordinates) &&
      geom.coordinates.length >= 2 &&
      geom.coordinates.every(isFinitePair)
    ) {
      lines.push({ obj, coordinates: geom.coordinates as [number, number][] });
    } else {
      console.warn(
        `splitObjects: объект ${obj.public_id} с неразбираемой геометрией (type=${String(
          geom?.type,
        )}) пропущен — координату не выдумываем`,
      );
    }
  }
  return { points, lines };
}

// --- Глиф-статус маркера (AC1, D3) ---

// MarkerKind — вид пина: флаг риска первичен (глиф «!», z 2), затем verified (глиф «✓»),
// остальное (auto/manual/wrong_reported/НЕИЗВЕСТНОЕ) — пустой пин: default-ветка обязательна (AR-16),
// неизвестный будущий статус не роняет и не приукрашивает.
export type MarkerKind = 'plain' | 'flagged' | 'confirmed';

export function markerKind(obj: Pick<MapObject, 'geocode_status' | 'has_active_flag'>): MarkerKind {
  if (obj.has_active_flag) return 'flagged';
  if (obj.geocode_status === 'verified') return 'confirmed';
  return 'plain';
}

// --- Story 3.5 (AC3, D5): множественное попадание — двойники координат ---

// coordKey — ключ совпадения координат (ε = 6 знаков ≈ 0.11 м): «один адрес» с плавающей точкой.
export function coordKey(lon: number, lat: number): string {
  return `${lon.toFixed(6)},${lat.toFixed(6)}`;
}

// coincidentPoints — точки-двойники цели (та же координата с точностью ε), ВКЛЮЧАЯ саму цель.
// Группа > 1 ⇒ тап по маркеру открывает превью-список, а не одиночное превью (AC3).
export function coincidentPoints(points: PointObject[], target: PointObject): PointObject[] {
  const key = coordKey(target.lon, target.lat);
  return points.filter((p) => coordKey(p.lon, p.lat) === key);
}

// allCoincident — совпадают ли ВСЕ позиции (ε). Детектор «кластер не разваливается зумом»:
// одного лишь expansionZoom > maxZoom мало (кластер, распадающийся ровно на maxZoom+1, выглядит так же) —
// сверяем координаты leaves. Пустой/одиночный набор — вырожденно true.
export function allCoincident(coords: [number, number][]): boolean {
  if (coords.length <= 1) return true;
  const key = coordKey(coords[0][0], coords[0][1]);
  return coords.every((c) => coordKey(c[0], c[1]) === key);
}

// --- Story 3.5 (AC1, D6): цена/км линии ---

// pricePerKm — целые ₸/км из суммы (каноничная строка целых ₸) и длины (км). Деривация из двух
// ВИДИМЫХ в том же листе фактов (UX-DR13), НЕ методика 4.3 (каноническая бэк-цена появится с живыми
// evidence — тогда приоритет бэку). Любой не-ok вход → null (честно скрываем, не выдумываем):
// отрицательная/неканоничная сумма, отсутствующая/нулевая/нефинитная длина. BigInt — точность > 2^53.
export function pricePerKm(
  amountTng: string | null | undefined,
  lengthKm: number | null | undefined,
): string | null {
  if (!amountTng || !/^\d+$/.test(amountTng)) return null;
  if (lengthKm == null || !Number.isFinite(lengthKm) || lengthKm <= 0) return null;
  // длина — double из БД; масштаб 1000 (точность до метра) держит деление целочисленным.
  // isFinite: lengthKm*1000 может переполниться в Infinity (монструозная длина из БД) —
  // BigInt(Infinity) бросил бы RangeError в рендере (код-ревью 3.5).
  const scaled = Math.round(lengthKm * 1000);
  if (!Number.isFinite(scaled) || scaled <= 0) return null;
  return ((BigInt(amountTng) * 1000n) / BigInt(scaled)).toString();
}

// contractPath — единственная точка сборки пути карточки из внешнего goszakup_contract_id
// (энкодинг спецсимволов id — natural key приходит из источника и не гарантирован URL-safe).
export function contractPath(goszakupContractId: string): string {
  return `/contracts/${encodeURIComponent(goszakupContractId)}`;
}

// --- bbox окна карты ---

// boundsToBBox — строка «minLon,minLat,maxLon,maxLat» из границ карты с ЗАЖИМОМ в WGS84:
// на малом зуме MapLibre отдаёт границы за пределами [-180,180] (обёртка мира) — сервер честно
// ответил бы 400. Вырожденное после зажима окно → null (запрос не имеет смысла).
export function boundsToBBox(b: {
  getWest(): number;
  getSouth(): number;
  getEast(): number;
  getNorth(): number;
}): string | null {
  const clamp = (v: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, v));
  const west = clamp(b.getWest(), -180, 180);
  const east = clamp(b.getEast(), -180, 180);
  const south = clamp(b.getSouth(), -90, 90);
  const north = clamp(b.getNorth(), -90, 90);
  if (!(west < east && south < north)) return null;
  return `${west},${south},${east},${north}`;
}

// debounce — troттлинг рефетча bbox на moveend (D6, ~300мс): панорама не бомбит API каждым кадром.
export function debounce<A extends unknown[]>(
  fn: (...args: A) => void,
  ms: number,
): { (...args: A): void; cancel(): void } {
  let timer: ReturnType<typeof setTimeout> | null = null;
  const wrapped = (...args: A) => {
    if (timer !== null) clearTimeout(timer);
    timer = setTimeout(() => {
      timer = null;
      fn(...args);
    }, ms);
  };
  wrapped.cancel = () => {
    if (timer !== null) {
      clearTimeout(timer);
      timer = null;
    }
  };
  return wrapped;
}
