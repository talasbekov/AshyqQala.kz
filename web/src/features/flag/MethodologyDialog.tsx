import { useRef } from 'react';
import { useTranslation } from 'react-i18next';
import type { Lang } from '../../shared/i18n';
import { formatMoney, formatDateSafe } from '../../shared/i18n/format';
import { Icon } from '../../shared/ui/Icon';
import { useDialogFocus } from '../../shared/ui/useDialogFocus';
import type { MethodologyTarget } from './apiFlag';
import { useMethodology } from './useMethodology';

// Экран методики флага (Story 5.3, FR-23/NFR-5). role=dialog + полный focus-trap/возврат фокуса на
// триггер (Story 3.5, useDialogFocus — закрыт долг deferred:158). Формула (×factor) и ПОРОГИ
// показываются ВСЕГДА из ЕДИНОГО источника /api/methodology
// (не литералы, не только evidence raised-флагов) — работает и для не-raised меток (AC-2). При raised — числа
// из evidence (воркшит, AC-3); при не-raised — «сигнал не выставлен» + какого порога не хватило, без выдумки.
export function MethodologyDialog({
  target,
  lang,
  sourceUrl,
  reportErrorHref,
  onReportError,
  onClose,
}: {
  target: MethodologyTarget;
  lang: Lang;
  sourceUrl: string | null;
  reportErrorHref: string;
  onReportError?: () => void; // Story 5.4: открыть форму вместо mailto (gate-дверь сохраняется)
  onClose: () => void;
}) {
  const { t } = useTranslation('chrome');
  const ref = useRef<HTMLDivElement>(null);
  // Focus-trap + Escape + возврат фокуса на триггер (Story 3.5). Escape обрабатывает верхний слой
  // стека (форма ошибки поверх методики закрывается первой — существующий паттерн карточки).
  useDialogFocus(ref, onClose);

  const { data: meth, isError: methError } = useMethodology();
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
        aria-modal="true"
        aria-labelledby="aq-meth-title"
        tabIndex={-1}
        onClick={(e) => e.stopPropagation()}
      >
        <p className="aq-meth__overlabel">{t('methodology.overlabel')}</p>
        <h2 id="aq-meth-title" className="aq-meth__title">
          {t(`flag.${target.flagId}.summary`)}
        </h2>
        <p className="aq-meth__frame">
          <Icon name="flag" /> {t('methodology.frame_note')}
        </p>
        <p className="aq-meth__simple">{t('methodology.simple_note')}</p>

        {/* Честность (review-фикс 5.3): пороги грузятся из /api/methodology. Пока их НЕТ — НЕ заявляем «показаны
            ВСЕГДА» (см. always_shown ниже), а честно говорим «загружаются»/«недоступна». */}
        {th === undefined && (
          <p className="aq-meth__note" role="status">
            {methError
              ? t('methodology.thresholds_unavailable')
              : t('methodology.thresholds_loading')}
          </p>
        )}

        {/* Формула — когда пороги загружены (factor из params). */}
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
            {/* review-фикс 5.3: порог из СОБСТВЕННОГО deviation_factor evidence (самосогласованный воркшит,
                FR-23), НЕ из params — воркшит переиспользуем даже при незагруженных params; label↔значение совпадают. */}
            {ev.thresholdPerKm && ev.deviationFactor !== undefined && (
              <div className="aq-meth__row">
                <dt>{t('methodology.ev_threshold', { factor: ev.deviationFactor })}</dt>
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
            {/* review-фикс 5.3 (AC-2): single_participant тоже объясняет, какое условие не выполнено. */}
            {!isPrice && <p>{t('methodology.insufficient_single_participant')}</p>}
            <p className="aq-meth__note">{t('methodology.insufficient_note')}</p>
            {/* «пороги показаны ВСЕГДА» — только когда они и правда загружены (иначе это ложь, review-фикс 5.3). */}
            {th !== undefined && <p className="aq-meth__note">{t('methodology.always_shown')}</p>}
          </div>
        )}

        {/* as_of: версия методики + дата расчёта (snapshot_id/permalink — Story 5.6). */}
        <p className="aq-meth__as-of">
          {target.detectedAt
            ? t('methodology.as_of', {
                version: target.methodologyVersion || '—',
                date: formatDateSafe(target.detectedAt, lang),
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
          {onReportError ? (
            <button
              type="button"
              className="aq-meth__link"
              aria-haspopup="dialog"
              onClick={onReportError}
            >
              {t('report_error.link')}
            </button>
          ) : (
            <a className="aq-meth__link" href={reportErrorHref}>
              {t('report_error.link')}
            </a>
          )}
          <button type="button" className="aq-meth__close" onClick={onClose}>
            {t('methodology.close')}
          </button>
        </div>
      </div>
    </div>
  );
}
