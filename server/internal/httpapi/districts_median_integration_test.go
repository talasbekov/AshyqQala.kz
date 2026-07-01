//go:build integration

// Интеграционный ШОВ медианы ₸/км района (Story 6.4 → НАПОЛНЕН Story 3.1, FR-18) против реальной схемы
// (миграции 0001-0020). Гоняется: `go test -tags=integration -count=1 ./internal/httpapi/...` (нужен
// DATABASE_URL, порт 55432). Story 3.1 ввела geo_objects.length_km → ₸/км = amount_tng/length_km ВЫЧИСЛИМА.
package httpapi

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/store/gen"
)

// TestDistrictMedianSamples_LengthKmLit_Integration — ШОВ ₸/км ЗАЖЁГСЯ (Story 3.1): geo_objects.length_km
// теперь СУЩЕСТВУЕТ, и PricePerKMSamples собирает реальную выборку цены/км (computable=true), а не честно-пустое
// (nil,false) как до Epic 3 (прецедент 4.3). Проверяем НА ИЗОЛИРОВАННОМ КАТО-префиксе (tx+rollback, не пересекается
// с seed): 6 дорожных контрактов + geo_objects с length_km → 6 выборок, цена/км = amount/length_km целочисленно.
// Медианная арифметика (порог MinSample, окно) покрыта юнит-тестами benchmark — здесь проверяем именно ШОВ сбора.
func TestDistrictMedianSamples_LengthKmLit_Integration(t *testing.T) {
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

	// Story 3.1 ЗАКРЫЛА структурную причину not_comparable: колонка length_km обязана существовать.
	var hasLengthKm bool
	if err := pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns
		 WHERE table_name = 'geo_objects' AND column_name = 'length_km')`).Scan(&hasLengthKm); err != nil {
		t.Fatalf("проверка наличия length_km: %v", err)
	}
	if !hasLengthKm {
		t.Fatal("geo_objects.length_km ОТСУТСТВУЕТ — миграция 0020 (Story 3.1) не применена?")
	}

	// tx+rollback: свои дорожные контракты+geo на ИЗОЛИРОВАННОМ префиксе '719999999' (не 710000000 seed) →
	// PricePerKMSamples('road','719999999%') видит ТОЛЬКО их. Откат в конце (не мутируем БД).
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // тест: откат обязателен, ошибка отката не важна

	if _, err := tx.Exec(ctx,
		`INSERT INTO districts (kato_code, name_ru, name_kk, geom)
		 VALUES ('719999999','ТестРайон','ТестРайон',
		         ST_GeomFromText('POLYGON((71.3 51.05,71.55 51.05,71.55 51.2,71.3 51.2,71.3 51.05))',4326))`); err != nil {
		t.Fatalf("insert district: %v", err)
	}
	// 6 дорожных контрактов + geo_objects (length_km). Цена/км = amount/len: 48,40,30,60,25,44 (млн).
	amounts := []int64{240_000_000, 200_000_000, 180_000_000, 300_000_000, 150_000_000, 220_000_000}
	lens := []float64{5.0, 5.0, 6.0, 5.0, 6.0, 5.0}
	for i := range amounts {
		var cid int64
		if err := tx.QueryRow(ctx,
			`INSERT INTO contracts (goszakup_contract_id, amount_tng, sign_date, direction, kato_code)
			 VALUES ($1,$2,'2026-05-01','road','719999999') RETURNING id`,
			"TEST-GEO-"+string(rune('A'+i)), amounts[i]).Scan(&cid); err != nil {
			t.Fatalf("insert contract %d: %v", i, err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO geo_objects (contract_id, geom, length_km, geocode_status, geocoded_by)
			 VALUES ($1, ST_GeomFromText('LINESTRING(71.40 51.10,71.45 51.14)',4326), $2, 'auto','test')`,
			cid, lens[i]); err != nil {
			t.Fatalf("insert geo_object %d: %v", i, err)
		}
	}

	store := NewDistrictStore(gen.New(tx))
	samples, computable, err := store.PricePerKMSamples(ctx, "road", "719999999%")
	if err != nil {
		t.Fatalf("PricePerKMSamples: %v", err)
	}
	if !computable {
		t.Fatal("length_km есть → ожидалось computable=true (шов зажжён), получено false")
	}
	if len(samples) != 6 {
		t.Fatalf("ожидалось 6 выборок ₸/км, получено %d", len(samples))
	}
	// Проверяем детерминированную цену/км хотя бы одной точки (240M/5 = 48M).
	got := map[int64]bool{}
	for _, s := range samples {
		got[s.PricePerKM] = true
	}
	for _, want := range []int64{48_000_000, 40_000_000, 30_000_000, 60_000_000, 25_000_000, 44_000_000} {
		if !got[want] {
			t.Fatalf("ожидалась цена/км %d в выборке; получено %v", want, got)
		}
	}
}
