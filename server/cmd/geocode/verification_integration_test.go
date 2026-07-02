//go:build integration

// Интеграционные стражи Story 3.2 (верификация гео, миграция 0022) против реальной БД: расширенный
// закрытый enum geocode_status (+verified/wrong_reported), honesty-CHECK на новых статусах, гейт
// UpsertGeoObject «batch перезаписывает ТОЛЬКО auto|unmatched», триггер length_km для прямой записи
// (Directus-путь), допустимые/запрещённые переходы кураторских методов и адаптированная приёмка AR-10
// «прогон → курация → прогон» (курация переживает ре-геокод). Самодостаточен: транзакция + rollback,
// якорь-изоляция «ZZGEO32-» (БД может быть засеяна — dev-volume персистентен; глобальных счётчиков нет).
// Переиспользует харнесс idempotency_integration_test.go (mustConn/insertContract/curationStoreOn) и
// фейки main_test.go (fakeGeocoder — main_test.go без build-тега, компилируется и под integration).
package main

import (
	"context"
	"io"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"ashyqqala/server/internal/geo"
	"ashyqqala/server/internal/store/gen"
)

// insertGeoRaw — ПРЯМАЯ вставка строки geo_objects (мимо UpsertGeoObject) — путь «сырого писателя»
// (Directus/seed): произвольный статус/явный length_km. lengthKm=nil ⇒ SQL NULL (триггер 0022 решает сам).
func insertGeoRaw(t *testing.T, tx pgx.Tx, contractID int64, status, geomWKT string, lengthKm *float64) int64 {
	t.Helper()
	var geomArg, lenArg any
	if geomWKT != "" {
		geomArg = geomWKT
	}
	if lengthKm != nil {
		lenArg = *lengthKm
	}
	var id int64
	err := tx.QueryRow(context.Background(),
		`INSERT INTO geo_objects (contract_id, geom, length_km, geocode_status, geocoded_by)
		 VALUES ($1, ST_GeomFromText($2, 4326), $3, $4, 'test-fixture') RETURNING id`,
		contractID, geomArg, lenArg, status).Scan(&id)
	if err != nil {
		t.Fatalf("insertGeoRaw (contract=%d, status=%s): %v", contractID, status, err)
	}
	return id
}

// geoRowState — статус/провенанс/геометрия/длина строки по contract_id (raw SQL: ST_AsText для точного
// сравнения геометрии, вне GeoJSON-сериализации).
func geoRowState(t *testing.T, tx pgx.Tx, contractID int64) (status, by, geomText string, lengthKm pgtype.Float8) {
	t.Helper()
	var byN, geomN pgtype.Text
	err := tx.QueryRow(context.Background(),
		`SELECT geocode_status, geocoded_by, COALESCE(ST_AsText(geom), ''), length_km
		 FROM geo_objects WHERE contract_id = $1`, contractID).Scan(&status, &byN, &geomN, &lengthKm)
	if err != nil {
		t.Fatalf("geoRowState contract=%d: %v", contractID, err)
	}
	return status, byN.String, geomN.String, lengthKm
}

// --- Стражи CHECK-констрейнтов (0022). Каждое нарушение — в СВОЁМ тесте: CHECK-нарушение абортит всю
// транзакцию (SQLSTATE 25P02), общий tx для нескольких REJECT'ов невозможен (зеркало стражей 0020/3.1). ---

// Negative-control (memory guards-must-prove-red): CHECK вообще жив — мусорный статус отклоняется.
func TestGeoStatusCheck_RejectsUnknownStatus(t *testing.T) {
	_, tx := mustConn(t)
	cid := insertContract(t, tx, "ZZGEO32-CHK-BOGUS", "710000000")
	_, err := tx.Exec(context.Background(),
		`INSERT INTO geo_objects (contract_id, geom, geocode_status)
		 VALUES ($1, ST_GeomFromText('POINT(30 10)', 4326), 'bogus')`, cid)
	if err == nil {
		t.Error("статус 'bogus' должен быть REJECTED (geo_objects_status_chk — закрытый enum)")
	}
}

