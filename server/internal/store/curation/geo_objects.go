package curation

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	"ashyqqala/server/internal/store/gen"
)

// GeoObjectStore — доступ к КУРАТОРСКИМ таблицам geo_objects/districts (Story 3.1, AC1/AC2). Граница AR-4:
// batch-геокодер (canon, Story 3.1) и Directus (manual, Story 3.2) пишут; импортёр только читает. Повторный
// batch-прогон НИКОГДА не затирает manual-строки (гейт в UpsertGeoObject WHERE geocode_status IS DISTINCT
// FROM 'manual' — курация переживает ре-геокод, зеркало AliasStore/UpsertAlias для org_name_aliases).
type GeoObjectStore struct{ q *gen.Queries }

// NewGeoObjectStore — конструктор поверх пула/транзакции (gen.DBTX).
func NewGeoObjectStore(db gen.DBTX) *GeoObjectStore { return &GeoObjectStore{q: gen.New(db)} }

// UpsertGeoObject — идемпотентная запись геопривязки по contract_id. manual (курация 3.2) не затирается.
func (s *GeoObjectStore) UpsertGeoObject(ctx context.Context, p gen.UpsertGeoObjectParams) error {
	return s.q.UpsertGeoObject(ctx, p)
}

// FindDistrictIDByPoint — район, чей полигон содержит геокодированную точку (AC2, «при наличии точки»).
// pgx.ErrNoRows = честное «нет совпавшего района» (НЕ ошибка) — вызывающий трактует как district_id=NULL.
func (s *GeoObjectStore) FindDistrictIDByPoint(ctx context.Context, geomWKT string) (int64, error) {
	return s.q.FindDistrictIDByPoint(ctx, geomWKT)
}

// FindDistrictIDByKATOPrefix — район по КАТО-префиксу без точки (AC2, зеркало 6.3/district.Catalog.NameByKATO).
// pgx.ErrNoRows = честное «нет совпавшего района».
func (s *GeoObjectStore) FindDistrictIDByKATOPrefix(ctx context.Context, contractKato string) (int64, error) {
	return s.q.FindDistrictIDByKATOPrefix(ctx, contractKato)
}

// GetGeoObjectByContractID — гео-результат по contract_id (для тестов/проверки, аналог GetGeoLotByLotID).
func (s *GeoObjectStore) GetGeoObjectByContractID(ctx context.Context, contractID int64) (gen.GetGeoObjectByContractIDRow, error) {
	return s.q.GetGeoObjectByContractID(ctx, pgtype.Int8{Int64: contractID, Valid: true})
}
