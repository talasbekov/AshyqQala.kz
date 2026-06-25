package benchmark_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ashyqqala/server/internal/benchmark"
	"ashyqqala/server/internal/clock"
	"ashyqqala/server/internal/registry"
)

// goldenMedian — форма fixtures/golden/medians/*.json (пересчитываемый вручную артефакт OK-пути).
type goldenMedian struct {
	Group struct {
		Direction string `json:"direction"`
		Kato      string `json:"kato"`
	} `json:"group"`
	ReferenceNow string `json:"reference_now"`
	WindowMonths int    `json:"window_months"`
	Samples      []struct {
		PricePerKM int64  `json:"price_per_km"`
		SignDate   string `json:"sign_date"`
	} `json:"samples"`
	Expected struct {
		MedianPricePerKM int64  `json:"median_price_per_km"`
		State            string `json:"state"`
		SampleSize       int    `json:"sample_size"`
	} `json:"expected"`
}

// TestGroupMedian_GoldenFixture — прогон чистого движка по golden-фикстуре: результат БИТ-В-БИТ совпадает
// с пересчитанным вручную (median 1800000, ok, n=5; выборка вне 24-мес окна отброшена). Доказывает
// пересчитываемость третьим лицом (несущий гардрейл) и стабильность формата группы/окна.
func TestGroupMedian_GoldenFixture(t *testing.T) {
	path := filepath.Join("../../../", "fixtures", "golden", "medians", "road_astana_24mo.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("чтение golden %s: %v", path, err)
	}
	var g goldenMedian
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatalf("парс golden: %v", err)
	}

	refNow, err := time.Parse(time.RFC3339, g.ReferenceNow)
	if err != nil {
		t.Fatalf("reference_now %q: %v", g.ReferenceNow, err)
	}
	samples := make([]benchmark.Sample, len(g.Samples))
	for i, s := range g.Samples {
		d, err := time.Parse("2006-01-02", s.SignDate)
		if err != nil {
			t.Fatalf("sign_date %q: %v", s.SignDate, err)
		}
		samples[i] = benchmark.Sample{PricePerKM: s.PricePerKM, SignDateUnix: d.Unix()}
	}

	v, st, size := benchmark.GroupMedian(samples, g.WindowMonths, clock.Fixed{T: refNow})
	if g.Expected.State != "ok" || st != registry.StateOK {
		t.Fatalf("state = %s, ожидалось ok (golden)", st)
	}
	if v == nil || *v != g.Expected.MedianPricePerKM {
		t.Fatalf("median = %v, ожидалось golden %d", v, g.Expected.MedianPricePerKM)
	}
	if size != g.Expected.SampleSize {
		t.Fatalf("sample_size = %d, ожидалось golden %d (выборка вне окна должна быть отброшена)", size, g.Expected.SampleSize)
	}
}
