//go:build integration

// Интеграционный тест канон-геопривязки (Story 3.1, Task 2) против реальной БД (миграции 0001-0021):
// проверяет ПОВЕДЕНИЕ БД, не покрываемое юнитом с фейками — идемпотентность UPSERT по contract_id,
// AR-4-гейт «manual переживает ре-геокод» на уровне SQL WHERE, geometry round-trip (ST_Contains/КАТО-
// префикс), и honesty-CHECK geo_objects_geom_null_chk (unmatched-с-точкой / auto-без-точки REJECTED —
// зеркало 0020). Самодостаточен: своя транзакция + синтетический goszakup_contract_id-префикс "ZZGEO-",
// откатывается в конце (изоляция, аналог internal/store/districts_integration_test.go). Гоняется:
// `go test -tags=integration -count=1 ./cmd/geocode/...`.
package main

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/store/curation"
	"ashyqqala/server/internal/store/gen"
)

func mustConn(t *testing.T) (*pgxpool.Pool, pgx.Tx) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL не задан — пропуск интеграционного теста")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("подключение к БД: %v", err)
	}
	t.Cleanup(pool.Close)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) }) //nolint:errcheck // изоляция: всё откатываем
	return pool, tx
}

func insertContract(t *testing.T, tx pgx.Tx, gid, kato string) int64 {
	t.Helper()
	var id int64
	err := tx.QueryRow(context.Background(),
		`INSERT INTO contracts (goszakup_contract_id, subject_ru, status, direction, kato_code)
		 VALUES ($1, 'Синтетика geocode-интеграции', 'active', 'road', $2) RETURNING id`,
		gid, kato).Scan(&id)
	if err != nil {
		t.Fatalf("insert contract %s: %v", gid, err)
	}
	return id
}

// insertDistrict — kato="" вставляет SQL NULL (не пустую строку — districts_kato_uniq частичен
// «WHERE kato_code IS NOT NULL», пустая строка ВСЁ РАВНО попала бы под него).
func insertDistrict(t *testing.T, tx pgx.Tx, nameRu, kato, polygonWKT string) int64 {
	t.Helper()
	var id int64
	var katoArg any
	if kato != "" {
		katoArg = kato
	}
	err := tx.QueryRow(context.Background(),
		`INSERT INTO districts (kato_code, name_ru, name_kk, geom) VALUES ($1, $2, $2, ST_GeomFromText($3, 4326)) RETURNING id`,
		katoArg, nameRu, polygonWKT).Scan(&id)
	if err != nil {
		t.Fatalf("insert district %s: %v", nameRu, err)
	}
	return id
}

