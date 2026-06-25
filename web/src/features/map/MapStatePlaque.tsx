import { useTranslation } from 'react-i18next';

// Плашка честного состояния контейнера ранней карты (Story 0.8, AC1). container_state=no_contracts:
// источник временный (scraped-лоты) — контрактов, флагов и медиан нет (ждут токен ows_v2). Видна
// ОТДЕЛЬНОЙ строкой (AR-17): частичный/пустой экран не должен читаться как «всё чисто». Выборка
// отобрана по ключевым словам направлений → непоказательна → «предв.».
export function MapStatePlaque() {
  const { t } = useTranslation('chrome');
  return (
    <aside className="aq-map-plaque" role="status">
      <p className="aq-map-plaque__line">{t('map.partial_data')}</p>
      <p className="aq-map-plaque__line aq-map-plaque__line--muted">{t('map.keyword_bias')}</p>
    </aside>
  );
}
