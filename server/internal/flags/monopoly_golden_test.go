package flags_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"ashyqqala/server/internal/flags"
	"ashyqqala/server/internal/registry"
)

type goldenMonopoly struct {
	Params struct {
		ConcentrationShare float64 `json:"concentration_share"`
		MinGroupContracts  int     `json:"min_group_contracts"`
		MethodologyVersion string  `json:"methodology_version"`
	} `json:"params"`
	Cases []struct {
		Name           string `json:"name"`
		SupplierSum    *int64 `json:"supplier_sum"`    // null → nil (нет данных)
		GroupTotalSum  *int64 `json:"group_total_sum"` // null → nil (нет знаменателя)
		GroupContracts int    `json:"group_contracts"`
		Resolved       bool   `json:"resolved"`
		Expected       string `json:"expected"`
	} `json:"cases"`
}

// TestMonopoly_GoldenFixture — прогон чистого флага по golden-фикстуре: каждое состояние совпадает с
// пересчитанным вручную (raised/not_raised/insufficient_data, граница ровно 0.5 инклюзивно). Пересчитываемость
// третьим лицом: expected — литералы в JSON (не вычисляются формулой в тесте).
func TestMonopoly_GoldenFixture(t *testing.T) {
	path := filepath.Join("../../../", "fixtures", "golden", "flags", "monopoly.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("чтение golden %s: %v", path, err)
	}
	var g goldenMonopoly
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatalf("парс golden: %v", err)
	}
	params := flags.Params{
		MethodologyVersion:         g.Params.MethodologyVersion,
		MonopolyConcentrationShare: g.Params.ConcentrationShare,
		MonopolyMinGroupContracts:  g.Params.MinGroupContracts,
	}
	for _, c := range g.Cases {
		t.Run(c.Name, func(t *testing.T) {
			in := flags.Inputs{
				SupplierSum:         c.SupplierSum,
				GroupTotalSum:       c.GroupTotalSum,
				GroupContracts:      c.GroupContracts,
				SupplierBINResolved: c.Resolved,
				SupplierBIN:         "123456789012",
				ComparabilityKey:    "direction=road|kato=710000000",
			}
			st, _ := flags.Monopoly(in, params)
			if st != registry.FlagState(c.Expected) {
				t.Fatalf("состояние = %s, ожидалось golden %s", st, c.Expected)
			}
		})
	}
}
