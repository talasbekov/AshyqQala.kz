package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"ashyqqala/server/internal/store/gen"
)

type mockStore struct {
	c          gen.Contract
	err        error
	actPresent bool
	act        gen.Act
	actErr     error
	flags      []gen.RiskFlag
	flagsErr   error
}

func (m mockStore) GetContractByID(context.Context, string) (gen.Contract, error) {
	return m.c, m.err
}

func (m mockStore) GetLatestActByContractID(context.Context, int64) (gen.Act, error) {
	if m.actErr != nil {
		return gen.Act{}, m.actErr
	}
	if !m.actPresent {
		return gen.Act{}, pgx.ErrNoRows // нет акта (честное состояние)
	}
	return m.act, nil
}

func (m mockStore) ListContractFlags(context.Context, pgtype.Int8) ([]gen.RiskFlag, error) {
	return m.flags, m.flagsErr
}

// loadSchema грузит рукописный OpenAPI-арбитр и возвращает разрешённую схему по имени.
func loadSchema(t *testing.T, name string) *openapi3.Schema {
	t.Helper()
	doc, err := openapi3.NewLoader().LoadFromFile("../../../docs/api-contracts/openapi.yaml")
	if err != nil {
		t.Fatalf("load openapi: %v", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("openapi сам по себе невалиден: %v", err)
	}
	ref := doc.Components.Schemas[name]
	if ref == nil || ref.Value == nil {
		t.Fatalf("схема %q не найдена", name)
	}
	return ref.Value
}

func newRouter(store ContractStore) http.Handler {
	r := chi.NewRouter()
	r.Get("/api/contracts/{goszakup_id}", ContractsHandler{Store: store}.Get)
	return r
}

func sampleContract() gen.Contract {
	mkDate := func(s string) pgtype.Date {
		tt, _ := time.Parse("2006-01-02", s)
		return pgtype.Date{Time: tt, Valid: true}
	}
	ts := pgtype.Timestamptz{Time: time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC), Valid: true}
	return gen.Contract{
		GoszakupContractID: "DEMO-0001",
		SubjectRu:          pgtype.Text{String: "Ремонт автодороги", Valid: true},
		SubjectKk:          pgtype.Text{Valid: false}, // NULL → no_data
		AmountTng:          pgtype.Int8{Int64: 123456789, Valid: true},
		SignDate:           mkDate("2026-03-15"),
		// PlanStart/PlanEnd оставлены NULL (zero-value pgtype.Date → Valid:false)
		Status:     pgtype.Text{String: "active", Valid: true},
		Direction:  pgtype.Text{String: "road", Valid: true},
		KatoCode:   pgtype.Text{String: "710000000", Valid: true},
		SourceUrl:  pgtype.Text{String: "https://goszakup.gov.kz/ru/contract/DEMO-0001", Valid: true},
		ImportedAt: ts,
		UpdatedAt:  ts,
	}
}

