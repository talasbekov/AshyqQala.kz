package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"ashyqqala/server/internal/store/gen"
)

// mockListStore — фейковый ContractsListStore: возвращает канонные строки/ошибки И захватывает переданные
// params (для проверки, что хендлер действительно прокидывает фасеты). Канонный mock не фильтрует сам —
// РЕАЛЬНУЮ фильтрацию SQL проверяет integration-тест; здесь — парсинг/валидация/wire/keyset/прокидка.
type mockListStore struct {
	rows          []gen.ListContractsRow
	listErr       error
	gotParams     gen.ListContractsParams
	orgID         int64
	orgErr        error
	resolveBin    string
	resolveCalled bool
}

func (m *mockListStore) ListContracts(_ context.Context, arg gen.ListContractsParams) ([]gen.ListContractsRow, error) {
	m.gotParams = arg
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.rows, nil
}

func (m *mockListStore) ResolveSupplierOrgID(_ context.Context, bin string) (int64, error) {
	m.resolveCalled = true
	m.resolveBin = bin
	if m.orgErr != nil {
		return 0, m.orgErr
	}
	return m.orgID, nil
}

func newListRouter(store ContractsListStore) http.Handler {
	r := chi.NewRouter()
	r.Get("/api/contracts", ContractsListHandler{Store: store}.List)
	return r
}

// mkListRow — строка списка; sd=="" ⇒ NULL sign_date (честный no_data / хвост NULLS LAST).
func mkListRow(gid, sd string, flag bool) gen.ListContractsRow {
	row := gen.ListContractsRow{
		GoszakupContractID: gid,
		SubjectRu:          pgtype.Text{String: "Ремонт автодороги " + gid, Valid: true},
		SubjectKk:          pgtype.Text{Valid: false}, // NULL → no_data
		AmountTng:          pgtype.Int8{Int64: 240000000, Valid: true},
		Status:             pgtype.Text{String: "active", Valid: true},
		Direction:          pgtype.Text{String: "road", Valid: true},
		KatoCode:           pgtype.Text{String: "710000000", Valid: true},
		HasActiveFlag:      flag,
	}
	if sd != "" {
		tm, _ := time.Parse("2006-01-02", sd)
		row.SignDate = pgtype.Date{Time: tm, Valid: true}
	}
	return row
}

func getList(t *testing.T, store ContractsListStore, qs string) (*http.Response, []byte) {
	t.Helper()
	srv := httptest.NewServer(newListRouter(store))
	t.Cleanup(srv.Close)
	resp, err := http.Get(srv.URL + "/api/contracts" + qs)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return resp, body
}

func TestListContracts_WireFormat_ValidatesAgainstOpenAPI(t *testing.T) {
	store := &mockListStore{rows: []gen.ListContractsRow{
		mkListRow("DEMO-0002", "2024-05-01", true),
		mkListRow("DEMO-0003", "", false), // NULL sign_date
	}}
	resp, body := getList(t, store, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, ожидалось 200: %s", resp.StatusCode, body)
	}
	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err != nil {
		t.Fatal(err)
	}
	if err := loadSchema(t, "ContractListResponse").VisitJSON(obj); err != nil {
		t.Fatalf("ответ не валиден против ContractListResponse: %v\n%s", err, body)
	}
	// NULL sign_date → честный no_data (не выдуманная дата).
	items := obj["items"].([]any)
	second := items[1].(map[string]any)["sign_date"].(map[string]any)
	if second["state"] != "no_data" || second["value"] != nil {
		t.Errorf("NULL sign_date должен быть no_data/null, получено %v", second)
	}
	// has_active_flag прокинут как bool.
	if items[0].(map[string]any)["has_active_flag"] != true {
		t.Errorf("has_active_flag первого айтема должен быть true")
	}
}

