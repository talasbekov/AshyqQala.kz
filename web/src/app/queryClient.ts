import { QueryClient } from '@tanstack/react-query';

// Единый QueryClient. TanStack Query владеет загрузкой (Router-loader НЕ фетчит — AC1).
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: 1, refetchOnWindowFocus: false, staleTime: 30_000 },
  },
});
