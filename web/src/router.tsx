// Каркас маршрутизации. React Router (data API) + TanStack Query подключаются в Story 1.7
// (Query владеет загрузкой; Router-loader НЕ фетчит). Внешний id карточки = natural goszakup_id
// (URL-схема — общий контракт, Story 1.3). Здесь — типизированный задел путей.
export type RouteId = 'contract' | 'contractor' | 'district' | 'map' | 'search';

export const routePaths: Record<RouteId, string> = {
  contract: '/contracts/:goszakupId',
  contractor: '/contractors/:bin',
  district: '/districts/:kato',
  map: '/map',
  search: '/search',
};
