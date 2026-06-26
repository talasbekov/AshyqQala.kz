package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ashyqqala/server/internal/store/gen"
)

// fakeErrStore — мок ErrorReportStore: считает вызовы, запоминает последние params, может вернуть ошибку.
type fakeErrStore struct {
	calls int
	last  gen.InsertErrorReportParams
	err   error
	id    int64
}

func (f *fakeErrStore) InsertErrorReport(_ context.Context, arg gen.InsertErrorReportParams) (gen.InsertErrorReportRow, error) {
	f.calls++
	f.last = arg
	if f.err != nil {
		return gen.InsertErrorReportRow{}, f.err
	}
	return gen.InsertErrorReportRow{ID: f.id}, nil
}

func postER(h *ErrorReportsHandler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/error-reports", bytes.NewBufferString(body))
	req.RemoteAddr = "203.0.113.7:5555"
	rec := httptest.NewRecorder()
	h.Create(rec, req)
	return rec
}

func TestErrorReports_Valid_Inserts_And_Conforms(t *testing.T) {
	store := &fakeErrStore{id: 42}
	h := NewErrorReportsHandler(store, nil)

	// message/contact с пробелами — проверяем trim; source_url пустой → NULL (не выдумываем «»).
	body := `{"kind":"flag_error","subject_type":"contract","subject_ref":"DEMO-0002","message":"  цена завышена  ","contact":" me@x.kz ","source_url":""}`
	rec := postER(h, body)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, ожидалось 201; body=%s", rec.Code, rec.Body.String())
	}
	if store.calls != 1 {
		t.Fatalf("InsertErrorReport вызван %d раз, ожидался 1", store.calls)
	}
	if store.last.Message != "цена завышена" {
		t.Fatalf("message не обрезан: %q", store.last.Message)
	}
	if !store.last.Contact.Valid || store.last.Contact.String != "me@x.kz" {
		t.Fatalf("contact = %+v, ожидался trimmed me@x.kz", store.last.Contact)
	}
	if store.last.SourceUrl.Valid {
		t.Fatalf("пустой source_url должен быть NULL, получено %+v", store.last.SourceUrl)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if err := loadSchema(t, "ErrorReportResponse").VisitJSON(resp); err != nil {
		t.Fatalf("ответ не валиден по OpenAPI ErrorReportResponse: %v", err)
	}
	if resp["status"] != "received" || resp["id"].(float64) != 42 {
		t.Fatalf("ответ = %v, ожидалось {id:42,status:received}", resp)
	}
}

func TestErrorReports_Honeypot_AcceptsButDoesNotInsert(t *testing.T) {
	store := &fakeErrStore{id: 99}
	h := NewErrorReportsHandler(store, nil)

	body := `{"kind":"data_error","subject_type":"contract","subject_ref":"DEMO-0002","message":"бот","leave_blank":"http://spam.example"}`
	rec := postER(h, body)

	if rec.Code != http.StatusCreated {
		t.Fatalf("honeypot: status = %d, ожидалось 201 (молчаливый приём)", rec.Code)
	}
	if store.calls != 0 {
		t.Fatalf("honeypot: запись НЕ должна попасть в очередь, calls=%d", store.calls)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if _, hasID := resp["id"]; hasID {
		t.Fatalf("honeypot-ответ не должен нести id, получено %v", resp)
	}
	if err := loadSchema(t, "ErrorReportResponse").VisitJSON(resp); err != nil {
		t.Fatalf("honeypot-ответ не валиден по OpenAPI: %v", err)
	}
}

func TestErrorReports_Validation_Rejects(t *testing.T) {
	cases := map[string]string{
		"неизвестный kind":         `{"kind":"nope","subject_type":"contract","subject_ref":"X","message":"m"}`,
		"неизвестный subject_type": `{"kind":"data_error","subject_type":"nope","subject_ref":"X","message":"m"}`,
		"пустой message":           `{"kind":"data_error","subject_type":"contract","subject_ref":"X","message":"   "}`,
		"пустой subject_ref":       `{"kind":"data_error","subject_type":"contract","subject_ref":"","message":"m"}`,
		"битый json":               `{not-json`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			store := &fakeErrStore{}
			h := NewErrorReportsHandler(store, nil)
			rec := postER(h, body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, ожидалось 400; body=%s", rec.Code, rec.Body.String())
			}
			if store.calls != 0 {
				t.Fatalf("при невалидном входе запись НЕ должна происходить, calls=%d", store.calls)
			}
		})
	}
}

func TestErrorReports_BodyTooLarge_Rejected(t *testing.T) {
	store := &fakeErrStore{}
	h := NewErrorReportsHandler(store, nil)
	huge := strings.Repeat("a", 20_000) // > maxBodyBytes (16 КБ)
	body := `{"kind":"data_error","subject_type":"contract","subject_ref":"X","message":"` + huge + `"}`
	rec := postER(h, body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, ожидалось 400 (тело > лимита)", rec.Code)
	}
	if store.calls != 0 {
		t.Fatalf("тело > лимита — запись не должна происходить, calls=%d", store.calls)
	}
}

