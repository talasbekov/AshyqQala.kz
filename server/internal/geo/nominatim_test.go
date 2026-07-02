package geo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// newStubServer — httptest-сервер, отдающий заданное тело/код вместо живого Nominatim (детерминизм, без сети).
func newStubServer(t *testing.T, status int, body string) *Nominatim {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Errorf("ожидался путь /search, получен %q", r.URL.Path)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return NewNominatim(srv.URL, "test-ua", AstanaViewbox, true)
}

// TestGeocode_Matched — распознанный адрес → координаты + ok=true.
func TestGeocode_Matched(t *testing.T) {
	n := newStubServer(t, http.StatusOK, `[{"lat":"51.1605","lon":"71.4704"}]`)
	lat, lon, ok, err := n.Geocode(context.Background(), "проспект Абая, Астана")
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if !ok {
		t.Fatal("ok=false, ожидался матч")
	}
	if lat != 51.1605 || lon != 71.4704 {
		t.Errorf("координаты = (%v,%v), ожидалось (51.1605,71.4704)", lat, lon)
	}
}

// TestGeocode_Empty_NotError — пустой результат → (0,0,false,nil): честное «не сматчилось», НЕ ошибка,
// НЕ выдуманная координата (гардрейл AC2).
func TestGeocode_Empty_NotError(t *testing.T) {
	n := newStubServer(t, http.StatusOK, `[]`)
	lat, lon, ok, err := n.Geocode(context.Background(), "несуществующий адрес")
	if err != nil {
		t.Fatalf("пустой результат не должен быть ошибкой, получено: %v", err)
	}
	if ok {
		t.Error("ok=true на пустом результате — нельзя выдавать матч")
	}
	if lat != 0 || lon != 0 {
		t.Errorf("координаты = (%v,%v), ожидалось (0,0) при ok=false (потребитель НЕ пишет их)", lat, lon)
	}
}

// TestGeocode_HTTPError — не-200 → ошибка (не тихий промах).
func TestGeocode_HTTPError(t *testing.T) {
	n := newStubServer(t, http.StatusInternalServerError, `upstream boom`)
	if _, _, ok, err := n.Geocode(context.Background(), "x"); err == nil || ok {
		t.Fatalf("HTTP 500 должен дать ошибку (ok=%v err=%v)", ok, err)
	}
}

// TestGeocode_BadJSON — битый JSON → ошибка.
func TestGeocode_BadJSON(t *testing.T) {
	n := newStubServer(t, http.StatusOK, `{not json`)
	if _, _, _, err := n.Geocode(context.Background(), "x"); err == nil {
		t.Fatal("битый JSON должен дать ошибку разбора")
	}
}

// TestGeocode_NonFiniteOrOutOfRange_NotMatch — гардрейл честности: NaN/Inf (ParseFloat их ПРИНИМАЕТ)
// и координаты вне [-90,90]/[-180,180] → ok=false (НЕ выдуманная точка), не ошибка.
func TestGeocode_NonFiniteOrOutOfRange_NotMatch(t *testing.T) {
	cases := []string{
		`[{"lat":"NaN","lon":"71.4"}]`,
		`[{"lat":"51.1","lon":"Inf"}]`,
		`[{"lat":"999","lon":"71.4"}]`,  // широта вне [-90,90]
		`[{"lat":"51.1","lon":"-200"}]`, // долгота вне [-180,180]
	}
	for _, body := range cases {
		n := newStubServer(t, http.StatusOK, body)
		lat, lon, ok, err := n.Geocode(context.Background(), "x")
		if err != nil {
			t.Errorf("%s: неожиданная ошибка %v (ожидался тихий не-матч)", body, err)
		}
		if ok || lat != 0 || lon != 0 {
			t.Errorf("%s: ok=%v (%v,%v), ожидалось ok=false без координат (не выдумывать точку)", body, ok, lat, lon)
		}
	}
}

// newSequenceStubServer — httptest-сервер, отдающий i-й элемент statuses/bodies/retryAfters на i-й запрос
// (последний элемент повторяется при выходе за границы). Для тестов повтора (429→200, исчерпание попыток).
// Возвращает клиент с sleep=no-op (whitebox: тест не ждёт реальный backoff) + счётчик запросов.
func newSequenceStubServer(t *testing.T, statuses []int, bodies []string, retryAfters []string) (*Nominatim, *int32) {
	t.Helper()
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := int(atomic.AddInt32(&calls, 1)) - 1
		if idx >= len(statuses) {
			idx = len(statuses) - 1
		}
		if retryAfters[idx] != "" {
			w.Header().Set("Retry-After", retryAfters[idx])
		}
		w.WriteHeader(statuses[idx])
		_, _ = w.Write([]byte(bodies[idx]))
	}))
	t.Cleanup(srv.Close)
	n := NewNominatim(srv.URL, "test-ua", AstanaViewbox, true)
	n.sleep = func(time.Duration) {} // whitebox: не спать в тесте (тот же пакет geo)
	return n, &calls
}

