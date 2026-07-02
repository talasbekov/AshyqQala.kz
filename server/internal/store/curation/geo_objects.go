package curation

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	"ashyqqala/server/internal/store/gen"
)

// GeoObjectStore — доступ к КУРАТОРСКИМ таблицам geo_objects/districts (Story 3.1 AC1/AC2 + 3.2). Граница
// AR-4: batch-геокодер (canon, 3.1) и Directus (курация, 3.2) пишут; импортёр только читает. Повторный
// batch-прогон НИКОГДА не затирает курацию (гейт в UpsertGeoObject: WHERE geocode_status IN
// ('auto','unmatched') — manual/verified/wrong_reported переживают ре-геокод, зеркало AliasStore/UpsertAlias).
// Кураторские методы ниже (Resolve/Mark/Clear) — «имитация Directus» для тестов (прецедент 2.3) и шов для
// будущего провода error_reports→wrong_reported; сам Directus пишет в таблицу напрямую (решение D3/D5).
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

// ListGeoObjectsByStatus — очередь куратора (Story 3.2, зеркало AliasStore.ListAliasesByStatus):
// unmatched/wrong_reported — требуют ручной работы; auto — кандидаты верификации (сомнительные первыми).
func (s *GeoObjectStore) ListGeoObjectsByStatus(ctx context.Context, status string) ([]gen.ListGeoObjectsByStatusRow, error) {
	return s.q.ListGeoObjectsByStatus(ctx, status)
}

// ResolveGeoObjectManually — куратор ставит/корректирует геометрию (POINT|LINESTRING) → manual (AC1).
// Из любого статуса, КРОМЕ verified (D1: перерисовка подтверждённой точки — сначала wrong_reported,
// двухшаговый след SM-C2). length_km выводит триггер 0022; confidence обнуляется (importance неприменим
// к ручной точке). Возврат — число затронутых строк: 0 = id не найден ИЛИ запрещённый verified→manual.
func (s *GeoObjectStore) ResolveGeoObjectManually(ctx context.Context, id int64, geomWKT, by string) (int64, error) {
	return s.q.ResolveGeoObjectManually(ctx, gen.ResolveGeoObjectManuallyParams{
		ID: id, GeomWkt: geomWKT, GeocodedBy: pgtype.Text{String: by, Valid: true},
	})
}

// MarkGeoObjectVerified — куратор подтверждает авто-точку (AC3, AR-28): ТОЛЬКО auto→verified; точка/
// провенанс не меняются. 0 строк = запрещённый переход (не-auto) или id не найден.
func (s *GeoObjectStore) MarkGeoObjectVerified(ctx context.Context, id int64) (int64, error) {
	return s.q.MarkGeoObjectVerified(ctx, id)
}

// MarkGeoObjectWrongReported — пометка «точка не там» (AC3, AR-28, питает SM-C2): auto|manual|verified →
// wrong_reported; спорная геометрия сохраняется до решения куратора. 0 строк = unmatched/не найден.
func (s *GeoObjectStore) MarkGeoObjectWrongReported(ctx context.Context, id int64) (int64, error) {
	return s.q.MarkGeoObjectWrongReported(ctx, id)
}

// ClearGeoObjectToUnmatched — куратор снимает неверную точку (wrong_reported→unmatched, geom=NULL —
// честное «без точки на карте», AC2). ТОЛЬКО из wrong_reported (двухшаговый след для SM-C2).
func (s *GeoObjectStore) ClearGeoObjectToUnmatched(ctx context.Context, id int64, by string) (int64, error) {
	return s.q.ClearGeoObjectToUnmatched(ctx, gen.ClearGeoObjectToUnmatchedParams{
		ID: id, GeocodedBy: pgtype.Text{String: by, Valid: true},
	})
}
