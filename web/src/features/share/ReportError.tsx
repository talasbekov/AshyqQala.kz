import { useTranslation } from 'react-i18next';
import { reportErrorMailto } from './reportError';

// Дверь «Сообщить об ошибке» (Story 1.9, FR-28 stub) — безаккаунтный mailto-канал. Полная форма +
// Directus-очередь — Epic 5 (Story 5.4). Используется в подвале карточки; рядом с флагом ссылка строится
// через reportErrorMailto напрямую (gate-проп бейджа).
export function ReportError({ contractId, className }: { contractId: string; className?: string }) {
  const { t } = useTranslation('chrome');
  const href = reportErrorMailto(t('report_error.subject', { id: contractId }));
  return (
    <a className={className ?? 'aq-report-error'} href={href}>
      {t('report_error.link')}
    </a>
  );
}
