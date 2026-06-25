//go:build integration

// Интеграционный тест флага «наличие в РНУ» (Story 4.5, AC2/AC4): seed rnu_entries → RNURecalculator (с clock.Fixed)
// пересчитывает по CONTRACTOR-субъекту (organization_id) → raised (активная)/not_raised (будущая)/АВТО-СНЯТИЕ
// (сдвиг clock за end_date)/идемпотентность. Гоняется: `go test -tags=integration ./internal/store/...` (миграции 0001-0008).
package projection_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"ashyqqala/server/internal/clock"
	"ashyqqala/server/internal/flags"
	"ashyqqala/server/internal/store/projection"
)

var rnuParamsInt = flags.Params{MethodologyVersion: "v1.0", RNUEnabled: true}

// тестовые organization_id (высокие, чтобы не пересекаться с seed).
const (
	oOpen   = int64(97001) // start прошлое, end NULL → всегда raised
	oTemp   = int64(97002) // start прошлое, end 2026-06-20 → raised до, снят после
	oFuture = int64(97003) // start 2026-12-01 → всегда not_raised
)

func TestRNUFlag_RaiseAutoClear(t *testing.T) {
	pool := riskFlagPool(t)
	defer pool.Close()
	ctx := context.Background()
	store := projection.NewRiskFlagStore(pool, rnuParamsInt)

	clean := func() {
		_, _ = pool.Exec(ctx, "DELETE FROM risk_flags WHERE organization_id = ANY($1)", []int64{oOpen, oTemp, oFuture})
		_, _ = pool.Exec(ctx, "DELETE FROM rnu_entries WHERE organization_id = ANY($1)", []int64{oOpen, oTemp, oFuture})
	}
	clean()
	defer clean()

	ins := `INSERT INTO rnu_entries (organization_id, goszakup_rnu_id, start_date, end_date, reason_ref, source_url) VALUES ($1,$2,$3,$4,$5,$6)`
	if _, err := pool.Exec(ctx, ins, oOpen, "RNU-OPEN", "2026-01-01", nil, "приказ-1", "https://goszakup.gov.kz/ru/egzrnu/index"); err != nil {
		t.Fatalf("seed oOpen: %v", err)
	}
	if _, err := pool.Exec(ctx, ins, oTemp, "RNU-TEMP", "2026-01-01", "2026-06-20", "приказ-2", "https://goszakup.gov.kz/ru/egzrnu/index"); err != nil {
		t.Fatalf("seed oTemp: %v", err)
	}
	if _, err := pool.Exec(ctx, ins, oFuture, "RNU-FUT", "2026-12-01", nil, "приказ-3", "https://goszakup.gov.kz/ru/egzrnu/index"); err != nil {
		t.Fatalf("seed oFuture: %v", err)
	}

	// ПРОГОН #1: «сейчас» = 2026-06-10 (до end_date oTemp) → oOpen + oTemp активны, oFuture нет.
	before := projection.RNURecalculator{Store: store, Clock: clock.Fixed{T: time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)}, Provider: store.ListRNUInputs}
	if err := before.RecalcFlags(ctx); err != nil {
		t.Fatalf("recalc #1: %v", err)
	}

	row, ok, err := store.GetRNU(ctx, oOpen)
	if err != nil || !ok || !row.IsActive {
		t.Fatalf("oOpen: ожидался активный rnu-флаг (ok=%v active=%v err=%v)", ok, row.IsActive, err)
	}
	if row.SubjectType != "contractor" || !row.OrganizationID.Valid || row.OrganizationID.Int64 != oOpen || row.ContractID.Valid {
		t.Errorf("oOpen: ожидался contractor/organization_id=%d, contract_id NULL; got type=%q org=%+v contract=%+v", oOpen, row.SubjectType, row.OrganizationID, row.ContractID)
	}
	var ev flags.RNUEvidence
	if err := json.Unmarshal(row.Evidence, &ev); err != nil {
		t.Fatalf("oOpen evidence невалиден: %v", err)
	}
	if ev.GoszakupRNUID != "RNU-OPEN" || ev.SourceURL == "" || ev.StartDateUnix == nil || !ev.Enabled {
		t.Errorf("oOpen evidence неполон (атрибуция государству): %+v", ev)
	}
	if row.MethodologyVersion != "v1.0" {
		t.Errorf("oOpen methodology_version=%q, ожидалось v1.0", row.MethodologyVersion)
	}
	if _, ok, _ := store.GetRNU(ctx, oFuture); ok {
		t.Errorf("oFuture: rnu-флаг не должен существовать (start в будущем)")
	}
	if n, _ := store.CountActiveRNU(ctx); n != 2 {
		t.Fatalf("после #1 активных rnu-флагов = %d, ожидалось 2 (oOpen + oTemp)", n)
	}

	// ИДЕМПОТЕНТНОСТЬ: повтор #1 → по-прежнему 2 (org_uniq).
	if err := before.RecalcFlags(ctx); err != nil {
		t.Fatalf("recalc #1 повтор: %v", err)
	}
	if n, _ := store.CountActiveRNU(ctx); n != 2 {
		t.Fatalf("после повтора активных = %d, ожидалось 2 (org_uniq)", n)
	}

	// ПРОГОН #2: «сейчас» = 2026-06-30 (ПОСЛЕ end_date oTemp) → АВТО-СНЯТИЕ oTemp; oOpen остаётся.
	after := projection.RNURecalculator{Store: store, Clock: clock.Fixed{T: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)}, Provider: store.ListRNUInputs}
	if err := after.RecalcFlags(ctx); err != nil {
		t.Fatalf("recalc #2 (авто-снятие): %v", err)
	}
	row, ok, _ = store.GetRNU(ctx, oTemp)
	if !ok || row.IsActive || !row.ClearedAt.Valid {
		t.Fatalf("oTemp: ожидался снятый флаг (is_active=false, cleared_at), got active=%v cleared=%v", row.IsActive, row.ClearedAt.Valid)
	}
	if r, ok, _ := store.GetRNU(ctx, oOpen); !ok || !r.IsActive {
		t.Errorf("oOpen: должен оставаться активным после сдвига clock")
	}
	if n, _ := store.CountActiveRNU(ctx); n != 1 {
		t.Fatalf("после авто-снятия активных = %d, ожидалось 1 (только oOpen)", n)
	}
}

