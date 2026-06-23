package projection

import (
	"context"

	"ashyqqala/server/internal/store/gen"
)

// GeoLotStore — ⏳ ИНТЕРИМ (Story 0.7, трек «Парсер-мост»): запись batch-Nominatim-координат scraped-лотов
// в interim_geo_lots. Это batch-ПРОИЗВОДНАЯ запись (геокодер выводит координаты из адреса лота), а НЕ
// кураторская правка — каноническая курация geo_objects (Directus, verified/wrong_reported) появится в Epic 3.
type GeoLotStore struct{ q *gen.Queries }

// NewGeoLotStore — конструктор поверх пула/транзакции (gen.DBTX).
func NewGeoLotStore(db gen.DBTX) *GeoLotStore { return &GeoLotStore{q: gen.New(db)} }

// UpsertGeoLot — идемпотентная запись гео-результата (повтор batch не плодит дубли — UPSERT по goszakup_lot_id).
// unmatched → Lat/Lon с Valid=false (NULL), НЕ 0,0 (честность).
func (s *GeoLotStore) UpsertGeoLot(ctx context.Context, p gen.UpsertGeoLotParams) error {
	return s.q.UpsertGeoLot(ctx, p)
}

// GetGeoLotByLotID — гео-результат по natural goszakup_lot_id.
func (s *GeoLotStore) GetGeoLotByLotID(ctx context.Context, goszakupLotID string) (gen.InterimGeoLot, error) {
	return s.q.GetGeoLotByLotID(ctx, goszakupLotID)
}
