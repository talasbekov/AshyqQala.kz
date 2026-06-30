// Маршрутизация: React Router data API. Внешний id карточки = natural goszakup_id (URL-схема —
// общий контракт, Story 1.3). ВАЖНО (AC1): Router-loader НЕ фетчит — данные грузит TanStack Query
// в компоненте маршрута (useContract). Здесь — только определения маршрутов.
import { lazy, Suspense } from 'react';
import { createBrowserRouter } from 'react-router-dom';
import { App, HomeView, NotFoundView, RouteError } from './App';
import { ContractRoute } from './features/contract';
import { ContractorRoute } from './features/contractor';
import { DistrictRoute } from './features/district';
import { SearchRoute } from './features/search';

// Карта тянет крупный чанк MapLibre — грузим ЛЕНИВО, чтобы не раздувать бандл главной/карточки
// (NFR-1 <3с). MapLibre попадает в отдельный чанк, подгружаемый только на маршруте /map.
const MapView = lazy(() => import('./features/map').then((m) => ({ default: m.MapView })));

export type RouteId = 'contract' | 'contractor' | 'district' | 'map' | 'search';

export const routePaths: Record<RouteId, string> = {
  contract: '/contracts/:goszakupId',
  contractor: '/contractors/:bin',
  district: '/districts/:kato',
  map: '/map',
  search: '/search',
};

export const router = createBrowserRouter([
  {
    path: '/',
    element: <App />,
    errorElement: <RouteError />,
    children: [
      { index: true, element: <HomeView /> },
      {
        path: 'map',
        element: (
          <Suspense fallback={null}>
            <MapView />
          </Suspense>
        ),
      },
      { path: 'contracts/:goszakupId', element: <ContractRoute /> },
      { path: 'contractors/:bin', element: <ContractorRoute /> },
      // Story 6.3 (FR-17): страница района по КАТО (агрегаты, container_state, список объектов).
      { path: 'districts/:kato', element: <DistrictRoute /> },
      // Story 6.1 (FR-15): фасетный поиск/фильтрация списка контрактов (состояние фильтров — в URL).
      { path: 'search', element: <SearchRoute /> },
      // Нейтральная 404-заглушка для неизвестных путей (review-фикс P2; полноценная — позже).
      { path: '*', element: <NotFoundView /> },
    ],
  },
]);
