// Скелет загрузки страницы района (нейтральный, без выдуманных данных).
export function DistrictSkeleton() {
  return (
    <article className="district-card district-card--skeleton" aria-busy="true" aria-live="polite">
      <div className="district-card__sk-line district-card__sk-line--title" />
      <div className="district-card__sk-line" />
      <div className="district-card__sk-line" />
      <div className="district-card__sk-line" />
    </article>
  );
}
