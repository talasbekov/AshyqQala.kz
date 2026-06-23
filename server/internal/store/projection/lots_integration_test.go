//go:build integration

// Интеграционный тест проекции lots: file-Source → decode → projection (UPSERT идемпотентно) → GetLotByID.
// Гоняется отдельно: `go test -tags=integration ./internal/store/...` с DATABASE_URL (миграции 0001-0003 + лоты).
// CI (`go test ./...` без тега) этот файл не компилирует → не требует БД.
package projection_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/goszakup"
	"ashyqqala/server/internal/ingest/decode"
	"ashyqqala/server/internal/store/gen"
	"ashyqqala/server/internal/store/projection"
)

// lotSchemaHash — набор полей фикстуры testdata/lots.json (= goldenLotSchemaHash из decode/lots_test.go).
const lotSchemaHash = "0dcd872b0001371952b0046ae6fbff89954c45ee4d72644519a5eb53767a4b20"

func TestLotsWiring_Integration(t *testing.T) {
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

	// file → decode
	lots, err := decode.DecodeLotsFrom(goszakup.NewFileSource("testdata"), lotSchemaHash, 0)
	if err != nil {
		t.Fatalf("file→decode lots: %v", err)
	}
	if len(lots) != 2 {
		t.Fatalf("ожидалось 2 лота из фикстуры, получено %d", len(lots))
	}

	store := projection.NewLotStore(pool)
	// UPSERT дважды — идемпотентность (повтор по goszakup_lot_id не плодит дубли).
	for range 2 {
		for _, l := range lots {
			if err := store.UpsertLot(ctx, toParams(l)); err != nil {
				t.Fatalf("UpsertLot %s: %v", l.GoszakupLotID, err)
			}
		}
	}

	got, err := store.GetLotByID(ctx, "LOT-0001")
	if err != nil {
		t.Fatalf("GetLotByID(LOT-0001): %v (применены ли миграции 0001-0003?)", err)
	}
	if got.GoszakupLotID != "LOT-0001" {
		t.Fatalf("goszakup_lot_id = %q, ожидалось LOT-0001", got.GoszakupLotID)
	}
	if !got.Amount.Valid || got.Amount.Int64 != 240000000 {
		t.Fatalf("amount = %+v, ожидалось 240000000", got.Amount)
	}
	t.Logf("OK: lot id=%d goszakup=%s amount=%d", got.ID, got.GoszakupLotID, got.Amount.Int64)
}

// toParams — маппинг доменного decode.Lot → sqlc-параметры (NULL для пустых: честное «нет данных»).
func toParams(l decode.Lot) gen.UpsertLotParams {
	p := gen.UpsertLotParams{GoszakupLotID: l.GoszakupLotID}
	if l.TitleRu != "" {
		p.TitleRu = pgtype.Text{String: l.TitleRu, Valid: true}
	}
	if l.TitleKk != "" {
		p.TitleKk = pgtype.Text{String: l.TitleKk, Valid: true}
	}
	if l.Amount != nil {
		p.Amount = pgtype.Int8{Int64: *l.Amount, Valid: true}
	}
	if l.KatoCode != "" {
		p.KatoCode = pgtype.Text{String: l.KatoCode, Valid: true}
	}
	return p
}