// verified ОБЯЗАН нести геометрию: honesty-CHECK (geom IS NULL)=(status='unmatched') покрывает новый статус.
func TestGeoStatusCheck_VerifiedWithoutGeom_Rejected(t *testing.T) {
	_, tx := mustConn(t)
	cid := insertContract(t, tx, "ZZGEO32-CHK-VNG", "710000000")
	_, err := tx.Exec(context.Background(),
		`INSERT INTO geo_objects (contract_id, geocode_status) VALUES ($1, 'verified')`, cid)
	if err == nil {
		t.Error("verified-без-геометрии должен быть REJECTED (geo_objects_geom_null_chk)")
	}
}

// wrong_reported тоже несёт (спорную) геометрию — «точка не там» ≠ «точки нет».
func TestGeoStatusCheck_WrongReportedWithoutGeom_Rejected(t *testing.T) {
	_, tx := mustConn(t)
	cid := insertContract(t, tx, "ZZGEO32-CHK-WNG", "710000000")
	_, err := tx.Exec(context.Background(),
		`INSERT INTO geo_objects (contract_id, geocode_status) VALUES ($1, 'wrong_reported')`, cid)
	if err == nil {
		t.Error("wrong_reported-без-геометрии должен быть REJECTED (geo_objects_geom_null_chk)")
	}
}

// Контракт типа геометрии (AC1, ревью 3.2): map-интерфейс Directus не ограничивает тип фигуры —
// MULTILINESTRING/POLYGON отвергает CHECK geo_objects_geom_type_chk (иначе «дорога» молча без длины).
func TestGeoTypeCheck_MultiLineString_Rejected(t *testing.T) {
	_, tx := mustConn(t)
	cid := insertContract(t, tx, "ZZGEO32-CHK-MLS", "710000000")
	_, err := tx.Exec(context.Background(),
		`INSERT INTO geo_objects (contract_id, geom, geocode_status)
		 VALUES ($1, ST_GeomFromText('MULTILINESTRING((30 10, 30.05 10.04),(30.06 10.05, 30.1 10.08))', 4326), 'manual')`, cid)
	if err == nil {
		t.Error("MULTILINESTRING должен быть REJECTED (geo_objects_geom_type_chk: только POINT|LINESTRING — AC1)")
	}
}

// --- Переходы кураторских методов (AC1/AC3): допустимые меняют ровно 1 строку, запрещённые — 0. ---

func TestMarkVerified_OnlyFromAuto(t *testing.T) {
	_, tx := mustConn(t)
	ctx := context.Background()
	s := curationStoreOn(tx)

	autoC := insertContract(t, tx, "ZZGEO32-VER-A", "710000000")
	autoID := insertGeoRaw(t, tx, autoC, "auto", "POINT(30 10)", nil)
	manualC := insertContract(t, tx, "ZZGEO32-VER-M", "710000000")
	manualID := insertGeoRaw(t, tx, manualC, "manual", "POINT(30 11)", nil)

	if n, err := s.MarkGeoObjectVerified(ctx, autoID); err != nil || n != 1 {
		t.Fatalf("auto→verified: n=%d err=%v, ожидалось 1/nil", n, err)
	}
	if st, _, _, _ := geoRowState(t, tx, autoC); st != "verified" {
		t.Errorf("статус = %q, ожидалось verified", st)
	}
	// Повторная верификация уже-verified — 0 строк (не auto), НЕ тихий успех.
	if n, err := s.MarkGeoObjectVerified(ctx, autoID); err != nil || n != 0 {
		t.Errorf("verified→verified: n=%d err=%v, ожидалось 0/nil", n, err)
	}
	// manual подтверждать нечем (он и так человеческий) — запрещённый переход.
	if n, err := s.MarkGeoObjectVerified(ctx, manualID); err != nil || n != 0 {
		t.Errorf("manual→verified: n=%d err=%v, ожидалось 0/nil (запрещённый переход)", n, err)
	}
}

