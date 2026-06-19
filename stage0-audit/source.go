package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
)

// Source — абстракция источника данных. Реализации: ows (API по токену),
// file (локальные дампы), scrape (публичный портал — добавляется отдельно).
// Ресурсы: journal | contract | lots | trd-buy | rnu | subject | acts.
type Source interface {
	Name() string
	Fetch(resource string, scopeBINs []string, max int, handle func(items []map[string]any) error) error
}

// ---- ows: REST API по токену ----

type owsSource struct{ c *Client }

func (s owsSource) Name() string { return "ows" }

func (s owsSource) Fetch(resource string, bins []string, max int, handle func([]map[string]any) error) error {
	wrap := func(raw json.RawMessage) (int, error) {
		items, err := decodeItems(raw)
		if err != nil {
			return 0, err
		}
		if err := handle(items); err != nil {
			return 0, err
		}
		return len(items), nil
	}
	q := url.Values{}
	q.Set("limit", "50")
	switch resource {
	case "journal":
		// /v2/journal — курсор инкрементального импорта (Epic 2, FR-1). В аудите Stage-0
		// не нужен; здесь только для подтверждения доступа (Story 0.1, AC2). Точный
		// контракт курсора/гранулярности/ретеншна — задача B-2 (Story 0.4), не этой истории.
		_, err := s.c.FetchAll("/journal", q, max, wrap)
		return err
	case "contract":
		if len(bins) > 0 {
			return s.byBins("/contract/customer/", bins, max, wrap)
		}
		_, err := s.c.FetchAll("/contract", q, max, wrap)
		return err
	case "lots":
		if len(bins) > 0 {
			return s.byBins("/lots/bin/", bins, max, wrap)
		}
		_, err := s.c.FetchAll("/lots", q, max, wrap)
		return err
	case "trd-buy":
		_, err := s.c.FetchAll("/trd-buy", q, max, wrap)
		return err
	case "rnu":
		_, err := s.c.FetchAll("/rnu", q, max, wrap)
		return err
	case "subject":
		// /subject — РЕЕСТР субъектов (юрлиц). Это НЕ участники конкретной закупки.
		// Число участников для флага FR-19 берётся ПОЛЕМ из trd-buy (см. models.go).
		_, err := s.c.FetchAll("/subject", q, max, wrap)
		return err
	case "acts":
		_, err := s.c.FetchAll("/acts", q, max, wrap)
		return err
	default:
		return fmt.Errorf("неизвестный ресурс %q", resource)
	}
}

func (s owsSource) byBins(prefix string, bins []string, max int, wrap func(json.RawMessage) (int, error)) error {
	total := 0
	for _, b := range bins {
		rem := 0
		if max > 0 {
			rem = max - total
			if rem <= 0 {
				break
			}
		}
		n, err := s.c.FetchAll(prefix+url.PathEscape(b), nil, rem, wrap)
		total += n
		if err != nil {
			return err
		}
	}
	return nil
}

// ---- file: локальные дампы <dir>/<resource>.json (массив объектов) ----

type fileSource struct{ dir string }

func (s fileSource) Name() string { return "file:" + s.dir }

func (s fileSource) Fetch(resource string, _ []string, max int, handle func([]map[string]any) error) error {
	fname := filepath.Join(s.dir, resource+".json")
	data, err := os.ReadFile(fname)
	if err != nil {
		return err
	}
	var items []map[string]any
	if err := json.Unmarshal(data, &items); err != nil {
		return fmt.Errorf("разбор %s: %w", fname, err)
	}
	if max > 0 && len(items) > max {
		items = items[:max]
	}
	return handle(items)
}
