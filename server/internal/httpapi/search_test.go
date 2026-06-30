package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"ashyqqala/server/internal/store/gen"
)

// mockSearchStore — фейковый SearchStore: канонные строки/ошибки + захват params (для проверки роутинга
// БИН-ветка vs имя-ветка и прокидки лимита). Реальную trgm-фильтрацию проверяет integration-тест; здесь —
// валидация/роутинг/wire/слияние.
type mockSearchStore struct {
	orgs              []gen.SearchOrganizationsRow
	contracts         []gen.SearchContractsRow
	orgErr            error
	contractErr       error
	gotOrgParams      gen.SearchOrganizationsParams
	gotContractParams gen.SearchContractsParams
	orgCalled         bool
	contractCalled    bool
}

func (m *mockSearchStore) SearchOrganizations(_ context.Context, arg gen.SearchOrganizationsParams) ([]gen.SearchOrganizationsRow, error) {
	m.orgCalled = true
	m.gotOrgParams = arg
	if m.orgErr != nil {
		return nil, m.orgErr
	}
	return m.orgs, nil
}

func (m *mockSearchStore) SearchContracts(_ context.Context, arg gen.SearchContractsParams) ([]gen.SearchContractsRow, error) {
	m.contractCalled = true
	m.gotContractParams = arg
	if m.contractErr != nil {
		return nil, m.contractErr
	}
	return m.contracts, nil
}

func newSearchRouter(store SearchStore) http.Handler {
	r := chi.NewRouter()
	r.Get("/api/search", SearchHandler{Store: store}.Search)
	return r
}

func getSearch(t *testing.T, store SearchStore, qs string) (*http.Response, []byte) {
	t.Helper()
	srv := httptest.NewServer(newSearchRouter(store))
	t.Cleanup(srv.Close)
	resp, err := http.Get(srv.URL + "/api/search" + qs)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return resp, body
}

func mkOrgRow(bin, nameRu string) gen.SearchOrganizationsRow {
	return gen.SearchOrganizationsRow{
		ID:      1,
		Bin:     bin,
		NameRu:  pgtype.Text{String: nameRu, Valid: true},
		NameKk:  pgtype.Text{Valid: false}, // NULL → no_data
		RegKato: pgtype.Text{String: "710000000", Valid: true},
	}
}

func mkSearchContractRow(gid string, flag bool) gen.SearchContractsRow {
	return gen.SearchContractsRow{
		GoszakupContractID: gid,
		SubjectRu:          pgtype.Text{String: "Ремонт автодороги " + gid, Valid: true},
		SubjectKk:          pgtype.Text{Valid: false},
		AmountTng:          pgtype.Int8{Int64: 240000000, Valid: true},
		Status:             pgtype.Text{String: "active", Valid: true},
		Direction:          pgtype.Text{String: "road", Valid: true},
		KatoCode:           pgtype.Text{String: "710000000", Valid: true},
		HasActiveFlag:      flag,
	}
}

// TestSearch_WireFormat — ветка имени возвращает организации + контракты; ответ валиден против SearchResponse
// (oneOf-дискриминатор kind), деньги строкой, nullable — честный конверт, has_geo честно false.
func TestSearch_WireFormat_ValidatesAgainstOpenAPI(t *testing.T) {
	store := &mockSearchStore{
		orgs:      []gen.SearchOrganizationsRow{mkOrgRow("123456789012", "ТОО Жол Курылыс")},
		contracts: []gen.SearchContractsRow{mkSearchContractRow("DEMO-0002", true)},
	}
	resp, body := getSearch(t, store, "?q=%D0%B6%D0%BE%D0%BB") // q=жол
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, ожидалось 200: %s", resp.StatusCode, body)
	}
	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err != nil {
		t.Fatal(err)
	}
	if err := loadSchema(t, "SearchResponse").VisitJSON(obj); err != nil {
		t.Fatalf("ответ не валиден против SearchResponse: %v\n%s", err, body)
	}
	items := obj["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("ожидалось 2 элемента (орг+контракт), получено %d: %s", len(items), body)
	}
	// Организации раньше контрактов (детерминированный порядок слияния).
	first := items[0].(map[string]any)
	if first["kind"] != kindOrganization {
		t.Errorf("первый элемент должен быть organization, получено %v", first["kind"])
	}
	if first["bin"] != "123456789012" {
		t.Errorf("bin организации = %v", first["bin"])
	}
	if first["has_geo"] != false {
		t.Errorf("has_geo организации должен быть false (гео Epic 3 не наполнено), получено %v", first["has_geo"])
	}
	// name_kk NULL → честный no_data.
	nameKk := first["name_kk"].(map[string]any)
	if nameKk["state"] != "no_data" || nameKk["value"] != nil {
		t.Errorf("NULL name_kk должен быть no_data/null, получено %v", nameKk)
	}
	second := items[1].(map[string]any)
	if second["kind"] != kindContract {
		t.Errorf("второй элемент должен быть contract, получено %v", second["kind"])
	}
	if second["has_active_flag"] != true {
		t.Errorf("has_active_flag контракта должен быть true")
	}
}

