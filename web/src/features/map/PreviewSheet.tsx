import { useRef, useState, type PointerEvent as ReactPointerEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';
import type { Lang } from '../../shared/i18n';
import { formatKm, formatPricePerKm } from '../../shared/i18n/format';
import { DataState, dataStateFromValueState } from '../../shared/state/DataState';
import { ErrorBoundary } from '../../shared/ui/ErrorBoundary';
import { useDialogFocus } from '../../shared/ui/useDialogFocus';
import { useContract, type ContractFetchError } from '../contract/useContract';
import {
  FlagBadge,
  MethodologyDialog,
  MethodologyFallback,
  apiToViewFlag,
  apiToMethodologyTarget,
  type MethodologyTarget,
} from '../flag';
import { ReportErrorForm, reportErrorMailto, type ReportTarget } from '../share';
import type { components } from '../../shared/api/schema.gen';
import { safeFormatMoney } from './lots';
import { contractPath, pricePerKm, type MapObject } from './objects';
import '../flag/flag.css';

type StringField = components['schemas']['StringField'];

// Превью-лист {preview-sheet} (Story 3.5, FR-8). Контент — существующий GET /api/contracts/{id}
// через useContract (D2: query-key греет карточку → «Подробнее» мгновенный). Детенты peek→half
// (EXPERIENCE.md:222): peek = мини-карточка (предмет/сумма/подрядчик/{source-link}), half = + флаги.
// objects.length > 1 — превью-список множественного попадания (AC3, D5).
//
// «Один уровень слоёв» (D7, UX-DR34/39): «Сообщить об ошибке»/методика открываются ЗАМЕНОЙ листа —
// лист размонтируется (его focus-cleanup возвращает фокус на триггер-маркер), слой монтируется и
// захватывает тот же триггер → закрытие любого слоя честно возвращает фокус на маркер.

const SWIPE_PX = 48; // порог свайпа детента/закрытия (>| tap-target: случайный тап не листает)

type Overlay =
  | { kind: 'report'; target: ReportTarget }
  | { kind: 'meth'; target: MethodologyTarget };

// Value — честный рендер поля (паттерн LotPreviewSheet/ContractCard): значение при state=ok &&
// value!==null; format→null (неканоничная сумма) — честное «нет данных», НЕ «0 ₸» и НЕ краш роута.
function Value({ field, format }: { field: StringField; format?: (v: string) => string | null }) {
  if (field.state === 'ok' && field.value !== null) {
    const formatted = format ? format(field.value) : field.value;
    if (formatted !== null) return <>{formatted}</>;
    return <DataState kind={dataStateFromValueState('no_data')} state="no_data" />;
  }
  const state = field.state === 'ok' ? 'no_data' : field.state;
  return <DataState kind={dataStateFromValueState(state)} state={state} />;
}

const fieldMissing = (f: StringField): boolean => f.state !== 'ok' || f.value === null;

export function PreviewSheet({
  objects,
  lang,
  onClose,
  returnFallback,
}: {
  objects: MapObject[]; // ≥1; >1 = превью-список (AC3)
  lang: Lang;
  onClose: () => void;
  returnFallback?: () => HTMLElement | null; // фокус-фолбэк, если триггер-маркер снесён рекластеризацией
}) {
  const { t } = useTranslation('chrome');
  const [overlay, setOverlay] = useState<Overlay | null>(null);

  const multi = objects.length > 1;
  const single: MapObject | undefined = objects[0];
  const cid = !multi && single !== undefined ? single.goszakup_contract_id : null;
  // Хуки безусловны (overlay ветвится ниже): контракт одиночного превью; для multi/без-контракта/
  // пустого массива запрос выключен (enabled: id непуст).
  const q = useContract(cid ?? '');
  const contract = q.data;
  const sourceUrl =
    contract !== undefined && contract.source_url.state === 'ok' ? contract.source_url.value : null;
  const mailto = reportErrorMailto(
    t('report_error.subject', { id: cid ?? single?.public_id ?? '' }),
  );
  const flagReportTarget: ReportTarget | null =
    cid !== null
      ? {
          kind: 'flag_error',
          subjectType: 'contract',
          subjectRef: cid,
          ...(sourceUrl !== null ? { sourceUrl } : {}),
        }
      : null;

  // Пустой массив от будущего вызывателя (контракт «≥1» — комментарием): честный no-op, не TypeError.
  if (objects.length === 0) return null;

  // Замена слоя (D7): лист ниже НЕ рендерится — его unmount вернул фокус на маркер, слой захватил
  // маркер как свой триггер. Закрытие слоя = закрытие всего превью (не возврат к листу).
  if (overlay !== null && overlay.kind === 'report') {
    return <ReportErrorForm target={overlay.target} onClose={onClose} />;
  }
  if (overlay !== null && overlay.kind === 'meth') {
    return (
      <ErrorBoundary fallback={<MethodologyFallback onClose={onClose} />}>
        <MethodologyDialog
          target={overlay.target}
          lang={lang}
          sourceUrl={sourceUrl}
          reportErrorHref={mailto}
          onReportError={
            flagReportTarget !== null
              ? () => setOverlay({ kind: 'report', target: flagReportTarget })
              : undefined
          }
          onClose={onClose}
        />
      </ErrorBoundary>
    );
  }

  return (
    <SheetBody
      objects={objects}
      lang={lang}
      onClose={onClose}
      returnFallback={returnFallback}
      contractQuery={q}
      sourceUrl={sourceUrl}
      openOverlay={setOverlay}
      flagReportTarget={flagReportTarget}
      mailto={mailto}
    />
  );
}

// SheetBody — сам лист. Отдельный компонент: его mount/unmount управляет focus-механикой
// (useDialogFocus держит trap/возврат ровно пока лист в DOM).
function SheetBody({
  objects,
  lang,
  onClose,
  returnFallback,
  contractQuery,
  sourceUrl,
  openOverlay,
  flagReportTarget,
  mailto,
}: {
  objects: MapObject[];
  lang: Lang;
  onClose: () => void;
  returnFallback?: () => HTMLElement | null;
  contractQuery: ReturnType<typeof useContract>;
  sourceUrl: string | null;
  openOverlay: (o: Overlay) => void;
  flagReportTarget: ReportTarget | null;
  mailto: string;
}) {
  const { t } = useTranslation('chrome');
  const sheetRef = useRef<HTMLDivElement>(null);
  const swipe = useRef<{ id: number; startY: number } | null>(null);
  const suppressClick = useRef(false);
  const [detent, setDetent] = useState<'peek' | 'half'>('peek');

  const multi = objects.length > 1;
  const single = objects[0];

  useDialogFocus(sheetRef, onClose, { returnFallback });

  // Свайп — на РУЧКЕ листа, не на контенте (код-ревью 3.5, High): контент скроллится (overflow-y),
  // кнопки кликаются — жест на всём листе давал двойные действия и конфликт со скроллом на таче.
  // setPointerCapture: pointerup доставляется ручке, даже когда палец ушёл выше листа — иначе
  // синтезированный click по общему предку (бэкдропу) ЗАКРЫВАЛ лист на жесте «вверх».
  // touch-action: none на ручке (map.css) — браузер не уводит жест в скролл (pointercancel).
  // Полный gesture-arbitration «карта↔лист» — D4-полировка. Вверх — peek→half; вниз — half→peek,
  // из peek/списка — закрыть (UX-DR15).
  const onHandlePointerDown = (e: ReactPointerEvent<HTMLElement>) => {
    // Только первичный указатель и основная кнопка (второй палец/right-drag — не жест).
    if (!e.isPrimary || (e.pointerType === 'mouse' && e.button !== 0)) return;
    e.currentTarget.setPointerCapture(e.pointerId);
    swipe.current = { id: e.pointerId, startY: e.clientY };
  };
  const onHandlePointerUp = (e: ReactPointerEvent<HTMLElement>) => {
    const s = swipe.current;
    swipe.current = null;
    if (s === null || s.id !== e.pointerId) return;
    const dy = e.clientY - s.startY;
    if (dy <= -SWIPE_PX) {
      suppressClick.current = true; // жест сработал — синтезированный click ручки не дублирует действие
      if (!multi) setDetent('half');
    } else if (dy >= SWIPE_PX) {
      suppressClick.current = true;
      if (!multi && detent === 'half') setDetent('peek');
      else onClose();
    }
  };
  const onHandlePointerCancel = () => {
    swipe.current = null; // браузер увёл жест (скролл/пинч) — не оставляем висячий старт
  };

  // «Точка не там» (SM-C2, AR-28): subjectRef = public_id гео-объекта (UUIDv7, AR-19).
  const geoReportTarget: ReportTarget = {
    kind: 'geo_wrong_point',
    subjectType: 'geo_object',
    subjectRef: single.public_id,
  };

  const detentClass = multi
    ? 'aq-sheet--list'
    : detent === 'half'
      ? 'aq-sheet--half'
      : 'aq-sheet--peek';

  return (
    <div className="aq-sheet-backdrop" onClick={onClose}>
      <div
        ref={sheetRef}
        className={`aq-sheet ${detentClass}`}
        role="dialog"
        aria-modal="true"
        // Имя слоя = видимый заголовок (EXPERIENCE.md:352-357); в loading/error-ветках h2 нет —
        // aria-label остаётся честным фолбэком (accname: labelledby без цели → label).
        aria-labelledby="aq-sheet-title"
        aria-label={
          multi ? t('map.preview_multi_title', { n: objects.length }) : t('map.preview_title')
        }
        tabIndex={-1}
        data-testid="preview-sheet"
        onClick={(e) => e.stopPropagation()}
      >
        {multi ? (
          <>
            {/* Свайп-зона списка: вниз — закрыть (детентов у списка нет). */}
            <div
              className="aq-sheet__handle-zone"
              aria-hidden="true"
              onPointerDown={onHandlePointerDown}
              onPointerUp={onHandlePointerUp}
              onPointerCancel={onHandlePointerCancel}
            >
              <span className="aq-sheet__handle" />
            </div>
            <h2 id="aq-sheet-title" className="aq-sheet__title">
              {t('map.preview_multi_title', { n: objects.length })}
            </h2>
            <ul className="aq-sheet__list" data-testid="preview-list">
              {objects.map((o) => (
                <PreviewListRow key={o.public_id} obj={o} lang={lang} />
              ))}
            </ul>
            <div className="aq-sheet__actions">
              <button type="button" className="aq-sheet__close" onClick={onClose}>
                {t('map.preview_close')}
              </button>
            </div>
          </>
        ) : (
          <>
            {/* Ручка — не-жестовый путь к детентам (UX-DR31): Enter/Space переключает peek↔half;
                она же — свайп-зона (capture, см. onHandlePointerDown). */}
            <button
              type="button"
              className="aq-sheet__handle-btn"
              aria-expanded={detent === 'half'}
              aria-label={t('map.preview_detent')}
              data-testid="preview-detent"
              onPointerDown={onHandlePointerDown}
              onPointerUp={onHandlePointerUp}
              onPointerCancel={onHandlePointerCancel}
              onClick={() => {
                // Свайп уже отработал в pointerup — синтезированный click не должен дублировать.
                if (suppressClick.current) {
                  suppressClick.current = false;
                  return;
                }
                setDetent(detent === 'half' ? 'peek' : 'half');
              }}
            >
              <span className="aq-sheet__handle" aria-hidden="true" />
            </button>
            <SinglePreview
              obj={single}
              lang={lang}
              q={contractQuery}
              detent={detent}
              sourceUrl={sourceUrl}
              openOverlay={openOverlay}
              flagReportTarget={flagReportTarget}
              geoReportTarget={geoReportTarget}
              mailto={mailto}
              onClose={onClose}
            />
          </>
        )}
      </div>
    </div>
  );
}

// SinglePreview — мини-карточка контракта (UX-DR7 «компактная версия»; EXPERIENCE.md:213).
function SinglePreview({
  obj,
  lang,
  q,
  detent,
  sourceUrl,
  openOverlay,
  flagReportTarget,
  geoReportTarget,
  mailto,
  onClose,
}: {
  obj: MapObject;
  lang: Lang;
  q: ReturnType<typeof useContract>;
  detent: 'peek' | 'half';
  sourceUrl: string | null;
  openOverlay: (o: Overlay) => void;
  flagReportTarget: ReportTarget | null;
  geoReportTarget: ReportTarget;
  mailto: string;
  onClose: () => void;
}) {
  const { t } = useTranslation('chrome');
  const cid = obj.goszakup_contract_id;

  const actions = (withMore: boolean) => (
    <div className="aq-sheet__actions">
      {withMore && cid !== null && (
        <Link className="aq-sheet__more" data-testid="preview-more" to={contractPath(cid)}>
          {t('map.preview_more')}
        </Link>
      )}
      <button
        type="button"
        className="aq-sheet__close"
        aria-haspopup="dialog"
        onClick={() => openOverlay({ kind: 'report', target: geoReportTarget })}
      >
        {t('report_error.geo_link')}
      </button>
      <button type="button" className="aq-sheet__close" onClick={onClose}>
        {t('map.preview_close')}
      </button>
    </div>
  );

  // Гео-строка без контракта (schema это допускает): гео-факты честно, без «Подробнее» (D2).
  if (cid === null) {
    return (
      <>
        <h2 id="aq-sheet-title" className="aq-sheet__title">
          {obj.public_id}
        </h2>
        <p className="aq-sheet__awaiting">{t('map.preview_no_contract')}</p>
        {obj.length_km !== null && (
          <dl className="aq-sheet__meta">
            <div className="aq-sheet__meta-row">
              <dt>{t('map.preview_length')}</dt>
              <dd>{t('map.preview_length_value', { km: formatKm(obj.length_km, lang) })}</dd>
            </div>
          </dl>
        )}
        {actions(false)}
      </>
    );
  }

  if (q.isPending) {
    return (
      <>
        <p className="aq-sheet__loading" role="status">
          {t('map.preview_loading')}
        </p>
        {actions(false)}
      </>
    );
  }

  if (q.isError) {
    // 404 → честное «нет данных по объекту» (deferred:168), не общая ошибка.
    const status = (q.error as Partial<ContractFetchError>).status;
    return (
      <>
        <p className="aq-sheet__error" role="alert">
          {t(status === 404 ? 'contract.not_found' : 'contract.error')}
        </p>
        {actions(false)}
      </>
    );
  }

  const c = q.data;
  const subject = lang === 'kk' ? c.subject_kk : c.subject_ru;
  // P7 (0.8): оба наименования пусты → заголовок = идентификатор (юзер понимает, что открыто).
  const bothSubjectsMissing = fieldMissing(c.subject_kk) && fieldMissing(c.subject_ru);
  const raised = c.flags.filter((f) => f.state === 'raised');
  // Цена/км линии (UX-DR13, D6): деривация из двух видимых фактов; не-ok вход → честное состояние.
  const derivedPricePerKm =
    obj.length_km !== null && c.amount_tng.state === 'ok'
      ? pricePerKm(c.amount_tng.value, obj.length_km)
      : null;

  return (
    <>
      <h2 id="aq-sheet-title" className="aq-sheet__title" lang={lang}>
        {bothSubjectsMissing ? cid : <Value field={subject} />}
      </h2>
      <p className="aq-sheet__amount" data-testid="preview-amount">
        <Value field={c.amount_tng} format={(v) => safeFormatMoney(v, lang)} />
      </p>
      <dl className="aq-sheet__meta">
        <div className="aq-sheet__meta-row">
          <dt>{t('contract.field.supplier')}</dt>
          {/* lang="ru" — ТОЛЬКО на значении-цитате источника (NFR-6); честное состояние (DataState)
              локализовано под хром и НЕ должно озвучиваться ru-движком (код-ревью 3.5). */}
          <dd data-testid="preview-supplier">
            {c.supplier.state === 'ok' && c.supplier.value !== null ? (
              <span lang="ru">{c.supplier.value}</span>
            ) : (
              <Value field={c.supplier} />
            )}
          </dd>
        </div>
        {/* Линие-специфичные поля (UX-DR13): length_km + цена/км — только у LINESTRING. */}
        {obj.length_km !== null && (
          <>
            <div className="aq-sheet__meta-row">
              <dt>{t('map.preview_length')}</dt>
              <dd>{t('map.preview_length_value', { km: formatKm(obj.length_km, lang) })}</dd>
            </div>
            <div className="aq-sheet__meta-row">
              <dt>{t('map.preview_price_per_km')}</dt>
              <dd data-testid="preview-price-per-km">
                {derivedPricePerKm !== null ? (
                  formatPricePerKm(derivedPricePerKm, lang)
                ) : (
                  <DataState kind={dataStateFromValueState('no_data')} state="no_data" />
                )}
              </dd>
            </div>
          </>
        )}
      </dl>

      {/* half-детент: активные флаги (D3 — только raised; полная реконструкция всех состояний —
          карточка). UX-DR28: у каждого бейджа — путь к методике (замена листа) + «Сообщить об ошибке». */}
      {detent === 'half' && raised.length > 0 && flagReportTarget !== null && (
        <section
          className="aq-sheet__signals"
          aria-label={t('contract.signals_heading', { count: raised.length })}
        >
          {raised.map((f) => {
            const view = apiToViewFlag(f);
            if (view === null) return null;
            return (
              <FlagBadge
                key={f.flag_id}
                flag={view}
                lang={lang}
                onOpenMethodology={() =>
                  openOverlay({ kind: 'meth', target: apiToMethodologyTarget(f) })
                }
                reportErrorHref={mailto}
                onReportError={() => openOverlay({ kind: 'report', target: flagReportTarget })}
              />
            );
          })}
        </section>
      )}

      {sourceUrl !== null && (
        <a className="aq-sheet__source" href={sourceUrl} target="_blank" rel="noopener noreferrer">
          {t('contract.source')} ↗
        </a>
      )}

      {actions(true)}
    </>
  );
}

// PreviewListRow — ряд превью-списка (AC3): предмет + сумма из фетча карточки; тап → Карточка (D5).
// role=status/alert — ВНУТРИ li (роль на самом li ломала ARIA-семантику списка: не-listitem дети ul).
function PreviewListRow({ obj, lang }: { obj: MapObject; lang: Lang }) {
  const { t } = useTranslation('chrome');
  const cid = obj.goszakup_contract_id;
  const q = useContract(cid ?? '');

  if (cid === null) {
    // Гео-факты и в ряду (Task 5): идентификатор + длина — иначе два таких ряда неотличимы.
    return (
      <li className="aq-sheet__row">
        <span className="aq-sheet__row-muted">
          {obj.public_id} — {t('map.preview_no_contract')}
          {obj.length_km !== null
            ? ` · ${t('map.preview_length_value', { km: formatKm(obj.length_km, lang) })}`
            : ''}
        </span>
      </li>
    );
  }
  if (q.isPending) {
    return (
      <li className="aq-sheet__row">
        <span role="status">{t('map.preview_loading')}</span>
      </li>
    );
  }
  if (q.isError) {
    const status = (q.error as Partial<ContractFetchError>).status;
    return (
      <li className="aq-sheet__row">
        <span role="alert">{t(status === 404 ? 'contract.not_found' : 'contract.error')}</span>
      </li>
    );
  }

  const c = q.data;
  const subject = lang === 'kk' ? c.subject_kk : c.subject_ru;
  const bothMissing = fieldMissing(c.subject_kk) && fieldMissing(c.subject_ru);
  return (
    <li className="aq-sheet__row">
      <Link className="aq-sheet__row-link" to={contractPath(cid)}>
        <span className="aq-sheet__row-subject" lang={lang}>
          {bothMissing ? cid : <Value field={subject} />}
        </span>
        <span className="aq-sheet__row-amount">
          <Value field={c.amount_tng} format={(v) => safeFormatMoney(v, lang)} />
        </span>
      </Link>
    </li>
  );
}
