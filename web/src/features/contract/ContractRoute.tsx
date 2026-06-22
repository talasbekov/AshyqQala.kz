import { useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import type { Lang } from '../../shared/i18n';
import { useContract } from './useContract';
import { ContractCard } from './ContractCard';
import { ContractSkeleton } from './ContractSkeleton';

// Маршрут карточки. Router-loader НЕ фетчит — данные грузит useContract (TanStack Query).
export function ContractRoute() {
  const { goszakupId = '' } = useParams();
  const { i18n, t } = useTranslation('chrome');
  const lang: Lang = i18n.language === 'ru' ? 'ru' : 'kk';

  const q = useContract(goszakupId);

  if (q.isPending) return <ContractSkeleton />;
  if (q.isError) {
    return (
      <p className="contract-error" role="alert">
        {t('contract.error')}
      </p>
    );
  }
  return <ContractCard contract={q.data} lang={lang} />;
}
