//go:build scrape && integration

// Идемпотентность интерим-импорта: scrape-фикстура → decode → UpsertLot ДВАЖДЫ → одна строка
// (синтетический natural-ключ стабилен). Гоняется отдельно:
//
//	DATABASE_URL=postgres://... go test -tags 'scrape integration' ./tools/scrape/...
//
// с применёнными миграциями 0001-0003. CI без тегов этот файл не компилирует → БД не требуется.
package scrape

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/ingest/decode"
	"ashyqqala/server/internal/store/gen"
	"ashyqqala/server/internal/store/projection"
)

func TestScrapeIdempotency_Integration(t *testing.T) {
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

	rows := New("", "", 0, 0).parseRows(loadFixture(t), AstanaKATO, map[string]bool{})
	hash := decode.SchemaHash(RecordKeys)
	var lots []decode.Lot
	for _, raw := range rows {
		l, err := decode.DecodeLot(raw, hash)
		if err != nil {
			t.Fatalf("decode scrape-записи: %v", err)
		}
		lots = append(lots, l)
	}
	if len(lots) != 2 {
		t.Fatalf("ожидалось 2 лота из фикстуры, получено %d", len(lots))
	}

	store := projection.NewLotStore(pool)
	for range 2 { // двойной прогон — идемпотентность по goszakup_lot_id (повтор не плодит дубли)
		for _, l := range lots {
			if err := store.UpsertLot(ctx, toScrapeParams(l)); err != nil {
				t.Fatalf("UpsertLot %s: %v", l.GoszakupLotID, err)
			}
		}
	}

	got, err := store.GetLotByID(ctx, lots[0].GoszakupLotID)
	if err != nil {
		t.Fatalf("GetLotByID(%s): %v (применены ли миграции 0001-0003?)", lots[0].GoszakupLotID, err)
	}
	if got.GoszakupLotID != lots[0].GoszakupLotID {
		t.Fatalf("goszakup_lot_id = %q, ожидалось %q", got.GoszakupLotID, lots[0].GoszakupLotID)
	}
	if !got.Amount.Valid || got.Amount.Int64 != 240000000 {
		t.Fatalf("amount = %+v, ожидалось 240000000", got.Amount)
	}
	t.Logf("OK идемпотентность: lot id=%d goszakup=%s amount=%d", got.ID, got.GoszakupLotID, got.Amount.Int64)
}

// toScrapeParams — decode.Lot → sqlc UpsertLotParams (пустые поля → NULL: честное «нет данных»).
func toScrapeParams(l decode.Lot) gen.UpsertLotParams {
	p := gen.UpsertLotParams{GoszakupLotID: l.GoszakupLotID}
	if l.TitleRu != "" {
		p.TitleRu = pgtype.Text{String: l.TitleRu, Valid: true}
	}
	if l.Amount != nil {
		p.Amount = pgtype.Int8{Int64: *l.Amount, Valid: true}
	}
	if l.KatoCode != "" {
		p.KatoCode = pgtype.Text{String: l.KatoCode, Valid: true}
	}
	return p
}