func TestWrongReported_KeepsDisputedGeometry(t *testing.T) {
	_, tx := mustConn(t)
	ctx := context.Background()
	s := curationStoreOn(tx)

	c := insertContract(t, tx, "ZZGEO32-WR-A", "710000000")
	id := insertGeoRaw(t, tx, c, "auto", "POINT(30 10)", nil)

	if n, err := s.MarkGeoObjectWrongReported(ctx, id); err != nil || n != 1 {
		t.Fatalf("auto→wrong_reported: n=%d err=%v, ожидалось 1/nil", n, err)
	}
	st, by, geom, _ := geoRowState(t, tx, c)
	if st != "wrong_reported" {
		t.Errorf("статус = %q, ожидалось wrong_reported", st)
	}
	if geom != "POINT(30 10)" {
		t.Errorf("geom = %q, ожидалось POINT(30 10) — спорная геометрия СОХРАНЯЕТСЯ (факт, не подмена)", geom)
	}
	if by != "test-fixture" {
		t.Errorf("geocoded_by = %q, ожидалось test-fixture (провенанс точки не переписывается пометкой)", by)
	}

	// unmatched помечать «не там» нечем — точки нет.
	uc := insertContract(t, tx, "ZZGEO32-WR-U", "710000000")
	uid := insertGeoRaw(t, tx, uc, "unmatched", "", nil)
	if n, err := s.MarkGeoObjectWrongReported(ctx, uid); err != nil || n != 0 {
		t.Errorf("unmatched→wrong_reported: n=%d err=%v, ожидалось 0/nil", n, err)
	}
}

func TestClearToUnmatched_OnlyFromWrongReported(t *testing.T) {
	_, tx := mustConn(t)
	ctx := context.Background()
	s := curationStoreOn(tx)

	c := insertContract(t, tx, "ZZGEO32-CLR-W", "710000000")
	id := insertGeoRaw(t, tx, c, "auto", "POINT(30 10)", nil)
	if n, err := s.MarkGeoObjectWrongReported(ctx, id); err != nil || n != 1 {
		t.Fatalf("подготовка wrong_reported: n=%d err=%v", n, err)
	}
	if n, err := s.ClearGeoObjectToUnmatched(ctx, id, "directus"); err != nil || n != 1 {
		t.Fatalf("wrong_reported→unmatched: n=%d err=%v, ожидалось 1/nil", n, err)
	}
	st, by, geom, lkm := geoRowState(t, tx, c)
	if st != "unmatched" || geom != "" {
		t.Errorf("статус/geom = %q/%q, ожидалось unmatched/пусто (честное «без точки на карте», AC2)", st, geom)
	}
	if lkm.Valid {
		t.Errorf("length_km = %+v, ожидалось NULL (триггер 0022: geom NULL ⇒ длины нет)", lkm)
	}
	if by != "directus" {
		t.Errorf("geocoded_by = %q, ожидалось directus (снятие — кураторское действие)", by)
	}

	// manual снимать напрямую нельзя — только через wrong_reported (двухшаговый след SM-C2).
	mc := insertContract(t, tx, "ZZGEO32-CLR-M", "710000000")
	mid := insertGeoRaw(t, tx, mc, "manual", "POINT(30 12)", nil)
	if n, err := s.ClearGeoObjectToUnmatched(ctx, mid, "directus"); err != nil || n != 0 {
		t.Errorf("manual→unmatched напрямую: n=%d err=%v, ожидалось 0/nil", n, err)
	}
}

