// Story 6.1 (FR-15): экран поиска/фильтров. Фасет-чипы (shared/ui FilterChip) + список контрактов.
// Состояние фильтров — В URL (useSearchParams): шарабельно, немедленное применение по тапу чипа (AC1),
// повторный тап снимает. Пустой результат — честно «Ничего не найдено» + «расширить фильтр» (AC3), без
// выдуманных строк. Период — фасет (AC4), не окно медианы. Нейтральность: «сигнал, требующий проверки» (AC6).
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

function itemTitle(it: ContractListItem, noSubject: string): string {
  if (it.subject_ru.state === 'ok' && it.subject_ru.value) return it.subject_ru.value;
  if (it.subject_kk.state === 'ok' && it.subject_kk.value) return it.subject_kk.value;
  return noSubject;
}

export function SearchRoute() {
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
  const setPeriod = (p: PeriodPreset) => apply({ ...filters, period: filters.period === p ? 'all' : p });
  const toggleHasFlag = () => apply({ ...filters, hasFlag: !filters.hasFlag });
  const clearSupplier = () => apply({ ...filters, supplierBin: '' }); // снять ТОЛЬКО фасет подрядчика (P7)
  const reset = () => setSp(new URLSearchParams(), { replace: true });

  // Поля суммы: локальное состояние + дебаунс синхронизации в URL. Иначе каждый ввод цифры → новый queryKey →
  // лишний запрос (серия фетчей). Длину клэмпим (≤15 цифр) — защита от переполнения int64 → 400, которое
  // уронило бы весь список в общий error-экран. [code review 6.1]
  const clampAmount = (v: string) => v.replace(/[^\d]/g, '').slice(0, 15);
  const [amountMin, setAmountMin] = useState(filters.amountMin);
  const [amountMax, setAmountMax] = useState(filters.amountMax);
  // Синхронизация вниз: внешняя смена суммы в URL (reset / «назад» / ссылка) → подтянуть в поля.
  useEffect(() => setAmountMin(filters.amountMin), [filters.amountMin]);
  useEffect(() => setAmountMax(filters.amountMax), [filters.amountMax]);
  // Дебаунс вверх: применяем в URL спустя паузу после ввода.
  useEffect(() => {
    if (amountMin === filters.amountMin && amountMax === filters.amountMax) return;
    const id = window.setTimeout(() => apply({ ...filters, amountMin, amountMax }), 350);
    return () => window.clearTimeout(id);
  }, [amountMin, amountMax]);

  // supplier_bin из URL может быть мусором (deep-link): валидируем перед показом, иначе заголовок «Контракты
  // подрядчика {мусор}» при пустом списке вводит в заблуждение (бэкенд канонизирует мусор в пусто). [code review 6.1]
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
    <section className="search">
      <h1 className="search__title">{t('search.title')}</h1>
      <p className="search__intro">{t('search.intro')}</p>

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
          <FilterChip testId="chip-has-flag" label={t('search.has_flag')} selected={filters.hasFlag} onToggle={toggleHasFlag} />
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
          {/* AC3: быстрый сброс фасетов прямо в пустом состоянии (а не только в панели фильтров) — но лишь
              когда есть что снимать; при пустой БД без фильтров кнопка бессмысленна. [code review 6.1] */}
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
                    ? t(`search.direction.${it.direction.value}`, { defaultValue: it.direction.value })
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
                {/* при ошибке догрузки кнопка превращается в «Повторить» — ошибка не проглатывается (P1) */}
                {q.isFetchNextPageError ? t('search.load_more_retry') : t('search.load_more')}
              </button>
            </div>
          ) : null}
        </>
      )}
    </section>
  );
}
