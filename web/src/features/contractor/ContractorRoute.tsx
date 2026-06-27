import { useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import type { Lang } from '../../shared/i18n';
import { useContractor } from './useContractor';
import type { ContractorFetchError } from './useContractor';
import { ContractorCard } from './ContractorCard';
import { ContractorSkeleton } from './ContractorSkeleton';

// Маршрут карточки подрядчика. Публичный id = natural БИН. Данные грузит useContractor (TanStack Query).
export function ContractorRoute() {
  const { bin = '' } = useParams();
  const { i18n, t } = useTranslation('chrome');
  const lang: Lang = i18n.language.startsWith('ru') ? 'ru' : 'kk';

  const q = useContractor(bin);

  if (q.isPending) return <ContractorSkeleton />;
  if (q.isError) {
    const status = (q.error as Partial<ContractorFetchError>).status;
    const key = status === 404 ? 'contractor.not_found' : 'contractor.error';
    return (
      <p className="contractor-error" role="alert">
        {t(key)}
      </p>
    );
  }

  return <ContractorCard contractor={q.data} lang={lang} />;
}