// TestSearch_BinBranch — валидный 12-зн БИН → точный матч организации; контракты НЕ ищутся; BinExact задан.
func TestSearch_BinBranch_ExactMatch(t *testing.T) {
	store := &mockSearchStore{orgs: []gen.SearchOrganizationsRow{mkOrgRow("123456789012", "ТОО Тест")}}
	resp, body := getSearch(t, store, "?q=123456789012")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d: %s", resp.StatusCode, body)
	}
	if !store.orgCalled {
		t.Fatal("SearchOrganizations должен быть вызван")
	}
	if store.contractCalled {
		t.Error("SearchContracts НЕ должен вызываться для БИН-ветки (БИН — id организации)")
	}
	if !store.gotOrgParams.BinExact.Valid || store.gotOrgParams.BinExact.String != "123456789012" {
		t.Errorf("BinExact должен быть {123456789012, valid}, получено %v", store.gotOrgParams.BinExact)
	}
	var out SearchResponse
	_ = json.Unmarshal(body, &out)
	if len(out.Items) != 1 {
		t.Fatalf("ожидался 1 результат (орг по точному БИН), получено %d", len(out.Items))
	}
}

// negative control для БИН-роутинга: 11 цифр / мусор → CanonicalBIN="" → ветка ИМЕНИ (BinExact NULL, оба
// запроса). Доказывает, что БИН-ветка реально гейтится валидностью 12-значного БИН, а не «любые цифры».
func TestSearch_NameBranch_RoutesBoth(t *testing.T) {
	t.Run("текст → имя-ветка: оба запроса, BinExact NULL", func(t *testing.T) {
		store := &mockSearchStore{}
		resp, _ := getSearch(t, store, "?q=%D0%B6%D0%BE%D0%BB") // жол
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		if !store.orgCalled || !store.contractCalled {
			t.Errorf("ветка имени должна звать оба запроса: org=%v contract=%v", store.orgCalled, store.contractCalled)
		}
		if store.gotOrgParams.BinExact.Valid {
			t.Errorf("BinExact в ветке имени должен быть NULL, получено %v", store.gotOrgParams.BinExact)
		}
		if store.gotOrgParams.Q != "жол" || store.gotContractParams.Q != "жол" {
			t.Errorf("Q должен прокинуться как 'жол', получено org=%q contract=%q", store.gotOrgParams.Q, store.gotContractParams.Q)
		}
	})
	t.Run("11 цифр → НЕ БИН → имя-ветка (negative control БИН-гейта)", func(t *testing.T) {
		store := &mockSearchStore{}
		_, _ = getSearch(t, store, "?q=12345678901") // 11 цифр
		if store.gotOrgParams.BinExact.Valid {
			t.Error("11 цифр не валидный БИН → BinExact должен быть NULL (ветка имени)")
		}
		if !store.contractCalled {
			t.Error("11 цифр → ветка имени → SearchContracts должен вызваться")
		}
	})
}

func TestSearch_InvalidQuery_400(t *testing.T) {
	cases := []struct{ name, qs string }{
		{"пустой q", ""},
		{"q только пробелы", "?q=%20%20"},
		{"q короче минимума (2 буквы)", "?q=%D0%B6%D0%BE"}, // жо
		{"q короче минимума (2 цифры, не БИН)", "?q=12"},
		{"q длиннее максимума (101 символ)", "?q=" + strings.Repeat("%D0%B6", 101)}, // 101 «ж» > 100 рун

		{"limit ноль", "?q=%D0%B6%D0%BE%D0%BB&limit=0"},
		{"limit свыше max", "?q=%D0%B6%D0%BE%D0%BB&limit=101"},
		{"limit не число", "?q=%D0%B6%D0%BE%D0%BB&limit=x"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			store := &mockSearchStore{}
			resp, body := getSearch(t, store, c.qs)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, ожидалось 400 (%s): %s", resp.StatusCode, c.name, body)
			}
			var env map[string]struct {
				Code string `json:"code"`
			}
			_ = json.Unmarshal(body, &env)
			if env["error"].Code != "VALIDATION_FAILED" {
				t.Errorf("code = %q, ожидалось VALIDATION_FAILED", env["error"].Code)
			}
			if err := loadSchema(t, "Error").VisitJSON(jsonAny(t, body)); err != nil {
				t.Errorf("ошибка не валидна против Error: %v", err)
			}
		})
	}
}