// ResolveGeoObjectManually: unmatched→manual с LINESTRING — length_km ВЫВОДИТСЯ триггером (сверка с
// независимым ST_Length-расчётом); с POINT — length_km NULL. confidence обнуляется честно.
func TestResolveManually_LineDerivesLength_PointNull(t *testing.T) {
	_, tx := mustConn(t)
	ctx := context.Background()
	s := curationStoreOn(tx)

	const line = "LINESTRING(30.00 10.00, 30.05 10.04)"
	var wantKm float64
	if err := tx.QueryRow(ctx, `SELECT ST_Length(ST_GeomFromText($1,4326)::geography)/1000.0`, line).Scan(&wantKm); err != nil {
		t.Fatalf("независимый расчёт длины: %v", err)
	}

	lc := insertContract(t, tx, "ZZGEO32-RES-L", "710000000")
	lid := insertGeoRaw(t, tx, lc, "unmatched", "", nil)
	if n, err := s.ResolveGeoObjectManually(ctx, lid, line, "directus"); err != nil || n != 1 {
		t.Fatalf("unmatched→manual (LINESTRING): n=%d err=%v, ожидалось 1/nil", n, err)
	}
	st, by, _, lkm := geoRowState(t, tx, lc)
	if st != "manual" || by != "directus" {
		t.Errorf("статус/провенанс = %q/%q, ожидалось manual/directus", st, by)
	}
	if !lkm.Valid {
		t.Fatal("length_km NULL для ручной LINESTRING — триггер 0022 обязан вывести длину (иначе дорога выпадает из ₸/км)")
	}
	if diff := lkm.Float64 - wantKm; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("length_km = %v, ожидалось %v (независимый ST_Length-расчёт)", lkm.Float64, wantKm)
	}

	pc := insertContract(t, tx, "ZZGEO32-RES-P", "710000000")
	pid := insertGeoRaw(t, tx, pc, "unmatched", "", nil)
	if n, err := s.ResolveGeoObjectManually(ctx, pid, "POINT(30 13)", "directus"); err != nil || n != 1 {
		t.Fatalf("unmatched→manual (POINT): n=%d err=%v", n, err)
	}
	if _, _, _, plkm := geoRowState(t, tx, pc); plkm.Valid {
		t.Errorf("length_km = %+v для ручного POINT, ожидалось NULL (у точки нет длины)", plkm)
	}
	// Вырожденная LINESTRING (совпадающие вершины, ревью 3.2): длины нет — NULL, не «0» (NULLIF в триггере).
	dc := insertContract(t, tx, "ZZGEO32-RES-D", "710000000")
	did := insertGeoRaw(t, tx, dc, "unmatched", "", nil)
	if n, err := s.ResolveGeoObjectManually(ctx, did, "LINESTRING(30 10, 30 10)", "directus"); err != nil || n != 1 {
		t.Fatalf("unmatched→manual (вырожденная LINESTRING): n=%d err=%v", n, err)
	}
	if _, _, _, dlkm := geoRowState(t, tx, dc); dlkm.Valid {
		t.Errorf("length_km = %+v для вырожденной LINESTRING, ожидалось NULL (нет длины — не «0 км»)", dlkm)
	}
	// id не найден — 0 строк, не тихий успех.
	if n, err := s.ResolveGeoObjectManually(ctx, -1, "POINT(30 14)", "directus"); err != nil || n != 0 {
		t.Errorf("несуществующий id: n=%d err=%v, ожидалось 0/nil", n, err)
	}
	// verified перерисовывать напрямую нельзя (D1: сначала wrong_reported — двухшаговый след SM-C2).
	vc := insertContract(t, tx, "ZZGEO32-RES-V", "710000000")
	vid := insertGeoRaw(t, tx, vc, "auto", "POINT(30 15)", nil)
	if n, err := s.MarkGeoObjectVerified(ctx, vid); err != nil || n != 1 {
		t.Fatalf("подготовка verified: n=%d err=%v", n, err)
	}
	if n, err := s.ResolveGeoObjectManually(ctx, vid, "POINT(31 16)", "directus"); err != nil || n != 0 {
		t.Errorf("verified→manual напрямую: n=%d err=%v, ожидалось 0/nil (запрещённый переход, ревью 3.2)", n, err)
	}
	if st, _, geom, _ := geoRowState(t, tx, vc); st != "verified" || geom != "POINT(30 15)" {
		t.Errorf("verified-строка = %q/%q, ожидалось verified/POINT(30 15) — Resolve не должен её трогать", st, geom)
	}
}

