import { useParams, useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import type { Lang } from '../../shared/i18n';
import { useContract } from './useContract';
import type { ContractFetchError } from './useContract';
import { ContractCard } from './ContractCard';
import { ContractSkeleton } from './ContractSkeleton';
import { PermalinkButton } from './PermalinkButton';
import { buildPermalink } from './permalink';

// Маршрут карточки. Router-loader НЕ фетчит — данные грузит useContract (TanStack Query).
export function ContractRoute() {
  const { goszakupId = '' } = useParams();
  const [searchParams] = useSearchParams();
  // Story 5.4 (AC-2): deep-link ?report (цель для Telegram-пуша, Epic 7) сразу открывает форму ошибки.
  const autoReport = searchParams.get('report') !== null;
  // Story 5.6 (AR-29): ?mv из перманентной ссылки — сервер по ней детектирует дрейф методики.
  const permalinkMv = searchParams.get('mv');
  const { i18n, t } = useTranslation('chrome');
  const lang: Lang = i18n.language.startsWith('ru') ? 'ru' : 'kk';

  const q = useContract(goszakupId, permalinkMv);

  if (q.isPending) return <ContractSkeleton />;
  if (q.isError) {
    // 404 → честное «нет данных по объекту» (не общая ошибка) — закрывает deferred 1.8.
    const status = (q.error as Partial<ContractFetchError>).status;
    const key = status === 404 ? 'contract.not_found' : 'contract.error';
    return (
      <p className="contract-error" role="alert">
        {t(key)}
      </p>
    );
  }

  const c = q.data;
  // Перманентная ссылка-на-дату (AR-29): штамп (as_of, mv) только из честных ok-значений (пустое не штампуем).
  const permalink = buildPermalink(
    c.goszakup_contract_id,
    c.as_of.state === 'ok' ? c.as_of.value : null,
    c.methodology_version.state === 'ok' ? c.methodology_version.value : null,
  );
  // Дрейф методики: ссылка создана под иной версией → честный баннер «показано текущее, срез не сохраняется».
  const drift = c.methodology_drift;

  return (
    <>
      {drift.present ? (
        <p className="contract-drift" role="status">
          {t('methodology.drift', { from: drift.requested_version, to: drift.current_version })}
        </p>
      ) : null}
      <ContractCard contract={c} lang={lang} autoReport={autoReport} />
      <PermalinkButton href={permalink} />
    </>
  );
}
