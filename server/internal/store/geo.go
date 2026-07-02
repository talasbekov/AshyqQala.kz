// Package store — корень слоя доступа к данным.
// geo.go — pgx-raw динамический bbox-запрос карты (Story 3.1 Task 6): координаты окна приходят из запроса
// карты (не фиксированная форма) — сырой pgx, НЕ sqlc (architecture.md: «динамический bbox-запрос карты
// при необходимости — pgx-raw в internal/store/geo.go»). Только чтение геокодированных geo_objects; запись —
// store/curation/geo_objects.go (Task 2, AR-4-граница). Метаданные projection_snapshot — остаток S-0-стаба,
// вне Story 3.1.
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5/pgtype"

	"ashyqqala/server/internal/store/gen"
)

// GeoObjectPoint — один объект геопривязки в bbox-выборке карты. Geom — GeoJSON RFC7946 (AR-19, порядок
// [lon,lat], SRID на проводе не пишем — ST_AsGeoJSON(geom)::jsonb на стороне SQL). Указатели — честный
// nullable-конверт на уровне Go (без отдельной пары {value,state}: список карты — не карточка контракта,
// потребитель — будущий handler Story 3.4, который сам решит форму wire-ответа).
type GeoObjectPoint struct {
	PublicID      string
	ContractID    *int64
	DistrictID    *int64
	GeocodeStatus string
	Confidence    *float64
	LengthKm      *float64
	Geom          json.RawMessage
}

// ListGeoObjectsInBBox — объекты внутри bbox (WGS84 lon/lat), геокодированные (geom IS NOT NULL — карте
// нечего рисовать у unmatched, честно, НЕ 0,0). `&&` — оператор пересечения bounding box (GiST-индекс
// geo_objects_geom_gist, миграция 0020) — быстрый предфильтр перед точным ST_Intersects, если он
// когда-либо понадобится потребителю; сейчас bbox-пересечения достаточно (маркеры на карте не режутся по
// границе окна день в день).
func ListGeoObjectsInBBox(ctx context.Context, db gen.DBTX, minLon, minLat, maxLon, maxLat float64) ([]GeoObjectPoint, error) {
	if err := validateBBox(minLon, minLat, maxLon, maxLat); err != nil {
		return nil, err // код-ревью: NaN/Inf/инвертированный bbox иначе тихо ушёл бы в ST_MakeEnvelope
		// (PostGIS-поведение на мусорном envelope не документировано здесь и не проверено — честнее
		// отказать явно на границе функции, чем полагаться на непроверенную деградацию сервера).
	}
	const q = `
		SELECT public_id, contract_id, district_id, geocode_status, confidence, length_km,
		       ST_AsGeoJSON(geom)::jsonb AS geom_geojson
		FROM geo_objects
		WHERE geom IS NOT NULL
		  AND geom && ST_MakeEnvelope($1, $2, $3, $4, 4326)
		ORDER BY id`
	rows, err := db.Query(ctx, q, minLon, minLat, maxLon, maxLat)
	if err != nil {
		return nil, fmt.Errorf("store: ListGeoObjectsInBBox: %w", err)
	}
	defer rows.Close()

	out := []GeoObjectPoint{} // [] не nil — честная пустая коллекция (пустой bbox ⊄ «нет данных»)
	for rows.Next() {
		var (
			publicID      pgtype.UUID
			contractID    pgtype.Int8
			districtID    pgtype.Int8
			geocodeStatus string
			confidence    pgtype.Float8
			lengthKm      pgtype.Float8
			geomGeoJSON   []byte
		)
		if err := rows.Scan(&publicID, &contractID, &districtID, &geocodeStatus, &confidence, &lengthKm, &geomGeoJSON); err != nil {
			return nil, fmt.Errorf("store: ListGeoObjectsInBBox: scan: %w", err)
		}
		p := GeoObjectPoint{
			PublicID:      formatUUID(publicID),
			GeocodeStatus: geocodeStatus,
			Geom:          json.RawMessage(geomGeoJSON),
		}
		if contractID.Valid {
			id := contractID.Int64
			p.ContractID = &id
		}
		if districtID.Valid {
			id := districtID.Int64
			p.DistrictID = &id
		}
		if confidence.Valid {
			c := confidence.Float64
			p.Confidence = &c
		}
		if lengthKm.Valid {
			l := lengthKm.Float64
			p.LengthKm = &l
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: ListGeoObjectsInBBox: rows: %w", err)
	}
	return out, nil
}

// validateBBox — честный отказ на NaN/Inf/вне-диапазона/инвертированном bbox (код-ревью, Edge Case
// Hunter): без этой проверки мусорные координаты молча уходили бы в ST_MakeEnvelope — поведение PostGIS
// на таком вводе здесь не проверялось и не документировано, так что явная ошибка на границе функции
// честнее непроверенной деградации. WGS84: lon∈[-180,180], lat∈[-90,90]; min ОБЯЗАН быть < max по обеим осям.
func validateBBox(minLon, minLat, maxLon, maxLat float64) error {
	for _, v := range []float64{minLon, minLat, maxLon, maxLat} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("store: ListGeoObjectsInBBox: нефинитная координата bbox (%v)", v)
		}
	}
	if minLon < -180 || maxLon > 180 || minLat < -90 || maxLat > 90 {
		return fmt.Errorf("store: ListGeoObjectsInBBox: bbox вне диапазона WGS84 (lon=[%v,%v] lat=[%v,%v])", minLon, maxLon, minLat, maxLat)
	}
	if minLon >= maxLon || minLat >= maxLat {
		return fmt.Errorf("store: ListGeoObjectsInBBox: инвертированный/вырожденный bbox (minLon=%v maxLon=%v minLat=%v maxLat=%v)", minLon, maxLon, minLat, maxLat)
	}
	return nil
}

// formatUUID — каноническая текстовая форма UUID (RFC 4122, 8-4-4-4-12) из pgtype.UUID. "" на
// невалидном значении — честно (вызывающему НЕЧЕГО показать, не выдуманный id).
func formatUUID(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	b := u.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
