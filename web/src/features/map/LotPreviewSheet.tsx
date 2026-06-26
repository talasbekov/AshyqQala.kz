import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { Lang } from '../../shared/i18n';
import { DataState, dataStateFromValueState } from '../../shared/state/DataState';
import { ReportErrorForm } from '../share';
import type { MapLot } from './lots';
import { safeFormatMoney } from './lots';
import type { components } from '../../shared/api/schema.gen';

type StringField = components['schemas']['StringField'];

// Value — честный рендер поля (паттерн ContractCard, Story 1.7): значение при state=ok && value!==null,
// иначе явное состояние. Дыра ok+null закрыта.
// P1: format может вернуть null (см. safeFormatMoney) — это значит «значение есть, но непредставимо
// честно» (неканоничная сумма). Тогда рендерим no_data, а НЕ роняем роут и НЕ показываем «0 ₸».
function Value({ field, format }: { field: StringField; format?: (v: string) => string | null }) {
  if (field.state === 'ok' && field.value !== null) {
    const formatted = format ? format(field.value) : field.value;
    // format вернул null → неканоничное/непредставимое значение → честное «нет данных».
    if (formatted !== null) {
      return <>{formatted}</>;
    }
    return <DataState kind={dataStateFromValueState('no_data')} state="no_data" />;
  }
  const state = field.state === 'ok' ? 'no_data' : field.state;
  return <DataState kind={dataStateFromValueState(state)} state={state} />;
}

// Нижний лист превью ЛОТА (Story 0.8, AC2). Показывает данные УЖЕ загруженного лота (наименование/сумма
// из useLots) — НЕ фетчит несуществующий контракт (контрактов в источнике нет; фетч дал бы 404 → общую
// ошибку, нарушение AC2). Блок «контракт/сигналы» — честная плашка «ожидает официального источника»
// (no_contracts), не пустота и не выдумка. Полный preview-sheet (детенты/focus-trap) — Epic 3 (Story 3.5).
export function LotPreviewSheet({
  lot,
  lang,
  onClose,
}: {
  lot: MapLot;
  lang: Lang;
  onClose: () => void;
}) {
  const { t } = useTranslation('chrome');
  const sheetRef = useRef<HTMLDivElement>(null);
  const [reporting, setReporting] = useState(false); // Story 5.4 (AC-3): «точка не там» → geo_wrong_point

  // Базовое управление фокусом (полный focus-trap — Epic 3, Story 3.5): фокус на лист при открытии.
  useEffect(() => {
    sheetRef.current?.focus();
  }, []);

  const subject = lang === 'kk' ? lot.subject_kk : lot.subject_ru;
  // P7: если ОБА наименования (kk И ru) отсутствуют, заголовок схлопывался в «нет данных» без
  // идентификатора — пользователь не понимал, какой лот открыт. Фолбэк: показываем goszakup_lot_id.
  // Если язык-текущий пуст, но другой язык есть — обычный честный рендер выбранного поля (как было).
  const fieldMissing = (f: StringField): boolean => f.state !== 'ok' || f.value === null;
  const bothSubjectsMissing = fieldMissing(lot.subject_kk) && fieldMissing(lot.subject_ru);

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
        <h2 className="aq-sheet__title" lang={lang}>
          {bothSubjectsMissing ? lot.goszakup_lot_id : <Value field={subject} />}
        </h2>
        <p className="aq-sheet__amount">
          <Value field={lot.amount_tng} format={(v) => safeFormatMoney(v, lang)} />
        </p>
        {/* AC2: контракт и сигналы по лоту ждут официального источника — честно, не пустота/выдумка. */}
        <p className="aq-sheet__awaiting">{t('map.awaiting_official_source')}</p>
        <div className="aq-sheet__actions">
          <button
            type="button"
            className="aq-sheet__close"
            aria-haspopup="dialog"
            onClick={() => setReporting(true)}
          >
            {t('report_error.geo_link')}
          </button>
          <button type="button" className="aq-sheet__close" onClick={onClose}>
            {t('map.preview_close')}
          </button>
        </div>
        {/* Форма рендерится ВНУТРИ листа (его onClick stopPropagation) — клик по её бэкдропу не закроет лист. */}
        {reporting && (
          <ReportErrorForm
            target={{
              kind: 'geo_wrong_point',
              subjectType: 'geo_object',
              subjectRef: lot.goszakup_lot_id,
            }}
            onClose={() => setReporting(false)}
          />
        )}
      </div>
    </div>
  );
}
