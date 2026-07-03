package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"ashyqqala/server/internal/store"
)

// mockMapObjectsStore — мок узкого интерфейса MapObjectsStore (Story 3.4).
type mockMapObjectsStore struct {
	points    []store.GeoObjectPoint
	truncated bool
	count     int64
	listErr   error
	countErr  error
	// gotLimit фиксирует, что хендлер передал cap (не 0/не мусор).
	gotLimit int
}

func (m *mockMapObjectsStore) ListGeoObjectsInBBox(_ context.Context, minLon, minLat, maxLon, maxLat float64, limit int) ([]store.GeoObjectPoint, bool, error) {
	m.gotLimit = limit
	return m.points, m.truncated, m.listErr
}

func (m *mockMapObjectsStore) CountContractsWithoutPoint(context.Context) (int64, error) {
	return m.count, m.countErr
}

func newMapObjectsRouter(s MapObjectsStore) http.Handler {
	r := chi.NewRouter()
	r.Get("/api/map/objects", MapObjectsHandler{Store: s}.List)
	return r
}

func strPtr(s string) *string   { return &s }
func f64Ptr(f float64) *float64 { return &f }

// sampleGeoPoints — точка с флагом, линия verified с длиной, строка без контракта (goszakup NULL).
// lon≠lat в геометрии — различимость порядка [lon,lat] (AR-19).
func sampleGeoPoints() []store.GeoObjectPoint {
	return []store.GeoObjectPoint{
		{
			PublicID:           "0198a3b0-0000-7000-8000-000000000001",
			GoszakupContractID: strPtr("DEMO-GEO-01"),
			GeocodeStatus:      "auto",
			HasActiveFlag:      true,
			Geom:               json.RawMessage(`{"type":"Point","coordinates":[71.43,51.13]}`),
		},
		{
			PublicID:           "0198a3b0-0000-7000-8000-000000000002",
			GoszakupContractID: strPtr("DEMO-GEO-02"),
			GeocodeStatus:      "verified",
			HasActiveFlag:      false,
			LengthKm:           f64Ptr(5.0),
			Geom:               json.RawMessage(`{"type":"LineString","coordinates":[[71.40,51.10],[71.45,51.14]]}`),
		},
		{
			PublicID:      "0198a3b0-0000-7000-8000-000000000003",
			GeocodeStatus: "manual",
			Geom:          json.RawMessage(`{"type":"Point","coordinates":[71.50,51.20]}`),
		},
	}
}

func TestMapObjects_WireFormat_ValidatesAgainstOpenAPI(t *testing.T) {
	mock := &mockMapObjectsStore{points: sampleGeoPoints(), truncated: true, count: 7}
	srv := httptest.NewServer(newMapObjectsRouter(mock))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/map/objects?bbox=71.20,51.00,71.78,51.30")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, ожидалось 200", resp.StatusCode)
	}
	if mock.gotLimit <= 0 {
		t.Fatalf("хендлер передал limit=%d, ожидался положительный cap", mock.gotLimit)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	// Весь ответ валиден по OpenAPI-арбитру (включая вложенные MapObject).
	schema := loadSchema(t, "MapObjectsResponse")
	if err := schema.VisitJSON(body); err != nil {
		t.Fatalf("ответ не валиден по OpenAPI MapObjectsResponse: %v", err)
	}

	if body["truncated"] != true {
		t.Errorf("truncated = %v, ожидалось true (честная детекция cap'а)", body["truncated"])
	}
	if n, _ := body["ungeocoded_count"].(float64); n != 7 {
		t.Errorf("ungeocoded_count = %v, ожидалось 7 (AC3)", body["ungeocoded_count"])
	}

	items, _ := body["items"].([]any)
	if len(items) != 3 {
		t.Fatalf("items = %d, ожидалось 3", len(items))
	}
	first, _ := items[0].(map[string]any)
	if first["has_active_flag"] != true {
		t.Errorf("has_active_flag = %v, ожидалось true (AC2: амбер-кольцо кластера)", first["has_active_flag"])
	}
	geom, _ := first["geom"].(map[string]any)
	coords, _ := geom["coordinates"].([]any)
	if len(coords) != 2 || coords[0] != 71.43 || coords[1] != 51.13 {
		t.Errorf("geom.coordinates = %v, ожидалось [71.43,51.13] ([lon,lat], AR-19)", coords)
	}
	// Строка без контракта: goszakup_contract_id = null честно (не пустая строка).
	third, _ := items[2].(map[string]any)
	if v, present := third["goszakup_contract_id"]; !present || v != nil {
		t.Errorf("goszakup_contract_id = %v (present=%v), ожидался null", v, present)
	}
	if third["length_km"] != nil {
		t.Errorf("length_km = %v у точки, ожидался null (у точки нет длины)", third["length_km"])
	}
}

