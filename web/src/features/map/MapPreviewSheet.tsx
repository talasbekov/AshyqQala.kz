import { useEffect, useRef } from 'react';
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import type { Lang } from '../../shared/i18n';
import { formatMoney } from '../../shared/i18n/format';
import { DataState, dataStateFromValueState } from '../../shared/state/DataState';
import { useContract } from '../contract';
import type { ContractFetchError } from '../contract';
import type { components } from '../../shared/api/schema.gen';

type StringField = components['schemas']['StringField'];

// Value — честный рендер поля (паттерн ContractCard, Story 1.7): значение при state=ok && value!==null,
// иначе явное состояние. Дыра ok+null закрыта (как в карточке).
function Value({ field, format }: { field: StringField; format?: (v: string) => string }) {
  if (field.state === 'ok' && field.value !== null) {
    return <>{format ? format(field.value) : field.value}</>;
  }
  const state = field.state === 'ok' ? 'no_data' : field.state;
  return <DataState kind={dataStateFromValueState(state)} state={state} />;
}

// Нижний лист превью объекта (AC2). Минимальный: subject + сумма + «Подробнее →» в карточку.
// Лист рендерится в React-дереве (НЕ maplibregl.Popup) → i18n работает штатно через useTranslation.
// Данные грузит useContract (переиспользование, Story 1.7) — Router-loader НЕ фетчит.
export function MapPreviewSheet({
  goszakupId,
  lang,
  onClose,
}: {
  goszakupId: string;
  lang: Lang;
  onClose: () => void;
}) {
  const { t } = useTranslation('chrome');
  const q = useContract(goszakupId);
  const sheetRef = useRef<HTMLDivElement>(null);

  // Базовое управление фокусом (полный focus-trap — Epic 3, Story 3.5): фокус на лист при открытии.
  useEffect(() => {
    sheetRef.current?.focus();
  }, []);

  const subject = q.data ? (lang === 'kk' ? q.data.subject_kk : q.data.subject_ru) : null;

  return (
    <div className="aq-sheet-backdrop" onClick={onClose}>
      <div
        ref={sheetRef}
        className="aq-sheet"
        role="dialog"
        aria-modal="false"
        aria-label={t('map.preview_title')}
        tabIndex={-1}
        onClick={(e) => e.stopPropagation()}
        onKeyDown={(e) => {
          if (e.key === 'Escape') onClose();
        }}
      >
        <div className="aq-sheet__handle" aria-hidden="true" />
        {q.isPending && (
          <p className="aq-sheet__loading" role="status">
            {t('map.preview_loading')}
          </p>
        )}
        {q.isError && (
          <p className="aq-sheet__error" role="alert">
            {t(
              (q.error as Partial<ContractFetchError>).status === 404
                ? 'contract.not_found'
                : 'contract.error',
            )}
          </p>
        )}
        {q.data && subject && (
          <>
            <h2 className="aq-sheet__title" lang={lang}>
              <Value field={subject} />
            </h2>
            <p className="aq-sheet__amount">
              <Value field={q.data.amount_tng} format={(v) => formatMoney(v, lang)} />
            </p>
          </>
        )}
        <div className="aq-sheet__actions">
          <Link className="aq-sheet__more" to={`/contracts/${encodeURIComponent(goszakupId)}`}>
            {t('map.preview_more')} →
          </Link>
          <button type="button" className="aq-sheet__close" onClick={onClose}>
            {t('map.preview_close')}
          </button>
        </div>
      </div>
    </div>
  );
}
