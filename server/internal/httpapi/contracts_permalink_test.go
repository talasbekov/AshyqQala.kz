package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

// newRouterVersioned — роутер карточки с заданной канонической methodology_version (AR-29 штамп/дрейф).
func newRouterVersioned(store ContractStore, version string) http.Handler {
	r := chi.NewRouter()
	r.Get("/api/contracts/{goszakup_id}", ContractsHandler{Store: store, Version: version}.Get)
	return r
}

// TestContract_Permalink_MethodologyStampAndDrift — AR-29 (Story 5.6): карточка несёт штамп текущей версии
// методики + as_of для построения перманентной ссылки; ссылка под ИНОЙ версией → честный дрейф-сигнал;
// совпадение версии / без штампа → дрейфа нет (negative-controls — страж краснеет ТОЛЬКО по причине смены методики).
func TestContract_Permalink_MethodologyStampAndDrift(t *testing.T) {
	srv := httptest.NewServer(newRouterVersioned(mockStore{c: sampleContract()}, "v1.0"))
	defer srv.Close()

	get := func(qs string) map[string]any {
		t.Helper()
		resp, err := http.Get(srv.URL + "/api/contracts/DEMO-0001" + qs)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status=%d; want 200", resp.StatusCode)
		}
		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if err := loadSchema(t, "Contract").VisitJSON(body); err != nil {
			t.Fatalf("ответ не валиден по OpenAPI Contract: %v", err)
		}
		return body
	}

	// Штамп текущей версии методики (для построения перманентной ссылки клиентом).
	base := get("")
	mv, _ := base["methodology_version"].(map[string]any)
	if mv["state"] != "ok" || mv["value"] != "v1.0" {
		t.Fatalf("methodology_version штамп = %v; want {state:ok, value:v1.0}", mv)
	}
	// as_of — дата-штамп: нет raised-флагов → updated_at, state ok.
	asof, _ := base["as_of"].(map[string]any)
	if asof["state"] != "ok" {
		t.Fatalf("as_of должен нести дату-штамп (updated_at) при state ok, got %v", asof)
	}
	// negative-control: без ?mv дрейфа нет.
	if drift, _ := base["methodology_drift"].(map[string]any); drift["present"] != false {
		t.Fatalf("без ?mv дрейфа быть не должно, got %v", drift)
	}

	// negative-control: ?mv == текущей → дрейфа нет (совпадение НЕ краснеет).
	same := get("?mv=v1.0")
	if drift, _ := same["methodology_drift"].(map[string]any); drift["present"] != false {
		t.Fatalf("?mv=v1.0 (== текущей) → дрейфа быть не должно, got %v", drift)
	}

	// ПОЛОЖИТЕЛЬНЫЙ: ?mv под ИНОЙ версией → честный дрейф (страж краснеет по ПРИЧИНЕ — сменилась методика).
	drifted := get("?mv=v0.9")
	drift, _ := drifted["methodology_drift"].(map[string]any)
	if drift["present"] != true || drift["requested_version"] != "v0.9" || drift["current_version"] != "v1.0" {
		t.Fatalf("?mv=v0.9 → дрейф {present:true, requested:v0.9, current:v1.0}, got %v", drift)
	}
}

// TestContract_Permalink_EmptyVersion_NoFabrication — honesty-hole 5.1: пустая каноническая версия НЕ
// штампуется (methodology_version → no_data, НЕ «ok» с пустым value), и ?mv не порождает ложный дрейф
// (current пуст → дрейф не выставляется). Negative-control honesty-конверта.
func TestContract_Permalink_EmptyVersion_NoFabrication(t *testing.T) {
	srv := httptest.NewServer(newRouterVersioned(mockStore{c: sampleContract()}, ""))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/contracts/DEMO-0001?mv=v0.9")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if err := loadSchema(t, "Contract").VisitJSON(body); err != nil {
		t.Fatalf("ответ не валиден по OpenAPI Contract: %v", err)
	}
	mv, _ := body["methodology_version"].(map[string]any)
	if mv["state"] != "no_data" || mv["value"] != nil {
		t.Fatalf("пустая версия НЕ должна штамповаться: methodology_version = %v; want {no_data, null}", mv)
	}
	// Пустая current-версия → дрейф не фабрикуется (нельзя честно сравнить с неизвестной текущей).
	if drift, _ := body["methodology_drift"].(map[string]any); drift["present"] != false {
		t.Fatalf("при пустой текущей версии дрейф не выставляется, got %v", drift)
	}
}