func TestContract_WireFormat_ValidatesAgainstOpenAPI(t *testing.T) {
	srv := httptest.NewServer(newRouter(mockStore{c: sampleContract()}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/contracts/DEMO-0001")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, ожидалось 200", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}

	// AC3: ответ валиден по OpenAPI-схеме Contract (kin-openapi)
	if err := loadSchema(t, "Contract").VisitJSON(body); err != nil {
		t.Fatalf("ответ не валиден по OpenAPI Contract: %v", err)
	}

	// AC2: деньги СТРОКОЙ + state ok
	amount, _ := body["amount_tng"].(map[string]any)
	if amount["state"] != "ok" {
		t.Fatalf("amount_tng.state = %v, ожидалось ok", amount["state"])
	}
	if _, isStr := amount["value"].(string); !isStr {
		t.Fatalf("amount_tng.value должно быть СТРОКОЙ, got %T (%v)", amount["value"], amount["value"])
	}
	// AC2: NULL → {value:null, state:no_data}
	subjKk, _ := body["subject_kk"].(map[string]any)
	if subjKk["state"] != "no_data" || subjKk["value"] != nil {
		t.Fatalf("subject_kk (NULL) → {value:null,state:no_data}, got %v", subjKk)
	}
}

func TestContract_WithActAndRaisedFlag_ValidatesAgainstOpenAPI(t *testing.T) {
	mkDate := func(s string) pgtype.Date {
		tt, _ := time.Parse("2006-01-02", s)
		return pgtype.Date{Time: tt, Valid: true}
	}
	store := mockStore{
		c:          sampleContract(),
		actPresent: true,
		act: gen.Act{
			ContractID:    1,
			GoszakupActID: "ACT-1",
			ActDate:       mkDate("2026-09-30"),
			SignerInfo:    pgtype.Text{String: "Руководитель отдела государственных закупок", Valid: true},
			SourceUrl:     pgtype.Text{String: "https://goszakup.gov.kz/ru/act/ACT-1", Valid: true},
		},
		// Только single_participant raised; price_per_km строки НЕТ → честный insufficient (AC4).
		flags: []gen.RiskFlag{activeFlag("single_participant", "v1.0", `{"participant_count":1}`)},
	}
	srv := httptest.NewServer(newRouter(store))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/contracts/DEMO-0001")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, ожидалось 200", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if err := loadSchema(t, "Contract").VisitJSON(body); err != nil {
		t.Fatalf("ответ с актом/флагами не валиден по OpenAPI Contract: %v", err)
	}

	// FR-11: акт присутствует, дата/подписант в state=ok
	act, _ := body["act"].(map[string]any)
	if act["present"] != true {
		t.Fatalf("act.present должно быть true, got %v", act["present"])
	}
	actDate, _ := act["act_date"].(map[string]any)
	if actDate["state"] != "ok" {
		t.Fatalf("act.act_date.state = %v, ожидалось ok", actDate["state"])
	}

	// AC3/AC4: single_participant raised (+evidence), price_per_km — НЕТ строки → insufficient (не «чисто»)
	flags, _ := body["flags"].([]any)
	if len(flags) != 2 {
		t.Fatalf("ожидалось 2 дескриптора флага (контракт-субъект), got %d", len(flags))
	}
	states := map[string]string{}
	for _, f := range flags {
		fm, _ := f.(map[string]any)
		id, _ := fm["flag_id"].(string)
		st, _ := fm["state"].(string)
		states[id] = st
	}
	if states["single_participant"] != "raised" {
		t.Fatalf("single_participant: ожидалось raised, got %q", states["single_participant"])
	}
	if states["price_per_km"] != "insufficient_data" {
		t.Fatalf("price_per_km без строки → insufficient_data (не not_raised/чисто), got %q", states["price_per_km"])
	}
}

func TestContract_AllNull_ValidatesAndNoData(t *testing.T) {
	ts := pgtype.Timestamptz{Time: time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC), Valid: true}
	// Все card-поля NULL (zero-value pgtype → Valid:false); imported/updated NOT NULL по схеме.
	c := gen.Contract{GoszakupContractID: "DEMO-NULL", ImportedAt: ts, UpdatedAt: ts}

	srv := httptest.NewServer(newRouter(mockStore{c: c}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/contracts/DEMO-NULL")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, ожидалось 200", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if err := loadSchema(t, "Contract").VisitJSON(body); err != nil {
		t.Fatalf("all-NULL ответ не валиден по OpenAPI Contract: %v", err)
	}
	for _, f := range []string{
		"subject_ru", "subject_kk", "amount_tng", "sign_date", "plan_start",
		"plan_end", "status", "direction", "kato_code", "source_url",
	} {
		obj, _ := body[f].(map[string]any)
		if obj["state"] != "no_data" || obj["value"] != nil {
			t.Fatalf("%s (NULL) → {value:null,state:no_data}, got %v", f, obj)
		}
	}
}

func TestContract_NotFound(t *testing.T) {
	srv := httptest.NewServer(newRouter(mockStore{err: pgx.ErrNoRows}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/contracts/NOPE")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, ожидалось 404", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if err := loadSchema(t, "Error").VisitJSON(body); err != nil {
		t.Fatalf("ответ ошибки не валиден по OpenAPI Error: %v", err)
	}
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "NOT_FOUND" {
		t.Fatalf("error.code = %v, ожидалось NOT_FOUND", errObj["code"])
	}
}
