import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { components } from '../../shared/api/schema.gen';
import type { Lang } from '../../shared/i18n';
import { formatMoney, formatDate } from '../../shared/i18n/format';
import { DataState, dataStateFromValueState } from '../../shared/state/DataState';
import { FlagBadge, MethodologyDialog, flagsFor, type ContractFlag } from '../flag';
import { ReportError, reportErrorMailto } from '../share';
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

export function ContractCard({ contract, lang }: { contract: Contract; lang: Lang }) {
  const { t } = useTranslation('chrome');
  // subject — в активном языке; lang-атрибут DOM обязателен (доменные данные на языке источника).
  const subject = lang === 'kk' ? contract.subject_kk : contract.subject_ru;
  const src = contract.source_url;
  const sourceUrl = src.state === 'ok' && src.value !== null ? src.value : null;

  // Ручные флаги-истории (Story 1.9). Gate нейтральности: бейдж публикуется только с путём к методике
  // (onOpenMethodology) И дверью «Сообщить об ошибке» (reportErrorHref) — оба обязательны.
  const flags = flagsFor(contract.goszakup_contract_id);
  const [methFlag, setMethFlag] = useState<ContractFlag | null>(null);
  const reportHref = reportErrorMailto(
    t('report_error.subject', { id: contract.goszakup_contract_id }),
  );

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
        <div className="contract-card__row">
          <dt>{t('contract.field.kato_code')}</dt>
          <dd>
            <Value field={contract.kato_code} />
          </dd>
        </div>
      </dl>

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

      {flags.length > 0 && (
        <section className="contract-card__signals">
          <h3 className="contract-card__signals-heading">
            {t('contract.signals_heading', { count: flags.length })}
          </h3>
          <p className="contract-card__signals-note">{t('contract.signals_note')}</p>
          {flags.map((f) => (
            <FlagBadge
              key={f.flagId}
              flag={f}
              lang={lang}
              onOpenMethodology={() => setMethFlag(f)}
              reportErrorHref={reportHref}
            />
          ))}
          <ReportError
            contractId={contract.goszakup_contract_id}
            className="contract-card__report"
          />
        </section>
      )}

      {methFlag && (
        <MethodologyDialog
          flag={methFlag}
          lang={lang}
          sourceUrl={sourceUrl}
          reportErrorHref={reportHref}
          onClose={() => setMethFlag(null)}
        />
      )}
    </article>
  );
}
