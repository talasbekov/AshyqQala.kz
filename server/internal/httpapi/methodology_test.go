package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
	// review-фикс 5.3: проверяем ВСЕ 6 порогов (раньше окно/доля/способы не сверялись).
	if th["comparability_window_months"].(float64) != 24 {
		t.Fatalf("comparability_window_months = %v, ожидалось 24", th["comparability_window_months"])
	}
	if th["monopoly_concentration_share"].(float64) != 0.5 {
		t.Fatalf("monopoly_concentration_share = %v, ожидалось 0.5", th["monopoly_concentration_share"])
	}
	methods, ok := th["single_participant_exclude_methods"].([]any)
	if !ok || len(methods) != 1 || methods[0] != "из_одного_источника" {
		t.Fatalf("single_participant_exclude_methods = %v, ожидался [из_одного_источника]", th["single_participant_exclude_methods"])
	}
}

// TestMethodology_NilExcludeMethods_SerializesAsEmptyArray — nil-слайс способов исключения ДОЛЖЕН отдаваться
// как `[]`, а не `null` (review-фикс 5.3). Negative-control: до коэрсии sample-фикстура (non-nil) маскировала
// баг — OpenAPI required `array` ловит `null`. [[guards-must-prove-red]]
func TestMethodology_NilExcludeMethods_SerializesAsEmptyArray(t *testing.T) {
	p := sampleParams()
	p.SingleParticipantExcludeMethods = nil // конфиг без исключений (пустой/отсутствующий список)

	r := chi.NewRouter()
	r.Get("/api/methodology", MethodologyHandler{Params: p}.Get)
	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/methodology")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	// Сырой JSON: поле — [], НЕ null (negative-control: без фикса было бы `"...":null`).
	if !strings.Contains(string(raw), `"single_participant_exclude_methods":[]`) {
		t.Fatalf("nil exclude_methods должно сериализоваться как [], получено: %s", raw)
	}
	if strings.Contains(string(raw), `"single_participant_exclude_methods":null`) {
		t.Fatalf("nil exclude_methods НЕ должно быть null, получено: %s", raw)
	}
	// И по-прежнему валидно по OpenAPI (required `array`): `null` бы это провалил.
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if err := loadSchema(t, "Methodology").VisitJSON(body); err != nil {
		t.Fatalf("nil-кейс не валиден по OpenAPI Methodology: %v", err)
	}
}
