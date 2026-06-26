import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { components } from '../../shared/api/schema.gen';
import type { Lang } from '../../shared/i18n';
import { formatMoney, formatDate } from '../../shared/i18n/format';
import { DataState, dataStateFromValueState } from '../../shared/state/DataState';
import {
  FlagBadge,
  MethodologyDialog,
  apiToViewFlag,
  apiToMethodologyTarget,
  type MethodologyTarget,
} from '../flag';
import { ErrorBoundary } from '../../shared/ui/ErrorBoundary';
import { ReportError, ReportErrorForm, reportErrorMailto, type ReportTarget } from '../share';
import '../flag/flag.css';
import './contract-card.css';

type Contract = components['schemas']['Contract'];
type StringField = components['schemas']['StringField'];

// Value — честный рендер поля: значение при state=ok, иначе явное состояние (никогда «0»/пусто).
function Value({ field, format }: { field: StringField; format?: (v: string) => string }) {
  if (field.state === 'ok' && field.value !== null) {
    return <>{format ? format(field.value) : field.value}</>;
  }
  // state='ok' но value=null (wire-тип это допускает) — честность: «нет данных», а НЕ пустой
  // success-фрагмент (review-фикс P1: дыра ok+null в honest-автомате).
  const state = field.state === 'ok' ? 'no_data' : field.state;
  return <DataState kind={dataStateFromValueState(state)} state={state} />;
}

