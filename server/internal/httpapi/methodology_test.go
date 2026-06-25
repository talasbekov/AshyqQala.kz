package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"ashyqqala/server/internal/flags"
)

func sampleParams() flags.Params {
	return flags.Params{
		MethodologyVersion:              "v1.0",
		MinSample:                       5,
		ComparabilityWindowMonths:       24,
		PricePerKMDeviationFactor:       1.5,
		MonopolyConcentrationShare:      0.5,
		MonopolyMinGroupContracts:       5,
		SingleParticipantEnabled:        true,
		SingleParticipantExcludeMethods: []string{"из_одного_источника"},
		RNUEnabled:                      true,
	}
}

func TestMethodology_WireFormat_ValidatesAgainstOpenAPI(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/api/methodology", MethodologyHandler{Params: sampleParams()}.Get)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/methodology")
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
	if err := loadSchema(t, "Methodology").VisitJSON(body); err != nil {
		t.Fatalf("ответ методики не валиден по OpenAPI Methodology: %v", err)
	}

	// Пороги == methodology_params (единый источник): несколько ключевых значений.
	if body["version"] != "v1.0" {
		t.Fatalf("version = %v, ожидалось v1.0", body["version"])
	}
	th, _ := body["thresholds"].(map[string]any)
	if th["min_sample"].(float64) != 5 {
		t.Fatalf("min_sample = %v, ожидалось 5", th["min_sample"])
	}
	if th["price_per_km_deviation_factor"].(float64) != 1.5 {
		t.Fatalf("deviation_factor = %v, ожидалось 1.5", th["price_per_km_deviation_factor"])
	}
	if th["monopoly_min_group_contracts"].(float64) != 5 {
		t.Fatalf("min_group_contracts = %v, ожидалось 5", th["monopoly_min_group_contracts"])
	}
}
