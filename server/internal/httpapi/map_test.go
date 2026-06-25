package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"ashyqqala/server/internal/registry"
	"ashyqqala/server/internal/store/gen"
)

type mockLotsStore struct {
	rows []gen.ListLotsWithGeoRow
	err  error
}

func (m mockLotsStore) ListLotsWithGeo(context.Context) ([]gen.ListLotsWithGeoRow, error) {
	return m.rows, m.err
}

func newMapRouter(store LotsStore) http.Handler {
	r := chi.NewRouter()
	r.Get("/api/lots", MapLotsHandler{Store: store}.List)
	return r
}

// sampleLotRows — три честных случая: matched (auto+координата), unmatched (без координаты),
// и «нет гео-строки» (LEFT JOIN NULL → ещё не геокодирован).
func sampleLotRows() []gen.ListLotsWithGeoRow {
	txt := func(s string) pgtype.Text { return pgtype.Text{String: s, Valid: true} }
	return []gen.ListLotsWithGeoRow{
		{ // matched: lon=71.43, lat=51.13 (различимы → проверка порядка [lon,lat])
			GoszakupLotID: "scrape-matched",
			TitleRu:       txt("Реконструкция автодороги по ул. Абая"),
			TitleKk:       pgtype.Text{Valid: false}, // NULL → no_data
			Amount:        pgtype.Int8{Int64: 240000000, Valid: true},
			Lat:           pgtype.Float8{Float64: 51.13, Valid: true},
			Lon:           pgtype.Float8{Float64: 71.43, Valid: true},
			GeocodeStatus: txt("auto"),
		},
		{ // unmatched: координаты NULL, статус есть
			GoszakupLotID: "scrape-unmatched",
			TitleRu:       txt("Ремонт сетей водоснабжения"),
			Amount:        pgtype.Int8{Int64: 5000000, Valid: true},
			Lat:           pgtype.Float8{Valid: false},
			Lon:           pgtype.Float8{Valid: false},
			GeocodeStatus: txt("unmatched"),
		},
		{ // нет гео-строки (LEFT JOIN NULL): geocode_status невалиден
			GoszakupLotID: "scrape-pending",
			TitleRu:       txt("Содержание автодорог"),
			Amount:        pgtype.Int8{Valid: false}, // NULL сумма → no_data
			GeocodeStatus: pgtype.Text{Valid: false},
		},
	}
}

func TestMapLots_WireFormat_ValidatesAgainstOpenAPI(t *testing.T) {
	srv := httptest.NewServer(newMapRouter(mockLotsStore{rows: sampleLotRows()}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/lots")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, ожидалось 200", resp.StatusCode)
	}

	var body []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body) != 3 {
		t.Fatalf("ожидалось 3 лота, got %d", len(body))
	}

	schema := loadSchema(t, "MapLot")
	for i, item := range body {
		if err := schema.VisitJSON(item); err != nil {
			t.Fatalf("лот %d не валиден по OpenAPI MapLot: %v", i, err)
		}
	}

	// matched: точка присутствует, порядок [lon,lat] (lon=71.43 ≠ lat=51.13), state ok
	m := body[0]
	if m["geocode_state"] != "ok" {
		t.Fatalf("matched.geocode_state = %v, ожидалось ok", m["geocode_state"])
	}
	if lon, _ := m["lon"].(float64); lon != 71.43 {
		t.Fatalf("matched.lon = %v, ожидалось 71.43 (порядок [lon,lat])", m["lon"])
	}
	if lat, _ := m["lat"].(float64); lat != 51.13 {
		t.Fatalf("matched.lat = %v, ожидалось 51.13", m["lat"])
	}
	// сумма СТРОКОЙ; subject_kk NULL → no_data
	amount, _ := m["amount_tng"].(map[string]any)
	if _, isStr := amount["value"].(string); !isStr || amount["state"] != "ok" {
		t.Fatalf("amount_tng должно быть {value:string,state:ok}, got %v", amount)
	}
	subjKk, _ := m["subject_kk"].(map[string]any)
	if subjKk["state"] != "no_data" || subjKk["value"] != nil {
		t.Fatalf("subject_kk (NULL) → {value:null,state:no_data}, got %v", subjKk)
	}

	// unmatched: координаты null (НЕ 0,0), state geocode_failed
	u := body[1]
	if u["geocode_state"] != "geocode_failed" {
		t.Fatalf("unmatched.geocode_state = %v, ожидалось geocode_failed", u["geocode_state"])
	}
	if u["lon"] != nil || u["lat"] != nil {
		t.Fatalf("unmatched координаты должны быть null (не 0,0), got lon=%v lat=%v", u["lon"], u["lat"])
	}

	// нет гео-строки: state geocode_pending, координаты null
	p := body[2]
	if p["geocode_state"] != "geocode_pending" {
		t.Fatalf("no-geo-row.geocode_state = %v, ожидалось geocode_pending", p["geocode_state"])
	}
	if p["lon"] != nil || p["lat"] != nil {
		t.Fatalf("pending координаты должны быть null, got lon=%v lat=%v", p["lon"], p["lat"])
	}
	amountP, _ := p["amount_tng"].(map[string]any)
	if amountP["state"] != "no_data" || amountP["value"] != nil {
		t.Fatalf("pending.amount_tng (NULL) → no_data, got %v", amountP)
	}
}

