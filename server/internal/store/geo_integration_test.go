//go:build integration

// Интеграционный тест bbox-чтения geo_objects (Story 3.1, Task 6) против реальной БД (миграции 0001-0021):
// проверяет ПОВЕДЕНИЕ БД, не покрываемое юнитом — bbox-пересечение (&&/ST_MakeEnvelope), исключение
// unmatched (geom NULL), порядок координат GeoJSON [lon,lat] (AR-19). Самодостаточен: своя транзакция +
// координаты вне bbox Астаны/seed (изоляция, аналог districts_integration_test.go/cmd/geocode). Гоняется:
// `go test -tags=integration -count=1 ./internal/store/...`.
package store

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestListGeoObjectsInBBox_Integration(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL не задан — пропуск интеграционного теста")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("подключение к БД: %v", err)
	}
	defer pool.Close()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // изоляция: всё откатываем, БД остаётся чистой

	// Координаты УМЫШЛЕННО вне bbox Астаны (geo.AstanaViewbox 71.20-71.78/51.00-51.30) и вне seed-района
	// «Есиль» — БД персистентна между сессиями (docker volume), пересечение дало бы ложный результат
	// (урок cmd/geocode/idempotency_integration_test.go).
	insertContract := func(gid string) int64 {
		t.Helper()
		var id int64
		if err := tx.QueryRow(ctx,
			`INSERT INTO contracts (goszakup_contract_id, subject_ru, status, direction) VALUES ($1, 'ZZBBOX', 'active', 'road') RETURNING id`,
			gid).Scan(&id); err != nil {
			t.Fatalf("insert contract %s: %v", gid, err)
		}
		return id
	}
	// Сырой SQL (НЕ через UpsertGeoObject/Task 2) — тест читает bbox, не переверяет вывод length_km
	// (это покрыто cmd/geocode/idempotency_integration_test.go); length_km у линии задаётся явно, как
	// делает fixtures/seed/geo_objects.sql (сырой INSERT — тот же путь, что и у будущей Directus-курации,
	// НЕ через Go UpsertGeoObject).
	insertGeoObject := func(gid string, contractID int64, status, geomWKT string, lengthKm *float64) {
		t.Helper()
		var geomExpr, lengthExpr any
		if geomWKT != "" {
			geomExpr = geomWKT
		}
		if lengthKm != nil {
			lengthExpr = *lengthKm
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO geo_objects (contract_id, geocode_status, geom, length_km, geocoded_by)
			 VALUES ($1, $2, CASE WHEN $3::text IS NULL THEN NULL ELSE ST_GeomFromText($3, 4326) END, $4, 'test')`,
			contractID, status, geomExpr, lengthExpr); err != nil {
			t.Fatalf("insert geo_object %s: %v", gid, err)
		}
	}

	// lon≠lat УМЫШЛЕННО (код-ревью, Acceptance Auditor): симметричная точка (15.50,15.50) не может отличить
	// правильный порядок [lon,lat] от случайно перепутанного [lat,lon] — assert прошёл бы в обоих случаях.
	insidePoint := insertContract("ZZBBOX-IN-1")
	insertGeoObject("ZZBBOX-IN-1", insidePoint, "auto", "POINT(15.30 15.70)", nil)

	outsidePoint := insertContract("ZZBBOX-OUT-1")
	insertGeoObject("ZZBBOX-OUT-1", outsidePoint, "auto", "POINT(50.00 50.00)", nil)

	unmatched := insertContract("ZZBBOX-UNM-1")
	insertGeoObject("ZZBBOX-UNM-1", unmatched, "unmatched", "", nil)

	lineLenKm := 7.25
	insideLine := insertContract("ZZBBOX-LINE-1")
	insertGeoObject("ZZBBOX-LINE-1", insideLine, "manual", "LINESTRING(15.10 15.10, 15.20 15.20)", &lineLenKm)

	// bbox = (15.00,15.00)-(16.00,16.00): захватывает insidePoint + insideLine, НЕ outsidePoint/unmatched.
	got, err := ListGeoObjectsInBBox(ctx, tx, 15.00, 15.00, 16.00, 16.00)
	if err != nil {
		t.Fatalf("ListGeoObjectsInBBox: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("получено %d объектов, ожидалось 2 (точка+линия внутри bbox; снаружи и unmatched исключены)", len(got))
	}

	byContract := map[int64]GeoObjectPoint{}
	for _, p := range got {
		if p.ContractID == nil {
			t.Fatalf("ContractID = nil у %+v, ожидался не-nil", p)
		}
		byContract[*p.ContractID] = p
	}
	if _, ok := byContract[insidePoint]; !ok {
		t.Error("точка внутри bbox не найдена в результате")
	}
	if _, ok := byContract[insideLine]; !ok {
		t.Error("линия внутри bbox не найдена в результате")
	}
	if _, ok := byContract[outsidePoint]; ok {
		t.Error("точка СНАРУЖИ bbox попала в результат — bbox-фильтр не работает")
	}
	if _, ok := byContract[unmatched]; ok {
		t.Error("unmatched (geom NULL) попал в результат — честный фильтр geom IS NOT NULL нарушен")
	}

	// GeoJSON: порядок координат [lon,lat] (AR-19), НЕ [lat,lon].
	var geoJSON struct {
		Type        string    `json:"type"`
		Coordinates []float64 `json:"coordinates"`
	}
	pointResult := byContract[insidePoint]
	if err := json.Unmarshal(pointResult.Geom, &geoJSON); err != nil {
		t.Fatalf("парсинг GeoJSON точки: %v", err)
	}
	if geoJSON.Type != "Point" {
		t.Errorf("type = %q, ожидалось Point", geoJSON.Type)
	}
	if len(geoJSON.Coordinates) != 2 || geoJSON.Coordinates[0] != 15.30 || geoJSON.Coordinates[1] != 15.70 {
		t.Errorf("coordinates = %v, ожидалось [15.30,15.70] ([lon,lat] — AR-19; несимметричные значения ЛОВЯТ перепутанный порядок)", geoJSON.Coordinates)
	}
	if pointResult.PublicID == "" {
		t.Error("PublicID пуст, ожидался непустой UUID (public_id NOT NULL DEFAULT gen_random_uuid())")
	}
	if pointResult.GeocodeStatus != "auto" {
		t.Errorf("GeocodeStatus = %q, ожидалось auto", pointResult.GeocodeStatus)
	}

	lineResult := byContract[insideLine]
	var lineGeoJSON struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(lineResult.Geom, &lineGeoJSON); err != nil {
		t.Fatalf("парсинг GeoJSON линии: %v", err)
	}
	if lineGeoJSON.Type != "LineString" {
		t.Errorf("type = %q, ожидалось LineString", lineGeoJSON.Type)
	}
	if lineResult.LengthKm == nil || *lineResult.LengthKm != lineLenKm {
		t.Errorf("LengthKm = %v, ожидалось %v (read честно отдаёт то, что записано)", lineResult.LengthKm, lineLenKm)
	}

	// Пустой bbox (далеко от всего вставленного) → [] (не nil) — честная пустая коллекция.
	empty, err := ListGeoObjectsInBBox(ctx, tx, 80.00, 80.00, 81.00, 81.00)
	if err != nil {
		t.Fatalf("ListGeoObjectsInBBox (пустой bbox): %v", err)
	}
	if empty == nil {
		t.Error("empty == nil, ожидался непустой [] (честная пустая коллекция, не отсутствие данных)")
	}
	if len(empty) != 0 {
		t.Errorf("len(empty) = %d, ожидалось 0", len(empty))
	}
}
