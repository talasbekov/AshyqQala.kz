import { useParams, useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import type { Lang } from '../../shared/i18n';
import { useContract } from './useContract';
import type { ContractFetchError } from './useContract';
import { ContractCard } from './ContractCard';
import { ContractSkeleton } from './ContractSkeleton';

// Маршрут карточки. Router-loader НЕ фетчит — данные грузит useContract (TanStack Query).
export function ContractRoute() {
  const { goszakupId = '' } = useParams();
  const [searchParams] = useSearchParams();
  // Story 5.4 (AC-2): deep-link ?report (цель для Telegram-пуша, Epic 7) сразу открывает форму ошибки.
  const autoReport = searchParams.get('report') !== null;
  const { i18n, t } = useTranslation('chrome');
  const lang: Lang = i18n.language.startsWith('ru') ? 'ru' : 'kk';

  const q = useContract(goszakupId);

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
  return <ContractCard contract={q.data} lang={lang} autoReport={autoReport} />;
}
