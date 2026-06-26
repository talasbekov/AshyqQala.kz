package og

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"ashyqqala/server/internal/registry"
)

// TestResolveLocale — приоритет query ?lang= > cookie aq-lang > дефолт KK (NFR-6); неизвестное → дефолт.
func TestResolveLocale(t *testing.T) {
	mk := func(query, cookie string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/og/contracts/X?"+query, nil)
		if cookie != "" {
			r.AddCookie(&http.Cookie{Name: "aq-lang", Value: cookie})
		}
		return r
	}
	cases := []struct {
		name   string
		query  string
		cookie string
		want   registry.Locale
	}{
		{"default KK", "", "", registry.KK},
		{"query ru", "lang=ru", "", registry.RU},
		{"query kk", "lang=kk", "", registry.KK},
		{"cookie ru", "", "ru", registry.RU},
		{"query побеждает cookie", "lang=ru", "kk", registry.RU},
		{"неизвестный query → дефолт KK", "lang=en", "", registry.KK},
		{"kz (страна, не язык) → дефолт KK", "lang=kz", "", registry.KK},
		{"неизвестный cookie → дефолт KK", "", "fr", registry.KK},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveLocale(mk(c.query, c.cookie)); got != c.want {
				t.Errorf("resolveLocale = %q; want %q", got, c.want)
			}
		})
	}
}

// TestOGLocale — формат og:locale.
func TestOGLocale(t *testing.T) {
	if got := ogLocale(registry.KK); got != "kk_KZ" {
		t.Errorf("ogLocale(KK) = %q; want kk_KZ", got)
	}
	if got := ogLocale(registry.RU); got != "ru_KZ" {
		t.Errorf("ogLocale(RU) = %q; want ru_KZ", got)
	}
}
