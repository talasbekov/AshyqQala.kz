package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"ashyqqala/server/internal/registry"
	"ashyqqala/server/internal/store/gen"
)

type mockContractorStore struct {
	org             gen.Organization
	orgErr          error
	unresolved      int64
	unresolvedErr   error
	unresolvedNames []string
	namesErr        error
	agg             gen.ContractorAggregatesRow
	aggErr          error
	contracts       []gen.ListContractsBySupplierOrgRow
	contractsErr    error
	flags           []gen.RiskFlag
	flagsErr        error
	rnu             []gen.RnuEntry
	rnuErr          error
}

func (m mockContractorStore) GetOrganizationByBIN(context.Context, string) (gen.Organization, error) {
	return m.org, m.orgErr
}
func (m mockContractorStore) CountUnresolvedAliasesByOrg(context.Context, pgtype.Int8) (int64, error) {
	return m.unresolved, m.unresolvedErr
}
func (m mockContractorStore) ListUnresolvedAliasNames(context.Context) ([]string, error) {
	return m.unresolvedNames, m.namesErr
}
func (m mockContractorStore) ContractorAggregates(context.Context, pgtype.Int8) (gen.ContractorAggregatesRow, error) {
	return m.agg, m.aggErr
}
func (m mockContractorStore) ListContractsBySupplierOrg(context.Context, pgtype.Int8) ([]gen.ListContractsBySupplierOrgRow, error) {
	return m.contracts, m.contractsErr
}
func (m mockContractorStore) ListContractorFlags(context.Context, pgtype.Int8) ([]gen.RiskFlag, error) {
	return m.flags, m.flagsErr
}
func (m mockContractorStore) ListRNUByOrg(context.Context, int64) ([]gen.RnuEntry, error) {
	return m.rnu, m.rnuErr
}

func sampleOrg() gen.Organization {
	return gen.Organization{
		ID:         7,
		Bin:        "222222222222",
		NameRu:     pgtype.Text{String: "ТОО Астана Жол", Valid: true},
		NameKk:     pgtype.Text{String: "Астана Жол ЖШС", Valid: true},
		IsSupplier: true,
	}
}

func fixedNow() time.Time { return time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC) }

func newContractorRouter(store ContractorStore) http.Handler {
	r := chi.NewRouter()
	r.Get("/api/contractors/{bin}", ContractorsHandler{Store: store, Now: fixedNow}.Get)
	return r
}

// TestContractor_IncompleteProfile — нет связанных контрактов (linkage до 2.2) → «профиль неполный»:
// count/amount честный no_data (НЕ «0»), profile.state=incomplete.
func TestContractor_IncompleteProfile(t *testing.T) {
	h := ContractorsHandler{Store: mockContractorStore{org: sampleOrg(), agg: gen.ContractorAggregatesRow{}}, Now: fixedNow}
	dto, err := h.Projection(context.Background(), "222222222222")
	if err != nil {
		t.Fatalf("Projection: %v", err)
	}
	if dto.Profile.State != "incomplete" {
		t.Errorf("profile.state=%q, ожидалось incomplete", dto.Profile.State)
	}
	if dto.ContractCount.State != registry.StateNoData || dto.TotalAmountTng.State != registry.StateNoData {
		t.Errorf("при неполном профиле count/amount должны быть no_data, got %v/%v", dto.ContractCount.State, dto.TotalAmountTng.State)
	}
}

// TestContractor_LinkedContracts — есть связанные контракты → count/amount строкой (okField).
func TestContractor_LinkedContracts(t *testing.T) {
	h := ContractorsHandler{Store: mockContractorStore{
		org: sampleOrg(),
		agg: gen.ContractorAggregatesRow{ContractCount: 3, TotalAmountTng: 240000000, Regions: []string{"710000000"}},
	}, Now: fixedNow}
	dto, _ := h.Projection(context.Background(), "222222222222")
	if dto.ContractCount.State != registry.StateOK || *dto.ContractCount.Value != "3" {
		t.Errorf("contract_count=%+v, ожидалось ok/3", dto.ContractCount)
	}
	if *dto.TotalAmountTng.Value != "240000000" {
		t.Errorf("total=%q, ожидалось 240000000 (деньги строкой)", *dto.TotalAmountTng.Value)
	}
	// F6: есть связанные контракты → профиль partial (не «incomplete» с обещанием «появится после импорта»).
	if dto.Profile.State != "partial" {
		t.Errorf("profile.state=%q, ожидалось partial (есть связанные контракты)", dto.Profile.State)
	}
}