func TestErrorReports_StoreError_Returns500(t *testing.T) {
	store := &fakeErrStore{err: errors.New("db down")}
	h := NewErrorReportsHandler(store, nil)
	body := `{"kind":"data_error","subject_type":"contract","subject_ref":"X","message":"m"}`
	rec := postER(h, body)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, ожидалось 500", rec.Code)
	}
}

func TestErrorReports_RateLimited(t *testing.T) {
	store := &fakeErrStore{id: 1}
	h := &ErrorReportsHandler{Store: store, limiter: newRateLimiter(1, time.Hour)}
	body := `{"kind":"data_error","subject_type":"contract","subject_ref":"X","message":"m"}`

	if rec := postER(h, body); rec.Code != http.StatusCreated {
		t.Fatalf("1-е обращение: status = %d, ожидалось 201", rec.Code)
	}
	rec := postER(h, body) // тот же IP — превышение лимита
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("2-е обращение: status = %d, ожидалось 429", rec.Code)
	}
	if store.calls != 1 {
		t.Fatalf("после rate-limit запись не должна происходить повторно, calls=%d", store.calls)
	}
}

// Контракт запроса, который ждём от клиента, валиден по OpenAPI ErrorReportRequest (синхронность фронт↔бэк).
func TestErrorReports_RequestFixture_ConformsToOpenAPI(t *testing.T) {
	req := map[string]any{
		"kind":         "geo_wrong_point",
		"subject_type": "geo_object",
		"subject_ref":  "LOT-123",
		"message":      "точка не на той улице",
		"contact":      "",
		"source_url":   "https://goszakup.gov.kz/ru/contract/DEMO-0002",
	}
	if err := loadSchema(t, "ErrorReportRequest").VisitJSON(req); err != nil {
		t.Fatalf("фикстура запроса не валидна по OpenAPI ErrorReportRequest: %v", err)
	}
}

// clientIP берёт ПРАВЫЙ хоп X-Forwarded-For (реальный клиент за нашим прокси); левые — подделываемы.
func TestClientIP_RightmostXFFHop(t *testing.T) {
	mk := func(xff, remote string) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/api/error-reports", nil)
		r.RemoteAddr = remote
		if xff != "" {
			r.Header.Set("X-Forwarded-For", xff)
		}
		return r
	}
	cases := []struct{ name, xff, remote, want string }{
		{"правый реальный (левый поддельный)", "9.9.9.9, 1.2.3.4", "10.0.0.1:5", "1.2.3.4"},
		{"один хоп", "9.9.9.9", "10.0.0.1:5", "9.9.9.9"},
		{"пустые токены справа пропускаются", "fake, , 1.2.3.4", "10.0.0.1:5", "1.2.3.4"},
		{"без xff → host из RemoteAddr", "", "203.0.113.7:5555", "203.0.113.7"},
	}
	for _, c := range cases {
		if got := clientIP(mk(c.xff, c.remote)); got != c.want {
			t.Fatalf("%s: clientIP = %q, ожидалось %q", c.name, got, c.want)
		}
	}
}

// Negative-control: подмена ЛЕВОГО хопа XFF НЕ даёт новый rate-limit-бакет (ключ — реальный правый IP).
// До фикса (leftmost) 2-й запрос с другим левым хопом получил бы свежий бакет → 201 (тест бы покраснел).
func TestErrorReports_RateLimit_NotBypassableViaSpoofedLeftHop(t *testing.T) {
	store := &fakeErrStore{id: 1}
	h := &ErrorReportsHandler{Store: store, limiter: newRateLimiter(1, time.Hour)}
	body := `{"kind":"data_error","subject_type":"contract","subject_ref":"X","message":"m"}`
	send := func(xff string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/error-reports", bytes.NewBufferString(body))
		req.Header.Set("X-Forwarded-For", xff)
		rec := httptest.NewRecorder()
		h.Create(rec, req)
		return rec.Code
	}
	if c := send("9.9.9.9, 1.1.1.1"); c != http.StatusCreated {
		t.Fatalf("1-й запрос = %d, ожидалось 201", c)
	}
	if c := send("8.8.8.8, 1.1.1.1"); c != http.StatusTooManyRequests {
		t.Fatalf("2-й запрос (тот же реальный IP 1.1.1.1, другой поддельный левый) = %d, ожидалось 429", c)
	}
}

// rateLimiter: sliding-window — limit обращений в окне, затем отказ; после окна — снова allow.
func TestRateLimiter_SlidingWindow(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	rl := newRateLimiter(2, time.Minute)
	rl.now = func() time.Time { return now }

	if !rl.allow("ip") || !rl.allow("ip") {
		t.Fatal("первые 2 обращения должны проходить")
	}
	if rl.allow("ip") {
		t.Fatal("3-е обращение в окне должно отклоняться")
	}
	// другой ключ не затронут
	if !rl.allow("other") {
		t.Fatal("другой IP не должен быть ограничен")
	}
	// сдвигаем время за окно → снова allow
	now = now.Add(2 * time.Minute)
	if !rl.allow("ip") {
		t.Fatal("после окна обращения снова должны проходить")
	}
}
