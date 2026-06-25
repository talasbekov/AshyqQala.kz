package flags_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ashyqqala/server/internal/clock"
	"ashyqqala/server/internal/flags"
	"ashyqqala/server/internal/registry"
)

type goldenRNU struct {
	Params struct {
		Enabled            bool   `json:"enabled"`
		MethodologyVersion string `json:"methodology_version"`
		Now                string `json:"now"`
	} `json:"params"`
	Cases []struct {
		Name     string  `json:"name"`
		Start    *string `json:"start"` // null → nil (битая запись)
		End      *string `json:"end"`   // null → nil (открытая запись)
		Expected string  `json:"expected"`
	} `json:"cases"`
}

func parseDayUnix(t *testing.T, s string) *int64 {
	t.Helper()
	tm, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("парс даты %q: %v", s, err)
	}
	u := tm.Unix()
	return &u
}

// TestRNU_GoldenFixture — прогон дата-зависимого флага по golden-фикстуре с фиксированным «сейчас» (params.now):
// каждое состояние совпадает с пересчитанным вручную (raised/not_raised/insufficient_data, граница start/end ==
// now). Даты — ISO-строки (человекочитаемо, пересчитываемо третьим лицом); expected — литералы в JSON.
func TestRNU_GoldenFixture(t *testing.T) {
	path := filepath.Join("../../../", "fixtures", "golden", "flags", "rnu.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("чтение golden %s: %v", path, err)
	}
	var g goldenRNU
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatalf("парс golden: %v", err)
	}
	nowT, err := time.Parse("2006-01-02", g.Params.Now)
	if err != nil {
		t.Fatalf("парс params.now %q: %v", g.Params.Now, err)
	}
	params := flags.Params{MethodologyVersion: g.Params.MethodologyVersion, RNUEnabled: g.Params.Enabled}
	for _, c := range g.Cases {
		t.Run(c.Name, func(t *testing.T) {
			in := flags.Inputs{Now: clock.Fixed{T: nowT}, RNUGoszakupID: "RNU-G"}
			if c.Start != nil {
				in.RNUStartUnix = parseDayUnix(t, *c.Start)
			}
			if c.End != nil {
				in.RNUEndUnix = parseDayUnix(t, *c.End)
			}
			st, _ := flags.RNU(in, params)
			if st != registry.FlagState(c.Expected) {
				t.Fatalf("состояние = %s, ожидалось golden %s", st, c.Expected)
			}
		})
	}
}
