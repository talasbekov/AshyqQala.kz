//go:build integration

// Страж AC2 Story 3.2 против реальной схемы (миграции 0001-0022): unmatched-объект («без точки на
// карте») ОСТАЁТСЯ доступен в поиске, СПИСКЕ и карточке (все три поверхности AC2, ревью 3.2). Сегодня
// это архитектурно гарантировано (поиск 6.2, список 6.1 и карточка 5.1 работают по contracts, join'а
// на geo нет) — тест закрепляет инвариант против будущего регресса (например, JOIN geo_objects с
// фильтром по геометрии в любой из выдач уронит его).
// Самодостаточен: tx+rollback, якорь «ZZGEO32-VIS» (БД может быть засеяна — SEED-группа). Гоняется:
// `go test -tags=integration -count=1 ./internal/httpapi/...` (DATABASE_URL, локально порт 55432).
package httpapi

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/store/gen"
)

func TestUnmatchedContract_VisibleInSearchAndCard_Integration(t *testing.T) {
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
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // тест: откат обязателен

	// Контракт с честно-unmatched гео-строкой (geom NULL — «без точки на карте», CHECK 0020/0022 доволен).
	const gid = "ZZGEO32-VIS-1"
	const subject = "ZZGEO32-невидимка: ремонт дороги без распознанного адреса"
	var cid int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO contracts (goszakup_contract_id, subject_ru, amount_tng, sign_date, direction, kato_code)
		 VALUES ($1, $2, 90000000, '2026-05-07', 'road', '710000000') RETURNING id`,
		gid, subject).Scan(&cid); err != nil {
		t.Fatalf("insert contract: %v", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO geo_objects (contract_id, geocode_status, address_text, geocoded_by)
		 VALUES ($1, 'unmatched', 'мусорный адрес', 'test')`, cid); err != nil {
		t.Fatalf("insert unmatched geo_object: %v", err)
	}

	q := gen.New(tx)

	// Поиск (FR-16, тот же SQL, что у хендлера /api/search): unmatched-контракт НАХОДИТСЯ по предмету.
	found, err := q.SearchContracts(ctx, gen.SearchContractsParams{Q: "ZZGEO32-невидимка", Lim: 10})
	if err != nil {
		t.Fatalf("SearchContracts: %v", err)
	}
	hit := false
	for _, r := range found {
		if r.GoszakupContractID == gid {
			hit = true
		}
	}
	if !hit {
		t.Fatalf("unmatched-контракт %s НЕ найден поиском — «без точки на карте» исключил его из выдачи (нарушение AC2/FR-5)", gid)
	}

	// Карточка (FR-10, тот же SQL, что у хендлера /api/contracts/{id}): открывается.
	card, err := q.GetContractByID(ctx, gid)
	if err != nil {
		t.Fatalf("GetContractByID(%s): %v — карточка unmatched-контракта обязана открываться (AC2)", gid, err)
	}
	if card.GoszakupContractID != gid {
		t.Fatalf("карточка вернула %q, ожидался %q", card.GoszakupContractID, gid)
	}

	// Список (FR-15, тот же SQL, что у хендлера /api/contracts — третья поверхность AC2, ревью 3.2):
	// unmatched-контракт присутствует в листинге. Дата-фасет сужает выдачу до точной sign_date фикстуры —
	// страж не зависит от объёма seed-данных.
	day := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)
	listed, err := q.ListContracts(ctx, gen.ListContractsParams{
		SignedFrom: pgtype.Date{Time: day, Valid: true},
		SignedTo:   pgtype.Date{Time: day, Valid: true},
		Lim:        100,
	})
	if err != nil {
		t.Fatalf("ListContracts: %v", err)
	}
	inList := false
	for _, r := range listed {
		if r.GoszakupContractID == gid {
			inList = true
		}
	}
	if !inList {
		t.Fatalf("unmatched-контракт %s НЕ виден в списке — «без точки на карте» исключил его из листинга (нарушение AC2/FR-5)", gid)
	}
}
