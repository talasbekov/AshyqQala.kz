import { useQuery } from '@tanstack/react-query';
import type { components } from '../../shared/api/schema.gen';

export type Contractor = components['schemas']['Contractor'];
export type ApiError = components['schemas']['Error'];

export interface ContractorFetchError extends Error {
  status: number;
  code: string;
}

// fetchContractor — голый fetch к /api/contractors/{bin} (Story 5.2). На !ok бросает с честным кодом.
export async function fetchContractor(bin: string): Promise<Contractor> {
  const res = await fetch(`/api/contractors/${encodeURIComponent(bin)}`);
  if (!res.ok) {
    let code = 'INTERNAL';
    try {
      const body = (await res.json()) as ApiError;
      code = body.error?.code ?? code;
    } catch {
      // тело не JSON — оставляем INTERNAL
    }
    const err = new Error(`contractor fetch failed: ${res.status} ${code}`) as ContractorFetchError;
    err.status = res.status;
    err.code = code;
    throw err;
  }
  return (await res.json()) as Contractor;
}

// useContractor — Query владеет загрузкой; публичный id = natural БИН.
export function useContractor(bin: string) {
  return useQuery({
    queryKey: ['contractor', bin],
    queryFn: () => fetchContractor(bin),
    enabled: bin.length > 0,
  });
}
