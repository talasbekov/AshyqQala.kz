// Package projection — доступ к ПРОЕКЦИОННЫМ таблицам (derived из снапшота; импортёр перестраивает).
// Граница AR-4: проекционные ⊥ кураторские (`store/curation`) — разные пакеты и гранты. Story 2.0: lots.
package projection

import (
	"context"

	"ashyqqala/server/internal/store/gen"
)

// LotStore — доступ к проекционной таблице lots поверх sqlc-генерата.
type LotStore struct{ q *gen.Queries }

// NewLotStore — конструктор поверх пула/транзакции (gen.DBTX).
func NewLotStore(db gen.DBTX) *LotStore { return &LotStore{q: gen.New(db)} }

// UpsertLot — идемпотентная запись лота в проекцию (повтор не плодит дубли — UPSERT по goszakup_lot_id).
func (s *LotStore) UpsertLot(ctx context.Context, p gen.UpsertLotParams) error {
	return s.q.UpsertLot(ctx, p)
}

// GetLotByID — лот по публичному natural goszakup_lot_id (удалённые скрыты).
func (s *LotStore) GetLotByID(ctx context.Context, goszakupLotID string) (gen.Lot, error) {
	return s.q.GetLotByID(ctx, goszakupLotID)
}
