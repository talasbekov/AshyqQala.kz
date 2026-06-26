package og

import (
	"net/http"
	"strings"

	"ashyqqala/server/internal/registry"
)

// langCookie — имя cookie выбора языка. Согласовано с web (localStorage-ключ 'aq-lang'); для СЕРВЕРНОГО
// OG localStorage недоступен (UX-DR21), поэтому выбор читается из URL/cookie.
const langCookie = "aq-lang"

// resolveLocale выбирает локаль OG НА СЕРВЕРЕ: query ?lang= > cookie aq-lang > дефолт KK (NFR-6: казахский
// по умолчанию). Неизвестное/пустое значение игнорируется (→ дефолт), сервер не падает.
func resolveLocale(r *http.Request) registry.Locale {
	if l, ok := parseLocale(r.URL.Query().Get("lang")); ok {
		return l
	}
	if c, err := r.Cookie(langCookie); err == nil {
		if l, ok := parseLocale(c.Value); ok {
			return l
		}
	}
	return registry.KK
}

// parseLocale — строгий разбор кода языка в registry.Locale. Только "ru"/"kk" (kz — страна, не язык).
func parseLocale(s string) (registry.Locale, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "ru":
		return registry.RU, true
	case "kk":
		return registry.KK, true
	}
	return "", false
}

// ogLocale — OG-формат локали для <meta property="og:locale"> (BCP-47-подобный с регионом KZ).
func ogLocale(loc registry.Locale) string {
	if loc == registry.RU {
		return "ru_KZ"
	}
	return "kk_KZ"
}
