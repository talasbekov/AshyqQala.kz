import { useEffect, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import type { Lang } from '../../shared/i18n';
import { formatMoney, formatDate } from '../../shared/i18n/format';
import { Icon } from '../../shared/ui/Icon';
import type { ContractFlag } from './contractStories';

// Экран методики флага (Story 1.9, AC1). role=dialog, базовый фокус + Escape (полный focus-trap —
// Epic 3 Story 3.5). Показывает формулу ×1.5, ключ сопоставимости, evidence (raised) ИЛИ честное
// «недостаточно данных» (<5) с формулой/порогами ВСЕГДА. Числа — данные, не проза. [UX key-flag-methodology]
export function MethodologyDialog({
  flag,
  lang,
  sourceUrl,
  reportErrorHref,
  onClose,
}: {
  flag: ContractFlag;
  lang: Lang;
  sourceUrl: string | null;
  reportErrorHref: string;
  onClose: () => void;
}) {
  const { t } = useTranslation('chrome');
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    ref.current?.focus();
  }, []);

  const ev = flag.evidence;
  const raised = flag.flagState === 'raised';
  const factor = ev.deviationFactor ?? '1.5';

  return (
    <div className="aq-meth-backdrop" onClick={onClose}>
      <div
        ref={ref}
        className="aq-meth"
        role="dialog"
        aria-modal="false"
        aria-labelledby="aq-meth-title"
        tabIndex={-1}
        onClick={(e) => e.stopPropagation()}
        onKeyDown={(e) => {
          if (e.key === 'Escape') onClose();
        }}
      >
        <p className="aq-meth__overlabel">{t('methodology.overlabel')}</p>
        <h2 id="aq-meth-title" className="aq-meth__title">
          {t(`flag.${flag.flagId}.summary`)}
        </h2>
        <p className="aq-meth__frame">
          <Icon name="flag" /> {t('methodology.frame_note')}
        </p>
        <p className="aq-meth__simple">{t('methodology.simple_note')}</p>

        {flag.flagId === 'price_per_km' && (
          <p className="aq-meth__formula">{t('methodology.formula_price', { factor })}</p>
        )}
        {flag.flagId === 'single_participant' && (
          <p className="aq-meth__rule">{t('methodology.single_participant_rule')}</p>
        )}

        <dl className="aq-meth__key">
          {ev.direction && (
            <div className="aq-meth__row">
              <dt>{t('methodology.key_direction')}</dt>
              <dd>{t(`contract.direction.${ev.direction}`, { defaultValue: ev.direction })}</dd>
            </div>
          )}
          {ev.katoCode && (
            <div className="aq-meth__row">
              <dt>{t('methodology.key_district')}</dt>
              <dd>{ev.katoCode}</dd>
            </div>
          )}
          {ev.windowMonths !== undefined && (
            <div className="aq-meth__row">
              <dt>{t('methodology.key_window')}</dt>
              <dd>{t('methodology.key_window_value', { months: ev.windowMonths })}</dd>
            </div>
          )}
          {ev.minSample !== undefined && (
            <div className="aq-meth__row">
              <dt>{t('methodology.key_min_sample')}</dt>
              <dd>{ev.minSample}</dd>
            </div>
          )}
        </dl>

        {!raised ? (
          <div className="aq-meth__insufficient">
            <p className="aq-meth__insufficient-title">
              <Icon name="insufficient" /> {t('methodology.insufficient_title')}
            </p>
            <p>
              {t('methodology.insufficient_body', {
                sample: ev.sampleSize ?? 0,
                min: ev.minSample ?? 5,
              })}
            </p>
            <p className="aq-meth__note">{t('methodology.insufficient_note')}</p>
            <p className="aq-meth__note">{t('methodology.always_shown')}</p>
          </div>
        ) : flag.flagId === 'single_participant' ? (
          <dl className="aq-meth__ev">
            {ev.participants !== undefined && (
              <div className="aq-meth__row aq-meth__row--hl">
                <dt>{t('methodology.ev_participants')}</dt>
                <dd>{ev.participants}</dd>
              </div>
            )}
          </dl>
        ) : (
          <dl className="aq-meth__ev">
            {ev.sampleSize !== undefined && (
              <div className="aq-meth__row">
                <dt>{t('methodology.ev_sample')}</dt>
                <dd>{ev.sampleSize}</dd>
              </div>
            )}
            {ev.medianPerKm && (
              <div className="aq-meth__row">
                <dt>{t('methodology.ev_median')}</dt>
                <dd>{formatMoney(ev.medianPerKm, lang)}</dd>
              </div>
            )}
            {ev.thresholdPerKm && (
              <div className="aq-meth__row">
                <dt>{t('methodology.ev_threshold', { factor })}</dt>
                <dd>{formatMoney(ev.thresholdPerKm, lang)}</dd>
              </div>
            )}
            {ev.thisPerKm && (
              <div className="aq-meth__row aq-meth__row--hl">
                <dt>{t('methodology.ev_this')}</dt>
                <dd>{formatMoney(ev.thisPerKm, lang)}</dd>
              </div>
            )}
          </dl>
        )}

        <dl className="aq-meth__meta">
          <div className="aq-meth__row">
            <dt>{t('methodology.version')}</dt>
            <dd>{flag.methodologyVersion}</dd>
          </div>
          <div className="aq-meth__row">
            <dt>{t('methodology.computed_at')}</dt>
            <dd>{formatDate(flag.detectedAt, lang)}</dd>
          </div>
        </dl>

        {raised && flag.flagId === 'price_per_km' && (
          <p className="aq-meth__recalc">{t('methodology.recalc_note')}</p>
        )}

        <div className="aq-meth__actions">
          {sourceUrl && (
            <a className="aq-meth__link" href={sourceUrl} target="_blank" rel="noopener noreferrer">
              {t('contract.source')} ↗
            </a>
          )}
          <a className="aq-meth__link" href={reportErrorHref}>
            {t('report_error.link')}
          </a>
          <button type="button" className="aq-meth__close" onClick={onClose}>
            {t('methodology.close')}
          </button>
        </div>
      </div>
    </div>
  );
}
