package main

import (
	"strings"
	"time"
)

func classifyDirection(text string) string {
	t := strings.ToLower(text)
	for _, kw := range RoadKeywords {
		if strings.Contains(t, kw) {
			return "road"
		}
	}
	for _, kw := range WaterKeywords {
		if strings.Contains(t, kw) {
			return "water"
		}
	}
	return "other"
}

func hasLocationMarker(text string) bool {
	t := strings.ToLower(text)
	for _, m := range AddressMarkers {
		if strings.Contains(t, m) {
			return true
		}
	}
	return false
}

// extractLocation вытаскивает адресную часть из названия лота: ищет первый
// адресный маркер (улица/проспект/мкр…) и берёт подстроку от него до конца.
// Пусто, если локационного сигнала нет (объект не геокодируется).
func extractLocation(name string) string {
	t := strings.ToLower(name)
	best := -1
	for _, m := range AddressMarkers {
		if i := strings.Index(t, m); i >= 0 && (best < 0 || i < best) {
			best = i
		}
	}
	if best < 0 {
		return ""
	}
	return strings.TrimSpace(t[best:])
}

func isAstanaKATO(kato string) bool {
	k := strings.TrimSpace(kato)
	if k == "" {
		return false
	}
	for _, p := range AstanaKATOPrefixes {
		if strings.HasPrefix(k, p) {
			return true
		}
	}
	return false
}

func parseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	layouts := []string{"2006-01-02", "2006-01-02T15:04:05", "2006-01-02 15:04:05", time.RFC3339, "02.01.2006"}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	if len(s) >= 10 {
		if t, err := time.Parse("2006-01-02", s[:10]); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func pct(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return 100 * float64(n) / float64(d)
}
