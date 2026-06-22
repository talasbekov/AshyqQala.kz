// Маршрутизация: React Router data API. Внешний id карточки = natural goszakup_id (URL-схема —
// общий контракт, Story 1.3). ВАЖНО (AC1): Router-loader НЕ фетчит — данные грузит TanStack Query
// в компоненте маршрута (useContract). Здесь — только определения маршрутов.
import { createBrowserRouter } from 'react-router-dom';
import { App, HomeView } from './App';
import { ContractRoute } from './features/contract';

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
    children: [
      { index: true, element: <HomeView /> },
      { path: 'contracts/:goszakupId', element: <ContractRoute /> },
    ],
  },
]);
