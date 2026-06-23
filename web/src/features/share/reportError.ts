// Минимальная дверь «Сообщить об ошибке» (Story 1.9, FR-28 stub). Полная форма + Directus-очередь —
// Epic 5 (Story 5.4 «расширяет дверь-stub из 1.9»). Здесь — безаккаунтный mailto-канал.
// [Source: epics.md:1013,1557; prd.md:289–296]

// Адрес канала возражения (заглушка демо; реальный — в деплое/ENV позже).
export const REPORT_ERROR_EMAIL = 'errors@ashyqqala.kz';

// reportErrorMailto — mailto со заполненной нейтральной темой (id контракта для маршрутизации оператору).
export function reportErrorMailto(subject: string): string {
  return `mailto:${REPORT_ERROR_EMAIL}?subject=${encodeURIComponent(subject)}`;
}
