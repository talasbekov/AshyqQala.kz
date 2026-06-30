//go:build integration

// Интеграционный тест поиска (Story 6.2) против реальной БД с pg_trgm/GIN (миграция 0017). Проверяет ПОВЕДЕНИЕ
// БД, не покрываемое юнитом с mock: нечёткий substring (language-agnostic, «жол»→«жолдары»), ICU/collation
// case-fold казахских букв (`Қарағанды`↔`қарағанды`), точный матч БИН, скрытие is_deleted. Самодостаточен и
// БЕЗ зависимости от seed: вставляет свои строки в транзакцию, проверяет ЧЛЕНСТВО своих БИН в выдаче (устойчиво
// к прочим строкам), затем откатывает tx. Гоняется: `go test -tags=integration -count=1 ./internal/store/...`.
package store

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/store/gen"
)

func TestSearch_Trgm_Integration(t *testing.T) {
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

	// Свежие организации-«якоря» (уникальный токен Zztest62 в именах — изоляция от seed/прочих строк по членству).
	type orgSeed struct {
		bin, nameRu, nameKk string
		deleted             bool
	}
	orgSeeds := []orgSeed{
		{"990000000801", "Қарағанды Жолдары Zztest62", "Қарағанды жолдары Zztest62", false},
		{"990000000802", "Су Құбыры Zztest62", "Су құбыры Zztest62", false},
		{"990000000803", "Жол Zztest62 Deleted", "Жол Zztest62 Deleted", true}, // удалённая — должна быть скрыта
	}
	binToID := map[string]int64{}
	for _, o := range orgSeeds {
		var id int64
		if err := tx.QueryRow(ctx,
			`INSERT INTO organizations (bin, name_ru, name_kk, is_supplier, is_deleted)
			 VALUES ($1,$2,$3,true,$4) RETURNING id`,
			o.bin, o.nameRu, o.nameKk, o.deleted).Scan(&id); err != nil {
			t.Fatalf("insert org %s: %v", o.bin, err)
		}
		binToID[o.bin] = id
	}

	// Контракт с предметом, содержащим «жолы» и уникальный токен.
	if _, err := tx.Exec(ctx,
		`INSERT INTO contracts (goszakup_contract_id, subject_ru, subject_kk, supplier_org_id, status, kato_code)
		 VALUES ($1,$2,$3,$4,'active','710000000')`,
		"ZZIT62-C1", "Ремонт автомобильной жолы Zztest62", "Автожол жөндеу Zztest62", binToID["990000000801"]); err != nil {
		t.Fatalf("insert contract: %v", err)
	}

	q := gen.New(tx)

	orgBins := func(q2 string, binExact pgtype.Text) map[string]bool {
		rows, err := q.SearchOrganizations(ctx, gen.SearchOrganizationsParams{BinExact: binExact, Q: q2, Lim: 100})
		if err != nil {
			t.Fatalf("SearchOrganizations(%q): %v", q2, err)
		}
		set := map[string]bool{}
		for _, r := range rows {
			set[r.Bin] = true
		}
		return set
	}
	contractIDs := func(q2 string) map[string]bool {
		rows, err := q.SearchContracts(ctx, gen.SearchContractsParams{Q: q2, Lim: 100})
		if err != nil {
			t.Fatalf("SearchContracts(%q): %v", q2, err)
		}
		set := map[string]bool{}
		for _, r := range rows {
			set[r.GoszakupContractID] = true
		}
		return set
	}
	null := pgtype.Text{Valid: false}

	// AC2 — нечёткий substring (language-agnostic): «жол» матчит «...жолдары...». Это ПОЗИТИВНЫЙ страж: если
	// trgm/ILIKE-substring отвалится, ассерт краснеет ([[guards-must-prove-red]]).
	if !orgBins("жол", null)["990000000801"] {
		t.Error("substring «жол» должен матчить «Жолдары» (org 801) — нечёткий substring сломан")
	}
	// AC2 — ICU/collation case-fold казахских букв: запрос в нижнем регистре «қарағанды» матчит «Қарағанды».
	if !orgBins("қарағанды", null)["990000000801"] {
		t.Error("case-fold: «қарағанды» (нижний) должен матчить «Қарағанды» (org 801) — регистр/казахская буква отвалились")
	}
	// И обратно: верхний регистр «ҚАРАҒАНДЫ» матчит сохранённое смешанным регистром.
	if !orgBins("ҚАРАҒАНДЫ", null)["990000000801"] {
		t.Error("case-fold: «ҚАРАҒАНДЫ» (верхний) должен матчить «Қарағанды» (org 801)")
	}

	// AC3 — точный матч БИН: BinExact=801 возвращает 801 и НЕ 802.
	exact := orgBins("", pgtype.Text{String: "990000000801", Valid: true})
	if !exact["990000000801"] || exact["990000000802"] {
		t.Errorf("точный БИН 801 должен вернуть только 801, получено %v", exact)
	}
	// Несуществующий БИН → пусто (не наши строки).
	if none := orgBins("", pgtype.Text{String: "990000000999", Valid: true}); none["990000000801"] || none["990000000802"] {
		t.Error("несуществующий БИН не должен возвращать наши организации")
	}

	// is_deleted скрыт: «deleted» матчит имя org 803, но она удалена → НЕ в выдаче.
	if orgBins("deleted", null)["990000000803"] {
		t.Error("удалённая организация (is_deleted) не должна попадать в выдачу")
	}

	// Negative control (страж краснеет, если поиск перестал фильтровать): не-матч → наших строк нет.
	if nm := orgBins("zzqxnomatch62", null); nm["990000000801"] || nm["990000000802"] {
		t.Error("не-матч должен исключать наши организации — поиск перестал фильтровать")
	}

	// P2 (code review 6.2): LIKE-метасимволы экранируются — «%%%» трактуется буквально и НЕ матчит всё.
	// Наши орг (доказанно в таблице выше) НЕ содержат литерал «%%%» → не должны вернуться. Без экранирования
	// ILIKE превратил бы «%%%» в match-all и включил бы их — страж краснеет при регрессе экранирования.
	if mm := orgBins("%%%", null); mm["990000000801"] || mm["990000000802"] {
		t.Error("LIKE-метасимволы должны экранироваться: «%%%» не должен матчить всё (pattern-injection)")
	}

	// Контракты: substring по предмету «жолы» матчит ZZIT62-C1; не-матч исключает.
	if !contractIDs("жолы")["ZZIT62-C1"] {
		t.Error("substring «жолы» должен матчить предмет контракта ZZIT62-C1")
	}
	if contractIDs("zzqxnomatch62")["ZZIT62-C1"] {
		t.Error("не-матч не должен возвращать контракт ZZIT62-C1")
	}
}