func TestMapLots_NonFiniteCoord_HonestNotBrokenResponse(t *testing.T) {
	// auto-статус с NaN/Inf координатой (БД double precision это допускает, CHECK не ловит) НЕ должен
	// ломать весь ответ: json.Encode падает на NaN/Inf уже после WriteHeader(200) → обрезанный JSON.
	// Гард конечности → честный geocode_failed без координаты, ответ остаётся валидным.
	rows := []gen.ListLotsWithGeoRow{{
		GoszakupLotID: "scrape-nan",
		TitleRu:       pgtype.Text{String: "Лот с битой координатой", Valid: true},
		Amount:        pgtype.Int8{Int64: 1, Valid: true},
		Lat:           pgtype.Float8{Float64: math.NaN(), Valid: true},
		Lon:           pgtype.Float8{Float64: math.Inf(1), Valid: true},
		GeocodeStatus: pgtype.Text{String: "auto", Valid: true},
	}}
	srv := httptest.NewServer(newMapRouter(mockLotsStore{rows: rows}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/lots")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, ожидалось 200", resp.StatusCode)
	}
	var body []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("ответ должен оставаться валидным JSON (NaN/Inf не ломают кодирование): %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("ожидался 1 лот, got %d", len(body))
	}
	m := body[0]
	if m["geocode_state"] != "geocode_failed" {
		t.Fatalf("NaN/Inf координата → geocode_failed, got %v", m["geocode_state"])
	}
	if m["lon"] != nil || m["lat"] != nil {
		t.Fatalf("битая координата → null (НЕ NaN/Inf/0,0), got lon=%v lat=%v", m["lon"], m["lat"])
	}
	if err := loadSchema(t, "MapLot").VisitJSON(m); err != nil {
		t.Fatalf("ответ не валиден по OpenAPI MapLot: %v", err)
	}
}

