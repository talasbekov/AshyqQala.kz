package curation

import (
	"context"

	"ashyqqala/server/internal/store/gen"
)

// AliasStore — доступ к КУРАТОРСКОЙ таблице org_name_aliases (нормализация наименований→БИН, FR-2; Story 2.3).
// Граница AR-4: нормализатор пишет авто-строки и заводит очередь (manual/conflict); ручные разрешения
// оператора переживают ре-импорт (UpsertAlias гейтит обновление по resolve_status='auto').
type AliasStore struct{ q *gen.Queries }

// NewAliasStore — конструктор поверх пула/транзакции (gen.DBTX).
func NewAliasStore(db gen.DBTX) *AliasStore { return &AliasStore{q: gen.New(db)} }

// UpsertAlias — идемпотентная запись решения нормализатора (по raw_name+source). Ре-импорт пере-выводит
// только авто-строки; manual/conflict (очередь/разрешения оператора) не затираются.
func (s *AliasStore) UpsertAlias(ctx context.Context, p gen.UpsertAliasParams) error {
	return s.q.UpsertAlias(ctx, p)
}

// ResolveAliasManually — оператор разрешает запись очереди: проставляет organization_id (имитация Directus).
func (s *AliasStore) ResolveAliasManually(ctx context.Context, p gen.ResolveAliasManuallyParams) error {
	return s.q.ResolveAliasManually(ctx, p)
}

// GetAlias — запись псевдонима по (raw_name, source) — для проверки статуса резолва / «профиль уточняется».
func (s *AliasStore) GetAlias(ctx context.Context, p gen.GetAliasParams) (gen.OrgNameAlias, error) {
	return s.q.GetAlias(ctx, p)
}

// ListAliasesByStatus — очередь оператору: псевдонимы статуса (manual/conflict — «требует проверки»).
func (s *AliasStore) ListAliasesByStatus(ctx context.Context, status string) ([]gen.OrgNameAlias, error) {
	return s.q.ListAliasesByStatus(ctx, status)
}

// ListResolvedAliases — разрешённые псевдонимы + БИН (обратная связь резолва: дополнительные написания).
func (s *AliasStore) ListResolvedAliases(ctx context.Context) ([]gen.ListResolvedAliasesRow, error) {
	return s.q.ListResolvedAliases(ctx)
}
