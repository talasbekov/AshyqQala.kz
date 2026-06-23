//go:build scrape

package scrape

import (
	"crypto/sha256"
	"encoding/hex"
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

// AstanaKATO — код КАТО г.Астана (фолбэк справочника /search/getKato). Перенос из stage0-audit.
const AstanaKATO = "710000000"

// politeDelayDefault — вежливая пауза между запросами к порталу (политика ~1 req/sec). Из stage0-audit.
const politeDelayDefault = 1100 * time.Millisecond

// RecordKeys — НАБОР полей, который эмитит scrape-источник для каждого лота (контракт для
// decode.SchemaHash). Синтетический "id" добавлен как СТАБИЛЬНЫЙ natural-ключ (портал не отдаёт
// lot_id) → decode.DecodeLot заполняет GoszakupLotID, UPSERT идемпотентен по нему. Дрейф набора → ErrSchemaDrift.
var RecordKeys = []string{"id", "name_ru", "amount", "ref_kato", "trd_buy_number_anno"}

// Source — ВРЕМЕННЫЙ источник лотов через публичный портал (без токена). СТРУКТУРНО удовлетворяет
// ashyqqala/server/internal/goszakup.Source (тот же Name()+Fetch), но НЕ импортирует его — изоляция
// парсера + точка swap scrape→ows один флагом (AC3). Перенос stage0-audit/scrape.go.
type Source struct {
	base     string
	http     *http.Client
	ua       string
	delay    time.Duration
	maxPages int
}

// New — конструктор. Пустые аргументы → дефолты вежливости/портала (зеркалят stage0-audit).
func New(base, ua string, delay time.Duration, maxPages int) *Source {
	if base == "" {
		base = "https://goszakup.gov.kz/ru"
	}
	if ua == "" {
		ua = "AshyqQala-interim-scrape/0.6"
	}
	if delay <= 0 {
		delay = politeDelayDefault
	}
	if maxPages <= 0 {
		maxPages = 50
	}
	return &Source{
		base:     strings.TrimRight(base, "/"),
		http:     &http.Client{Timeout: 40 * time.Second},
		ua:       ua,
		delay:    delay,
		maxPages: maxPages,
	}
}

func (s *Source) Name() string { return "scrape:goszakup-public" }

var (
	reResultTable = regexp.MustCompile(`(?s)<table[^>]*\sid="search-result"[^>]*>(.*?)</table>`) // \s перед id=: не матчить data-id="search-result"
	reRow         = regexp.MustCompile(`(?s)<tr[^>]*>(.*?)</tr>`)
	reCell        = regexp.MustCompile(`(?s)<td[^>]*>(.*?)</td>`)
	reTag         = regexp.MustCompile(`(?s)<[^>]*>`)
	reWS          = regexp.MustCompile(`\s+`)
	reAnno        = regexp.MustCompile(`\d{6,}-\d+`)
	reMoneyClean  = regexp.MustCompile(`[^\d,.\-]`)
)

func (s *Source) get(path string, q url.Values) (string, error) {
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
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d на %s", resp.StatusCode, u)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		// Не глотать: усечённое тело → молча меньше строк → ложное «exhausted».
		return "", fmt.Errorf("чтение тела %s: %w", u, err)
	}
	return string(body), nil
}

func cellText(cellHTML string) string {
	t := reTag.ReplaceAllString(cellHTML, " ")
	t = html.UnescapeString(t)
	return strings.TrimSpace(reWS.ReplaceAllString(t, " "))
}

// parseMoney — «240 000 000,00» → (240000000.0, true). При нераспознаваемом входе (пусто, «-»,
// нестандартный формат) → (0, false): вызывающий эмитит amount=nil (→ NULL), а НЕ 0 — гардрейл
// честности («нет данных» ≠ бесплатно). Из stage0-audit, но с явным ok вместо проглоченной ошибки.
func parseMoney(s string) (float64, bool) {
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, " ", "")
	s = reMoneyClean.ReplaceAllString(s, "")
	if strings.Count(s, ",") > 0 && strings.Count(s, ".") > 0 {
		s = strings.ReplaceAll(s, ",", "") // запятая = разделитель тысяч
	} else if strings.Count(s, ",") == 1 && strings.Count(s, ".") == 0 {
		s = strings.ReplaceAll(s, ",", ".") // запятая = десятичный разделитель
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// Fetch — ТОЛЬКО ресурс "lots" (договоры/участники/РНУ требуют токен → честная ошибка). Сигнатура
// идентична goszakup.Source.Fetch (точка swap). scopeBINs игнорируется (фильтрация по БИН — забота ows).
func (s *Source) Fetch(resource string, _ []string, max int, handle func(items []map[string]any) error) error {
	if resource != "lots" {
		return fmt.Errorf("ресурс %q недоступен на публичном портале без авторизации (нужен токен ows_v2)", resource)
	}
	// По голому КАТО портал отдаёт в основном мелкие «прочие» лоты. Для дорог/воды (OQ-4) тянем
	// целевым поиском по ключевым словам направлений. ВЫБОРКА KEYWORD-СМЕЩЕНА (метить «предв.»).
	const kato = AstanaKATO                                                         // г.Астана (код города; подкоды КАТО — фолбэк к нему)
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

// parseRows парсит таблицу #search-result в decode-совместимые записи лота. Каждая запись несёт
// СИНТЕТИЧЕСКИЙ стабильный "id" (натуральный ключ для UPSERT — портал реального lot_id не отдаёт).
func (s *Source) parseRows(body, kato string, seen map[string]bool) []map[string]any {
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
		// Колонки лота: [0]=№лота+объявление, [1]=наименование, [2]=кол-во, [3]=сумма, [4]=способ, [5]=статус
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
		// Нераспознанная сумма → amount=nil (→ NULL), НЕ 0: честность над домыслом. Ключ "amount"
		// ОСТАЁТСЯ при любом исходе (schema_hash по НАБОРУ ключей, не значений → стабилен).
		var amount any
		if amt, ok := parseMoney(txt[3]); ok {
			amount = amt
		}
		out = append(out, map[string]any{
			"id":                  syntheticLotID(anno, name),
			"name_ru":             name,
			"amount":              amount,
			"ref_kato":            kato,
			"trd_buy_number_anno": anno,
		})
	}
	return out
}

// syntheticLotID — СТАБИЛЬНЫЙ natural-ключ scraped-лота: портал не отдаёт lot_id, поэтому ключ
// детерминирован из (объявление|наименование) — тот же ключ дедупа scrape. Повторный прогон → тот же
// id → UPSERT не плодит дубли (идемпотентность, AC1/Task 3). Префикс "scrape-" отделяет от боевых ows-id.
// 32 hex (128 бит) — birthday-коллизия исчезающе мала (vs 64 бита: тихая перезапись лота через UPSERT).
func syntheticLotID(anno, name string) string {
	sum := sha256.Sum256([]byte(anno + "\x00" + name))
	return "scrape-" + hex.EncodeToString(sum[:])[:32]
}