// TestGeocodeMatch_ParsesImportance — importance Nominatim → Match.Confidence (Story 3.1 Task 2:
// "сейчас nominatimResult читает только lat/lon" закрыто).
func TestGeocodeMatch_ParsesImportance(t *testing.T) {
	n := newStubServer(t, http.StatusOK, `[{"lat":"51.1605","lon":"71.4704","importance":0.734}]`)
	m, ok, err := n.GeocodeMatch(context.Background(), "проспект Абая, Астана")
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if !ok {
		t.Fatal("ok=false, ожидался матч")
	}
	if m.Lat != 51.1605 || m.Lon != 71.4704 {
		t.Errorf("координаты = (%v,%v), ожидалось (51.1605,71.4704)", m.Lat, m.Lon)
	}
	if m.Confidence == nil || *m.Confidence != 0.734 {
		t.Errorf("confidence = %v, ожидалось указатель на 0.734", m.Confidence)
	}
}

// TestGeocodeMatch_MissingImportance_NilConfidence — self-host часто не отдаёт importance → честное
// «неизвестно» (nil), НЕ выдуманный 0.0 (код-ревью нашёл конфликт с исходной версией: 0.0 был
// неотличим от РЕАЛЬНОГО нулевого importance — *float64 различает «нет значения» от «значение ноль»).
func TestGeocodeMatch_MissingImportance_NilConfidence(t *testing.T) {
	n := newStubServer(t, http.StatusOK, `[{"lat":"51.1605","lon":"71.4704"}]`)
	m, ok, err := n.GeocodeMatch(context.Background(), "x")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v, ожидался матч", ok, err)
	}
	if m.Confidence != nil {
		t.Errorf("confidence = %v, ожидалось nil (importance отсутствует в ответе — честно «неизвестно», не 0.0)", *m.Confidence)
	}
}

// TestGeocodeMatch_ExplicitZeroImportance_ZeroConfidence — importance РЕАЛЬНО пришла нулевой (ключ
// присутствует) → confidence = указатель на 0.0, отличимый от отсутствия (предыдущий тест). Доказывает,
// что *float64 действительно различает эти два случая, а не просто всегда nil/всегда 0.
func TestGeocodeMatch_ExplicitZeroImportance_ZeroConfidence(t *testing.T) {
	n := newStubServer(t, http.StatusOK, `[{"lat":"51.1605","lon":"71.4704","importance":0}]`)
	m, ok, err := n.GeocodeMatch(context.Background(), "x")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v, ожидался матч", ok, err)
	}
	if m.Confidence == nil {
		t.Fatal("confidence = nil, ожидался указатель на 0.0 (importance ЯВНО присутствует в ответе)")
	}
	if *m.Confidence != 0 {
		t.Errorf("confidence = %v, ожидалось 0.0", *m.Confidence)
	}
}

// TestGeocode_DelegatesToGeocodeMatch — Geocode (Geocoder-интерфейс, 0.7 interim-geocode) остаётся тонкой
// обёрткой: те же координаты/ok/err, что и GeocodeMatch, без разницы в поведении после рефакторинга.
func TestGeocode_DelegatesToGeocodeMatch(t *testing.T) {
	n := newStubServer(t, http.StatusOK, `[{"lat":"51.1605","lon":"71.4704","importance":0.5}]`)
	var _ Geocoder = n // compile-time: интерфейс не сломан
	lat, lon, ok, err := n.Geocode(context.Background(), "x")
	if err != nil || !ok || lat != 51.1605 || lon != 71.4704 {
		t.Fatalf("Geocode = (%v,%v,%v,%v), ожидалось (51.1605,71.4704,true,nil)", lat, lon, ok, err)
	}
}

// TestGeocodeMatch_429_RetriesThenSucceeds — HTTP 429 → повтор (Story 3.1 Task 2, долг 0.7
// deferred-work.md:129), второй запрос успешен → честный матч, ровно 2 запроса.
func TestGeocodeMatch_429_RetriesThenSucceeds(t *testing.T) {
	n, calls := newSequenceStubServer(t,
		[]int{http.StatusTooManyRequests, http.StatusOK},
		[]string{`too many requests`, `[{"lat":"51.1","lon":"71.4","importance":0.9}]`},
		[]string{"1", ""},
	)
	m, ok, err := n.GeocodeMatch(context.Background(), "x")
	if err != nil {
		t.Fatalf("неожиданная ошибка после успешного повтора: %v", err)
	}
	if !ok || m.Lat != 51.1 || m.Lon != 71.4 {
		t.Fatalf("m=%+v ok=%v, ожидался матч (51.1,71.4)", m, ok)
	}
	if got := atomic.LoadInt32(calls); got != 2 {
		t.Errorf("запросов = %d, ожидалось 2 (429 + успешный повтор)", got)
	}
}