func TestListContracts_Empty_IsEmptyArrayNotNull(t *testing.T) {
	store := &mockListStore{rows: []gen.ListContractsRow{}}
	resp, body := getList(t, store, "?direction=water&amount_min=999999999999")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	// AC3: честный пустой контейнер — items:[] (НЕ null), next_cursor:null. Никаких выдуманных строк.
	if !strings.Contains(string(body), `"items":[]`) {
		t.Errorf("ожидался items:[] (не null): %s", body)
	}
	if !strings.Contains(string(body), `"next_cursor":null`) {
		t.Errorf("ожидался next_cursor:null: %s", body)
	}
	var obj map[string]any
	_ = json.Unmarshal(body, &obj)
	if err := loadSchema(t, "ContractListResponse").VisitJSON(obj); err != nil {
		t.Fatalf("пустой ответ не валиден: %v", err)
	}
}

func TestListContracts_InvalidParams_400(t *testing.T) {
	cases := []struct {
		name, qs, code string
	}{
		{"direction вне enum", "?direction=plane", "VALIDATION_FAILED"},
		{"signed_from не дата", "?signed_from=notadate", "VALIDATION_FAILED"},
		{"signed_to не дата", "?signed_to=2024-13-40", "VALIDATION_FAILED"},
		{"amount_min отрицателен", "?amount_min=-5", "VALIDATION_FAILED"},
		{"amount_max не число", "?amount_max=abc", "VALIDATION_FAILED"},
		{"has_flag не bool", "?has_flag=maybe", "VALIDATION_FAILED"},
		{"signed_from позже signed_to", "?signed_from=2025-01-01&signed_to=2024-01-01", "VALIDATION_FAILED"}, // P6 [code review 6.1]
		{"amount_min больше amount_max", "?amount_min=900&amount_max=100", "VALIDATION_FAILED"},              // P6 [code review 6.1]
		{"limit ноль", "?limit=0", "VALIDATION_FAILED"},
		{"limit свыше max", "?limit=101", "VALIDATION_FAILED"},
		{"limit не число", "?limit=x", "VALIDATION_FAILED"},
		{"cursor без gid", "?cursor=e30", "INVALID_CURSOR"},         // base64("{}") → Gid пуст → отказ
		{"cursor не base64", "?cursor=%21%21%21", "INVALID_CURSOR"}, // "!!!" → не base64url
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			store := &mockListStore{rows: []gen.ListContractsRow{}}
			resp, body := getList(t, store, c.qs)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, ожидалось 400 (%s): %s", resp.StatusCode, c.name, body)
			}
			var env map[string]struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			_ = json.Unmarshal(body, &env)
			if env["error"].Code != c.code {
				t.Errorf("code = %q, ожидалось %q", env["error"].Code, c.code)
			}
			if err := loadSchema(t, "Error").VisitJSON(jsonAny(t, body)); err != nil {
				t.Errorf("ошибка не валидна против Error: %v", err)
			}
		})
	}
}

// negative control: валидный запрос НЕ должен давать 400 (иначе тест 400 выше прошёл бы мимо — любой запрос 400).
func TestListContracts_ValidParams_Not400(t *testing.T) {
	store := &mockListStore{rows: []gen.ListContractsRow{}}
	resp, body := getList(t, store, "?direction=road,water&signed_from=2024-01-01&has_flag=true&limit=50")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("валидный запрос дал %d: %s", resp.StatusCode, body)
	}
}

