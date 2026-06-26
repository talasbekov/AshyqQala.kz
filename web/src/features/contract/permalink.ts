// buildPermalink — перманентная ссылка-на-дату (AR-29, Story 5.6): путь карточки + честный штамп (as_of, mv).
// Пустые поля НЕ штампуются (honesty-hole 5.1: не фабрикуем штамп, которого нет). Возвращает ОТНОСИТЕЛЬНЫЙ
// путь; абсолютный URL для шаринга собирает вызывающий (origin + путь). Сервер по ?mv детектирует дрейф методики.
export function buildPermalink(
  goszakupId: string,
  asOf?: string | null,
  methodologyVersion?: string | null,
): string {
  const base = `/contracts/${encodeURIComponent(goszakupId)}`;
  const params = new URLSearchParams();
  if (asOf) params.set('as_of', asOf);
  if (methodologyVersion) params.set('mv', methodologyVersion);
  const qs = params.toString();
  return qs ? `${base}?${qs}` : base;
}
