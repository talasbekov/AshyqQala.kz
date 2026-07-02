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

// maxRetries429 — число повторов HTTP 429 (Too Many Requests) прежде чем честно сдаться (Story 3.1 Task 2,
// долг 0.7 deferred-work.md:129). Не выдумывать матч на исчерпании — ok=false/err, как обычный отказ.
const maxRetries429 = 2

// retryAfterCap — потолок паузы по заголовку Retry-After (защита от неадекватно большого значения).
const retryAfterCap = 30 * time.Second

// Geocoder — абстракция геокодера (тестируемость: подменяется фейком без сети).
type Geocoder interface {
	// Geocode возвращает координаты и ok=true, если адрес распознан (в пределах ограничения).
	// Пустой/невалидный результат → (0,0,false,nil) — НЕ ошибка (честное «не сматчилось»).
	// ctx прерывает HTTP-запрос (отмена/таймаут долгого batch).
	Geocode(ctx context.Context, query string) (lat, lon float64, ok bool, err error)
}

// Match — детальный результат геокодирования (Story 3.1 Task 2): координаты + confidence (Nominatim
// importance, [0..1]). Confidence=nil, если геокодер её не отдал (частый случай self-host) — честное
// «неизвестно», НЕ выдуманный 0.0 (0.0 ⇒ importance РЕАЛЬНО пришла нулевой — код-ревью нашёл эту
// путаницу в первой версии: *float64, а не float64, чтобы «нет значения» отличалось от «значение ноль»,
// зеркало того же принципа, что уже применён к geom: НЕ выдумывать, честный NULL).
type Match struct {
	Lat, Lon   float64
	Confidence *float64
}

// Nominatim — клиент публичного/самохостингового Nominatim (OSM). Перенос stage0-audit/geocoder.go.
type Nominatim struct {
	base    string
	http    *http.Client
	ua      string
	viewbox string // "lon_left,lat_top,lon_right,lat_bottom" — ограничение по Астане
	bounded bool
	sleep   func(time.Duration) // 429-backoff; тесты того же пакета подменяют на быстрый фейк (whitebox)
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
		sleep:   time.Sleep,
	}
}

// Importance — указатель: JSON encoding/json различает "ключ отсутствует" (nil) от "ключ есть, значение 0"
// (указатель на 0.0) ТОЛЬКО через указатель на плоском float64. Self-host Nominatim часто не отдаёт
// importance вовсе (ключ отсутствует) — эта разница обязана дойти до Match.Confidence как nil, не 0.0.
type nominatimResult struct {
	Lat        string   `json:"lat"`
	Lon        string   `json:"lon"`
	Importance *float64 `json:"importance"`
}

// Geocode — Geocoder-интерфейс (обратная совместимость, 0.7 interim-geocode): те же координаты, без
// confidence. Поведение/сигнатура НЕ меняются — тонкая обёртка над GeocodeMatch.
func (n *Nominatim) Geocode(ctx context.Context, q string) (float64, float64, bool, error) {
	m, ok, err := n.GeocodeMatch(ctx, q)
	return m.Lat, m.Lon, ok, err
}

// GeocodeMatch — freeform /search?q=…&format=json&limit=1&countrycodes=kz (+ viewbox/bounded), С importance→
// confidence (Story 3.1 Task 2). HTTP 429 → до maxRetries429 повторов, пауза по заголовку Retry-After
// (retryAfterDuration); исчерпание попыток → честная ошибка (НЕ выдуманный матч). q НЕ нормализуется здесь —
// вызывающий подаёт уже нормализованную строку (NormalizeAddress), чтобы address_text=реально-отправленное.
func (n *Nominatim) GeocodeMatch(ctx context.Context, q string) (Match, bool, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries429; attempt++ {
		m, ok, retryAfter, err := n.doGeocode(ctx, q)
		if err == nil {
			return m, ok, nil
		}
		if retryAfter <= 0 {
			return Match{}, false, err // не-429 ошибка (сеть/HTTP/JSON) — честно наружу, без повтора
		}
		lastErr = err
		if attempt == maxRetries429 {
			break
		}
		if err := sleepCtx(ctx, retryAfter, n.sleep); err != nil {
			return Match{}, false, err // отмена/таймаут ПРЕРЫВАЕТ сон немедленно (код-ревью: раньше n.sleep
			// блокировал до retryAfterCap=30с даже на SIGTERM/Ctrl-C — sleepCtx слушает ctx.Done() параллельно)
		}
	}
	return Match{}, false, fmt.Errorf("nominatim: HTTP 429 после %d попыток: %w", maxRetries429+1, lastErr)
}

