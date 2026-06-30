// Story 6.1 (FR-15): загрузка фасетно-отфильтрованного списка контрактов. Чистые функции маппинга
// (фасеты ↔ URL-параметры ↔ backend-query) вынесены отдельно и юнит-тестируемы (без рендера: @testing-library
// не установлен — компонентный охват через Playwright). Курсорная пагинация через useInfiniteQuery.
import { useMemo } from 'react';
import { useInfiniteQuery } from '@tanstack/react-query';
import type { components } from '../../shared/api/schema.gen';

export type ContractListItem = components['schemas']['ContractListItem'];
export type ContractListResponse = components['schemas']['ContractListResponse'];
export type ApiError = components['schemas']['Error'];

export type PeriodPreset = '12m' | '24m' | '36m' | 'all';

export const DIRECTIONS = ['road', 'water', 'other'] as const;
export const PERIODS: readonly PeriodPreset[] = ['12m', '24m', '36m', 'all'];

export interface SearchFilters {
  directions: string[]; // road|water|other (OR внутри фасета)
  period: PeriodPreset;
  hasFlag: boolean;
  amountMin: string; // '' = не задано
  amountMax: string;
  supplierBin: string; // '' = не задано
}

export const EMPTY_FILTERS: SearchFilters = {
  directions: [],
  period: 'all',
  hasFlag: false,
  amountMin: '',
  amountMax: '',
  supplierBin: '',
};

// periodToSignedFrom — пресет периода → дата signed_from. ЭТО ФАСЕТ ПОИСКА по sign_date (AC4): он НЕ меняет
// окно медианы (медиана всегда скользящие 24 мес). 'all' → '' (без границы). Считается от текущей даты.
export function periodToSignedFrom(p: PeriodPreset, now: Date = new Date()): string {
  const months: Record<PeriodPreset, number> = { '12m': 12, '24m': 24, '36m': 36, all: 0 };
  const m = months[p];
  if (m === 0) return '';
  // Вычитаем месяцы с КЛЭМПОМ дня к числу дней целевого месяца: иначе Date.UTC(y, monIdx, day) переполняется
  // на граничных днях (31 мар − 1 мес ≠ 3 мар; 29 фев високосного − 12 мес ≠ 1 мар), сдвигая границу на день.
  // [code review 6.1]
  const monIdx = now.getUTCMonth() - m;
  const targetYear = now.getUTCFullYear() + Math.floor(monIdx / 12);
  const targetMonth = ((monIdx % 12) + 12) % 12;
  const lastDay = new Date(Date.UTC(targetYear, targetMonth + 1, 0)).getUTCDate();
  const d = new Date(Date.UTC(targetYear, targetMonth, Math.min(now.getUTCDate(), lastDay)));
  return d.toISOString().slice(0, 10);
}

// buildQuery — фасеты → snake_case query бэкенда (пустые опускаем). period разворачивается в signed_from.
export function buildQuery(f: SearchFilters, cursor?: string, now?: Date): string {
  const p = new URLSearchParams();
  if (f.directions.length) p.set('direction', f.directions.join(','));
  const from = periodToSignedFrom(f.period, now);
  if (from) {
    p.set('signed_from', from);
    // Верхняя граница относительного периода = «сейчас»: «за последние N мес» = [now−N, now]. Без неё контракты
    // с будущей/ошибочной датой подписания всегда попадали бы в выдачу. [code review 6.1]
    p.set('signed_to', (now ?? new Date()).toISOString().slice(0, 10));
  }
  if (f.hasFlag) p.set('has_flag', 'true');
  if (f.amountMin) p.set('amount_min', f.amountMin);
  if (f.amountMax) p.set('amount_max', f.amountMax);
  if (f.supplierBin) p.set('supplier_bin', f.supplierBin);
  if (cursor) p.set('cursor', cursor);
  const s = p.toString();
  return s ? `?${s}` : '';
}

