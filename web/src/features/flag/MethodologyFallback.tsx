import { useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { useDialogFocus } from '../../shared/ui/useDialogFocus';

// Fallback ErrorBoundary диалога методики (Story 3.5 — закрыт долг deferred:77): путь восстановления
// после render-throw ведёт себя как честный диалог — Escape/бэкдроп/кнопка закрывают, фокус
// возвращается на триггер (AT-пользователь не теряется на пути ошибки).
export function MethodologyFallback({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation('chrome');
  const ref = useRef<HTMLDivElement>(null);
  useDialogFocus(ref, onClose);

  return (
    <div className="aq-meth-backdrop" role="presentation" onClick={onClose}>
      <div
        ref={ref}
        className="aq-meth"
        role="alertdialog"
        aria-modal="true"
        aria-label={t('error.title')}
        tabIndex={-1}
        onClick={(e) => e.stopPropagation()}
      >
        <p>{t('error.body')}</p>
        <button type="button" className="aq-meth__close" onClick={onClose}>
          {t('methodology.close')}
        </button>
      </div>
    </div>
  );
}
