package geo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
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