export function ContractCard({
  contract,
  lang,
  autoReport = false,
}: {
  contract: Contract;
  lang: Lang;
  autoReport?: boolean; // Story 5.4: deep-link ?report — сразу открыть форму (цель Epic 7)
}) {
  const { t } = useTranslation('chrome');
  // subject — в активном языке; lang-атрибут DOM обязателен (доменные данные на языке источника).
  const subject = lang === 'kk' ? contract.subject_kk : contract.subject_ru;
  const src = contract.source_url;
  const sourceUrl = src.state === 'ok' && src.value !== null ? src.value : null;

  // Story 5.1: флаги из РЕАЛЬНОГО compute Epic 4 через API (НЕ захардкоженный contractStories.ts). Честная
  // реконструкция состояний уже на бэке (resolveContractFlags, AC4): raised → бейдж под gate нейтральности;
  // not_raised / insufficient_data → видимая строка статуса (а не молчание = «всё чисто»).
  const raisedApiFlags = contract.flags.filter((f) => f.state === 'raised');
  const nonRaised = contract.flags.filter((f) => f.state !== 'raised');

  const [methTarget, setMethTarget] = useState<MethodologyTarget | null>(null);
  const reportHref = reportErrorMailto(
    t('report_error.subject', { id: contract.goszakup_contract_id }),
  );

  // Story 5.4: форма «Сообщить об ошибке» (FR-28) достижима у каждого флага (бейдж + методика) и из подвала.
  // Привязка к контракту из контекста. autoReport (deep-link ?report) сразу открывает форму.
  const flagReportTarget: ReportTarget = {
    kind: 'flag_error',
    subjectType: 'contract',
    subjectRef: contract.goszakup_contract_id,
    ...(sourceUrl ? { sourceUrl } : {}),
  };
  const [reportTarget, setReportTarget] = useState<ReportTarget | null>(
    autoReport
      ? {
          kind: 'data_error',
          subjectType: 'contract',
          subjectRef: contract.goszakup_contract_id,
          ...(sourceUrl ? { sourceUrl } : {}),
        }
      : null,
  );

  const act = contract.act;
  const actSourceUrl =
    act.source_url.state === 'ok' && act.source_url.value !== null ? act.source_url.value : null;

  return (
    <article className="contract-card" aria-labelledby="contract-subject">
      <p className="contract-card__overlabel">
        {t('contract.overlabel')} ·{' '}
        <Value
          field={contract.direction}
          format={(v) => t(`contract.direction.${v}`, { defaultValue: v })}
        />
      </p>
      <h2 id="contract-subject" className="contract-card__subject" lang={lang}>
        <Value field={subject} />
      </h2>
      <p className="contract-card__amount" data-testid="contract-amount">
        <Value field={contract.amount_tng} format={(v) => formatMoney(v, lang)} />
      </p>
      <dl className="contract-card__meta">
        <div className="contract-card__row">
          <dt>{t('contract.field.status')}</dt>
          <dd>
            <Value
              field={contract.status}
              format={(v) => t(`contract.status.${v}`, { defaultValue: v })}
            />
          </dd>
        </div>
        <div className="contract-card__row">
          <dt>{t('contract.field.sign_date')}</dt>
          <dd>
            <Value field={contract.sign_date} format={(v) => formatDate(v, lang)} />
          </dd>
        </div>
        <div className="contract-card__row">
          <dt>{t('contract.field.plan_start')}</dt>
          <dd>
            <Value field={contract.plan_start} format={(v) => formatDate(v, lang)} />
          </dd>
        </div>
        <div className="contract-card__row">
          <dt>{t('contract.field.plan_end')}</dt>
          <dd>
            <Value field={contract.plan_end} format={(v) => formatDate(v, lang)} />
          </dd>
        </div>
        {/* Заказчик/Подрядчик (FR-10) — данные орг ждут Epic 2; пока честный no_data, не выдумываем. */}
        <div className="contract-card__row">
          <dt>{t('contract.field.customer')}</dt>
          <dd lang="ru">
            <Value field={contract.customer} />
          </dd>
        </div>
        <div className="contract-card__row">
          <dt>{t('contract.field.supplier')}</dt>
          <dd lang="ru">
            <Value field={contract.supplier} />
          </dd>
        </div>
        <div className="contract-card__row">
          <dt>{t('contract.field.kato_code')}</dt>
          <dd>
            <Value field={contract.kato_code} />
          </dd>
        </div>
      </dl>

      {/* Акт приёмки (FR-11): только при наличии; иначе ничего (источник доверия — ссылка на первоисточник). */}
      {act.present ? (
        <section className="contract-card__act" aria-label={t('contract.act.heading')}>
          <h3 className="contract-card__act-heading">{t('contract.act.heading')}</h3>
          <dl className="contract-card__meta">
            <div className="contract-card__row">
              <dt>{t('contract.act.date')}</dt>
              <dd>
                <Value field={act.act_date} format={(v) => formatDate(v, lang)} />
              </dd>
            </div>
            <div className="contract-card__row">
              <dt>{t('contract.act.signer')}</dt>
              <dd lang="ru">
                <Value field={act.signer} />
              </dd>
            </div>
          </dl>
          {actSourceUrl !== null ? (
            <a
              className="contract-card__source"
              href={actSourceUrl}
              target="_blank"
              rel="noopener noreferrer"
            >
              {t('contract.act.source')} ↗
            </a>
          ) : null}
        </section>
      ) : null}

      {sourceUrl !== null ? (
        <a
          className="contract-card__source"
          href={sourceUrl}
          target="_blank"
          rel="noopener noreferrer"
        >
          {t('contract.source')} ↗
        </a>
      ) : null}

      <section className="contract-card__signals">
        <h3 className="contract-card__signals-heading">
          {raisedApiFlags.length > 0
            ? t('contract.signals_heading', { count: raisedApiFlags.length })
            : t('contract.signals_none_heading')}
        </h3>
        <p className="contract-card__signals-note">{t('contract.signals_note')}</p>

        {raisedApiFlags.map((f) => {
          const view = apiToViewFlag(f);
          if (view === null) return null;
          return (
            <FlagBadge
              key={f.flag_id}
              flag={view}
              lang={lang}
              onOpenMethodology={() => setMethTarget(apiToMethodologyTarget(f))}
              reportErrorHref={reportHref}
              onReportError={() => setReportTarget(flagReportTarget)}
            />
          );
        })}

        {/* Честная реконструкция видимой строкой (AC4): «проверено, сигнала нет» ≠ «недостаточно данных».
            Story 5.3 (AC-2): строка кликабельна → открывает методику (формула/пороги + какого порога не хватило). */}
        {nonRaised.length > 0 ? (
          <ul className="contract-card__flag-status" aria-label={t('contract.signals_status_heading')}>
            {nonRaised.map((f) => (
              <li key={f.flag_id}>
                <button
                  type="button"
                  className="contract-card__flag-status-row"
                  aria-haspopup="dialog"
                  onClick={() => setMethTarget(apiToMethodologyTarget(f))}
                >
                  <span>{t(`flag.${f.flag_id}.name`)}</span>
                  <span className="contract-card__flag-state">{t(`flag_state.${f.state}`)}</span>
                </button>
              </li>
            ))}
          </ul>
        ) : null}

        {/* AC5: монополия/РНУ — contractor-субъект → карточка подрядчика (здесь вне охвата, честно). */}
        <p className="contract-card__contractor-note">{t('contract.contractor_signals_note')}</p>

        <ReportError
          contractId={contract.goszakup_contract_id}
          className="contract-card__report"
        />
      </section>

      {methTarget && (
        // key по flagId: при смене флаг→флаг (truthy→truthy) ErrorBoundary РЕМОНТируется → сбрасывает hasError,
        // иначе застрявший error-fallback показался бы для следующего, исправного флага (review-фикс 5.3).
        <ErrorBoundary
          key={methTarget.flagId}
          fallback={
            <div
              className="aq-meth-backdrop"
              role="alertdialog"
              aria-label={t('error.title')}
              onClick={() => setMethTarget(null)}
            >
              <div className="aq-meth" onClick={(e) => e.stopPropagation()}>
                <p>{t('error.body')}</p>
                <button
                  type="button"
                  className="aq-meth__close"
                  onClick={() => setMethTarget(null)}
                >
                  {t('methodology.close')}
                </button>
              </div>
            </div>
          }
        >
          <MethodologyDialog
            target={methTarget}
            lang={lang}
            sourceUrl={sourceUrl}
            reportErrorHref={reportHref}
            onReportError={() => setReportTarget(flagReportTarget)}
            onClose={() => setMethTarget(null)}
          />
        </ErrorBoundary>
      )}

      {reportTarget && (
        <ReportErrorForm target={reportTarget} onClose={() => setReportTarget(null)} />
      )}
    </article>
  );
}
