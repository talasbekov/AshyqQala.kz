//go:build integration

// Интеграционный тест слоя данных: реальный GetContractByID против поднятой PostGIS + seed.
// Гоняется отдельно: `go test -tags=integration ./internal/store/...` с заданным DATABASE_URL.
// CI (`go test ./...` без тега) этот файл не компилирует → не требует БД.
package store

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/store/gen"
)

func TestGetContractByID_Integration(t *testing.T) {
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

	q := gen.New(pool)

	c, err := q.GetContractByID(ctx, "DEMO-0001")
	if err != nil {
		t.Fatalf("GetContractByID(DEMO-0001): %v (применены ли миграции и seed?)", err)
	}

	if c.GoszakupContractID != "DEMO-0001" {
		t.Fatalf("goszakup_contract_id = %q, ожидалось DEMO-0001", c.GoszakupContractID)
	}
	if !c.SubjectRu.Valid || c.SubjectRu.String == "" {
		t.Fatalf("subject_ru пуст/NULL: %+v", c.SubjectRu)
	}
	if !c.Direction.Valid || c.Direction.String != "road" {
		t.Fatalf("direction = %+v, ожидалось road", c.Direction)
	}
	// BIGINT amount_tng → pgtype.Int8; проверяем явно (целые тенге).
	if !c.AmountTng.Valid || c.AmountTng.Int64 != 123456789 {
		t.Fatalf("amount_tng = %+v, ожидалось 123456789", c.AmountTng)
	}
	t.Logf("OK: id=%d goszakup_id=%s subject_ru=%q direction=%s amount_tng=%d",
		c.ID, c.GoszakupContractID, c.SubjectRu.String, c.Direction.String, c.AmountTng.Int64)
}
