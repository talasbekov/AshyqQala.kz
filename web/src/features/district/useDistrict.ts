import { useQuery } from '@tanstack/react-query';
import type { components } from '../../shared/api/schema.gen';

export type District = components['schemas']['DistrictAggregates'];
export type DistrictObject = components['schemas']['DistrictObject'];
export type DistrictFlagCount = components['schemas']['DistrictFlagCount'];
export type ApiError = components['schemas']['Error'];

export interface DistrictFetchError extends Error {
  status: number;
  code: string;
}

// fetchDistrict — голый fetch к /api/districts/{kato} (Story 6.3). Публичный id = КАТО-код района.
// На !ok бросает с честным кодом (status+code) — маршрут отрисует нейтральную ошибку.
export async function fetchDistrict(kato: string): Promise<District> {
  const res = await fetch(`/api/districts/${encodeURIComponent(kato)}`);
  if (!res.ok) {
    let code = 'INTERNAL';
    try {
      const body = (await res.json()) as ApiError;
      code = body.error?.code ?? code;
    } catch {
      // тело не JSON — оставляем INTERNAL
    }
    const err = new Error(`district fetch failed: ${res.status} ${code}`) as DistrictFetchError;
    err.status = res.status;
    err.code = code;
    throw err;
  }
  return (await res.json()) as District;
}

// useDistrict — Query владеет загрузкой; queryKey по КАТО. 4xx (невалидный КАТО) НЕ ретраим (детерминирован).
export function useDistrict(kato: string) {
  return useQuery({
    queryKey: ['district', kato],
    queryFn: () => fetchDistrict(kato),
    enabled: kato.length > 0,
    retry: (n, err) => {
      const status = (err as Partial<DistrictFetchError>).status ?? 0;
      if (status >= 400 && status < 500) return false;
      return n < 3;
    },
  });
}
