package export_test

import (
	"encoding/json"
	"strings"
	"testing"

	"ashyqqala/server/internal/export"
)

// TestExport_RoundTripAndNeutral — экспорт даёт валидный канонический JSON (round-trip, evidence сохранён) +
// печатный текст с нейтральной рамкой и фактами, без оценочных (taboo) слов. AR-29 пересчитываемость.
func TestExport_RoundTripAndNeutral(t *testing.T) {
	f := export.FlagRecord{
		FlagType:           "price_per_km",
		SubjectType:        "contract",
		SubjectID:          42,
		MethodologyVersion: "v1.0",
		Evidence:           json.RawMessage(`{"price_per_km":2000000,"median":1000000,"deviation_factor":1.5}`),
	}
	jb, printable, err := export.Export(f, "сигнал, требующий проверки")
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	var back map[string]any
	if err := json.Unmarshal(jb, &back); err != nil {
		t.Fatalf("экспортный JSON невалиден: %v", err)
	}
	if back["flag_type"] != "price_per_km" || back["methodology_version"] != "v1.0" || back["subject_type"] != "contract" {
		t.Errorf("JSON потерял контекст: %v", back)
	}
	if back["evidence"] == nil {
		t.Errorf("evidence не экспортирован")
	}

	if !strings.Contains(printable, "сигнал, требующий проверки") {
		t.Errorf("печатный без нейтральной рамки: %q", printable)
	}
	for _, fact := range []string{"price_per_km", "v1.0", "contract", "#42"} {
		if !strings.Contains(printable, fact) {
			t.Errorf("печатный не содержит факт %q: %q", fact, printable)
		}
	}
	for _, taboo := range []string{"нарушение", "коррупц", "виновен", "преступл"} {
		if strings.Contains(strings.ToLower(printable), taboo) {
			t.Errorf("печатный содержит оценочное слово %q: %q", taboo, printable)
		}
	}
}
