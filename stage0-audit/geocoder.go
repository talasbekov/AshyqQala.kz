package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Geocoder — абстракция геокодера для реального замера автопокрытия (OQ-4).
type Geocoder interface {
	// Geocode возвращает координаты и ok=true, если адрес распознан (в пределах ограничения).
	Geocode(query string) (lat, lon float64, ok bool, err error)
}

// nominatim — клиент публичного/самохостингового Nominatim (OSM).
// Политика публичного Nominatim: ≤1 запрос/сек и валидный User-Agent — см. main (-geo-delay-ms, -geo-user-agent).
type nominatim struct {
	base    string
	http    *http.Client
	ua      string
	viewbox string // "lon_left,lat_top,lon_right,lat_bottom" — ограничение по Астане
	bounded bool
}

func NewNominatim(base, ua, viewbox string, bounded bool) *nominatim {
	if base == "" {
		base = "https://nominatim.openstreetmap.org"
	}
	if ua == "" {
		ua = "AshyqQala-stage0-audit/1.0 (+https://ashyqqala.kz)"
	}
	return &nominatim{
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

func (n *nominatim) Geocode(q string) (float64, float64, bool, error) {
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
	req, err := http.NewRequest(http.MethodGet, n.base+"/search?"+qs.Encode(), nil)
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
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return 0, 0, false, fmt.Errorf("nominatim HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	var rs []nominatimResult
	if err := json.Unmarshal(body, &rs); err != nil {
		return 0, 0, false, err
	}
	if len(rs) == 0 {
		return 0, 0, false, nil
	}
	lat, _ := strconv.ParseFloat(rs[0].Lat, 64)
	lon, _ := strconv.ParseFloat(rs[0].Lon, 64)
	return lat, lon, true, nil
}
