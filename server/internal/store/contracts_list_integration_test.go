//go:build integration

// Интеграционный тест фасетной фильтрации ListContracts (Story 6.1) против реальной PostGIS.
// Самодостаточен и БЕЗ зависимости от seed: вставляет свои данные в транзакции, привязывает их к СВЕЖЕЙ
// организации и фильтрует по supplier_org_id (изоляция от любых других строк), затем откатывает tx —
// БД остаётся чистой. Гоняется: `go test -tags=integration ./internal/store/...` с DATABASE_URL.
package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/store/gen"
)

func TestListContracts_Facets_Integration(t *testing.T) {
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

	mkDate := func(s string) pgtype.Date {
		if s == "" {
			return pgtype.Date{} // NULL sign_date → хвост NULLS LAST
		}
		tm, _ := time.Parse("2006-01-02", s)
		return pgtype.Date{Time: tm, Valid: true}
	}

	// свежая организация-«якорь»: все тестовые контракты на неё, фасет supplier_org_id изолирует от прочих строк.
	var orgID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO organizations (bin, name_ru, is_supplier) VALUES ($1,$2,true) RETURNING id`,
		"990000000001", "Тест-подрядчик 6.1").Scan(&orgID); err != nil {
		t.Fatalf("insert org: %v", err)
	}

	type seed struct {
		gid, dir, sd string
		amount       int64
		flag         bool
	}
	seeds := []seed{
		{"IT61-A", "road", "2099-01-05", 100, true},
		{"IT61-B", "water", "2099-01-04", 500, false},
		{"IT61-C", "road", "2099-01-03", 1000, false},
		{"IT61-D", "other", "2099-01-02", 50, false},
		{"IT61-E", "road", "", 2000, true},  // NULL sign_date
		{"IT61-F", "road", "", 3000, false}, // NULL sign_date (хвост из 2 строк — проверяет курсор ВНУТРИ NULLS LAST)
	}
	for _, s := range seeds {
		var cid int64
		if err := tx.QueryRow(ctx,
			`INSERT INTO contracts (goszakup_contract_id, direction, sign_date, amount_tng, supplier_org_id, status, kato_code)
			 VALUES ($1,$2,$3,$4,$5,'active','710000000') RETURNING id`,
			s.gid, s.dir, mkDate(s.sd), s.amount, orgID).Scan(&cid); err != nil {
			t.Fatalf("insert contract %s: %v", s.gid, err)
		}
		if s.flag {
			if _, err := tx.Exec(ctx,
				`INSERT INTO risk_flags (flag_type, subject_type, contract_id, severity, evidence, is_active, methodology_version)
				 VALUES ('single_participant','contract',$1,'medium','{}'::jsonb,true,'v1')`, cid); err != nil {
				t.Fatalf("insert flag %s: %v", s.gid, err)
			}
		}
	}

	q := gen.New(tx)
	anchor := pgtype.Int8{Int64: orgID, Valid: true}
	gids := func(rs []gen.ListContractsRow) []string {
		out := make([]string, len(rs))
		for i, r := range rs {
			out[i] = r.GoszakupContractID
		}
		return out
	}
	list := func(p gen.ListContractsParams) []gen.ListContractsRow {
		p.SupplierOrgID = anchor // изоляция: только наши строки
		if p.Lim == 0 {
			p.Lim = 100
		}
		rs, err := q.ListContracts(ctx, p)
		if err != nil {
			t.Fatalf("ListContracts: %v", err)
		}
		return rs
	}
	eq := func(name string, got, want []string) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("%s: got %v, want %v", name, got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("%s: got %v, want %v", name, got, want)
			}
		}
	}

	// 1) Без фасетов (только якорь): порядок sign_date DESC NULLS LAST, gid → A,B,C,D, затем NULL-хвост E,F.
	eq("order", gids(list(gen.ListContractsParams{})), []string{"IT61-A", "IT61-B", "IT61-C", "IT61-D", "IT61-E", "IT61-F"})

	// 2) direction road → A,C,E,F (negative control: 4 < 6 — фасет реально сужает, не возвращает всё).
	eq("dir road", gids(list(gen.ListContractsParams{Directions: []string{"road"}})), []string{"IT61-A", "IT61-C", "IT61-E", "IT61-F"})
	eq("dir road,water", gids(list(gen.ListContractsParams{Directions: []string{"road", "water"}})), []string{"IT61-A", "IT61-B", "IT61-C", "IT61-E", "IT61-F"})

	// 3) Сумма: min=500 → B,C,E,F; max=500 → A,B,D.
	eq("amount_min", gids(list(gen.ListContractsParams{AmountMin: pgtype.Int8{Int64: 500, Valid: true}})), []string{"IT61-B", "IT61-C", "IT61-E", "IT61-F"})
	eq("amount_max", gids(list(gen.ListContractsParams{AmountMax: pgtype.Int8{Int64: 500, Valid: true}})), []string{"IT61-A", "IT61-B", "IT61-D"})

	// 4) Период (фасет по sign_date; NULL-строки исключаются неравенством — AC4): from=01-04 → A,B; to=01-03 → C,D.
	eq("signed_from", gids(list(gen.ListContractsParams{SignedFrom: mkDate("2099-01-04")})), []string{"IT61-A", "IT61-B"})
	eq("signed_to", gids(list(gen.ListContractsParams{SignedTo: mkDate("2099-01-03")})), []string{"IT61-C", "IT61-D"})

	// 5) has_flag → A,E (negative control: 2 < 6).
	eq("has_flag", gids(list(gen.ListContractsParams{HasFlagOnly: true})), []string{"IT61-A", "IT61-E"})

	// 6) Keyset-пагинация страницами по 2: реконструирует ВЕСЬ набор по порядку, без дублей/пропусков,
	//    включая переход в NULL-хвост и курсор ВНУТРИ него (страница E,F → курсор с NULL sign_date).
	var page []string
	var curGid pgtype.Text
	var curSd pgtype.Date
	for {
		rs := list(gen.ListContractsParams{Lim: 2, CursorGid: curGid, CursorSd: curSd})
		if len(rs) == 0 {
			break
		}
		page = append(page, gids(rs)...)
		last := rs[len(rs)-1]
		curGid = pgtype.Text{String: last.GoszakupContractID, Valid: true}
		curSd = last.SignDate
		if len(rs) < 2 {
			break
		}
	}
	eq("keyset pages", page, []string{"IT61-A", "IT61-B", "IT61-C", "IT61-D", "IT61-E", "IT61-F"})

	// 7) Фасет supplier_org_id изолировал ровно наши 6 строк (не задел seed/прочие данные).
	if all := list(gen.ListContractsParams{}); len(all) != 6 {
		t.Fatalf("supplier_org_id-якорь вернул %d строк, ожидалось 6", len(all))
	}
}