// Триггер length_km и «сырой писатель» seed-стиля: явный length при INSERT уважается (seed задаёт СВОЙ
// length при одинаковой геометрии); перерисовка geom — пересчёт; правка не-geom колонок длину не трогает.
func TestLengthTrigger_ExplicitPreserved_RecomputedOnGeomChange(t *testing.T) {
	_, tx := mustConn(t)
	ctx := context.Background()

	c := insertContract(t, tx, "ZZGEO32-TRG-1", "710000000")
	explicit := 5.0
	id := insertGeoRaw(t, tx, c, "manual", "LINESTRING(30.00 10.00, 30.05 10.04)", &explicit)

	_, _, _, lkm := geoRowState(t, tx, c)
	if !lkm.Valid || lkm.Float64 != 5.0 {
		t.Fatalf("length_km = %+v, ожидалось явные 5.0 (INSERT с заданным length триггер НЕ перетирает — seed жив)", lkm)
	}

	// Правка НЕ-geom колонки — длина не трогается.
	if _, err := tx.Exec(ctx, `UPDATE geo_objects SET address_text = 'поправили адрес' WHERE id = $1`, id); err != nil {
		t.Fatalf("update address_text: %v", err)
	}
	if _, _, _, l2 := geoRowState(t, tx, c); !l2.Valid || l2.Float64 != 5.0 {
		t.Errorf("length_km = %+v после правки address_text, ожидалось нетронутые 5.0", l2)
	}

	// Перерисовка линии — длина следует за геометрией (пересчёт).
	const redrawn = "LINESTRING(30.00 10.00, 30.10 10.08)"
	var wantKm float64
	if err := tx.QueryRow(ctx, `SELECT ST_Length(ST_GeomFromText($1,4326)::geography)/1000.0`, redrawn).Scan(&wantKm); err != nil {
		t.Fatalf("независимый расчёт: %v", err)
	}
	if _, err := tx.Exec(ctx, `UPDATE geo_objects SET geom = ST_GeomFromText($2, 4326) WHERE id = $1`, id, redrawn); err != nil {
		t.Fatalf("update geom: %v", err)
	}
	_, _, _, l3 := geoRowState(t, tx, c)
	if !l3.Valid {
		t.Fatal("length_km NULL после перерисовки LINESTRING — триггер обязан пересчитать")
	}
	if diff := l3.Float64 - wantKm; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("length_km = %v после перерисовки, ожидалось %v (длина следует за геометрией)", l3.Float64, wantKm)
	}
}

// verified терминален для batch: контракт уходит из ListContractsForGeocode (закрывает деферред ревью 3.1
// «auto пере-геокодируется каждый прогон навсегда»), а прямая попытка batch-UPSERT не меняет строку (гейт).
func TestVerified_TerminalForBatch(t *testing.T) {
	_, tx := mustConn(t)
	ctx := context.Background()
	s := curationStoreOn(tx)
	q := gen.New(tx)

	c := insertContract(t, tx, "ZZGEO32-TERM-1", "710000000")
	id := insertGeoRaw(t, tx, c, "auto", "POINT(30 10)", nil)

	inList := func() bool {
		rows, err := q.ListContractsForGeocode(ctx)
		if err != nil {
			t.Fatalf("ListContractsForGeocode: %v", err)
		}
		for _, r := range rows {
			if r.ID == c {
				return true
			}
		}
		return false
	}
	if !inList() {
		t.Fatal("auto-контракт должен быть в очереди batch (пере-геокод auto допустим)")
	}
	if n, err := s.MarkGeoObjectVerified(ctx, id); err != nil || n != 1 {
		t.Fatalf("verify: n=%d err=%v", n, err)
	}
	if inList() {
		t.Error("verified-контракт НЕ должен пере-выбираться batch'ем (терминальность — деферред 3.1 закрыт)")
	}
	// Гейт в глубину: даже если verified-строку насильно скормить UPSERT'у — она неприкосновенна.
	err := s.UpsertGeoObject(ctx, gen.UpsertGeoObjectParams{
		ContractID:    pgtype.Int8{Int64: c, Valid: true},
		GeomWkt:       pgtype.Text{String: "POINT(0.02 0.02)", Valid: true},
		GeocodeStatus: "auto",
		GeocodedBy:    pgtype.Text{String: "nominatim", Valid: true},
	})
	if err != nil {
		t.Fatalf("UPSERT против verified (должен молча не сработать): %v", err)
	}
	st, _, geom, _ := geoRowState(t, tx, c)
	if st != "verified" || geom != "POINT(30 10)" {
		t.Errorf("verified-строка изменена batch'ем: %q/%q — гейт IN('auto','unmatched') нарушен", st, geom)
	}
}

