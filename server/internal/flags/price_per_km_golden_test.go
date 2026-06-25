package flags_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"ashyqqala/server/internal/flags"
	"ashyqqala/server/internal/registry"
)

type goldenPricePerKM struct {
	Params struct {
		DeviationFactor    float64 `json:"deviation_factor"`
		MinSample          int     `json:"min_sample"`
		MethodologyVersion string  `json:"methodology_version"`
	} `json:"params"`
	Cases []struct {
		Name       string `json:"name"`
		PricePerKM *int64 `json:"price_per_km"` // null → nil (не вычислима)
		Median     *int64 `json:"median"`       // null → nil (недостаточно)
		SampleSize int    `json:"sample_size"`
		Expected   string `json:"expected"`
	} `json:"cases"`
}

// TestPricePerKM_GoldenFixture — прогон чистого флага по golden-фикстуре: каждое состояние совпадает с
// пересчитанным вручную (raised/not_raised/insufficient_data, граница ровно ×1.5). Пересчитываемость третьим лицом.
func TestPricePerKM_GoldenFixture(t *testing.T) {
	path := filepath.Join("../../../", "fixtures", "golden", "flags", "price_per_km.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("чтение golden %s: %v", path, err)
	}
	var g goldenPricePerKM
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatalf("парс golden: %v", err)
	}
	params := flags.Params{
		MethodologyVersion:        g.Params.MethodologyVersion,
		MinSample:                 g.Params.MinSample,
		PricePerKMDeviationFactor: g.Params.DeviationFactor,
	}
	for _, c := range g.Cases {
		t.Run(c.Name, func(t *testing.T) {
			in := flags.Inputs{PricePerKM: c.PricePerKM, GroupMedian: c.Median, GroupSampleSize: c.SampleSize, ComparabilityKey: "direction=road|kato=710000000"}
			st, _ := flags.PricePerKM(in, params)
			if st != registry.FlagState(c.Expected) {
				t.Fatalf("состояние = %s, ожидалось golden %s", st, c.Expected)
			}
		})
	}
}
