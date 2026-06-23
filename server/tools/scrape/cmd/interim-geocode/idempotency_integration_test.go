//go:build scrape && integration

// Идемпотентность интерим-геокодинга: сеем лот → ListLots видит его → geocodeLots(stub) ДВАЖДЫ →
// одна гео-строка (UPSERT по goszakup_lot_id). Гоняется отдельно:
//
//	DATABASE_URL=postgres://... go test -tags 'scrape integration' ./tools/scrape/cmd/interim-geocode/...
//
// с применёнными миграциями 0001-0004. CI без тегов этот файл не компилирует → БД не требуется.
package main

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/store/gen"
	"ashyqqala/server/internal/store/projection"
)

func TestGeocodeIdempotency_Integration(t *testing.T) {
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
	if err := lotStore.UpsertLot(ctx, gen.UpsertLotParams{
		GoszakupLotID: "geo-it-1",
		TitleRu:       pgtype.Text{String: "проспект Абая", Valid: true},
		KatoCode:      pgtype.Text{String: "710000000", Valid: true},
	}); err != nil {
		t.Fatalf("seed lot: %v", err)
	}

	// ListLots видит засеянный лот (покрытие нового запроса).
	all, err := lotStore.ListLots(ctx)
	if err != nil {
		t.Fatalf("ListLots: %v", err)
	}
	found := false
	for _, l := range all {
		if l.GoszakupLotID == "geo-it-1" {
			found = true
		}
	}
	if !found {
		t.Fatal("ListLots не вернул засеянный лот geo-it-1")
	}

	// Геокодим ТОЛЬКО засеянный лот (изоляция теста), стаб всё матчит детерминированно. Двойной прогон.
	lots := []gen.Lot{{GoszakupLotID: "geo-it-1", TitleRu: pgtype.Text{String: "проспект Абая", Valid: true}, KatoCode: pgtype.Text{String: "710000000", Valid: true}}}
	gc := fakeGeocoder{fn: func(string) (float64, float64, bool, error) { return 51.16, 71.47, true, nil }}
	geoStore := projection.NewGeoLotStore(pool)
	for range 2 {
		if _, err := geocodeLots(ctx, lots, gc, geoStore, 0, io.Discard); err != nil {
			t.Fatalf("geocodeLots: %v", err)
		}
	}

	got, err := geoStore.GetGeoLotByLotID(ctx, "geo-it-1")
	if err != nil {
		t.Fatalf("GetGeoLotByLotID: %v (применены ли миграции 0001-0004?)", err)
	}
	if got.GeocodeStatus != "auto" || !got.Lat.Valid || got.Lat.Float64 != 51.16 {
		t.Fatalf("гео = %+v, ожидалось auto + lat 51.16", got)
	}
	t.Logf("OK идемпотентность: гео id=%d lot=%s lat=%v", got.ID, got.GoszakupLotID, got.Lat.Float64)
}