// filtersFromParams / filtersToParams — состояние фильтров В URL (шарабельность; AC1). В URL хранится пресет
// period (12m/24m/36m), а не развёрнутая дата — чтобы ссылка не «протухала» по времени.
export function filtersFromParams(sp: URLSearchParams): SearchFilters {
  const dirRaw = sp.get('direction') ?? '';
  const directions = dirRaw
    ? dirRaw.split(',').filter((d) => (DIRECTIONS as readonly string[]).includes(d))
    : [];
  const periodRaw = sp.get('period') ?? 'all';
  const period = (PERIODS as readonly string[]).includes(periodRaw)
    ? (periodRaw as PeriodPreset)
    : 'all';
  return {
    directions,
    period,
    hasFlag: sp.get('has_flag') === 'true',
    amountMin: sp.get('amount_min') ?? '',
    amountMax: sp.get('amount_max') ?? '',
    supplierBin: sp.get('supplier_bin') ?? '',
  };
}

export function filtersToParams(f: SearchFilters): URLSearchParams {
  const p = new URLSearchParams();
  if (f.directions.length) p.set('direction', f.directions.join(','));
  if (f.period !== 'all') p.set('period', f.period);
  if (f.hasFlag) p.set('has_flag', 'true');
  if (f.amountMin) p.set('amount_min', f.amountMin);
  if (f.amountMax) p.set('amount_max', f.amountMax);
  if (f.supplierBin) p.set('supplier_bin', f.supplierBin);
  return p;
}

export interface ContractsFetchError extends Error {
  status: number;
  code: string;
}

async function fetchContracts(
  f: SearchFilters,
  cursor?: string,
  now?: Date,
): Promise<ContractListResponse> {
  const res = await fetch(`/api/contracts${buildQuery(f, cursor, now)}`);
  if (!res.ok) {
    let code = 'INTERNAL';
    try {
      const body = (await res.json()) as ApiError;
      code = body.error?.code ?? code;
    } catch {
      // тело не JSON — оставляем INTERNAL
    }
    const err = new Error(`contracts fetch failed: ${res.status} ${code}`) as ContractsFetchError;
    err.status = res.status;
    err.code = code;
    throw err;
  }
  return (await res.json()) as ContractListResponse;
}

// useContracts — курсорная (keyset) пагинация: каждая страница несёт next_cursor (null ⇒ конец). Смена
// filters даёт новый queryKey → авто-перезагрузка (немедленное применение, AC1).
export function useContracts(f: SearchFilters) {
  // Замораживаем «сейчас» на время жизни ОДНОГО infinite-query (пересчитываем только при смене фильтров):
  // иначе periodToSignedFrom(period, new Date()) вычислялся бы заново на КАЖДУЮ страницу, и при пересечении
  // полуночи UTC во время пагинации нижняя граница относительного периода сдвинулась бы → строки в keyset-хвосте
  // молча выпали бы (gap, тихая потеря данных вопреки гардрейлу честности). [code review 6.1]
  const fKey = useMemo(() => JSON.stringify(f), [f]);
  // Новый набор фильтров (новый fKey) ⇒ новое «сейчас»; в пределах одного набора (пагинация) — заморожено.
  const now = useMemo(() => new Date(), [fKey]);
  return useInfiniteQuery({
    queryKey: ['contracts', f],
    queryFn: ({ pageParam }) => fetchContracts(f, pageParam, now),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor ?? undefined,
    // Не ретраить детерминированные 4xx (битый курсор/параметр → 400): ретрай бессмыслен и лишь задерживает
    // показ ошибки + грузит бэкенд. Сетевые/5xx — до 3 попыток (дефолт TanStack). [code review 6.1]
    retry: (failureCount, error) => {
      const status = (error as Partial<ContractsFetchError>).status;
      if (typeof status === 'number' && status >= 400 && status < 500) return false;
      return failureCount < 3;
    },
  });
}