func TestListContracts_ParsesFacetsIntoParams(t *testing.T) {
	store := &mockListStore{rows: []gen.ListContractsRow{}}
	_, _ = getList(t, store, "?direction=road,water&signed_from=2024-01-01&signed_to=2024-12-31&amount_min=100&amount_max=999&has_flag=true&limit=50")
	p := store.gotParams
	if len(p.Directions) != 2 || p.Directions[0] != "road" || p.Directions[1] != "water" {
		t.Errorf("Directions = %v, ожидалось [road water]", p.Directions)
	}
	if !p.SignedFrom.Valid || p.SignedFrom.Time.Format("2006-01-02") != "2024-01-01" {
		t.Errorf("SignedFrom = %v", p.SignedFrom)
	}
	if !p.SignedTo.Valid || p.SignedTo.Time.Format("2006-01-02") != "2024-12-31" {
		t.Errorf("SignedTo = %v", p.SignedTo)
	}
	if !p.AmountMin.Valid || p.AmountMin.Int64 != 100 || !p.AmountMax.Valid || p.AmountMax.Int64 != 999 {
		t.Errorf("Amount min/max = %v/%v", p.AmountMin, p.AmountMax)
	}
	if !p.HasFlagOnly {
		t.Error("HasFlagOnly должен быть true")
	}
	if p.Lim != 51 { // limit 50 + 1 (keyset-детект следующей страницы)
		t.Errorf("Lim = %d, ожидалось 51 (limit+1)", p.Lim)
	}
}

// negative control: без фасетов params пустые (фасет не задан ⇒ narg NULL ⇒ нет фильтра), Lim = дефолт+1.
func TestListContracts_NoFacets_EmptyParams(t *testing.T) {
	store := &mockListStore{rows: []gen.ListContractsRow{}}
	_, _ = getList(t, store, "")
	p := store.gotParams
	if p.Directions != nil {
		t.Errorf("Directions без фасета должен быть nil (не '{}'), получено %v", p.Directions)
	}
	if p.SignedFrom.Valid || p.SignedTo.Valid || p.AmountMin.Valid || p.AmountMax.Valid || p.SupplierOrgID.Valid {
		t.Error("пустые фасеты должны быть невалидны (NULL)")
	}
	if p.HasFlagOnly {
		t.Error("HasFlagOnly без фасета должен быть false")
	}
	if p.Lim != int32(defaultListLimit+1) {
		t.Errorf("Lim = %d, ожидалось %d", p.Lim, defaultListLimit+1)
	}
}

func TestListContracts_SupplierBin(t *testing.T) {
	t.Run("невалидный БИН → пустой список, resolve НЕ зван", func(t *testing.T) {
		store := &mockListStore{}
		resp, body := getList(t, store, "?supplier_bin=123") // не 12 цифр
		if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"items":[]`) {
			t.Fatalf("ожидался 200 + пустой список: %d %s", resp.StatusCode, body)
		}
		if store.resolveCalled {
			t.Error("ResolveSupplierOrgID не должен вызываться для невалидного БИН")
		}
	})
	t.Run("валидный БИН не найден → пустой список (не 500)", func(t *testing.T) {
		store := &mockListStore{orgErr: pgx.ErrNoRows}
		resp, body := getList(t, store, "?supplier_bin=123456789012")
		if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"items":[]`) {
			t.Fatalf("ожидался 200 + пустой список: %d %s", resp.StatusCode, body)
		}
		if !store.resolveCalled || store.resolveBin != "123456789012" {
			t.Errorf("resolve должен быть зван с каноническим БИН, got called=%v bin=%q", store.resolveCalled, store.resolveBin)
		}
	})
	t.Run("резолвится → SupplierOrgID прокинут", func(t *testing.T) {
		store := &mockListStore{orgID: 42, rows: []gen.ListContractsRow{}}
		_, _ = getList(t, store, "?supplier_bin=123456789012")
		if !store.gotParams.SupplierOrgID.Valid || store.gotParams.SupplierOrgID.Int64 != 42 {
			t.Errorf("SupplierOrgID = %v, ожидалось {42,valid}", store.gotParams.SupplierOrgID)
		}
	})
}

