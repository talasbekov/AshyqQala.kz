package methodology_test

import (
	"path/filepath"
	"testing"

	"ashyqqala/server/internal/methodology"
)

// TestFlatten_RealParams — flatten реального v1.0 YAML даёт ожидаемые плоские пороги (пин-литералы; drift → красный).
// Это «источник истины» для seed в methodology_params (Story 4.6) и перекрёстного инварианта YAML↔DB.
func TestFlatten_RealParams(t *testing.T) {
	p, err := methodology.Load(filepath.Join("../../../", "registry"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := methodology.Flatten(p)
	want := map[string]string{
		"median.min_sample":                       "5",
		"median.comparability_window_months":      "24",
		"flag.price_per_km.deviation_factor":      "1.5",
		"flag.monopoly.concentration_share":       "0.5",
		"flag.monopoly.min_group_contracts":       "5",
		"flag.single_participant.enabled":         "true",
		"flag.single_participant.exclude_methods": "из_одного_источника",
		"flag.rnu.enabled":                        "true",
	}
	if len(got) != len(want) {
		t.Fatalf("flatten дал %d порогов, ожидалось %d: %+v", len(got), len(want), got)
	}
	seen := map[string]bool{}
	for _, kv := range got {
		w, ok := want[kv.Key]
		if !ok {
			t.Errorf("неожиданный ключ %q", kv.Key)
			continue
		}
		if kv.Value != w {
			t.Errorf("%q = %q, ожидалось %q", kv.Key, kv.Value, w)
		}
		seen[kv.Key] = true
	}
	for k := range want {
		if !seen[k] {
			t.Errorf("отсутствует ключ %q", k)
		}
	}
}
