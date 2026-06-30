// Story 6.1 (FR-15): фасетный поиск/фильтры. Story 6.2 (FR-16): + текстовый поиск по БИН/наименованию ПОВЕРХ
// фичи. Два режима одного экрана: пустой `q` → браузинг фасетами (6.1, без изменений); непустой `q` → текстовый
// поиск (организации → карточка подрядчика, контракты → карточка контракта; негеопривязанные помечены).
// Состояние — В URL (useSearchParams): шарабельно. Нейтральность: «сигнал, требующий проверки» (AC6/AC7).
import { useEffect, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { FilterChip } from '../../shared/ui/FilterChip';
import { Icon } from '../../shared/ui/Icon';
import { DataState } from '../../shared/state/DataState';
import { formatMoney, formatDateSafe } from '../../shared/i18n/format';
import type { Lang } from '../../shared/i18n';
import {
  DIRECTIONS,
  PERIODS,
  filtersFromParams,
  filtersToParams,
  useContracts,
  type ContractListItem,
  type PeriodPreset,
  type SearchFilters,
} from './useContracts';
import {
  useSearch,
  queryIsSearchable,
  type OrganizationSearchItem,
  type ContractSearchItem,
} from './useSearch';
import './search.css';

type StringField = { value: string | null; state: string };

function moneyOrDash(f: StringField, lang: Lang): string {
  if (f.state !== 'ok' || !f.value) return '—';
  try {
    return formatMoney(f.value, lang);
  } catch {
    return '—'; // неканоничная сумма → честно «нет», не фабрикуем 0 ₸
  }
}

function bilingualTitle(ru: StringField, kk: StringField, fallback: string): string {
  if (ru.state === 'ok' && ru.value) return ru.value;
  if (kk.state === 'ok' && kk.value) return kk.value;
  return fallback;
}

export function SearchRoute() {
  const { t } = useTranslation('chrome');
  const [sp, setSp] = useSearchParams();
  const queryParam = sp.get('q') ?? '';
  // Активный поиск гейтится на SEARCHABLE-терм (а не просто непусто): иначе слишком короткий q монтировал бы
  // панель результатов и показывал ложное «Ничего не найдено», хотя поиск не выполнялся (code review 6.2 P1).
  const queryActive = queryIsSearchable(queryParam);

  // Поле текстового поиска: локальное состояние + дебаунс синхронизации в URL (как дебаунс суммы в 6.1).
  const [queryInput, setQueryInput] = useState(queryParam);
  // Синхронизация вниз: внешняя смена q в URL (reset / «назад» / ссылка) → подтянуть в поле.
  useEffect(() => setQueryInput(queryParam), [queryParam]);
  // Дебаунс вверх: применяем q в URL спустя паузу после ввода (прочие параметры сохраняем).
  useEffect(() => {
    if (queryInput.trim() === queryParam.trim()) return;
    const id = window.setTimeout(() => {
      const v = queryInput.trim();
      // Функциональный апдейт: читаем СВЕЖИЕ params в момент срабатывания (не устаревший sp из замыкания) —
      // иначе тоггл фасета в окне дебаунса был бы затёрт (code review 6.2 P4).
      setSp(
        (prev) => {
          const next = new URLSearchParams(prev);
          if (v) next.set('q', v);
          else next.delete('q');
          return next;
        },
        { replace: true },
      );
    }, 350);
    return () => window.clearTimeout(id);
  }, [queryInput]);

  // composing — в поле что-то введено; showMinHint — терм есть, но слишком короткий (не searchable).
  const composing = queryInput.trim() !== '';
  const showMinHint = composing && !queryIsSearchable(queryInput);

  return (
    <section className="search">
      <h1 className="search__title">{t('search.title')}</h1>
      <p className="search__intro">{t('search.intro')}</p>

      {/* Текстовый поиск (Story 6.2, FR-16) — поверх фасетов. */}
      <div className="search__textfield">
        <input
          type="search"
          className="search__query"
          data-testid="search-query"
          aria-label={t('search.query_placeholder')}
          placeholder={t('search.query_placeholder')}
          maxLength={100}
          value={queryInput}
          onChange={(e) => setQueryInput(e.target.value)}
        />
        {showMinHint ? (
          <p className="search__query-hint" data-testid="search-query-hint">
            {t('search.query_min_hint')}
          </p>
        ) : null}
      </div>

      {/* searchable → результаты; печатает (непусто, но коротко) → только подсказка выше (ни ложного пустого,
          ни мелькания browse); пусто → браузинг фасетами 6.1 (code review 6.2 P1). */}
      {queryActive ? (
        <TextSearchResults term={queryParam.trim()} />
      ) : composing ? null : (
        <BrowseContracts />
      )}
    </section>
  );
}

// TextSearchResults (Story 6.2) — результаты текстового поиска: организации + контракты, единым списком.
function TextSearchResults({ term }: { term: string }) {
  const { t, i18n } = useTranslation('chrome');
  const lang: Lang = i18n.language === 'kk' ? 'kk' : 'ru';
  const q = useSearch(term);
  const items = q.data?.items ?? [];

  return (
    <div className="search__results" data-testid="search-results">
      <h2 className="search__results-heading">{t('search.query_results_heading')}</h2>
      {q.isLoading ? (
        <DataState kind="loading" />
      ) : q.isError ? (
        <p className="search__error" role="alert">
          <Icon name="error" /> {t('search.error')}
        </p>
      ) : items.length === 0 ? (
        <div className="search__empty" data-testid="search-empty-query">
          <p className="search__empty-title">
            <Icon name="no-data" /> {t('search.query_empty_title')}
          </p>
        </div>
      ) : (
        <ul className="search__list" data-testid="search-results-list">
          {items.map((it) =>
            it.kind === 'organization' ? (
              <SearchOrgRow key={`org-${it.bin}`} org={it} t={t} />
            ) : (
              <SearchContractRow
                key={`contract-${it.goszakup_contract_id}`}
                contract={it}
                t={t}
                lang={lang}
              />
            ),
          )}
        </ul>
      )}
    </div>
  );
}

// Маркер «без точки на карте» (AC5) — выводится из РЕАЛЬНОГО has_geo (до Epic 3 равномерно false). Глиф ⦸
// + существующая строка map.ungeocoded_label (переиспользование, не новый словарь).
function UngeocodedMark({ t }: { t: (k: string) => string }) {
  return (
    <span className="search__item-ungeocoded" data-testid="search-ungeocoded">
      <Icon name="ungeocoded" /> {t('map.ungeocoded_label')}
    </span>
  );
}

function SearchOrgRow({
  org,
  t,
}: {
  org: OrganizationSearchItem;
  t: (k: string, o?: Record<string, unknown>) => string;
}) {
  return (
    <li className="search__item search__item--org" data-testid="search-result-item">
      <span className="search__item-kind">{t('search.result_kind_org')}</span>
      <Link className="search__item-link" to={`/contractors/${org.bin}`}>
        {bilingualTitle(org.name_ru, org.name_kk, t('search.item_no_subject'))}
      </Link>
      <span className="search__item-meta">{t('search.result_bin', { bin: org.bin })}</span>
      {!org.has_geo ? <UngeocodedMark t={t} /> : null}
    </li>
  );
}

function SearchContractRow({
  contract,
  t,
  lang,
}: {
  contract: ContractSearchItem;
  t: (k: string, o?: Record<string, unknown>) => string;
  lang: Lang;
}) {
  return (
    <li className="search__item search__item--contract" data-testid="search-result-item">
      <span className="search__item-kind">{t('search.result_kind_contract')}</span>
      <Link className="search__item-link" to={`/contracts/${contract.goszakup_contract_id}`}>
        {bilingualTitle(contract.subject_ru, contract.subject_kk, t('search.item_no_subject'))}
      </Link>
      <span className="search__item-meta">
        {contract.direction.state === 'ok' && contract.direction.value
          ? t(`search.direction.${contract.direction.value}`, {
              defaultValue: contract.direction.value,
            })
          : null}
        {' · '}
        {moneyOrDash(contract.amount_tng, lang)}
        {contract.sign_date.state === 'ok' && contract.sign_date.value
          ? ` · ${formatDateSafe(contract.sign_date.value, lang)}`
          : null}
      </span>
      {contract.has_active_flag ? (
        <span className="search__item-signal">
          <Icon name="flag" /> {t('search.signal_present')}
        </span>
      ) : null}
      {!contract.has_geo ? <UngeocodedMark t={t} /> : null}
    </li>
  );
}

function itemTitle(it: ContractListItem, noSubject: string): string {
  if (it.subject_ru.state === 'ok' && it.subject_ru.value) return it.subject_ru.value;
  if (it.subject_kk.state === 'ok' && it.subject_kk.value) return it.subject_kk.value;
  return noSubject;
}

// BrowseContracts (Story 6.1) — фасетный браузинг списка контрактов. Без изменений поведения: монтируется
// при пустом `q`; собственное URL-состояние фасетов (useSearchParams). testId сохранены (e2e 6.1).
function BrowseContracts() {
  const { t, i18n } = useTranslation('chrome');
  const lang: Lang = i18n.language === 'kk' ? 'kk' : 'ru';
  const [sp, setSp] = useSearchParams();
  const filters = filtersFromParams(sp);

  const apply = (next: SearchFilters) => setSp(filtersToParams(next), { replace: true });
  const toggleDirection = (d: string) =>
    apply({
      ...filters,
      directions: filters.directions.includes(d)
        ? filters.directions.filter((x) => x !== d)
        : [...filters.directions, d],
    });
  const setPeriod = (p: PeriodPreset) =>
    apply({ ...filters, period: filters.period === p ? 'all' : p });
  const toggleHasFlag = () => apply({ ...filters, hasFlag: !filters.hasFlag });
  const clearSupplier = () => apply({ ...filters, supplierBin: '' }); // снять ТОЛЬКО фасет подрядчика (P7)
  const reset = () => setSp(new URLSearchParams(), { replace: true });

  // Поля суммы: локальное состояние + дебаунс синхронизации в URL (иначе серия фетчей). Длину клэмпим (≤15
  // цифр) — защита от переполнения int64 → 400. [code review 6.1]
  const clampAmount = (v: string) => v.replace(/[^\d]/g, '').slice(0, 15);
  const [amountMin, setAmountMin] = useState(filters.amountMin);
  const [amountMax, setAmountMax] = useState(filters.amountMax);
  useEffect(() => setAmountMin(filters.amountMin), [filters.amountMin]);
  useEffect(() => setAmountMax(filters.amountMax), [filters.amountMax]);
  useEffect(() => {
    if (amountMin === filters.amountMin && amountMax === filters.amountMax) return;
    const id = window.setTimeout(() => apply({ ...filters, amountMin, amountMax }), 350);
    return () => window.clearTimeout(id);
  }, [amountMin, amountMax]);

  // supplier_bin из URL может быть мусором (deep-link): валидируем перед показом. [code review 6.1]
  const supplierBinDigits = filters.supplierBin.replace(/\D/g, '');
  const supplierBinValid = supplierBinDigits.length === 12;

  const hasActiveFilters =
    filters.directions.length > 0 ||
    filters.period !== 'all' ||
    filters.hasFlag ||
    !!filters.amountMin ||
    !!filters.amountMax ||
    !!filters.supplierBin;

  const q = useContracts(filters);
  const items = q.data?.pages.flatMap((pg) => pg.items) ?? [];

  return (
    <>
      <div className="search__facets">
        <fieldset className="search__facet">
          <legend>{t('search.facet.direction')}</legend>
          {DIRECTIONS.map((d) => (
            <FilterChip
              key={d}
              testId={`chip-dir-${d}`}
              label={t(`search.direction.${d}`)}
              selected={filters.directions.includes(d)}
              onToggle={() => toggleDirection(d)}
            />
          ))}
        </fieldset>

        <fieldset className="search__facet">
          <legend>{t('search.facet.period')}</legend>
          {PERIODS.map((p) => (
            <FilterChip
              key={p}
              testId={`chip-period-${p}`}
              label={t(`search.period.${p}`)}
              selected={filters.period === p}
              onToggle={() => setPeriod(p)}
            />
          ))}
          <p className="search__note">{t('search.period_note')}</p>
        </fieldset>

        <fieldset className="search__facet">
          <legend>{t('search.facet.signal')}</legend>
          <FilterChip
            testId="chip-has-flag"
            label={t('search.has_flag')}
            selected={filters.hasFlag}
            onToggle={toggleHasFlag}
          />
        </fieldset>

        <fieldset className="search__facet search__facet--amount">
          <legend>{t('search.facet.amount')}</legend>
          <input
            type="text"
            inputMode="numeric"
            className="search__amount"
            aria-label={t('search.amount_min')}
            placeholder={t('search.amount_min')}
            value={amountMin}
            onChange={(e) => setAmountMin(clampAmount(e.target.value))}
          />
          <input
            type="text"
            inputMode="numeric"
            className="search__amount"
            aria-label={t('search.amount_max')}
            placeholder={t('search.amount_max')}
            value={amountMax}
            onChange={(e) => setAmountMax(clampAmount(e.target.value))}
          />
        </fieldset>

        <button type="button" className="search__reset" onClick={reset}>
          {t('search.reset')}
        </button>
      </div>

      {filters.supplierBin ? (
        <p className="search__supplier">
          {supplierBinValid
            ? t('search.supplier_filter', { bin: supplierBinDigits })
            : t('search.supplier_filter_invalid')}
          <button
            type="button"
            className="search__supplier-clear"
            aria-label={t('search.supplier_clear')}
            onClick={clearSupplier}
          >
            ×
          </button>
        </p>
      ) : null}

      <h2 className="search__results-heading">{t('search.results_heading')}</h2>

      {q.isLoading ? (
        <DataState kind="loading" />
      ) : q.isError ? (
        <p className="search__error" role="alert">
          <Icon name="error" /> {t('search.error')}
        </p>
      ) : items.length === 0 ? (
        <div className="search__empty" data-testid="search-empty">
          <p className="search__empty-title">
            <Icon name="no-data" /> {t('search.empty_title')}
          </p>
          <p className="search__empty-hint">{t('search.empty_hint')}</p>
          {hasActiveFilters ? (
            <button type="button" className="search__empty-reset" onClick={reset}>
              {t('search.reset')}
            </button>
          ) : null}
        </div>
      ) : (
        <>
          <ul className="search__list" data-testid="search-list">
            {items.map((it) => (
              <li key={it.goszakup_contract_id} className="search__item" data-testid="search-item">
                <Link className="search__item-link" to={`/contracts/${it.goszakup_contract_id}`}>
                  {itemTitle(it, t('search.item_no_subject'))}
                </Link>
                <span className="search__item-meta">
                  {it.direction.state === 'ok' && it.direction.value
                    ? t(`search.direction.${it.direction.value}`, {
                        defaultValue: it.direction.value,
                      })
                    : null}
                  {' · '}
                  {moneyOrDash(it.amount_tng, lang)}
                  {it.sign_date.state === 'ok' && it.sign_date.value
                    ? ` · ${formatDateSafe(it.sign_date.value, lang)}`
                    : null}
                </span>
                {it.has_active_flag ? (
                  <span className="search__item-signal">
                    <Icon name="flag" /> {t('search.signal_present')}
                  </span>
                ) : null}
              </li>
            ))}
          </ul>
          {q.hasNextPage ? (
            <div className="search__more-wrap">
              {q.isFetchNextPageError ? (
                <p className="search__more-error" role="alert">
                  <Icon name="error" /> {t('search.load_more_error')}
                </p>
              ) : null}
              <button
                type="button"
                className="search__more"
                onClick={() => void q.fetchNextPage()}
                disabled={q.isFetchingNextPage}
              >
                {q.isFetchNextPageError ? t('search.load_more_retry') : t('search.load_more')}
              </button>
            </div>
          ) : null}
        </>
      )}
    </>
  );
}
