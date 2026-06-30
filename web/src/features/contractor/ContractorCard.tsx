import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';
import type { components } from '../../shared/api/schema.gen';
import type { Lang } from '../../shared/i18n';
import { formatMoney } from '../../shared/i18n/format';
import { DataState, dataStateFromValueState } from '../../shared/state/DataState';
import { reportErrorMailto } from '../share';
import './contractor-card.css';

type Contractor = components['schemas']['Contractor'];
type StringField = components['schemas']['StringField'];

// Value — честный рендер поля: значение при state=ok, иначе явное состояние (никогда «0»/пусто).
function Value({ field, format }: { field: StringField; format?: (v: string) => string }) {
  if (field.state === 'ok' && field.value !== null) {
    return <>{format ? format(field.value) : field.value}</>;
  }
  const state = field.state === 'ok' ? 'no_data' : field.state;
  return <DataState kind={dataStateFromValueState(state)} state={state} />;
}

export function ContractorCard({ contractor, lang }: { contractor: Contractor; lang: Lang }) {
  const { t } = useTranslation('chrome');
  const c = contractor;
  const name = lang === 'kk' ? c.name_kk : c.name_ru;

  // Профиль (FR-13): incomplete («профиль неполный») / unverified («профиль уточняется» — флаги НЕ основание).
  const profileState = c.profile.state; // 'incomplete' | 'unverified'

  // Флаги (monopoly/rnu) — честная реконструкция на бэке. raised → нейтральный бейдж (сигнал, требующий
  // проверки); прочие → видимая строка статуса (не молчание = «чисто»). При unverified все insufficient.
  const raised = c.flags.filter((f) => f.state === 'raised');
  const nonRaised = c.flags.filter((f) => f.state !== 'raised');

  // РНУ-метки (FR-14): активность реконструирована из дат на бэке. Только активные показываем меткой;
  // снятые (авто по end_date) — честная муть «снята». КАЖДАЯ метка несёт СВОЙ source-link + СВОЮ «Сообщить об ошибке» (AC-3).
  const activeRnu = c.rnu_marks.filter((m) => m.active);
  const clearedRnu = c.rnu_marks.filter((m) => !m.active);

  // Источник орг (реестр первоисточника) — служит {source-link} для агрегированных флагов (AC-3: своя ссылка на метку).
  const orgSrc =
    c.source_url.state === 'ok' && c.source_url.value !== null ? c.source_url.value : null;

  return (
    <article className="contractor-card" aria-labelledby="contractor-name">
      <p className="contractor-card__overlabel">{t('contractor.overlabel')}</p>
      <h2 id="contractor-name" className="contractor-card__name" lang={lang}>
        <Value field={name} />
      </h2>
      <p className="contractor-card__bin">
        {t('contractor.field.bin')}: <span data-testid="contractor-bin">{c.bin}</span>
      </p>

      {/* Честное состояние профиля (FR-13): всегда видимо, не скрываем неполноту. */}
      <p
        className={`contractor-card__profile contractor-card__profile--${profileState}`}
        role="status"
        data-testid="contractor-profile"
      >
        {t(`contractor.profile.${profileState}`)}
      </p>

      <dl className="contractor-card__meta">
        <div className="contractor-card__row">
          <dt>{t('contractor.field.contract_count')}</dt>
          <dd>
            <Value field={c.contract_count} />
          </dd>
        </div>
        <div className="contractor-card__row">
          <dt>{t('contractor.field.total_amount')}</dt>
          <dd data-testid="contractor-total">
            <Value field={c.total_amount_tng} format={(v) => formatMoney(v, lang)} />
          </dd>
        </div>
        <div className="contractor-card__row">
          <dt>{t('contractor.field.regions')}</dt>
          <dd>
            {c.regions.length > 0 ? (
              c.regions.join(', ')
            ) : (
              <DataState kind="empty" state="no_data" />
            )}
          </dd>
        </div>
      </dl>

      {/* Список контрактов подрядчика (FR-13). Пусто до наполнения связи (Story 2.2) → честная плашка. */}
      <section
        className="contractor-card__contracts"
        aria-label={t('contractor.contracts.heading')}
      >
        <h3 className="contractor-card__contracts-heading">{t('contractor.contracts.heading')}</h3>
        {c.contracts.length > 0 ? (
          <ul className="contractor-card__contract-list">
            {c.contracts.map((ct) => {
              const subj = lang === 'kk' ? ct.subject_kk : ct.subject_ru;
              return (
                <li key={ct.goszakup_contract_id} className="contractor-card__contract-item">
                  <Link
                    to={`/contracts/${encodeURIComponent(ct.goszakup_contract_id)}`}
                    lang={lang}
                  >
                    <Value field={subj} />
                  </Link>
                  <span className="contractor-card__contract-amount">
                    <Value field={ct.amount_tng} format={(v) => formatMoney(v, lang)} />
                  </span>
                </li>
              );
            })}
          </ul>
        ) : (
          <p className="contractor-card__contracts-empty" role="status">
            {t('contractor.contracts.empty')}
          </p>
        )}
      </section>

      {/* Агрегированные сигналы (FR-21/FR-22). При unverified-профиле они insufficient (флаги не основание). */}
      <section className="contractor-card__signals">
        <h3 className="contractor-card__signals-heading">
          {raised.length > 0
            ? t('contractor.signals_heading', { count: raised.length })
            : t('contractor.signals_none_heading')}
        </h3>
        <p className="contractor-card__signals-note">{t('contract.signals_note')}</p>

        {raised.map((f) => (
          <div key={f.flag_id} className="contractor-card__flag" data-testid={`flag-${f.flag_id}`}>
            <span className="contractor-card__flag-name">
              {t(`contractor.flag.${f.flag_id}.name`)}
            </span>
            <span className="contractor-card__flag-state">{t(`flag_state.${f.state}`)}</span>
            {/* AC-3: у КАЖДОЙ метки СВОЯ {source-link} (первоисточник для проверки) + СВОЯ «Сообщить об ошибке». */}
            {orgSrc !== null ? (
              <a
                className="contractor-card__source"
                href={orgSrc}
                target="_blank"
                rel="noopener noreferrer"
              >
                {t('contract.source')} ↗
              </a>
            ) : null}
            <a
              className="contractor-card__report"
              href={reportErrorMailto(
                t('contractor.report_subject', { bin: c.bin, mark: f.flag_id }),
              )}
            >
              {t('report_error.link')}
            </a>
          </div>
        ))}

        {nonRaised.length > 0 ? (
          <ul
            className="contractor-card__flag-status"
            aria-label={t('contract.signals_status_heading')}
          >
            {nonRaised.map((f) => (
              <li key={f.flag_id} className="contractor-card__flag-status-row">
                <span>{t(`contractor.flag.${f.flag_id}.name`)}</span>
                <span className="contractor-card__flag-state">{t(`flag_state.${f.state}`)}</span>
              </li>
            ))}
          </ul>
        ) : null}
      </section>

      {/* Метка РНУ (FR-14). «недобросовестный» допустим ТОЛЬКО как цитата названия гос-реестра (атрибуция государству). */}
      {activeRnu.length > 0 ? (
        <section className="contractor-card__rnu" aria-label={t('contractor.rnu.heading')}>
          <h3 className="contractor-card__rnu-heading">{t('contractor.rnu.heading')}</h3>
          {activeRnu.map((m, i) => {
            const src =
              m.source_url.state === 'ok' && m.source_url.value !== null
                ? m.source_url.value
                : null;
            const markKey =
              m.registry_id.state === 'ok' && m.registry_id.value !== null
                ? m.registry_id.value
                : `rnu-${i}`;
            return (
              <div key={markKey} className="contractor-card__rnu-mark" data-testid="rnu-mark">
                <p className="contractor-card__rnu-cite">{t('contractor.rnu.registry_name')}</p>
                <p className="contractor-card__rnu-date">
                  {t('contractor.rnu.since')} <Value field={m.start_date} />
                </p>
                {/* AC-3: своя {source-link} + своя локальная «Сообщить об ошибке» у КАЖДОЙ метки. */}
                {src !== null ? (
                  <a
                    className="contractor-card__source"
                    href={src}
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    {t('contractor.rnu.source')} ↗
                  </a>
                ) : null}
                <a
                  className="contractor-card__report"
                  href={reportErrorMailto(
                    t('contractor.report_subject', { bin: c.bin, mark: 'rnu' }),
                  )}
                >
                  {t('report_error.link')}
                </a>
              </div>
            );
          })}
        </section>
      ) : null}

      {clearedRnu.length > 0 ? (
        <p className="contractor-card__rnu-cleared" role="status">
          {t('contractor.rnu.cleared', { count: clearedRnu.length })}
        </p>
      ) : null}

      {c.source_url.state === 'ok' && c.source_url.value !== null ? (
        <a
          className="contractor-card__source"
          href={c.source_url.value}
          target="_blank"
          rel="noopener noreferrer"
        >
          {t('contract.source')} ↗
        </a>
      ) : null}
    </article>
  );
}
