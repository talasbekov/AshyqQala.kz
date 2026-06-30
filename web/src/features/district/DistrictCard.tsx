import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';
import type { components } from '../../shared/api/schema.gen';
import type { Lang } from '../../shared/i18n';
import { formatMoney, formatPricePerKm } from '../../shared/i18n/format';
import { DataState, dataStateFromValueState } from '../../shared/state/DataState';
import { Icon } from '../../shared/ui/Icon';
import './district-card.css';

type District = components['schemas']['DistrictAggregates'];
type StringField = components['schemas']['StringField'];

// Value — честный рендер поля: значение при state=ok, иначе явное состояние (никогда «0»/пусто).
function Value({ field, format }: { field: StringField; format?: (v: string) => string }) {
  if (field.state === 'ok' && field.value !== null) {
    return <>{format ? format(field.value) : field.value}</>;
  }
  const state = field.state === 'ok' ? 'no_data' : field.state;
  return <DataState kind={dataStateFromValueState(state)} state={state} />;
}

export function DistrictCard({ district, lang }: { district: District; lang: Lang }) {
  const { t } = useTranslation('chrome');
  const d = district;

  // Имя района показываем ТОЛЬКО когда КАТО-код подтверждён (Story 0.1). Иначе честный фолбэк «по коду КАТО»
  // (имя не выдумываем — гардрейл «коды не выдумываем»).
  const nameField = lang === 'kk' ? d.name_kk : d.name_ru;
  const nameKnown = nameField.state === 'ok' && nameField.value !== null;
  const heading = nameKnown
    ? (nameField.value as string)
    : t('district.name_pending', { kato: d.kato });

  const flagsTotal = Number(d.active_flags_count) || 0;
  const objectCount = Number(d.contract_count) || 0;

  return (
    <article className="district-card" aria-labelledby="district-name">
      <p className="district-card__overlabel">{t('district.overlabel')}</p>
      <h2 id="district-name" className="district-card__name" lang={lang}>
        {heading}
      </h2>
      <p className="district-card__kato">
        {t('district.field.kato')}: <span data-testid="district-kato">{d.kato}</span>
      </p>
      {!nameKnown ? (
        <p className="district-card__name-note" role="status">
          {t('district.name_pending_note')}
        </p>
      ) : null}

      {/* Сводка: число объектов + сумма + число активных сигналов. */}
      <dl className="district-card__stats">
        <div className="district-card__stat">
          <dt>{t('district.field.objects')}</dt>
          <dd data-testid="district-objects-count">{d.contract_count}</dd>
        </div>
        <div className="district-card__stat">
          <dt>{t('district.field.total_amount')}</dt>
          <dd data-testid="district-total">
            <Value field={d.total_amount_tng} format={(v) => formatMoney(v, lang)} />
          </dd>
        </div>
        <div className="district-card__stat">
          <dt>{t('district.field.active_flags')}</dt>
          <dd data-testid="district-active-flags">{d.active_flags_count}</dd>
        </div>
      </dl>

      {/* container_state (AR-17): КАЖДОЕ состояние — ОТДЕЛЬНОЙ строкой; пустой/частичный экран не «всё чисто». */}
      {d.container_state.length > 0 ? (
        <ul className="district-card__container-state" aria-label={t('district.overlabel')}>
          {d.container_state.map((cs) => (
            <li key={cs} role="status" data-testid={`district-state-${cs}`}>
              {t(`district.container.${cs}`, { defaultValue: cs })}
            </li>
          ))}
        </ul>
      ) : null}

      {/* Сигналы района (FR-17): разбивка по типам, каждый в нейтральной рамке «сигнал, требующий проверки». */}
      {flagsTotal > 0 ? (
        <section className="district-card__signals">
          <h3 className="district-card__signals-heading">
            {t('district.signals_heading', { count: flagsTotal })}
          </h3>
          <p className="district-card__signals-note">{t('district.signals_note')}</p>
          <ul className="district-card__flag-list">
            {d.flags_by_type.map((f) => (
              <li
                key={f.flag_type}
                className="district-card__flag"
                data-testid={`district-flag-${f.flag_type}`}
              >
                <span className="district-card__flag-icon">
                  <Icon name="flag" />
                </span>
                <span className="district-card__flag-name">
                  {t(`district.flag.${f.flag_type}.name`, { defaultValue: f.flag_type })}
                  {' — '}
                  {t('frame.signal')}
                </span>
                <span
                  className="district-card__flag-count"
                  data-testid={`district-flag-count-${f.flag_type}`}
                >
                  {f.count}
                </span>
              </li>
            ))}
          </ul>
        </section>
      ) : null}

      {/* Медиана ₸/км · район vs город (FR-18, Story 6.4): по каждому направлению две полосы (район/город) +
          % дельта. Числа методики — из API (никаких литералов); недостоверная медиана → честное состояние
          («данные несопоставимы» / «недостаточно сопоставимых данных»), НЕ число и НЕ 0. Секция при наличии
          объектов (для пустого района пустоту несёт container_state). */}
      {objectCount > 0 && d.medians.length > 0 ? (
        <section className="district-card__medians" aria-label={t('district.median.section')}>
          <h3 className="district-card__medians-heading">{t('district.median.section')}</h3>
          {d.medians.map((m) => {
            const distOk =
              m.district_median_tng.state === 'ok' && m.district_median_tng.value !== null;
            const cityOk = m.city_median_tng.state === 'ok' && m.city_median_tng.value !== null;
            // Ширина полос — косметика (Number, не деньги): доля от большей из показанных медиан.
            const maxVal =
              Math.max(
                distOk ? Number(m.district_median_tng.value) : 0,
                cityOk ? Number(m.city_median_tng.value) : 0,
              ) || 1;
            const pctOk = m.comparison_pct.state === 'ok' && m.comparison_pct.value !== null;
            const pctNum = pctOk ? Number(m.comparison_pct.value) : 0;
            return (
              <div
                key={m.direction}
                className="district-card__median"
                data-testid={`district-median-${m.direction}`}
              >
                <h4 className="district-card__median-heading">
                  {t('district.median.heading', {
                    direction: t(`contract.direction.${m.direction}`, {
                      defaultValue: m.direction,
                    }),
                  })}
                </h4>
                <div className="district-card__median-row">
                  <span className="district-card__median-label">{heading}</span>
                  {distOk ? (
                    <span className="district-card__median-bar district-card__median-bar--district">
                      <i
                        style={{
                          width: `${Math.round((Number(m.district_median_tng.value) / maxVal) * 100)}%`,
                        }}
                      />
                    </span>
                  ) : null}
                  <span
                    className="district-card__median-val"
                    data-testid={`district-median-${m.direction}-district`}
                  >
                    <Value
                      field={m.district_median_tng}
                      format={(v) => formatPricePerKm(v, lang)}
                    />
                  </span>
                </div>
                <div className="district-card__median-row">
                  <span className="district-card__median-label">
                    {t('district.median.city_label')}
                  </span>
                  {cityOk ? (
                    <span className="district-card__median-bar district-card__median-bar--city">
                      <i
                        style={{
                          width: `${Math.round((Number(m.city_median_tng.value) / maxVal) * 100)}%`,
                        }}
                      />
                    </span>
                  ) : null}
                  <span
                    className="district-card__median-val"
                    data-testid={`district-median-${m.direction}-city`}
                  >
                    <Value field={m.city_median_tng} format={(v) => formatPricePerKm(v, lang)} />
                  </span>
                </div>
                <p
                  className="district-card__median-cmp"
                  data-testid={`district-median-${m.direction}-delta`}
                >
                  {pctOk ? (
                    t('district.median.delta', { pct: `${pctNum > 0 ? '+' : ''}${pctNum}%` })
                  ) : (
                    <Value field={m.comparison_pct} />
                  )}
                  {distOk ? (
                    <span className="district-card__median-note">
                      {' '}
                      {t('district.median.computed', {
                        // i18next reserved `count` управляет плюрализацией → число, не строка
                        // (district_sample_size приходит строкой). review-патч 6.4.
                        count: Number(m.district_sample_size),
                        version: d.median_methodology_version,
                      })}
                    </span>
                  ) : null}
                </p>
              </div>
            );
          })}
        </section>
      ) : null}

      {/* Список объектов района (FR-17 AC1): каждый — ссылка на карточку контракта. Секция только при наличии
          объектов (пустоту честно несёт container_state выше — без молчащего заголовка). */}
      {d.objects.length > 0 ? (
        <section className="district-card__objects" aria-label={t('district.objects_heading')}>
          <h3 className="district-card__objects-heading">{t('district.objects_heading')}</h3>
          {/* Честность: список bounded — если объектов в районе больше, чем в срезе, говорим прямо. */}
          {objectCount > d.objects.length ? (
            <p
              className="district-card__objects-truncated"
              role="status"
              data-testid="district-objects-truncated"
            >
              {t('district.objects_truncated', {
                shown: d.objects.length,
                total: d.contract_count,
              })}
            </p>
          ) : null}
          <ul className="district-card__object-list" data-testid="district-object-list">
            {d.objects.map((o) => {
              const subj = lang === 'kk' ? o.subject_kk : o.subject_ru;
              const subjKnown = subj.state === 'ok' && subj.value !== null;
              return (
                <li
                  key={o.goszakup_contract_id}
                  className="district-card__object"
                  data-testid="district-object-item"
                >
                  {/* При no_data-имени доступное имя ссылки = id контракта (описательно), не плашка «нет данных». */}
                  <Link to={`/contracts/${encodeURIComponent(o.goszakup_contract_id)}`} lang={lang}>
                    {subjKnown ? <Value field={subj} /> : o.goszakup_contract_id}
                  </Link>
                  <span className="district-card__object-amount">
                    <Value field={o.amount_tng} format={(v) => formatMoney(v, lang)} />
                  </span>
                  {o.has_active_flag ? (
                    <span className="district-card__object-flag">
                      <Icon name="flag" /> {t('frame.signal')}
                    </span>
                  ) : null}
                </li>
              );
            })}
          </ul>
        </section>
      ) : null}
    </article>
  );
}
