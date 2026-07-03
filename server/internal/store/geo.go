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
// форму wire-ответа решает handler Story 3.4). GoszakupContractID/HasActiveFlag — Story 3.4: identity
// маркера для выбора/перехода (3.5) и признак активного риск-флага (глиф «!», амбер-кольцо кластера, AC2).
type GeoObjectPoint struct {
	PublicID           string
	ContractID         *int64
	GoszakupContractID *string
	DistrictID         *int64
	GeocodeStatus      string
	HasActiveFlag      bool
	Confidence         *float64
	LengthKm           *float64
	Geom               json.RawMessage
}

// ListGeoObjectsInBBox — объекты внутри bbox (WGS84 lon/lat), геокодированные (geom IS NOT NULL — карте
// нечего рисовать у unmatched, честно, НЕ 0,0). `&&` — оператор пересечения bounding box (GiST-индекс
// geo_objects_geom_gist, миграция 0020) — быстрый предфильтр перед точным ST_Intersects, если он
// когда-либо понадобится потребителю; сейчас bbox-пересечения достаточно (маркеры на карте не режутся по
// границе окна день в день).
// limit — cap выдачи (Story 3.4, NFR-1): запрашивается limit+1 строка, второй результат true = «в окне
// больше, чем показано» (честная truncated-детекция, не тихое обрезание). limit<=0 — ошибка вызывающего.
func ListGeoObjectsInBBox(ctx context.Context, db gen.DBTX, minLon, minLat, maxLon, maxLat float64, limit int) ([]GeoObjectPoint, bool, error) {
	if err := ValidateBBox(minLon, minLat, maxLon, maxLat); err != nil {
		return nil, false, err // код-ревью: NaN/Inf/инвертированный bbox иначе тихо ушёл бы в ST_MakeEnvelope
		// (PostGIS-поведение на мусорном envelope не документировано здесь и не проверено — честнее
		// отказать явно на границе функции, чем полагаться на непроверенную деградацию сервера).
	}
	if limit <= 0 {
		return nil, false, fmt.Errorf("store: ListGeoObjectsInBBox: неположительный limit (%d)", limit)
	}
	// LEFT JOIN contracts: contract_id у geo_objects nullable — строка без контракта честно несёт
	// goszakup=nil/flag=false. EXISTS по risk_flags — тот же паттерн, что has_active_flag в ListContracts.
	// is_deleted (код-ревью 3.4): удалённый контракт невидим в списке/поиске/счётчике «без точки» — маркер
	// на карте был единственным исключением; гео-строка БЕЗ контракта (c.id IS NULL) остаётся видимой.
	// ORDER BY has_active_flag DESC (код-ревью 3.4, решение владельца): при cap-усечении окна активные
	// сигналы переживают срез первыми — иначе флаг-объекты могли целиком выпасть за LIMIT, оставив
	// пользователю только generic-плашку «показаны не все».
	const q = `
		SELECT g.public_id, g.contract_id, g.district_id, g.geocode_status, g.confidence, g.length_km,
		       ST_AsGeoJSON(g.geom)::jsonb AS geom_geojson,
		       c.goszakup_contract_id,
		       EXISTS (SELECT 1 FROM risk_flags rf WHERE rf.contract_id = c.id AND rf.is_active) AS has_active_flag
		FROM geo_objects g
		LEFT JOIN contracts c ON c.id = g.contract_id
		WHERE g.geom IS NOT NULL
		  AND g.geom && ST_MakeEnvelope($1, $2, $3, $4, 4326)
		  AND (c.id IS NULL OR NOT c.is_deleted)
		ORDER BY has_active_flag DESC, g.id
		LIMIT $5`
	rows, err := db.Query(ctx, q, minLon, minLat, maxLon, maxLat, limit+1)
	if err != nil {
		return nil, false, fmt.Errorf("store: ListGeoObjectsInBBox: %w", err)
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
			goszakupID    pgtype.Text
			hasActiveFlag bool
		)
		if err := rows.Scan(&publicID, &contractID, &districtID, &geocodeStatus, &confidence, &lengthKm, &geomGeoJSON, &goszakupID, &hasActiveFlag); err != nil {
			return nil, false, fmt.Errorf("store: ListGeoObjectsInBBox: scan: %w", err)
		}
		p := GeoObjectPoint{
			PublicID:      formatUUID(publicID),
			GeocodeStatus: geocodeStatus,
			HasActiveFlag: hasActiveFlag,
			Geom:          json.RawMessage(geomGeoJSON),
		}
		if contractID.Valid {
			id := contractID.Int64
			p.ContractID = &id
		}
		if goszakupID.Valid {
			g := goszakupID.String
			p.GoszakupContractID = &g
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
		return nil, false, fmt.Errorf("store: ListGeoObjectsInBBox: rows: %w", err)
	}
	truncated := false
	if len(out) > limit {
		truncated = true
		out = out[:limit]
	}
	return out, truncated, nil
}

// CountContractsWithoutPoint — счётчик «без точки на карте» (Story 3.4, AC3): живые контракты, у которых
// либо нет гео-строки вовсе, либо она честно unmatched (geom NULL). Питает аффордансу-счётчик
// «Ещё N объектов без точки на карте» — их видимость на карте, раз маркера нет структурно.
func CountContractsWithoutPoint(ctx context.Context, db gen.DBTX) (int64, error) {
	const q = `
		SELECT count(*)
		FROM contracts c
		LEFT JOIN geo_objects g ON g.contract_id = c.id
		WHERE NOT c.is_deleted
		  AND (g.id IS NULL OR g.geocode_status = 'unmatched')`
	var n int64
	if err := db.QueryRow(ctx, q).Scan(&n); err != nil {
		return 0, fmt.Errorf("store: CountContractsWithoutPoint: %w", err)
	}
	return n, nil
}

// ValidateBBox — честный отказ на NaN/Inf/вне-диапазона/инвертированном bbox (код-ревью, Edge Case
// Hunter): без этой проверки мусорные координаты молча уходили бы в ST_MakeEnvelope — поведение PostGIS
// на таком вводе здесь не проверялось и не документировано, так что явная ошибка на границе функции
// честнее непроверенной деградации. WGS84: lon∈[-180,180], lat∈[-90,90]; min ОБЯЗАН быть < max по обеим осям.
// Экспортирована для хендлера карты (Story 3.4): невалидный bbox из query — это 400 VALIDATION_FAILED
// пользователя, а не 500 сервера — хендлер отличает валидационную ошибку, вызывая проверку сам.
func ValidateBBox(minLon, minLat, maxLon, maxLat float64) error {
	for _, v := range []float64{minLon, minLat, maxLon, maxLat} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("store: bbox: нефинитная координата (%v)", v)
		}
	}
	if minLon < -180 || maxLon > 180 || minLat < -90 || maxLat > 90 {
		return fmt.Errorf("store: bbox: вне диапазона WGS84 (lon=[%v,%v] lat=[%v,%v])", minLon, maxLon, minLat, maxLat)
	}
	if minLon >= maxLon || minLat >= maxLat {
		return fmt.Errorf("store: bbox: инвертированный/вырожденный (minLon=%v maxLon=%v minLat=%v maxLat=%v)", minLon, maxLon, minLat, maxLat)
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
