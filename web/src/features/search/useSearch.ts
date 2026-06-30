// Story 6.2 (FR-16): текстовый поиск по БИН и наименованию. Чистые функции (детект БИН, гейт searchable,
// сборка query) вынесены и юнит-тестируемы (vitest, без рендера: @testing-library не установлен). Запрос —
// одностраничный (next_cursor всегда null: relevance-порядок не keyset-дружелюбен — Dev Notes 6.2), потому
// useQuery, а не useInfiniteQuery. Не ретраим 4xx (битый/короткий запрос → 400): паттерн useContracts.
import { useQuery } from '@tanstack/react-query';
import type { components } from '../../shared/api/schema.gen';

export type SearchResponse = components['schemas']['SearchResponse'];
export type SearchResultItem = components['schemas']['SearchResultItem'];
export type OrganizationSearchItem = components['schemas']['OrganizationSearchItem'];
export type ContractSearchItem = components['schemas']['ContractSearchItem'];
export type ApiError = components['schemas']['Error'];

// Мин. длина терма для поиска по ИМЕНИ (зеркало бэкенда minNameQueryLen). БИН — всегда ровно 12 цифр (точный).
export const MIN_QUERY_LEN = 3;

// binDigits — цифры терма (зеркало normalize.NormalizeBIN: бэкенд извлекает цифры и требует ровно 12).
function binDigits(q: string): string {
  return q.replace(/\D/g, '');
}

// isLikelyBin — фронтовый детект 12-значного БИН (для подсказки/обхода мин-длины имени). Бэкенд канонизирует
// окончательно (normalize.CanonicalBIN); здесь — лишь UX-предсказание ветки.
export function isLikelyBin(q: string): boolean {
  return binDigits(q).length === 12;
}

// queryIsSearchable — терм достаточен для запроса: валидный БИН ИЛИ имя ≥ MIN_QUERY_LEN символов (по кодпойнтам,
// как RuneCountInString на бэкенде). Гейт против заведомых 400 (слишком короткий) и пустого скана.
export function queryIsSearchable(q: string): boolean {
  const t = q.trim();
  if (t === '') return false;
  if (isLikelyBin(t)) return true;
  return [...t].length >= MIN_QUERY_LEN;
}

// buildSearchQuery — терм → snake_case query бэкенда.
export function buildSearchQuery(q: string): string {
  const p = new URLSearchParams();
  p.set('q', q.trim());
  return `?${p.toString()}`;
}

export interface SearchFetchError extends Error {
  status: number;
  code: string;
}

async function fetchSearch(q: string): Promise<SearchResponse> {
  const res = await fetch(`/api/search${buildSearchQuery(q)}`);
  if (!res.ok) {
    let code = 'INTERNAL';
    try {
      const body = (await res.json()) as ApiError;
      code = body.error?.code ?? code;
    } catch {
      // тело не JSON — оставляем INTERNAL
    }
    const err = new Error(`search fetch failed: ${res.status} ${code}`) as SearchFetchError;
    err.status = res.status;
    err.code = code;
    throw err;
  }
  return (await res.json()) as SearchResponse;
}

// useSearch — одностраничный текстовый поиск. enabled только для searchable-терма (иначе не шлём заведомый 400 /
// пустой скан). Смена терма → новый queryKey → авто-перезагрузка. 4xx не ретраим.
export function useSearch(q: string) {
  const term = q.trim();
  return useQuery({
    queryKey: ['search', term],
    queryFn: () => fetchSearch(term),
    enabled: queryIsSearchable(term),
    retry: (failureCount, error) => {
      const status = (error as Partial<SearchFetchError>).status;
      if (typeof status === 'number' && status >= 400 && status < 500) return false;
      return failureCount < 3;
    },
  });
}
