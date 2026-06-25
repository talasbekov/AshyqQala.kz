package flags_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"ashyqqala/server/internal/flags"
	"ashyqqala/server/internal/registry"
)

// goldenSingleParticipant — форма fixtures/golden/flags/single_participant.json (пересчитываемый вручную артефакт).
type goldenSingleParticipant struct {
	Params struct {
		Enabled            bool     `json:"enabled"`
		ExcludeMethods     []string `json:"exclude_methods"`
		MethodologyVersion string   `json:"methodology_version"`
	} `json:"params"`
	Cases []struct {
		Name              string `json:"name"`
		ParticipantCount  *int   `json:"participant_count"` // null → nil (нет данных)
		ProcurementMethod string `json:"procurement_method"`
		Expected          string `json:"expected"`
	} `json:"cases"`
}

// TestSingleParticipant_GoldenFixture — прогон чистого флага по golden-фикстуре: каждое состояние совпадает
// с пересчитанным вручную (raised/not_raised/insufficient_data). Доказывает пересчитываемость третьим лицом
// (несущий гардрейл) и фиксирует семантику FR-19 человекочитаемым артефактом.
func TestSingleParticipant_GoldenFixture(t *testing.T) {
	path := filepath.Join("../../../", "fixtures", "golden", "flags", "single_participant.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("чтение golden %s: %v", path, err)
	}
	var g goldenSingleParticipant
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatalf("парс golden: %v", err)
	}
	params := flags.Params{
		MethodologyVersion:              g.Params.MethodologyVersion,
		SingleParticipantEnabled:        g.Params.Enabled,
		SingleParticipantExcludeMethods: g.Params.ExcludeMethods,
	}
	for _, c := range g.Cases {
		t.Run(c.Name, func(t *testing.T) {
			st, _ := flags.SingleParticipant(flags.Inputs{ParticipantCount: c.ParticipantCount, ProcurementMethod: c.ProcurementMethod}, params)
			if st != registry.FlagState(c.Expected) {
				t.Fatalf("состояние = %s, ожидалось golden %s", st, c.Expected)
			}
		})
	}
}
