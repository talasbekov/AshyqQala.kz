package main

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// scrapeSource — фолбэк через ПУБЛИЧНЫЙ портал goszakup.gov.kz (без токена).
// ВНИМАНИЕ: парсинг идёт вразрез с принципом «официальный канал, не парсинг»
// (см. README и конкурсный документ §6.1/§6.4) — использовать только для разовой выборки.
// Публичный портал отдаёт ТОЛЬКО список лотов (поиск по КАТО); записи договоров,
// участников и РНУ без авторизации недоступны (нужен токен ows_v2).
type scrapeSource struct {
	base     string // https://goszakup.gov.kz/ru
	http     *http.Client
	ua       string
	delay    time.Duration
	maxPages int
}

func NewScrape(base, ua string, delay time.Duration, maxPages int) *scrapeSource {
	if base == "" {
		base = "https://goszakup.gov.kz/ru"
	}
	if ua == "" {
		ua = "AshyqQala-stage0-audit/1.0"
	}
	if maxPages <= 0 {
		maxPages = 50
	}
	return &scrapeSource{
		base:     strings.TrimRight(base, "/"),
		http:     &http.Client{Timeout: 40 * time.Second},
		ua:       ua,
		delay:    delay,
		maxPages: maxPages,
	}
}

func (s *scrapeSource) Name() string { return "scrape:goszakup-public" }

var (
	reResultTable = regexp.MustCompile(`(?s)<table[^>]*id="search-result"[^>]*>(.*?)</table>`)
	reRow         = regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)
	reCell        = regexp.MustCompile(`(?s)<td[^>]*>(.*?)</td>`)
	reTag         = regexp.MustCompile(`(?s)<[^>]*>`)
	reWS          = regexp.MustCompile(`\s+`)
	reAnno        = regexp.MustCompile(`\d{6,}-\d+`)
	reMoneyClean  = regexp.MustCompile(`[^\d,.\-]`)
)

func (s *scrapeSource) get(path string, q url.Values) (string, error) {
	u := s.base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", s.ua)
	resp, err := s.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d на %s", resp.StatusCode, u)
	}
	return string(body), nil
}

func cellText(cellHTML string) string {
	t := reTag.ReplaceAllString(cellHTML, " ")
	t = html.UnescapeString(t)
	return strings.TrimSpace(reWS.ReplaceAllString(t, " "))
}

func parseMoney(s string) float64 {
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, " ", "")
	s = reMoneyClean.ReplaceAllString(s, "")
	if strings.Count(s, ",") > 0 && strings.Count(s, ".") > 0 {
		s = strings.ReplaceAll(s, ",", "") // запятая = разделитель тысяч
	} else if strings.Count(s, ",") == 1 && strings.Count(s, ".") == 0 {
		s = strings.ReplaceAll(s, ",", ".") // запятая = десятичный разделитель
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// astanaKatos берёт коды КАТО Астаны из справочника портала (фолбэк — код города).
func (s *scrapeSource) astanaKatos() []string {
	if body, err := s.get("/search/getKato", url.Values{"name": {"Астана"}}); err == nil {
		var r struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		}
		if json.Unmarshal([]byte(body), &r) == nil && len(r.Items) > 0 {
			out := make([]string, 0, len(r.Items))
			for _, it := range r.Items {
				if it.ID != "" {
					out = append(out, it.ID)
				}
			}
			return out
		}
	}
	return []string{"710000000"}
}

func (s *scrapeSource) Fetch(resource string, _ []string, max int, handle func([]map[string]any) error) error {
	if resource != "lots" {
		return fmt.Errorf("ресурс %q недоступен на публичном портале без авторизации (нужен токен ows_v2)", resource)
	}
	// По голому КАТО портал отдаёт в основном мелкие «прочие» лоты. Для аудита важны
	// дороги/вода (OQ-4), поэтому тянем целевым поиском по ключевым словам направлений.
	const kato = "710000000"                                                        // г.Астана (коды см. /search/getKato → astanaKatos)
	terms := []string{"дорог", "водоснабж", "автодорог", "водопровод", "канализац"} // чередуем дорогу/воду
	total := 0
	seen := map[string]bool{}
	exhausted := map[string]bool{}
	for page := 1; page <= s.maxPages; page++ { // round-robin: страница P по каждому термину, затем P+1
		active := 0
		for _, term := range terms {
			if exhausted[term] {
				continue
			}
			if max > 0 && total >= max {
				return nil
			}
			body, err := s.get("/search/lots", url.Values{
				"filter[kato]": {kato}, "filter[name]": {term}, "page": {strconv.Itoa(page)},
			})
			if err != nil {
				return err
			}
			rows := s.parseRows(body, kato, seen)
			if len(rows) == 0 {
				exhausted[term] = true
				continue
			}
			active++
			if max > 0 && total+len(rows) > max {
				rows = rows[:max-total]
			}
			total += len(rows)
			if err := handle(rows); err != nil {
				return err
			}
			if s.delay > 0 {
				time.Sleep(s.delay)
			}
		}
		if active == 0 {
			break
		}
	}
	return nil
}

func (s *scrapeSource) parseRows(body, kato string, seen map[string]bool) []map[string]any {
	mt := reResultTable.FindStringSubmatch(body)
	if mt == nil {
		return nil
	}
	var out []map[string]any
	for _, rm := range reRow.FindAllStringSubmatch(mt[1], -1) {
		cells := reCell.FindAllStringSubmatch(rm[1], -1)
		if len(cells) < 5 {
			continue // заголовок или служебная строка (нет <td>)
		}
		txt := make([]string, len(cells))
		for i, c := range cells {
			txt[i] = cellText(c[1])
		}
		// Колонки строки лота: [0]=№лота+объявление, [1]=наименование лота, [2]=кол-во, [3]=сумма, [4]=способ, [5]=статус
		name := strings.TrimSpace(strings.TrimSuffix(txt[1], "История"))
		if name == "" {
			name = txt[0]
		}
		anno := reAnno.FindString(txt[0])
		key := anno + "|" + name
		if name == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, map[string]any{
			"name_ru":             name,
			"amount":              parseMoney(txt[3]),
			"ref_kato":            kato,
			"trd_buy_number_anno": anno,
		})
	}
	return out
}
