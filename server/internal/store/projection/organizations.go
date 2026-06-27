package projection

import (
	"context"

	"ashyqqala/server/internal/store/gen"
)

// OrgStore — доступ к проекционной таблице organizations поверх sqlc-генерата (Story 2.3).
// Граница AR-4: проекционная (импортёр перестраивает) ⊥ кураторская (`store/curation` — org_name_aliases).
type OrgStore struct{ q *gen.Queries }

// NewOrgStore — конструктор поверх пула/транзакции (gen.DBTX).
func NewOrgStore(db gen.DBTX) *OrgStore { return &OrgStore{q: gen.New(db)} }

// UpsertOrganization — идемпотентная запись организации в проекцию (UPSERT по bin; роли OR-ятся,
// наименования COALESCE — повтор не плодит дубли и не затирает известные имена).
func (s *OrgStore) UpsertOrganization(ctx context.Context, p gen.UpsertOrganizationParams) error {
	return s.q.UpsertOrganization(ctx, p)
}

// GetOrganizationByBIN — организация по natural bin (удалённые скрыты).
func (s *OrgStore) GetOrganizationByBIN(ctx context.Context, bin string) (gen.Organization, error) {
	return s.q.GetOrganizationByBIN(ctx, bin)
}

// ListOrganizations — все неудалённые организации (движок нормализации строит из них кандидатов).
func (s *OrgStore) ListOrganizations(ctx context.Context) ([]gen.Organization, error) {
	return s.q.ListOrganizations(ctx)
}