// TestMapLots_AutoWithoutCoords_HonestGeocodeFailed (P9) закрывает дыру в покрытии: status=auto, но
// координаты NULL (нарушен инвариант Story 0.7 «auto ⇒ coordinates present»). Ответ обязан остаться
// корректным, а состояние — честным geocode_failed без точки (PATCH A эмитит WARN, но wire-вывод тот же).
// Под-кейсы: обе координаты NULL и одна NULL (БД CHECK обычно ловит «одна из двух», но защищаемся всё равно).
func TestMapLots_AutoWithoutCoords_HonestGeocodeFailed(t *testing.T) {
	txt := func(s string) pgtype.Text { return pgtype.Text{String: s, Valid: true} }
	cases := []struct {
		name string
		lat  pgtype.Float8
		lon  pgtype.Float8
	}{
		{"both-null", pgtype.Float8{Valid: false}, pgtype.Float8{Valid: false}},
		{"lat-null-only", pgtype.Float8{Valid: false}, pgtype.Float8{Float64: 71.43, Valid: true}},
		{"lon-null-only", pgtype.Float8{Float64: 51.13, Valid: true}, pgtype.Float8{Valid: false}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows := []gen.ListLotsWithGeoRow{{
				GoszakupLotID: "scrape-auto-no-coords",
				TitleRu:       txt("Лот auto без координат (нарушен инвариант)"),
				Amount:        pgtype.Int8{Int64: 1, Valid: true},
				Lat:           tc.lat,
				Lon:           tc.lon,
				GeocodeStatus: txt("auto"),
			}}
			srv := httptest.NewServer(newMapRouter(mockLotsStore{rows: rows}))
			defer srv.Close()

			resp, err := http.Get(srv.URL + "/api/lots")
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, ожидалось 200", resp.StatusCode)
			}
			var body []map[string]any
			if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
				t.Fatalf("ответ должен оставаться валидным JSON: %v", err)
			}
			if len(body) != 1 {
				t.Fatalf("ожидался 1 лот, got %d", len(body))
			}
			m := body[0]
			if m["geocode_state"] != "geocode_failed" {
				t.Fatalf("auto без координат → честный geocode_failed, got %v", m["geocode_state"])
			}
			if m["lon"] != nil || m["lat"] != nil {
				t.Fatalf("auto без координат → точка null (НЕ 0,0), got lon=%v lat=%v", m["lon"], m["lat"])
			}
			if err := loadSchema(t, "MapLot").VisitJSON(m); err != nil {
				t.Fatalf("ответ не валиден по OpenAPI MapLot: %v", err)
			}
		})
	}
}

// TestMapLot_GeocodeStates_MatchOpenAPIEnum (P5, серверная сторона) — перекрёстный гард: три состояния,
// которые эндпоинт реально способен отдать, обязаны сериализоваться РОВНО в три литерала из OpenAPI-enum
// MapLot.geocode_state. Ловит дрейф «Go-тип шире контракта»: если wire-значение константы registry
// когда-нибудь разойдётся с enum, тест краснеет. GeocodeState маршалится как обычная строка
// (registry.ValueState — это string), поэтому JSON-литерал и есть кавычки вокруг wire-значения.
func TestMapLot_GeocodeStates_MatchOpenAPIEnum(t *testing.T) {
	// Литералы зафиксированы из docs/api-contracts/openapi.yaml → MapLot.geocode_state.enum (PATCH D).
	want := map[registry.ValueState]string{
		registry.StateOK:             `"ok"`,
		registry.StateGeocodePending: `"geocode_pending"`,
		registry.StateGeocodeFailed:  `"geocode_failed"`,
	}
	for state, lit := range want {
		got, err := json.Marshal(MapLot{GeocodeState: state}.GeocodeState)
		if err != nil {
			t.Fatalf("marshal %q: %v", state, err)
		}
		if string(got) != lit {
			t.Fatalf("registry-состояние %q сериализуется в %s, а OpenAPI-enum ждёт %s (Go-тип разошёлся с контрактом)",
				state, got, lit)
		}
	}
}

func TestMapLots_Empty_IsEmptyArrayNotNull(t *testing.T) {
	srv := httptest.NewServer(newMapRouter(mockLotsStore{rows: []gen.ListLotsWithGeoRow{}}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/lots")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, ожидалось 200", resp.StatusCode)
	}
	var body []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body == nil {
		t.Fatal("пустой результат должен быть [] (не null) — честная деградация контейнера на фронте")
	}
	if len(body) != 0 {
		t.Fatalf("ожидался пустой массив, got %d", len(body))
	}
}

func TestMapLots_StoreError_500Envelope(t *testing.T) {
	srv := httptest.NewServer(newMapRouter(mockLotsStore{err: errors.New("db down")}))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/lots")
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
	if err := loadSchema(t, "Error").VisitJSON(body); err != nil {
		t.Fatalf("ответ ошибки не валиден по OpenAPI Error: %v", err)
	}
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "INTERNAL" {
		t.Fatalf("error.code = %v, ожидалось INTERNAL", errObj["code"])
	}
}
