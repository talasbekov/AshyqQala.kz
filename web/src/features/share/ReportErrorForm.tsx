import { useEffect, useRef, useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { useMutation } from '@tanstack/react-query';
import {
  buildErrorReportRequest,
  postErrorReport,
  reportErrorMailto,
  type ReportTarget,
  type ReportErrorFailure,
} from './reportError';
import '../flag/flag.css'; // переиспуем стили диалога (aq-meth-backdrop/aq-meth)
import './report-error.css';

// ReportErrorForm — полная форма канала «Сообщить об ошибке» (Story 5.4, FR-28). Честный автомат состояний:
// idle/filling → submitting (кнопка заблокирована — защита от двойной) → success | error (текст НЕ теряется).
// Привязка к объекту — из контекста (target), не вводится руками. role=dialog, базовый фокус + Escape
// (полный focus-trap — Epic 3 Story 3.5). Mailto — no-JS фолбэк в действиях.
export function ReportErrorForm({ target, onClose }: { target: ReportTarget; onClose: () => void }) {
  const { t } = useTranslation('chrome');
  const [message, setMessage] = useState('');
  const [contact, setContact] = useState('');
  const [leaveBlank, setLeaveBlank] = useState(''); // honeypot — настоящие люди не заполняют
  const dialogRef = useRef<HTMLDivElement>(null);
  const messageRef = useRef<HTMLTextAreaElement>(null);
  const inFlight = useRef(false); // synchronous-guard от двойной отправки до коммита isPending

  useEffect(() => {
    messageRef.current?.focus();
  }, []);

  const mutation = useMutation({
    mutationFn: () => postErrorReport(buildErrorReportRequest(target, { message, contact, leaveBlank })),
  });

  const trimmed = message.trim();
  const canSubmit = !mutation.isPending && trimmed !== '';

  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (!canSubmit || inFlight.current) return; // защита от двойной/пустой отправки (синхронно)
    inFlight.current = true;
    mutation.mutate(undefined, { onSettled: () => { inFlight.current = false; } });
  };

  const errCode = (mutation.error as ReportErrorFailure | null)?.code;
  const errorText =
    errCode === 'RATE_LIMITED'
      ? t('report_error.form.error_rate_limited')
      : t('report_error.form.error');

  return (
    <div className="aq-meth-backdrop" onClick={onClose}>
      <div
        ref={dialogRef}
        className="aq-meth aq-report"
        role="dialog"
        aria-modal="false"
        aria-labelledby="aq-report-title"
        tabIndex={-1}
        onClick={(e) => e.stopPropagation()}
        onKeyDown={(e) => {
          if (e.key === 'Escape') {
            e.stopPropagation(); // закрыть ТОЛЬКО форму, не родителя (напр. лист лота на карте)
            onClose();
          }
        }}
      >
        <h2 id="aq-report-title" className="aq-meth__title">
          {t('report_error.form.title')}
        </h2>

        {mutation.isSuccess ? (
          <div className="aq-report__done" role="status" aria-live="polite">
            <p className="aq-report__success">{t('report_error.form.success')}</p>
            <div className="aq-meth__actions">
              <button type="button" className="aq-meth__close" onClick={onClose}>
                {t('methodology.close')}
              </button>
            </div>
          </div>
        ) : (
          <form onSubmit={submit} noValidate>
            <p className="aq-meth__simple">{t('report_error.form.intro')}</p>

            <label className="aq-report__label" htmlFor="aq-report-message">
              {t('report_error.form.message_label')}
            </label>
            <textarea
              id="aq-report-message"
              ref={messageRef}
              className="aq-report__textarea"
              required
              rows={4}
              maxLength={5000}
              value={message}
              onChange={(e) => setMessage(e.target.value)}
              placeholder={t('report_error.form.message_placeholder')}
            />

            <label className="aq-report__label" htmlFor="aq-report-contact">
              {t('report_error.form.contact_label')}
            </label>
            <input
              id="aq-report-contact"
              className="aq-report__input"
              type="text"
              maxLength={200}
              value={contact}
              onChange={(e) => setContact(e.target.value)}
              placeholder={t('report_error.form.contact_placeholder')}
            />

            {/* honeypot: скрыт от людей (CSS), бот заполнит → сервер молча отбросит. */}
            <input
              className="aq-report__hp"
              type="text"
              name="leave_blank"
              tabIndex={-1}
              autoComplete="off"
              aria-hidden="true"
              value={leaveBlank}
              onChange={(e) => setLeaveBlank(e.target.value)}
            />

            {mutation.isError && (
              <p className="aq-report__error" role="alert">
                {errorText}
              </p>
            )}

            <div className="aq-meth__actions">
              <button type="submit" className="aq-report__submit" disabled={!canSubmit}>
                {mutation.isPending ? t('report_error.form.submitting') : t('report_error.form.submit')}
              </button>
              <a
                className="aq-meth__link"
                href={reportErrorMailto(t('report_error.subject', { id: target.subjectRef }))}
              >
                {t('report_error.form.mailto_fallback')}
              </a>
              <button type="button" className="aq-meth__close" onClick={onClose}>
                {t('methodology.close')}
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  );
}