func TestMapObjects_BBoxValidation_Returns400(t *testing.T) {
	mock := &mockMapObjectsStore{}
	srv := httptest.NewServer(newMapObjectsRouter(mock))
	defer srv.Close()

	errSchema := loadSchema(t, "Error")
	cases := []struct {
		name string
		qs   string
	}{
		{"bbox отсутствует", ""},
		{"три координаты", "?bbox=71.20,51.00,71.78"},
		{"не число", "?bbox=71.20,abc,71.78,51.30"},
		{"NaN парсится ParseFloat — ловит ValidateBBox", "?bbox=NaN,51.00,71.78,51.30"},
		{"инвертированный", "?bbox=71.78,51.00,71.20,51.30"},
		{"вне WGS84", "?bbox=-200,51.00,71.78,51.30"},
		{"вырожденный (min==max)", "?bbox=71.20,51.00,71.20,51.30"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp, err := http.Get(srv.URL + "/api/map/objects" + c.qs)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, ожидалось 400 (валидация bbox — ввод клиента, не 500)", resp.StatusCode)
			}
			var body map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if err := errSchema.VisitJSON(body); err != nil {
				t.Fatalf("тело 400 не валидно по OpenAPI Error: %v", err)
			}
			errObj, _ := body["error"].(map[string]any)
			if errObj["code"] != "VALIDATION_FAILED" {
				t.Errorf("code = %v, ожидалось VALIDATION_FAILED", errObj["code"])
			}
			// Публичный detail без внутренней пакетной номенклатуры (код-ревью 3.4).
			if msg, _ := errObj["message"].(string); strings.HasPrefix(msg, "store: ") {
				t.Errorf("message = %q — внутренний префикс «store: » утёк в публичный API-конверт", msg)
			}
		})
	}
}

func TestMapObjects_EmptyBBox_ReturnsEmptyArrayNotNull(t *testing.T) {
	mock := &mockMapObjectsStore{points: []store.GeoObjectPoint{}, count: 0}
	srv := httptest.NewServer(newMapObjectsRouter(mock))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/map/objects?bbox=10.0,10.0,11.0,11.0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if string(body["items"]) != "[]" {
		t.Errorf(`items = %s, ожидалось [] (не null — честная пустая коллекция)`, body["items"])
	}
}

func TestMapObjects_StoreErrors_Return500ErrorEnvelope(t *testing.T) {
	errSchema := loadSchema(t, "Error")
	for name, mock := range map[string]*mockMapObjectsStore{
		"ошибка bbox-ридера": {listErr: errors.New("db down")},
		"ошибка счётчика":    {points: []store.GeoObjectPoint{}, countErr: errors.New("db down")},
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(newMapObjectsRouter(mock))
			defer srv.Close()
			resp, err := http.Get(srv.URL + "/api/map/objects?bbox=71.20,51.00,71.78,51.30")
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusInternalServerError {
				t.Fatalf("status = %d, ожидалось 500", resp.StatusCode)
			}
			var body map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if err := errSchema.VisitJSON(body); err != nil {
				t.Fatalf("тело 500 не валидно по OpenAPI Error: %v", err)
			}
		})
	}
}