// TestContractor_Unverified_ByNameMatch (F1) — conflict-псевдоним (org_id NULL) детектируется по СОВПАДЕНИЮ
// ИМЕНИ → «профиль уточняется». Negative-control: непохожее имя в очереди НЕ делает профиль unverified.
func TestContractor_Unverified_ByNameMatch(t *testing.T) {
	// дефис vs пробел канонизируются одинаково даже с пустым лексиконом → матч.
	h := ContractorsHandler{Store: mockContractorStore{
		org:             sampleOrg(), // name_ru "ТОО Астана Жол"
		unresolvedNames: []string{"ТОО Астана-Жол"},
	}, Now: fixedNow}
	dto, _ := h.Projection(context.Background(), "222222222222")
	if dto.Profile.State != "unverified" {
		t.Fatalf("совпадение имени с conflict-псевдонимом → unverified, got %q", dto.Profile.State)
	}
	// negative-control: чужое имя в очереди не трогает профиль.
	h2 := ContractorsHandler{Store: mockContractorStore{
		org:             sampleOrg(),
		unresolvedNames: []string{"ТОО Совсем Другая Фирма"},
	}, Now: fixedNow}
	dto2, _ := h2.Projection(context.Background(), "222222222222")
	if dto2.Profile.State == "unverified" {
		t.Fatalf("непохожее имя в очереди НЕ должно делать профиль unverified")
	}
}

// TestContractor_NotRaisedFlag (F7) — снятый contractor-флаг (is_active=false) → not_raised (оценивалось,
// сигнала нет), НЕ insufficient (нет данных). Закрывает вторую половину честной реконструкции AC-4.
func TestContractor_NotRaisedFlag(t *testing.T) {
	flags := resolveContractorFlags([]gen.RiskFlag{
		{FlagType: "monopoly", IsActive: false, MethodologyVersion: "v1.0"},
	}, false)
	byID := map[string]ContractorFlagDTO{}
	for _, f := range flags {
		byID[f.FlagID] = f
	}
	if byID["monopoly"].State != registry.FlagNotRaised {
		t.Errorf("снятый monopoly → not_raised, got %s", byID["monopoly"].State)
	}
}

// TestContractor_Unverified — есть manual/conflict-псевдонимы → «профиль уточняется» + флаги НЕ основание
// (все insufficient_data, даже если в сторе raised).
func TestContractor_Unverified(t *testing.T) {
	h := ContractorsHandler{Store: mockContractorStore{
		org:        sampleOrg(),
		unresolved: 1,
		flags:      []gen.RiskFlag{{FlagType: "monopoly", IsActive: true, MethodologyVersion: "v1.0"}},
	}, Now: fixedNow}
	dto, _ := h.Projection(context.Background(), "222222222222")
	if dto.Profile.State != "unverified" {
		t.Fatalf("profile.state=%q, ожидалось unverified", dto.Profile.State)
	}
	for _, f := range dto.Flags {
		if f.State != registry.FlagInsufficientData {
			t.Errorf("при unverified флаг %s должен быть insufficient_data, got %s (флаги не основание)", f.FlagID, f.State)
		}
	}
}

// TestContractor_FlagsHonestReconstruction — монополия raised, РНУ-флаг без строки → insufficient (не «чисто»).
func TestContractor_FlagsHonestReconstruction(t *testing.T) {
	flags := resolveContractorFlags([]gen.RiskFlag{
		{FlagType: "monopoly", IsActive: true, MethodologyVersion: "v1.0", Evidence: []byte(`{"share":0.7}`)},
	}, false)
	byID := map[string]ContractorFlagDTO{}
	for _, f := range flags {
		byID[f.FlagID] = f
	}
	if byID["monopoly"].State != registry.FlagRaised {
		t.Errorf("monopoly должна быть raised, got %s", byID["monopoly"].State)
	}
	if byID["rnu"].State != registry.FlagInsufficientData {
		t.Errorf("rnu без строки → insufficient_data (не not_raised «чисто»), got %s", byID["rnu"].State)
	}
}