// negative control: валидный запрос (ровно 3 буквы — граница min-длины) НЕ должен давать 400 — иначе тест 400
// выше прошёл бы мимо (любой запрос 400). Также не зовём БИН-ветку для короткого имени.
func TestSearch_ValidMinLength_Not400(t *testing.T) {
	store := &mockSearchStore{}
	resp, body := getSearch(t, store, "?q=%D0%B6%D0%BE%D0%BB") // жол — ровно 3 символа
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("3-символьный запрос дал %d: %s", resp.StatusCode, body)
	}
}

func TestSearch_Empty_IsEmptyArrayNotNull(t *testing.T) {
	store := &mockSearchStore{orgs: nil, contracts: nil}
	resp, body := getSearch(t, store, "?q=%D0%B6%D0%BE%D0%BB")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), `"items":[]`) {
		t.Errorf("ожидался items:[] (не null): %s", body)
	}
	if !strings.Contains(string(body), `"next_cursor":null`) {
		t.Errorf("ожидался next_cursor:null: %s", body)
	}
	var obj map[string]any
	_ = json.Unmarshal(body, &obj)
	if err := loadSchema(t, "SearchResponse").VisitJSON(obj); err != nil {
		t.Fatalf("пустой ответ не валиден: %v", err)
	}
}

// TestSearch_MergeOrderAndTruncation — round-robin слияние (орг, контракт, орг, …), общий размер ≤ limit.
func TestSearch_MergeOrderAndTruncation(t *testing.T) {
	store := &mockSearchStore{
		orgs:      []gen.SearchOrganizationsRow{mkOrgRow("100000000001", "Орг A"), mkOrgRow("100000000002", "Орг B")},
		contracts: []gen.SearchContractsRow{mkSearchContractRow("C1", false), mkSearchContractRow("C2", false)},
	}
	_, body := getSearch(t, store, "?q=%D0%B6%D0%BE%D0%BB&limit=3") // limit=3 → round-robin: org,contract,org
	var obj map[string]any
	_ = json.Unmarshal(body, &obj)
	items := obj["items"].([]any)
	if len(items) != 3 {
		t.Fatalf("ожидалось 3 (усечение до limit), получено %d: %s", len(items), body)
	}
	kinds := []string{
		items[0].(map[string]any)["kind"].(string),
		items[1].(map[string]any)["kind"].(string),
		items[2].(map[string]any)["kind"].(string),
	}
	if kinds[0] != kindOrganization || kinds[1] != kindContract || kinds[2] != kindOrganization {
		t.Errorf("ожидался round-robin [org,contract,org], получено %v", kinds)
	}
	// Lim прокинут как limit (без +1: keyset не используется в поиске).
	if store.gotOrgParams.Lim != 3 {
		t.Errorf("Lim = %d, ожидалось 3 (без keyset +1)", store.gotOrgParams.Lim)
	}
}

// D1 (code review 6.2): контракты НЕ голодают за организациями. При ≥limit орг-матчей round-robin всё равно
// вставляет контракт 2-м элементом. Negative control против прежнего «организации-первыми» (там контракт
// при 20 орг-матчах и limit=20 не попал бы вовсе).
func TestSearch_Interleave_ContractsNotStarved(t *testing.T) {
	orgs := make([]gen.SearchOrganizationsRow, 0, 20)
	for i := range 20 {
		orgs = append(orgs, mkOrgRow(fmt.Sprintf("10000000%04d", i), "Орг"))
	}
	store := &mockSearchStore{orgs: orgs, contracts: []gen.SearchContractsRow{mkSearchContractRow("C1", false)}}
	_, body := getSearch(t, store, "?q=%D0%B6%D0%BE%D0%BB&limit=20")
	var out SearchResponse
	_ = json.Unmarshal(body, &out)
	if len(out.Items) != 20 {
		t.Fatalf("ожидалось 20 items, получено %d", len(out.Items))
	}
	second, ok := out.Items[1].(map[string]any)
	if !ok || second["kind"] != kindContract {
		t.Errorf("round-robin: 2-й элемент должен быть contract (контракт не голодает), получено %v", out.Items[1])
	}
}

func TestSearch_StoreError_500(t *testing.T) {
	t.Run("org error → 500", func(t *testing.T) {
		store := &mockSearchStore{orgErr: errors.New("db down")}
		resp, body := getSearch(t, store, "?q=%D0%B6%D0%BE%D0%BB")
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("status = %d, ожидалось 500: %s", resp.StatusCode, body)
		}
		if err := loadSchema(t, "Error").VisitJSON(jsonAny(t, body)); err != nil {
			t.Errorf("ошибка не валидна против Error: %v", err)
		}
	})
	t.Run("contract error → 500", func(t *testing.T) {
		store := &mockSearchStore{contractErr: errors.New("db down")}
		resp, _ := getSearch(t, store, "?q=%D0%B6%D0%BE%D0%BB")
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("status = %d, ожидалось 500", resp.StatusCode)
		}
	})
}
