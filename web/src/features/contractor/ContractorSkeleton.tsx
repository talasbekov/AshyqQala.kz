// Скелет загрузки карточки подрядчика (нейтральный, без выдуманных данных).
export function ContractorSkeleton() {
  return (
    <article
      className="contractor-card contractor-card--skeleton"
      aria-busy="true"
      aria-live="polite"
    >
      <div className="contractor-card__sk-line contractor-card__sk-line--title" />
      <div className="contractor-card__sk-line" />
      <div className="contractor-card__sk-line" />
      <div className="contractor-card__sk-line" />
    </article>
  );
}