// Приёмка AR-10 (адаптация 3.2, epics.md:215-217): «прогон → курация → прогон» — ручные правки
// (manual/verified/wrong_reported) переживают повторный batch-прогон ЦЕЛИКОМ (через geocodeContracts
// с реальным store), а очередь второго прогона содержит только непокрытые auto.
func TestAcceptance_AR10_BatchCurationBatch(t *testing.T) {
	_, tx := mustConn(t)
	ctx := context.Background()
	s := curationStoreOn(tx)
	q := gen.New(tx)

	// Якорь-изоляция: БД может быть засеяна — работаем ТОЛЬКО со своими контрактами (фильтр по id).
	aID := insertContract(t, tx, "ZZGEO32-AR10-A", "") // auto → verified
	bID := insertContract(t, tx, "ZZGEO32-AR10-B", "") // auto → wrong_reported
	cID := insertContract(t, tx, "ZZGEO32-AR10-C", "") // unmatched → manual
	dID := insertContract(t, tx, "ZZGEO32-AR10-D", "") // auto, не курирован — пере-геокодится
	eID := insertContract(t, tx, "ZZGEO32-AR10-E", "") // unmatched, НЕ курирован — НЕ пере-выбирается (ревью 3.2)
	mine := map[int64]bool{aID: true, bID: true, cID: true, dID: true, eID: true}

	// insertContract хардкодит ОДИНАКОВЫЙ subject_ru → у всех контрактов был бы один addr()-ключ, и карта
	// фейк-геокодера не различила бы C (матч «за компанию»). Делаем предметы уникальными per-контракт.
	if _, err := tx.Exec(ctx,
		`UPDATE contracts SET subject_ru = subject_ru || ' ' || goszakup_contract_id WHERE id = ANY($1)`,
		[]int64{aID, bID, cID, dID, eID}); err != nil {
		t.Fatalf("уникализация subject_ru: %v", err)
	}

	myQueue := func() []gen.ListContractsForGeocodeRow {
		all, err := q.ListContractsForGeocode(ctx)
		if err != nil {
			t.Fatalf("ListContractsForGeocode: %v", err)
		}
		var out []gen.ListContractsForGeocodeRow
		for _, r := range all {
			if mine[r.ID] {
				out = append(out, r)
			}
		}
		return out
	}

	// Прогон 1: A/B/D матчатся в P1, C и E — честный не-матч (нет ключа в карте фейка).
	round1 := myQueue()
	if len(round1) != 5 {
		t.Fatalf("очередь прогона 1: %d наших контрактов, ожидалось 5", len(round1))
	}
	conf := 0.7
	p1 := geo.Match{Lat: 10, Lon: 30, Confidence: &conf} // → POINT(30 10), вдали от любых полигонов districts
	m1 := map[string]geo.Match{}
	for _, r := range round1 {
		if r.ID != cID && r.ID != eID {
			m1[addr(r)] = p1
		}
	}
	st1, err := geocodeContracts(ctx, round1, &fakeGeocoder{matches: m1}, s, 0, io.Discard)
	if err != nil {
		t.Fatalf("прогон 1: %v", err)
	}
	if st1.Matched != 3 || st1.Unmatched != 2 || st1.Errors != 0 {
		t.Fatalf("прогон 1: %+v, ожидалось 3/2/0 (A,B,D matched; C,E unmatched)", st1)
	}

	// Курация между прогонами (имитация Directus — кураторские методы store).
	rowID := func(contractID int64) int64 {
		row, err := s.GetGeoObjectByContractID(ctx, contractID)
		if err != nil {
			t.Fatalf("GetGeoObjectByContractID %d: %v", contractID, err)
		}
		return row.ID
	}
	if n, err := s.MarkGeoObjectVerified(ctx, rowID(aID)); err != nil || n != 1 {
		t.Fatalf("verify A: n=%d err=%v", n, err)
	}
	if n, err := s.MarkGeoObjectWrongReported(ctx, rowID(bID)); err != nil || n != 1 {
		t.Fatalf("wrong_report B: n=%d err=%v", n, err)
	}
	const p2 = "POINT(31 11)"
	if n, err := s.ResolveGeoObjectManually(ctx, rowID(cID), p2, "directus"); err != nil || n != 1 {
		t.Fatalf("resolve C: n=%d err=%v", n, err)
	}

	// Прогон 2: очередь содержит ТОЛЬКО D (A verified, B wrong_reported, C manual — выпали; НЕТРОНУТЫЙ
	// unmatched E тоже НЕ пере-выбирается — клейм CLI-сообщения и ListContractsForGeocode, ревью 3.2);
	// геокодер теперь отдаёт ДРУГУЮ точку P3 — перезаписаться может только D.
	round2 := myQueue()
	if len(round2) != 1 || round2[0].ID != dID {
		ids := make([]int64, 0, len(round2))
		for _, r := range round2 {
			ids = append(ids, r.ID)
		}
		t.Fatalf("очередь прогона 2: %v, ожидался только D=%d (курация И нетронутый unmatched выпали из очереди)", ids, dID)
	}
	conf3 := 0.9
	p3 := geo.Match{Lat: 12, Lon: 32, Confidence: &conf3}
	m2 := map[string]geo.Match{}
	for _, r := range round2 {
		m2[addr(r)] = p3
	}
	if _, err := geocodeContracts(ctx, round2, &fakeGeocoder{matches: m2}, s, 0, io.Discard); err != nil {
		t.Fatalf("прогон 2: %v", err)
	}

	// Итог: все три правки целы, некурированный D честно пере-геокодирован.
	if st, _, geom, _ := geoRowState(t, tx, aID); st != "verified" || geom != "POINT(30 10)" {
		t.Errorf("A = %q/%q, ожидалось verified/POINT(30 10) — верификация пережила прогон", st, geom)
	}
	if st, _, geom, _ := geoRowState(t, tx, bID); st != "wrong_reported" || geom != "POINT(30 10)" {
		t.Errorf("B = %q/%q, ожидалось wrong_reported/POINT(30 10) — пометка пережила прогон", st, geom)
	}
	if st, by, geom, _ := geoRowState(t, tx, cID); st != "manual" || geom != p2 || by != "directus" {
		t.Errorf("C = %q/%q/%q, ожидалось manual/%s/directus — ручная точка пережила прогон", st, geom, by, p2)
	}
	if st, _, geom, _ := geoRowState(t, tx, dID); st != "auto" || geom != "POINT(32 12)" {
		t.Errorf("D = %q/%q, ожидалось auto/POINT(32 12) — некурированный auto пере-геокодится", st, geom)
	}
	if st, _, geom, _ := geoRowState(t, tx, eID); st != "unmatched" || geom != "" {
		t.Errorf("E = %q/%q, ожидалось unmatched/пусто — нетронутый unmatched не трогается вторым прогоном", st, geom)
	}
}