// TestGeocodeMatch_429_ExhaustsRetries — все попытки дают 429 → честная ошибка (НЕ выдуманный матч),
// ровно maxRetries429+1 запросов (страж числа попыток).
func TestGeocodeMatch_429_ExhaustsRetries(t *testing.T) {
	statuses := make([]int, maxRetries429+1)
	bodies := make([]string, maxRetries429+1)
	retryAfters := make([]string, maxRetries429+1)
	for i := range statuses {
		statuses[i], bodies[i], retryAfters[i] = http.StatusTooManyRequests, `rate limited`, "0"
	}
	n, calls := newSequenceStubServer(t, statuses, bodies, retryAfters)
	_, ok, err := n.GeocodeMatch(context.Background(), "x")
	if err == nil || ok {
		t.Fatalf("ожидалась честная ошибка после исчерпания попыток (ok=%v err=%v)", ok, err)
	}
	if got := atomic.LoadInt32(calls); got != int32(maxRetries429+1) {
		t.Errorf("запросов = %d, ожидалось %d (1 исходная + %d повтора)", got, maxRetries429+1, maxRetries429)
	}
}

// TestGeocodeMatch_500_NoRetry — не-429 HTTP-ошибка НЕ повторяется (страж: backoff только для 429,
// остальное — как раньше, честно и быстро наружу).
func TestGeocodeMatch_500_NoRetry(t *testing.T) {
	n, calls := newSequenceStubServer(t, []int{http.StatusInternalServerError}, []string{"boom"}, []string{""})
	_, ok, err := n.GeocodeMatch(context.Background(), "x")
	if err == nil || ok {
		t.Fatalf("HTTP 500 должен дать ошибку (ok=%v err=%v)", ok, err)
	}
	if got := atomic.LoadInt32(calls); got != 1 {
		t.Errorf("запросов = %d, ожидался 1 (500 не повторяется)", got)
	}
}

// TestGeocodeMatch_429_CtxCancelledDuringBackoff_ReturnsPromptly — код-ревью нашёл: голый n.sleep(retryAfter)
// блокировал SIGTERM/Ctrl-C до полного retryAfter (капается 30с), несмотря на комментарий «прерывают
// чисто». sleepCtx обязан вернуться СРАЗУ по отмене ctx, не дожидаясь d. Используем РЕАЛЬНЫЙ time.Sleep
// (не фейк) с заметным Retry-After (2с), чтобы гонка была настоящей, но отменяем ctx через ~50мс — тест
// должен завершиться на порядок быстрее 2с, если фикс работает (иначе — таймаут теста докажет регресс).
func TestGeocodeMatch_429_CtxCancelledDuringBackoff_ReturnsPromptly(t *testing.T) {
	n, _ := newSequenceStubServer(t,
		[]int{http.StatusTooManyRequests, http.StatusOK},
		[]string{"rate limited", `[{"lat":"1","lon":"2"}]`},
		[]string{"2", ""},
	)
	n.sleep = time.Sleep // реальный sleep — фейковый no-op не проверил бы прерывание

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, ok, err := n.GeocodeMatch(ctx, "x")
	elapsed := time.Since(start)

	if err == nil || ok {
		t.Fatalf("ожидалась ошибка отмены (ok=%v err=%v)", ok, err)
	}
	if elapsed >= time.Second {
		t.Errorf("GeocodeMatch занял %v — ctx.Done() не прервал sleep(2с) вовремя (регресс: sleepCtx не работает)", elapsed)
	}
}

// TestRetryAfterDuration — парсинг заголовка Retry-After: секунды/HTTP-дата/мусор/потолок.
func TestRetryAfterDuration(t *testing.T) {
	future := time.Now().Add(10 * time.Second).UTC().Format(http.TimeFormat)
	past := time.Now().Add(-10 * time.Second).UTC().Format(http.TimeFormat)
	cases := []struct {
		name string
		in   string
		want time.Duration
	}{
		{"секунды", "5", 5 * time.Second},
		{"ноль_секунд→дефолт", "0", PoliteDelayDefault},
		{"пусто→дефолт", "", PoliteDelayDefault},
		{"мусор→дефолт", "не-число", PoliteDelayDefault},
		{"огромное→капается", "999999", retryAfterCap},
		{"HTTP-дата_в_прошлом→дефолт", past, PoliteDelayDefault},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := retryAfterDuration(c.in)
			if got != c.want {
				t.Errorf("retryAfterDuration(%q) = %v, ожидалось %v", c.in, got, c.want)
			}
		})
	}
	// HTTP-дата в будущем — отдельно (не детерминированный want из-за секундной точности форматирования).
	if got := retryAfterDuration(future); got <= 0 || got > 11*time.Second {
		t.Errorf("retryAfterDuration(future) = %v, ожидалось ~10s", got)
	}
}

// TestNormalizeAddress — схлопывание пробелов/табов/переводов строк, обрезка краёв (Story 3.1 Task 2,
// долг 0.7 deferred-work.md:128).
func TestNormalizeAddress(t *testing.T) {
	cases := []struct{ in, want string }{
		{"  проспект Абая, Астана  ", "проспект Абая, Астана"},
		{"проспект   Абая,\tАстана", "проспект Абая, Астана"},
		{"строка\nс переводом\n\nстроки", "строка с переводом строки"},
		{"", ""},
		{"   ", ""},
		{"однослово", "однослово"},
	}
	for _, c := range cases {
		if got := NormalizeAddress(c.in); got != c.want {
			t.Errorf("NormalizeAddress(%q) = %q, ожидалось %q", c.in, got, c.want)
		}
	}
}
