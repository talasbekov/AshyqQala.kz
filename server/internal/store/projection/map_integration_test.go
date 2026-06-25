//go:build integration

// Интеграционный тест LEFT JOIN lots↔interim_geo_lots (Story 0.8, запрос ListLotsWithGeo для /api/lots).
// Гоняется отдельно: `go test -tags=integration ./internal/store/...` с DATABASE_URL (миграции 0001-0004).
// CI (`go test ./...` без тега) этот файл не компилирует → не требует БД.
//
// Проверяет несущее: лот С гео-строкой → координата + geocode_status='auto'; лот БЕЗ гео-строки →
// LEFT JOIN даёт NULL geocode_status (потребитель трактует как geocode_pending — честно, не «нет точки=0,0»).
package projection_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/store/gen"
	"ashyqqala/server/internal/store/projection"
)

func TestListLotsWithGeo_LeftJoin_Integration(t *testing.T) {
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

	lotStore := projection.NewLotStore(pool)
	geoStore := projection.NewGeoLotStore(pool)

	// Два лота: один получит гео-строку (matched), второй — нет (LEFT JOIN NULL).
	const idGeo = "LOT-MAP-GEO"
	const idNoGeo = "LOT-MAP-NOGEO"
	for _, id := range []string{idGeo, idNoGeo} {
		if err := lotStore.UpsertLot(ctx, gen.UpsertLotParams{
			GoszakupLotID: id,
			TitleRu:       pgtype.Text{String: "Тестовый лот " + id, Valid: true},
			Amount:        pgtype.Int8{Int64: 240000000, Valid: true},
		}); err != nil {
			t.Fatalf("UpsertLot %s: %v (применены ли миграции 0001-0004?)", id, err)
		}
	}
	// Гео-строка только для первого (auto + координата Астаны).
	if err := geoStore.UpsertGeoLot(ctx, gen.UpsertGeoLotParams{
		GoszakupLotID: idGeo,
		Lat:           pgtype.Float8{Float64: 51.13, Valid: true},
		Lon:           pgtype.Float8{Float64: 71.43, Valid: true},
		GeocodeStatus: "auto",
		AddressText:   pgtype.Text{String: "Тестовый лот " + idGeo, Valid: true},
	}); err != nil {
		t.Fatalf("UpsertGeoLot %s: %v", idGeo, err)
	}

	rows, err := gen.New(pool).ListLotsWithGeo(ctx)
	if err != nil {
		t.Fatalf("ListLotsWithGeo: %v", err)
	}

	var gotGeo, gotNoGeo *gen.ListLotsWithGeoRow
	for i := range rows {
		switch rows[i].GoszakupLotID {
		case idGeo:
			gotGeo = &rows[i]
		case idNoGeo:
			gotNoGeo = &rows[i]
		}
	}
	if gotGeo == nil || gotNoGeo == nil {
		t.Fatalf("оба лота должны попасть в LEFT JOIN: geo=%v noGeo=%v", gotGeo != nil, gotNoGeo != nil)
	}

	// matched: координата + статус 'auto'
	if !gotGeo.GeocodeStatus.Valid || gotGeo.GeocodeStatus.String != "auto" {
		t.Fatalf("%s geocode_status = %+v, ожидалось auto", idGeo, gotGeo.GeocodeStatus)
	}
	if !gotGeo.Lat.Valid || !gotGeo.Lon.Valid {
		t.Fatalf("%s координаты должны быть непусты, got lat=%+v lon=%+v", idGeo, gotGeo.Lat, gotGeo.Lon)
	}
	if gotGeo.Lon.Float64 != 71.43 || gotGeo.Lat.Float64 != 51.13 {
		t.Fatalf("%s [lon,lat] = [%v,%v], ожидалось [71.43,51.13]", idGeo, gotGeo.Lon.Float64, gotGeo.Lat.Float64)
	}

	// нет гео-строки: LEFT JOIN → NULL geocode_status (→ geocode_pending у потребителя), координаты NULL
	if gotNoGeo.GeocodeStatus.Valid {
		t.Fatalf("%s без гео-строки → geocode_status должен быть NULL, got %+v", idNoGeo, gotNoGeo.GeocodeStatus)
	}
	if gotNoGeo.Lat.Valid || gotNoGeo.Lon.Valid {
		t.Fatalf("%s без гео-строки → координаты NULL, got lat=%+v lon=%+v", idNoGeo, gotNoGeo.Lat, gotNoGeo.Lon)
	}
	t.Logf("OK: LEFT JOIN — %s(auto,[71.43,51.13]) + %s(NULL geo)", idGeo, idNoGeo)
}