// doGeocode — один HTTP-запрос к Nominatim. retryAfter>0 ТОЛЬКО для HTTP 429 (сигнал вызывающему: стоит
// повторить); для любой другой ошибки/честного не-матча — 0 (не повторять).
func (n *Nominatim) doGeocode(ctx context.Context, q string) (Match, bool, time.Duration, error) {
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
		return Match{}, false, 0, err
	}
	req.Header.Set("User-Agent", n.ua) // политика Nominatim требует идентифицирующий UA
	req.Header.Set("Accept", "application/json")
	resp, err := n.http.Do(req)
	if err != nil {
		return Match{}, false, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // cap 1MB: limit=1 → тело крошечное; защита от гигантского ответа
	if err != nil {
		return Match{}, false, 0, fmt.Errorf("nominatim чтение тела: %w", err)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return Match{}, false, retryAfterDuration(resp.Header.Get("Retry-After")), fmt.Errorf("nominatim HTTP 429: %s", truncate(string(body), 200))
	}
	if resp.StatusCode != http.StatusOK {
		return Match{}, false, 0, fmt.Errorf("nominatim HTTP %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	var rs []nominatimResult
	if err := json.Unmarshal(body, &rs); err != nil {
		return Match{}, false, 0, err
	}
	if len(rs) == 0 {
		return Match{}, false, 0, nil // честно: адрес не сматчился (НЕ ошибка, НЕ выдуманная координата)
	}
	lat, err := strconv.ParseFloat(rs[0].Lat, 64)
	if err != nil {
		return Match{}, false, 0, fmt.Errorf("nominatim разбор lat %q: %w", rs[0].Lat, err)
	}
	lon, err := strconv.ParseFloat(rs[0].Lon, 64)
	if err != nil {
		return Match{}, false, 0, fmt.Errorf("nominatim разбор lon %q: %w", rs[0].Lon, err)
	}
	// Честность: НЕ финитная (NaN/Inf — ParseFloat их принимает!) или вне географического диапазона
	// координата — это НЕ валидный матч. Не выдумывать точку: → ok=false (как «не сматчилось»), не писать в БД.
	if math.IsNaN(lat) || math.IsInf(lat, 0) || math.IsNaN(lon) || math.IsInf(lon, 0) ||
		lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return Match{}, false, 0, nil
	}
	return Match{Lat: lat, Lon: lon, Confidence: rs[0].Importance}, true, 0, nil
}

// sleepCtx — прерываемый sleep: возвращает раньше, если ctx завершится (отмена/таймаут) — НЕ ждёт полный
// d вслепую (код-ревью нашёл: голый n.sleep(d) блокировал SIGTERM/Ctrl-C до retryAfterCap=30с). sleep —
// инжектируемая функция (тесты подменяют на быстрый фейк); реальная n.sleep=time.Sleep не умеет сама
// прерываться по ctx, поэтому она выполняется в горутине, а результат гонки решает select.
func sleepCtx(ctx context.Context, d time.Duration, sleep func(time.Duration)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	done := make(chan struct{})
	go func() {
		sleep(d)
		close(done)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

// retryAfterDuration — парсит заголовок Retry-After: секунды (частый случай Nominatim) или HTTP-дата.
// Пусто/невалидно/в прошлом → PoliteDelayDefault (честная дефолт-пауза, НЕ 0 — иначе повтор долбил бы
// немедленно). Капается retryAfterCap (защита от неадекватно большого значения).
func retryAfterDuration(v string) time.Duration {
	d := PoliteDelayDefault
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		d = time.Duration(secs) * time.Second
	} else if t, err := http.ParseTime(v); err == nil {
		if until := time.Until(t); until > 0 {
			d = until
		}
	}
	if d > retryAfterCap {
		d = retryAfterCap
	}
	if d <= 0 {
		d = PoliteDelayDefault
	}
	return d
}

// NormalizeAddress — нормализация адресной строки ПЕРЕД геокодингом (Story 3.1 Task 2, долг 0.7
// deferred-work.md:128): схлопывает повторные/краевые пробелы и прочие пробельные символы (таб/перевод
// строки) в единичные пробелы. Пустая/пробельная строка → "" (вызывающий трактует как «нечего геокодировать»,
// сеть не дёргается — честный unmatched, не ошибка).
func NormalizeAddress(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// truncate — обрезка строки до n рун (для безопасного лога тела ошибки). Перенос stage0-audit-хелпера.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
