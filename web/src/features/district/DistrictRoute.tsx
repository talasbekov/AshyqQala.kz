import { useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import type { Lang } from '../../shared/i18n';
import { useDistrict } from './useDistrict';
import { DistrictCard } from './DistrictCard';
import { DistrictSkeleton } from './DistrictSkeleton';

// Маршрут страницы района (Story 6.3). Публичный id = КАТО-код. Данные грузит useDistrict (TanStack Query).
export function DistrictRoute() {
  const { kato = '' } = useParams();
  const { i18n, t } = useTranslation('chrome');
  const lang: Lang = i18n.language.startsWith('ru') ? 'ru' : 'kk';

  const q = useDistrict(kato);

  if (q.isPending) return <DistrictSkeleton />;
  if (q.isError) {
    // Невалидный КАТО (400) или сбой — нейтральная ошибка. Неизвестный-но-валидный КАТО возвращает 200
    // с честным container_state (no_contracts), а не ошибку.
    return (
      <p className="district-error" role="alert">
        {t('district.error')}
      </p>
    );
  }

  return <DistrictCard district={q.data} lang={lang} />;
}
