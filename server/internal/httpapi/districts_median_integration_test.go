//go:build integration

// Интеграционный ШОВ медианы ₸/км района (Story 6.4, FR-18) против реальной схемы (миграции 0001-0018).
// Гоняется: `go test -tags=integration -count=1 ./internal/httpapi/...` (нужен DATABASE_URL, порт 55432).
package httpapi

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/store/gen"
)

// TestDistrictMedianSamples_SeamEmpty_Integration — ДОКУМЕНТИРУЕТ шов «честно-пусто» (прецедент 4.3): на
// ТЕКУЩЕЙ схеме geo_objects.length_km НЕТ → ₸/км = amount_tng/length_km невычислима → PricePerKMSamples
// честно отдаёт (nil,false) → ядро даёт not_comparable (медиана НЕ показывается, не выдуманное число).
// Тест проверяет СТРУКТУРНУЮ причину (length_km отсутствует) и поведение адаптера. КРАСНЕЕТ, когда Epic 3
// добавит length_km, — сигнал подключить реальный сбор ₸/км в PricePerKMSamples (шов закрывается БЕЗ слома
// формы DTO/хендлера). Negative-control логики «число при достаточных данных» — в юните (mock samples ≥5).
func TestDistrictMedianSamples_SeamEmpty_Integration(t *testing.T) {
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

	// Структурная причина not_comparable: колонки geo_objects.length_km ещё нет (Epic 3 / Story 3.1).
	var hasLengthKm bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns
		 WHERE table_name = 'geo_objects' AND column_name = 'length_km')`).Scan(&hasLengthKm); err != nil {
		t.Fatalf("проверка наличия length_km: %v", err)
	}
	if hasLengthKm {
		t.Fatal("geo_objects.length_km ПОЯВИЛАСЬ (Epic 3) — подключить реальный сбор ₸/км в PricePerKMSamples и обновить шов 6.4")
	}

	// Адаптер честно не вычисляет ₸/км без length_km → (nil, false) для любой группы.
	store := NewDistrictStore(gen.New(pool))
	samples, computable, err := store.PricePerKMSamples(ctx, "road", "710512%")
	if err != nil {
		t.Fatalf("PricePerKMSamples: %v", err)
	}
	if computable || samples != nil {
		t.Fatalf("без length_km ожидалось (nil, false) → not_comparable; получено (%v, computable=%v)", samples, computable)
	}
}