// TestUpsertGeoObject_Idempotent — повторный UPSERT по тому же contract_id ОБНОВЛЯЕТ строку, не плодит
// дубль (Story 3.1, AC1 — batch-прогон переиграть безопасно).
func TestUpsertGeoObject_Idempotent(t *testing.T) {
	_, tx := mustConn(t)
	ctx := context.Background()
	s := curationStoreOn(tx)

	contractID := insertContract(t, tx, "ZZGEO-IDEMP-1", "710000000")

	first := gen.UpsertGeoObjectParams{
		ContractID:    pgtype.Int8{Int64: contractID, Valid: true},
		GeomWkt:       pgtype.Text{String: "POINT(71.40 51.10)", Valid: true},
		GeocodeStatus: "auto",
		Confidence:    pgtype.Float8{Float64: 0.5, Valid: true},
		GeocodedBy:    pgtype.Text{String: "nominatim", Valid: true},
	}
	if err := s.UpsertGeoObject(ctx, first); err != nil {
		t.Fatalf("первый UPSERT: %v", err)
	}
	second := first
	second.GeomWkt = pgtype.Text{String: "POINT(71.45 51.12)", Valid: true}
	second.Confidence = pgtype.Float8{Float64: 0.9, Valid: true}
	if err := s.UpsertGeoObject(ctx, second); err != nil {
		t.Fatalf("второй UPSERT (переиграть): %v", err)
	}

	row, err := s.GetGeoObjectByContractID(ctx, contractID)
	if err != nil {
		t.Fatalf("GetGeoObjectByContractID: %v", err)
	}
	if !row.Confidence.Valid || row.Confidence.Float64 != 0.9 {
		t.Errorf("confidence = %+v, ожидалось 0.9 (вторая запись ПЕРЕЗАПИСАЛА первую — идемпотентность)", row.Confidence)
	}

	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM geo_objects WHERE contract_id = $1`, contractID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("строк geo_objects для контракта = %d, ожидалась 1 (UPSERT не должен плодить дубли)", count)
	}
}

// TestUpsertGeoObject_LengthKm_DerivedFromLineString — Story 3.1 Task 3: length_km ВЫВОДИТСЯ из geom
// (ST_Length(geom::geography)/1000 для LINESTRING), не принимается параметром — «забыли пересчитать»
// структурно невозможно. Значение сверяется с независимым psql-расчётом (не просто «не NULL»).
func TestUpsertGeoObject_LengthKm_DerivedFromLineString(t *testing.T) {
	_, tx := mustConn(t)
	ctx := context.Background()
	s := curationStoreOn(tx)

	const line = "LINESTRING(71.40 51.10, 71.45 51.14)"
	var wantKm float64
	if err := tx.QueryRow(ctx, `SELECT ST_Length(ST_GeomFromText($1,4326)::geography)/1000.0`, line).Scan(&wantKm); err != nil {
		t.Fatalf("независимый расчёт длины: %v", err)
	}

	contractID := insertContract(t, tx, "ZZGEO-LEN-1", "710000000")
	err := s.UpsertGeoObject(ctx, gen.UpsertGeoObjectParams{
		ContractID:    pgtype.Int8{Int64: contractID, Valid: true},
		GeomWkt:       pgtype.Text{String: line, Valid: true},
		GeocodeStatus: "manual", // LINESTRING — ручная/Directus-курация (3.2) или синтетик; batch-геокод даёт только точки
		GeocodedBy:    pgtype.Text{String: "test-fixture", Valid: true},
	})
	if err != nil {
		t.Fatalf("UPSERT LINESTRING: %v", err)
	}

	row, err := s.GetGeoObjectByContractID(ctx, contractID)
	if err != nil {
		t.Fatalf("GetGeoObjectByContractID: %v", err)
	}
	if !row.LengthKm.Valid {
		t.Fatal("LengthKm.Valid = false для LINESTRING, ожидалось вычисленное значение")
	}
	if diff := row.LengthKm.Float64 - wantKm; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("LengthKm = %v, ожидалось %v (независимый ST_Length-расчёт)", row.LengthKm.Float64, wantKm)
	}
}

// TestUpsertGeoObject_LengthKm_NullForPoint — POINT (обычный batch-геокод по адресу) → length_km ЧЕСТНО
// NULL, НЕ 0 (нет длины у точки — не выдумываем число).
func TestUpsertGeoObject_LengthKm_NullForPoint(t *testing.T) {
	_, tx := mustConn(t)
	ctx := context.Background()
	s := curationStoreOn(tx)

	contractID := insertContract(t, tx, "ZZGEO-LEN-2", "710000000")
	err := s.UpsertGeoObject(ctx, gen.UpsertGeoObjectParams{
		ContractID:    pgtype.Int8{Int64: contractID, Valid: true},
		GeomWkt:       pgtype.Text{String: "POINT(71.40 51.10)", Valid: true},
		GeocodeStatus: "auto",
		GeocodedBy:    pgtype.Text{String: "nominatim", Valid: true},
	})
	if err != nil {
		t.Fatalf("UPSERT POINT: %v", err)
	}

	row, err := s.GetGeoObjectByContractID(ctx, contractID)
	if err != nil {
		t.Fatalf("GetGeoObjectByContractID: %v", err)
	}
	if row.LengthKm.Valid {
		t.Errorf("LengthKm = %+v для POINT, ожидалось NULL (честно — у точки нет длины, не 0)", row.LengthKm)
	}
}

// TestUpsertGeoObject_ManualSurvivesReGeocode — AR-4: batch-геокод (auto) НИКОГДА не затирает manual-строку
// (курация Directus 3.2 переживает ре-геокод). Гейт проверяется НА УРОВНЕ SQL (WHERE geocode_status IS
// DISTINCT FROM 'manual'), не только в Go-обёртке.
func TestUpsertGeoObject_ManualSurvivesReGeocode(t *testing.T) {
	_, tx := mustConn(t)
	ctx := context.Background()
	s := curationStoreOn(tx)

	contractID := insertContract(t, tx, "ZZGEO-MANUAL-1", "710000000")

	manual := gen.UpsertGeoObjectParams{
		ContractID:    pgtype.Int8{Int64: contractID, Valid: true},
		GeomWkt:       pgtype.Text{String: "POINT(71.50 51.15)", Valid: true},
		GeocodeStatus: "manual",
		GeocodedBy:    pgtype.Text{String: "directus", Valid: true},
	}
	if err := s.UpsertGeoObject(ctx, manual); err != nil {
		t.Fatalf("UPSERT manual: %v", err)
	}

	// Batch-геокод пытается перезаписать другой точкой/статусом auto — ДОЛЖЕН быть проигнорирован.
	reGeocode := gen.UpsertGeoObjectParams{
		ContractID:    pgtype.Int8{Int64: contractID, Valid: true},
		GeomWkt:       pgtype.Text{String: "POINT(0.01 0.01)", Valid: true},
		GeocodeStatus: "auto",
		Confidence:    pgtype.Float8{Float64: 0.99, Valid: true},
		GeocodedBy:    pgtype.Text{String: "nominatim", Valid: true},
	}
	if err := s.UpsertGeoObject(ctx, reGeocode); err != nil {
		t.Fatalf("UPSERT re-geocode (должен молча не сработать, не ошибиться): %v", err)
	}

	row, err := s.GetGeoObjectByContractID(ctx, contractID)
	if err != nil {
		t.Fatalf("GetGeoObjectByContractID: %v", err)
	}
	if row.GeocodeStatus != "manual" {
		t.Errorf("geocode_status = %q, ожидалось manual (AR-4: batch НЕ должен затирать курацию)", row.GeocodeStatus)
	}
	if !row.GeocodedBy.Valid || row.GeocodedBy.String != "directus" {
		t.Errorf("geocoded_by = %+v, ожидалось directus (не перезаписано)", row.GeocodedBy)
	}
}

// TestFindDistrictIDByPoint_GeometryRoundTrip — точка ВНУТРИ полигона района находится (ST_Contains);
// точка СНАРУЖИ — честный pgx.ErrNoRows (AC2 «при наличии точки»). Координаты УМЫШЛЕННО далеко от bbox
// Астаны/seed-района «Есиль» (71.30–71.55/51.05–51.20, fixtures/seed/geo_objects.sql) — БД персистентна
// между сессиями (docker volume), пересечение с реальными/seed-полигонами дало бы ложный матч чужого id.
func TestFindDistrictIDByPoint_GeometryRoundTrip(t *testing.T) {
	_, tx := mustConn(t)
	ctx := context.Background()
	s := curationStoreOn(tx)

	districtID := insertDistrict(t, tx, "ZZGEO-Район", "", "POLYGON((10.00 10.00,10.00 11.00,11.00 11.00,11.00 10.00,10.00 10.00))")

	insideID, err := s.FindDistrictIDByPoint(ctx, "POINT(10.50 10.50)")
	if err != nil {
		t.Fatalf("точка внутри полигона: неожиданная ошибка %v", err)
	}
	if insideID != districtID {
		t.Errorf("districtID = %d, ожидалось %d (точка внутри полигона)", insideID, districtID)
	}

	_, err = s.FindDistrictIDByPoint(ctx, "POINT(20.00 20.00)")
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("точка снаружи всех полигонов: err=%v, ожидался pgx.ErrNoRows (честное «нет района»)", err)
	}
}

// TestFindDistrictIDByKATOPrefix_LongestMatch — самое длинное совпадение КАТО-префикса выигрывает
// (иерархическая вложенность город⊃район, зеркало district.Catalog.NameByKATO). КАТО-префикс «7195» —
// синтетика, изолирована от seed («71»/«710000000», fixtures/seed/geo_objects.sql) префиксным несовпадением
// (зеркало internal/store/districts_integration_test.go: «710512» против seed-«710000000»).
func TestFindDistrictIDByKATOPrefix_LongestMatch(t *testing.T) {
	_, tx := mustConn(t)
	ctx := context.Background()
	s := curationStoreOn(tx)

	cityID := insertDistrict(t, tx, "ZZGEO-Город", "7195", "POLYGON((0 0,0 0.001,0.001 0.001,0.001 0,0 0))")
	districtID := insertDistrict(t, tx, "ZZGEO-Есиль", "719500000", "POLYGON((0 0,0 0.001,0.001 0.001,0.001 0,0 0))")

	gotDistrict, err := s.FindDistrictIDByKATOPrefix(ctx, "719500000123")
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if gotDistrict != districtID {
		t.Errorf("districtID = %d, ожидалось %d (самое длинное совпадение «719500000», не «7195»)", gotDistrict, districtID)
	}

	gotCity, err := s.FindDistrictIDByKATOPrefix(ctx, "719599999")
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if gotCity != cityID {
		t.Errorf("districtID = %d, ожидалось %d (совпадает только городской префикс «7195»)", gotCity, cityID)
	}
}

// TestUpsertGeoObject_HonestyCheck_RejectsUnmatchedWithPoint / …RejectsAutoWithoutPoint — DB CHECK
// geo_objects_geom_null_chk (миграция 0020) — последняя линия защиты НЕЗАВИСИМО от корректности Go-
// маппера (toGeoParams структурно не производит эти комбинации, но страж должен существовать на уровне
// схемы, зеркало 0020 dev notes). ДВА теста (не один): CHECK-нарушение абортит всю Postgres-транзакцию
// (SQLSTATE 25P02 на любой следующей команде) — один общий tx для обеих проверок невозможен без SAVEPOINT.
func TestUpsertGeoObject_HonestyCheck_RejectsUnmatchedWithPoint(t *testing.T) {
	_, tx := mustConn(t)
	ctx := context.Background()
	s := curationStoreOn(tx)

	unmatchedWithPoint := insertContract(t, tx, "ZZGEO-CHK-1", "710000000")
	err := s.UpsertGeoObject(ctx, gen.UpsertGeoObjectParams{
		ContractID:    pgtype.Int8{Int64: unmatchedWithPoint, Valid: true},
		GeomWkt:       pgtype.Text{String: "POINT(71.4 51.1)", Valid: true}, // точка есть...
		GeocodeStatus: "unmatched",                                          // ...но статус unmatched → CHECK обязан отклонить
	})
	if err == nil {
		t.Error("unmatched-с-точкой должен быть REJECTED honesty-CHECK'ом (geo_objects_geom_null_chk)")
	}
}

func TestUpsertGeoObject_HonestyCheck_RejectsAutoWithoutPoint(t *testing.T) {
	_, tx := mustConn(t)
	ctx := context.Background()
	s := curationStoreOn(tx)

	autoWithoutPoint := insertContract(t, tx, "ZZGEO-CHK-2", "710000000")
	err := s.UpsertGeoObject(ctx, gen.UpsertGeoObjectParams{
		ContractID:    pgtype.Int8{Int64: autoWithoutPoint, Valid: true},
		GeocodeStatus: "auto", // auto без точки → CHECK обязан отклонить (auto ⇒ coords, инвариант 0.7/0.8)
	})
	if err == nil {
		t.Error("auto-без-точки должен быть REJECTED honesty-CHECK'ом (geo_objects_geom_null_chk)")
	}
}

// curationStoreOn — конструктор GeoObjectStore поверх ОДНОЙ tx (все методы теста делят транзакцию —
// откат в mustConn отменяет всё разом).
func curationStoreOn(tx pgx.Tx) *curation.GeoObjectStore { return curation.NewGeoObjectStore(tx) }
