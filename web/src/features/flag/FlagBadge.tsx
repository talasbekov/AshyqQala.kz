import { useTranslation } from 'react-i18next';
import type { Lang } from '../../shared/i18n';
import { formatDateSafe } from '../../shared/i18n/format';
import { Icon } from '../../shared/ui/Icon';
import type { ContractFlag } from './contractStories';

// Бейдж нейтрального флага (Story 1.9, AC1/AC2). GATE НЕЙТРАЛЬНОСТИ — пропсы onOpenMethodology И
// reportErrorHref ОБЯЗАТЕЛЬНЫ (TS required): бейдж нельзя отрендерить без пути к методике и двери
// «Сообщить об ошибке» → «флаг без сопровождения не публикуется» enforced типом. [epics.md:1011–1013; AR-14]
export function FlagBadge({
  flag,
  lang,
  onOpenMethodology,
  reportErrorHref,
  onReportError,
}: {
  flag: ContractFlag;
  lang: Lang;
  onOpenMethodology: () => void;
  reportErrorHref: string;
  onReportError?: () => void; // Story 5.4: открыть форму вместо mailto (gate-дверь сохраняется)
}) {
  const { t } = useTranslation('chrome');
  const summary = t(`flag.${flag.flagId}.summary`);
  const raised = flag.flagState === 'raised';

  // Текст ВСЕГДА нейтральный. raised → «<сводка> — сигнал, требующий проверки» (амбер, глиф «!»);
  // insufficient → «<сводка>: недостаточно сопоставимых данных» (честно, приглушённо, глиф «≈», НЕ тревога).
  const text = raised
    ? `${summary} — ${t('frame.signal')}`
    : `${summary}: ${t('flag.insufficient')}`;

  return (
    <div className={`aq-flag aq-flag--${raised ? 'raised' : 'insufficient'}`}>
      <button
        type="button"
        className="aq-flag__badge"
        aria-label={text}
        onClick={onOpenMethodology}
      >
        <Icon name={raised ? 'flag' : 'insufficient'} />
        <span className="aq-flag__text">{text}</span>
      </button>
      {flag.detectedAt ? (
        <p className="aq-flag__date">
          {t('flag.detected_at', { date: formatDateSafe(flag.detectedAt, lang) })}
        </p>
      ) : null}
      <div className="aq-flag__links">
        <button type="button" className="aq-flag__link" onClick={onOpenMethodology}>
          {t('flag.how_calculated')} ↗
        </button>
        {onReportError ? (
          <button
            type="button"
            className="aq-flag__link"
            aria-haspopup="dialog"
            onClick={onReportError}
          >
            {t('report_error.link')}
          </button>
        ) : (
          <a className="aq-flag__link" href={reportErrorHref}>
            {t('report_error.link')}
          </a>
        )}
      </div>
    </div>
  );
}
