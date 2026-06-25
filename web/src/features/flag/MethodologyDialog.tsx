import { useEffect, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import type { Lang } from '../../shared/i18n';
import { formatMoney, formatDate } from '../../shared/i18n/format';
import { Icon } from '../../shared/ui/Icon';
import type { MethodologyTarget } from './apiFlag';
import { useMethodology } from './useMethodology';

// Экран методики флага (Story 5.3, FR-23/NFR-5). role=dialog, базовый фокус + Escape (полный focus-trap —
// Epic 3 Story 3.5). Формула (×factor) и ПОРОГИ показываются ВСЕГДА из ЕДИНОГО источника /api/methodology
// (не литералы, не только evidence raised-флагов) — работает и для не-raised меток (AC-2). При raised — числа
// из evidence (воркшит, AC-3); при не-raised — «сигнал не выставлен» + какого порога не хватило, без выдумки.
export function MethodologyDialog({
  target,
  lang,
  sourceUrl,
  reportErrorHref,
  onClose,
}: {
  target: MethodologyTarget;
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

  const { data: meth } = useMethodology();
  const th = meth?.thresholds;
  const ev = target.evidence;
  const raised = target.state === 'raised';
  const isPrice = target.flagId === 'price_per_km';

  // Пороги — ТОЛЬКО из methodology_params (th); НЕ дефолтим (review-фикс 5-1: без фабрикации). Пока не
  // загружено — соответствующий блок не рендерим (кэш Infinity → практически мгновенно).
  const factor = th !== undefined ? String(th.price_per_km_deviation_factor) : undefined;
  const minSample = th?.min_sample;
  const months = th?.comparability_window_months;

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
          {t(`flag.${target.flagId}.summary`)}
        </h2>
        <p className="aq-meth__frame">
          <Icon name="flag" /> {t('methodology.frame_note')}
        </p>
        <p className="aq-meth__simple">{t('methodology.simple_note')}</p>

        {/* Формула — ВСЕГДА (пороги из params). */}
        {isPrice && factor !== undefined && (
          <p className="aq-meth__formula">{t('methodology.formula_price', { factor })}</p>
        )}
        {!isPrice && <p className="aq-meth__rule">{t('methodology.single_participant_rule')}</p>}

        {/* Ключ сопоставимости + пороги — ВСЕГДА из methodology_params. */}
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
          {isPrice && months !== undefined && (
            <div className="aq-meth__row">
              <dt>{t('methodology.key_window')}</dt>
              <dd>{t('methodology.key_window_value', { months })}</dd>
            </div>
          )}
          {isPrice && minSample !== undefined && (
            <div className="aq-meth__row">
              <dt>{t('methodology.key_min_sample')}</dt>
              <dd>{minSample}</dd>
            </div>
          )}
        </dl>

        {/* Критерий «похожести» (AC-3) — для цены/км. */}
        {isPrice && months !== undefined && minSample !== undefined && (
          <p className="aq-meth__similarity">
            {t('methodology.similarity_criterion', { months, min: minSample })}
          </p>
        )}

        {/* Состояние сигнала. */}
        {raised ? (
          <dl className="aq-meth__ev" aria-label={t('methodology.worksheet_heading')}>
            {ev.participants !== undefined && (
              <div className="aq-meth__row aq-meth__row--hl">
                <dt>{t('methodology.ev_participants')}</dt>
                <dd>{ev.participants}</dd>
              </div>
            )}
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
            {ev.thresholdPerKm && factor !== undefined && (
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
        ) : (
          // Не-raised: сигнал не выставлен. Формула/пороги показаны ВЫШЕ; здесь — честно, какого порога не хватило.
          <div className="aq-meth__insufficient">
            <p className="aq-meth__insufficient-title">
              <Icon name="insufficient" /> {t('methodology.not_raised_title')}
            </p>
            {isPrice && minSample !== undefined && months !== undefined && (
              <p>{t('methodology.insufficient_price', { min: minSample, months })}</p>
            )}
            <p className="aq-meth__note">{t('methodology.insufficient_note')}</p>
            <p className="aq-meth__note">{t('methodology.always_shown')}</p>
          </div>
        )}

        {/* as_of: версия методики + дата расчёта (snapshot_id/permalink — Story 5.6). */}
        <p className="aq-meth__as-of">
          {target.detectedAt
            ? t('methodology.as_of', {
                version: target.methodologyVersion || '—',
                date: formatDate(target.detectedAt, lang),
              })
            : t('methodology.as_of_no_date', { version: target.methodologyVersion || '—' })}
        </p>

        {raised && isPrice && <p className="aq-meth__recalc">{t('methodology.recalc_note')}</p>}

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