// P3 [code review 6.1] negative-control: невалидный limit/cursor НЕ должен маскироваться ранним 200 от
// supplier_bin. Валидация чистых параметров идёт ДО резолва БИН; иначе «битый параметр + несуществующий БИН»
// молча дал бы 200 [] вместо 400. Тест обязан краснеть, если резолв вернут обратно вперёд limit/cursor.
func TestListContracts_ValidationBeforeSupplierResolve(t *testing.T) {
	t.Run("битый limit + несуществующий БИН → 400 (не 200), resolve НЕ зван", func(t *testing.T) {
		store := &mockListStore{orgErr: pgx.ErrNoRows}
		resp, body := getList(t, store, "?supplier_bin=123456789012&limit=0")
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, ожидалось 400: %s", resp.StatusCode, body)
		}
		if store.resolveCalled {
			t.Error("ResolveSupplierOrgID не должен вызываться при невалидном limit (валидация раньше резолва)")
		}
	})
	t.Run("битый cursor + невалидный БИН → 400 (не 200)", func(t *testing.T) {
		store := &mockListStore{}
		resp, body := getList(t, store, "?supplier_bin=123&cursor=%21%21%21")
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, ожидалось 400: %s", resp.StatusCode, body)
		}
	})
}

func TestListContracts_Keyset_NextCursor(t *testing.T) {
	// Возвращаем pageSize+1 (21) строк → есть следующая страница: items усечены до 20, next_cursor не null.
	rows := make([]gen.ListContractsRow, 0, 21)
	for i := 0; i < 21; i++ {
		rows = append(rows, mkListRow("DEMO-"+strings.Repeat("0", 1)+itoa(i), "2024-05-01", false))
	}
	store := &mockListStore{rows: rows}
	resp, body := getList(t, store, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var out ContractListResponse
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Items) != defaultListLimit {
		t.Fatalf("items = %d, ожидалось %d (усечение до страницы)", len(out.Items), defaultListLimit)
	}
	if out.NextCursor == nil {
		t.Fatal("next_cursor должен быть не null при наличии следующей страницы")
	}
	// курсор указывает на ПОСЛЕДНЮЮ строку страницы (index 19), а не на лишнюю 21-ю.
	cur, err := decodeCursor(*out.NextCursor)
	if err != nil {
		t.Fatalf("курсор не декодируется: %v", err)
	}
	if cur.Gid != rows[defaultListLimit-1].GoszakupContractID {
		t.Errorf("курсор gid = %q, ожидалось %q", cur.Gid, rows[defaultListLimit-1].GoszakupContractID)
	}
	// хендлер запросил pageSize+1.
	if store.gotParams.Lim != int32(defaultListLimit+1) {
		t.Errorf("Lim = %d", store.gotParams.Lim)
	}
}

// negative control: ровно pageSize строк ⇒ следующей страницы НЕТ (next_cursor null) — иначе «всегда есть
// курсор» прошло бы мимо.
func TestListContracts_Keyset_NoCursorWhenExactlyPage(t *testing.T) {
	rows := make([]gen.ListContractsRow, 0, 3)
	for i := 0; i < 3; i++ {
		rows = append(rows, mkListRow("DEMO-"+itoa(i), "2024-05-01", false))
	}
	store := &mockListStore{rows: rows}
	resp, body := getList(t, store, "?limit=3")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var out ContractListResponse
	_ = json.Unmarshal(body, &out)
	if len(out.Items) != 3 {
		t.Fatalf("items = %d, ожидалось 3", len(out.Items))
	}
	if out.NextCursor != nil {
		t.Errorf("next_cursor должен быть null (нет лишней строки), получено %q", *out.NextCursor)
	}
}

func TestListContracts_StoreError_500(t *testing.T) {
	store := &mockListStore{listErr: errors.New("db down")}
	resp, body := getList(t, store, "")
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, ожидалось 500", resp.StatusCode)
	}
	if err := loadSchema(t, "Error").VisitJSON(jsonAny(t, body)); err != nil {
		t.Errorf("ошибка не валидна против Error: %v", err)
	}
}

// --- мелкие хелперы (локальные, без зависимостей) ---

func jsonAny(t *testing.T, b []byte) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("json: %v", err)
	}
	return v
}

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10))
}