// TestContractor_RNUMarks_AutoClearByDate — активность метки реконструируется из дат (авто-снятие по end_date).
func TestContractor_RNUMarks_AutoClearByDate(t *testing.T) {
	d := func(y, m, day int) pgtype.Date {
		return pgtype.Date{Time: time.Date(y, time.Month(m), day, 0, 0, 0, 0, time.UTC), Valid: true}
	}
	marks := resolveRNUMarks([]gen.RnuEntry{
		{StartDate: d(2026, 1, 1), EndDate: pgtype.Date{}},   // без end_date → активна
		{StartDate: d(2025, 1, 1), EndDate: d(2025, 12, 31)}, // истекла до asOf → снята
		{StartDate: d(2026, 1, 1), EndDate: d(2026, 12, 31)}, // end_date в будущем → активна
		{StartDate: d(2026, 1, 1), EndDate: d(2026, 6, 27)},  // end_date == asOf-день → снята (строго, F3)
	}, fixedNow()) // fixedNow = 2026-06-27
	if !marks[0].Active {
		t.Error("метка без end_date должна быть active")
	}
	if marks[1].Active {
		t.Error("метка с истёкшим end_date должна быть снята (auto-clear)")
	}
	if !marks[2].Active {
		t.Error("метка с end_date в будущем должна быть active")
	}
	if marks[3].Active {
		t.Error("метка с end_date == сегодня должна быть снята (строгая граница, как RecomputeRNU)")
	}
}

// TestContractor_WireFormat_ValidatesAgainstOpenAPI — ответ валиден по OpenAPI-схеме Contractor (kin-openapi).
func TestContractor_WireFormat_ValidatesAgainstOpenAPI(t *testing.T) {
	store := mockContractorStore{
		org: sampleOrg(),
		agg: gen.ContractorAggregatesRow{ContractCount: 2, TotalAmountTng: 480000000, Regions: []string{"710000000"}},
		flags: []gen.RiskFlag{{
			FlagType: "monopoly", IsActive: true, MethodologyVersion: "v1.0",
			Evidence:   []byte(`{"share":0.7}`),
			DetectedAt: pgtype.Timestamptz{Time: fixedNow(), Valid: true},
		}},
		rnu: []gen.RnuEntry{{
			OrganizationID: 7,
			GoszakupRnuID:  pgtype.Text{String: "RNU-1", Valid: true},
			StartDate:      pgtype.Date{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true},
			SourceUrl:      pgtype.Text{String: "https://goszakup.gov.kz/ru/rnu", Valid: true},
		}},
	}
	srv := httptest.NewServer(newContractorRouter(store))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/contractors/222222222222")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d, ожидалось 200", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if err := loadSchema(t, "Contractor").VisitJSON(body); err != nil {
		t.Fatalf("ответ не валиден по OpenAPI Contractor: %v", err)
	}
	// деньги строкой; rnu-метка несёт source_url (AC-3)
	amt, _ := body["total_amount_tng"].(map[string]any)
	if _, isStr := amt["value"].(string); !isStr {
		t.Fatalf("total_amount_tng.value должно быть строкой, got %T", amt["value"])
	}
	marks, _ := body["rnu_marks"].([]any)
	if len(marks) != 1 {
		t.Fatalf("ожидалась 1 rnu-метка, got %d", len(marks))
	}
}

// TestContractor_NotFound — нет организации → 404.
func TestContractor_NotFound(t *testing.T) {
	srv := newContractorRouter(mockContractorStore{orgErr: pgx.ErrNoRows})
	req := httptest.NewRequest(http.MethodGet, "/api/contractors/999999999999", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d, ожидалось 404", w.Code)
	}
}
