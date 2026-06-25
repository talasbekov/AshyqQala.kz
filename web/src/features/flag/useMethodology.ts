import { useQuery } from '@tanstack/react-query';
import type { components } from '../../shared/api/schema.gen';

// Story 5.3: пороги методики из ЕДИНОГО источника (/api/methodology → methodology_params). Экран методики
// показывает формулу/пороги ВСЕГДА (в т.ч. когда сигнал не выставлен), без литералов на фронте. Конфиг
// иммутабелен в рантайме → кэшируем навсегда (staleTime: Infinity).
export type Methodology = components['schemas']['Methodology'];

export async function fetchMethodology(): Promise<Methodology> {
  const res = await fetch('/api/methodology');
  if (!res.ok) {
    throw new Error(`methodology fetch failed: ${res.status}`);
  }
  return (await res.json()) as Methodology;
}

export function useMethodology() {
  return useQuery({
    queryKey: ['methodology'],
    queryFn: fetchMethodology,
    staleTime: Number.POSITIVE_INFINITY, // иммутабельный конфиг — не перезапрашиваем
  });
}
