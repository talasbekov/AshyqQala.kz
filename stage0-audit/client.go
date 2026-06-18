package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client — минимальный клиент ows_v2: Bearer-аутентификация + пагинация.
type Client struct {
	baseURL  string
	host     string // scheme://host — для разрешения относительных next_page
	token    string
	http     *http.Client
	delay    time.Duration
	maxPages int
}

func NewClient(cfg Config) *Client {
	base := strings.TrimRight(cfg.BaseURL, "/")
	host := base
	if u, err := url.Parse(cfg.BaseURL); err == nil && u.Host != "" {
		host = u.Scheme + "://" + u.Host
	}
	return &Client{
		baseURL:  base,
		host:     host,
		token:    cfg.Token,
		http:     &http.Client{Timeout: 30 * time.Second},
		delay:    cfg.RequestDelay,
		maxPages: cfg.MaxPages,
	}
}

// pagedResponse — конверт ответа ows_v2: total / next_page / items.
type pagedResponse struct {
	Total    int             `json:"total"`
	NextPage string          `json:"next_page"`
	Items    json.RawMessage `json:"items"`
}

func (c *Client) getPage(rawURL string) (*pagedResponse, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s -> HTTP %d: %s", rawURL, resp.StatusCode, truncate(string(body), 300))
	}
	var pr pagedResponse
	if err := json.Unmarshal(body, &pr); err != nil {
		return nil, fmt.Errorf("decode %s: %w (тело: %s)", rawURL, err, truncate(string(body), 300))
	}
	return &pr, nil
}

// FetchAll проходит по страницам эндпоинта, вызывая handle на каждый блок items.
// Останавливается на maxRecords (0 = без лимита) или когда next_page пуст.
func (c *Client) FetchAll(path string, query url.Values, maxRecords int, handle func(items json.RawMessage) (int, error)) (int, error) {
	next := c.baseURL + path
	if len(query) > 0 {
		next += "?" + query.Encode()
	}
	total := 0
	for page := 0; next != "" && page < c.maxPages; page++ {
		pr, err := c.getPage(next)
		if err != nil {
			return total, err
		}
		n, err := handle(pr.Items)
		if err != nil {
			return total, err
		}
		total += n
		if maxRecords > 0 && total >= maxRecords {
			break
		}
		next = resolveNext(c.host, pr.NextPage)
		if c.delay > 0 {
			time.Sleep(c.delay)
		}
	}
	return total, nil
}

func resolveNext(host, next string) string {
	if next == "" {
		return ""
	}
	if strings.HasPrefix(next, "http://") || strings.HasPrefix(next, "https://") {
		return next
	}
	if strings.HasPrefix(next, "/") {
		return host + next
	}
	return host + "/" + next
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