const oMulti = int64(97004)

// TestRNUFlag_MultiRecordPerOrg — P1 ревью: подрядчик с НЕСКОЛЬКИМИ записями РНУ (старая истёкшая + новая
// активная) → орг RAISED (агрегация per-org: есть ≥1 активная), активная запись НЕ затёрта истёкшей в одной
// транзакции; evidence — от активной (наибольший start_date по детерминированному ORDER BY).
func TestRNUFlag_MultiRecordPerOrg(t *testing.T) {
	pool := riskFlagPool(t)
	defer pool.Close()
	ctx := context.Background()
	store := projection.NewRiskFlagStore(pool, rnuParamsInt)

	clean := func() {
		_, _ = pool.Exec(ctx, "DELETE FROM risk_flags WHERE organization_id = $1", oMulti)
		_, _ = pool.Exec(ctx, "DELETE FROM rnu_entries WHERE organization_id = $1", oMulti)
	}
	clean()
	defer clean()

	ins := `INSERT INTO rnu_entries (organization_id, goszakup_rnu_id, start_date, end_date, reason_ref, source_url) VALUES ($1,$2,$3,$4,$5,$6)`
	// Старая истёкшая запись (start 2026-01-01, end 2026-03-01) + новая активная (start 2026-04-01, end NULL).
	if _, err := pool.Exec(ctx, ins, oMulti, "RNU-OLD", "2026-01-01", "2026-03-01", "приказ-старый", "https://goszakup.gov.kz/ru/egzrnu/index"); err != nil {
		t.Fatalf("seed old: %v", err)
	}
	if _, err := pool.Exec(ctx, ins, oMulti, "RNU-NEW", "2026-04-01", nil, "приказ-новый", "https://goszakup.gov.kz/ru/egzrnu/index"); err != nil {
		t.Fatalf("seed new: %v", err)
	}

	rec := projection.RNURecalculator{Store: store, Clock: clock.Fixed{T: time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)}, Provider: store.ListRNUInputs}
	if err := rec.RecalcFlags(ctx); err != nil {
		t.Fatalf("recalc: %v", err)
	}

	row, ok, err := store.GetRNU(ctx, oMulti)
	if err != nil || !ok || !row.IsActive {
		t.Fatalf("oMulti: ожидался активный флаг (активная запись НЕ должна быть затёрта истёкшей); ok=%v active=%v err=%v", ok, row.IsActive, err)
	}
	var ev flags.RNUEvidence
	if err := json.Unmarshal(row.Evidence, &ev); err != nil {
		t.Fatalf("oMulti evidence невалиден: %v", err)
	}
	if ev.GoszakupRNUID != "RNU-NEW" {
		t.Errorf("evidence должна быть от АКТИВНОЙ записи RNU-NEW (наибольший start_date), got %q", ev.GoszakupRNUID)
	}
	if n, _ := store.CountActiveRNU(ctx); n != 1 {
		t.Fatalf("активных rnu-флагов = %d, ожидалось 1 (один на org, без дублей)", n)
	}
}
