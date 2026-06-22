import './contract-card.css';

// Скелетон ПОД РАСКЛАДКУ карточки (не спиннер-стена, AC1): повторяет DOM-структуру ContractCard,
// чтобы не было layout-shift. Показывается, пока Query грузит.
export function ContractSkeleton() {
  return (
    <article className="contract-card contract-skeleton" aria-busy="true" aria-label="loading">
      <div className="contract-skeleton__line contract-skeleton__line--label" />
      <div className="contract-skeleton__line contract-skeleton__line--subject" />
      <div className="contract-skeleton__line contract-skeleton__line--amount" />
      <div className="contract-skeleton__meta">
        {[0, 1, 2, 3, 4].map((i) => (
          <div key={i} className="contract-skeleton__line contract-skeleton__line--row" />
        ))}
      </div>
    </article>
  );
}
