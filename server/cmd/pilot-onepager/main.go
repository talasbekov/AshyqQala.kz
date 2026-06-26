// Command pilot-onepager — генератор pilot-one-pager (Story 5.6 AC-3): метрики этапа 5 (N/M/X из проекции) +
// вердикт Stage-0 (Epic 0) → нейтральный Markdown в docs/ops/. Артефакт-генератор (НЕ публичный эндпоинт;
// defer 1.1: метрики наружу не светим). Токен-независим: работает против синтетической/интерим-БД.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/store/gen"
)

// verdictFile — подмножество docs/ops/stage0-verdict-*.json (struct Verdict живёт в отдельном модуле stage0-audit).
type verdictFile struct {
	MethodologyVersion string  `json:"methodology_version"`
	DataSource         string  `json:"data_source"`
	GeoGateThreshold   float64 `json:"geo_gate_threshold"`
	Oq1Volume          struct {
		ContractsInWindow int64 `json:"contracts_in_window"`
	} `json:"oq1_volume"`
	Oq4GeoCoverage struct {
		Coverage *float64 `json:"coverage"`
		State    string   `json:"state"`
	} `json:"oq4_geo_coverage"`
	Verdict       string `json:"verdict"`
	VerdictReason string `json:"verdict_reason"`
}

func main() {
	verdictPath := flag.String("verdict", "docs/ops/stage0-verdict-20260620.json", "путь к артефакту вердикта Stage-0")
	out := flag.String("out", "", "путь Markdown-вывода (по умолчанию docs/ops/pilot-onepager-<date>.md)")
	dateArg := flag.String("date", "", "дата генерации YYYY-MM-DD (по умолчанию сегодня)")
	flag.Parse()

	date := *dateArg
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	vs, err := readVerdict(*verdictPath)
	if err != nil {
		log.Fatalf("pilot-onepager: вердикт: %v", err)
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("pilot-onepager: DATABASE_URL пуст")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("pilot-onepager: подключение к БД: %v", err)
	}
	defer pool.Close()

	m, err := readMetrics(ctx, gen.New(pool))
	if err != nil {
		log.Fatalf("pilot-onepager: метрики: %v", err)
	}

	md := renderOnePager(m, vs, date)
	outPath := *out
	if outPath == "" {
		outPath = "docs/ops/pilot-onepager-" + date + ".md"
	}
	if err := os.WriteFile(outPath, []byte(md), 0o644); err != nil {
		log.Fatalf("pilot-onepager: запись %s: %v", outPath, err)
	}
	fmt.Println("pilot-onepager: записан", outPath)
}

// readVerdict — парсит закоммиченный артефакт вердикта Stage-0 в выжимку для one-pager (без БД, токен-независимо).
func readVerdict(path string) (VerdictSummary, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return VerdictSummary{}, err
	}
	var vf verdictFile
	if err := json.Unmarshal(b, &vf); err != nil {
		return VerdictSummary{}, fmt.Errorf("разбор вердикта: %w", err)
	}
	return VerdictSummary{
		Verdict:            vf.Verdict,
		Reason:             vf.VerdictReason,
		GeoGateThreshold:   vf.GeoGateThreshold,
		GeoCoverage:        vf.Oq4GeoCoverage.Coverage,
		GeoState:           vf.Oq4GeoCoverage.State,
		ContractsInWindow:  vf.Oq1Volume.ContractsInWindow,
		DataSource:         vf.DataSource,
		MethodologyVersion: vf.MethodologyVersion,
	}, nil
}

// metricsQuerier — узкий интерфейс к count-запросам проекции (реализует *gen.Queries; мок в тестах).
type metricsQuerier interface {
	CountMarkedContracts(ctx context.Context) (int64, error)
	CountActiveFlags(ctx context.Context) (int64, error)
	CountRecomputableFlags(ctx context.Context) (int64, error)
}

// readMetrics — агрегирует N/M/X из проекции. Ошибка любого count'а → честная ошибка (не фабрикованный 0).
func readMetrics(ctx context.Context, q metricsQuerier) (Metrics, error) {
	var m Metrics
	var err error
	if m.Marked, err = q.CountMarkedContracts(ctx); err != nil {
		return Metrics{}, err
	}
	if m.ActiveFlags, err = q.CountActiveFlags(ctx); err != nil {
		return Metrics{}, err
	}
	if m.Recomputable, err = q.CountRecomputableFlags(ctx); err != nil {
		return Metrics{}, err
	}
	return m, nil
}
