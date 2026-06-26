import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ReportErrorForm } from './ReportErrorForm';

// Дверь «Сообщить об ошибке» (Story 5.4, FR-28). Кнопка → открывает полную форму (POST в Directus-очередь).
// Подвал карточки: обращение о данных контракта (kind=data_error). Mailto — фолбэк внутри формы.
export function ReportError({ contractId, className }: { contractId: string; className?: string }) {
  const { t } = useTranslation('chrome');
  const [open, setOpen] = useState(false);
  return (
    <>
      <button
        type="button"
        className={className ?? 'aq-report-error'}
        aria-haspopup="dialog"
        onClick={() => setOpen(true)}
      >
        {t('report_error.link')}
      </button>
      {open && (
        <ReportErrorForm
          target={{ kind: 'data_error', subjectType: 'contract', subjectRef: contractId }}
          onClose={() => setOpen(false)}
        />
      )}
    </>
  );
}
