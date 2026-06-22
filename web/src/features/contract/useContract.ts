import { useQuery } from '@tanstack/react-query';
import type { components } from '../../shared/api/schema.gen';

export type Contract = components['schemas']['Contract'];
export type ApiError = components['schemas']['Error'];

export interface ContractFetchError extends Error {
  status: number;
  code: string;
}

// fetchContract — голый fetch к существующему эндпоинту (Story 1.3). На !ok бросает с честным кодом.
export async function fetchContract(goszakupId: string): Promise<Contract> {
  const res = await fetch(`/api/contracts/${encodeURIComponent(goszakupId)}`);
  if (!res.ok) {
    let code = 'INTERNAL';
    try {
      const body = (await res.json()) as ApiError;
      code = body.error?.code ?? code;
    } catch {
      // тело не JSON — оставляем INTERNAL
    }
    const err = new Error(`contract fetch failed: ${res.status} ${code}`) as ContractFetchError;
    err.status = res.status;
    err.code = code;
    throw err;
  }
  return (await res.json()) as Contract;
}

// useContract — Query владеет загрузкой (loading/error/success); Router-loader НЕ фетчит.
export function useContract(goszakupId: string) {
  return useQuery({
    queryKey: ['contract', goszakupId],
    queryFn: () => fetchContract(goszakupId),
    enabled: goszakupId.length > 0,
  });
}
