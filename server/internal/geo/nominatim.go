// Package geo — batch-геокодер (Nominatim) для гео-привязки. Перенос концепта из stage0-audit/geocoder.go
// (другой Go-модуль — импорт запрещён). Эфемерный batch (AR-22): результат кэшируется, контейнер гасится.
// Stdlib-only клиент; запись координат — слой store (pgx). Переиспользуется Epic 3 (Story 3.1), не выбрасывается.
package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// AstanaViewbox — bbox Астаны для ограничения Nominatim (lon_left,lat_top,lon_right,lat_bottom).
// VERIFY: приблизительный bbox (перенос stage0-audit/config.go). [Source: stage0-audit/config.go:30]
const AstanaViewbox = "71.20,51.30,71.78,51.00"

// PoliteDelayDefault — пауза между запросами к ПУБЛИЧНОМУ Nominatim (политика ≤1 req/sec). Для self-host
// (AR-22) можно меньше. [Source: stage0-audit geocoder politeness; server/tools/scrape/scrape.go politeDelayDefault]
const PoliteDelayDefault = 1100 * time.Millisecond

// Geocoder — абстракция геокодера (тестируемость: подменяется фейком без сети).
type Geocoder interface {
	// Geocode возвращает координаты и ok=true, если адрес распознан (в пределах ограничения).
	// Пустой/невалидный результат → (0,0,false,nil) — НЕ ошибка (честное «не сматчилось»).
	// ctx прерывает HTTP-запрос (отмена/таймаут долгого batch).
	Geocode(ctx context.Context, query string) (lat, lon float64, ok bool, err error)
}

// Nominatim — клиент публичного/самохостингового Nominatim (OSM). Перенос stage0-audit/geocoder.go.
type Nominatim struct {
	base    string
	http    *http.Client
	ua      string
	viewbox string // "lon_left,lat_top,lon_right,lat_bottom" — ограничение по Астане
	bounded bool
}

// NewNominatim — конструктор. Пустые аргументы → дефолты (публичный Nominatim, идентифицирующий UA).
func NewNominatim(base, ua, viewbox string, bounded bool) *Nominatim {
	if base == "" {
		base = "https://nominatim.openstreetmap.org"
	}
	if ua == "" {
		ua = "AshyqQala-interim-geocode/0.7 (+https://ashyqqala.kz)"
	}
	return &Nominatim{
		base:    strings.TrimRight(base, "/"),
		http:    &http.Client{Timeout: 20 * time.Second},
		ua:      ua,
		viewbox: viewbox,
		bounded: bounded,
	}
}

type nominatimResult struct {
	Lat string `json:"lat"`
	Lon string `json:"lon"`
}

// Geocode — freeform /search?q=…&format=json&limit=1&countrycodes=kz (+ viewbox/bounded). lat/lon в ответе —
// СТРОКИ → ParseFloat. Пустой результат → (0,0,false,nil). HTTP-/JSON-ошибка → err (не тихо).
func (n *Nominatim) Geocode(ctx context.Context, q string) (float64, float64, bool, error) {
	qs := url.Values{}
	qs.Set("q", q)
	qs.Set("format", "json")
	qs.Set("limit", "1")
	qs.Set("countrycodes", "kz")
	if n.viewbox != "" {
		qs.Set("viewbox", n.viewbox)
		if n.bounded {
			qs.Set("bounded", "1")
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, n.base+"/search?"+qs.Encode(), nil)
	if err != nil {
		return 0, 0, false, err
	}
	req.Header.Set("User-Agent", n.ua) // политика Nominatim требует идентифицирующий UA
	req.Header.Set("Accept", "application/json")
	resp, err := n.http.Do(req)
	if err != nil {
		return 0, 0, false, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // cap 1MB: limit=1 → тело крошечное; защита от гигантского ответа
	if err != nil {
		return 0, 0, false, fmt.Errorf("nominatim чтение тела: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return 0, 0, false, fmt.Errorf("nominatim HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	var rs []nominatimResult
	if err := json.Unmarshal(body, &rs); err != nil {
		return 0, 0, false, err
	}
	if len(rs) == 0 {
		return 0, 0, false, nil // честно: адрес не сматчился (НЕ ошибка, НЕ выдуманная координата)
	}
	lat, err := strconv.ParseFloat(rs[0].Lat, 64)
	if err != nil {
		return 0, 0, false, fmt.Errorf("nominatim разбор lat %q: %w", rs[0].Lat, err)
	}
	lon, err := strconv.ParseFloat(rs[0].Lon, 64)
	if err != nil {
		return 0, 0, false, fmt.Errorf("nominatim разбор lon %q: %w", rs[0].Lon, err)
	}
	// Честность: НЕ финитная (NaN/Inf — ParseFloat их принимает!) или вне географического диапазона
	// координата — это НЕ валидный матч. Не выдумывать точку: → ok=false (как «не сматчилось»), не писать в БД.
	if math.IsNaN(lat) || math.IsInf(lat, 0) || math.IsNaN(lon) || math.IsInf(lon, 0) ||
		lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return 0, 0, false, nil
	}
	return lat, lon, true, nil
}

// truncate — обрезка строки до n рун (для безопасного лога тела ошибки). Перенос stage0-audit-хелпера.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
