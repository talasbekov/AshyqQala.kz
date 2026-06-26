package export_test

import (
	"encoding/json"
	"strings"
	"testing"

	"ashyqqala/server/internal/export"
)

// TestExport_RoundTripAndNeutral — экспорт даёт валидный канонический JSON (round-trip, evidence сохранён) +
// печатный текст с нейтральной рамкой и фактами, без оценочных (taboo) слов. AR-29 пересчитываемость.
// subject_ref — ПУБЛИЧНЫЙ goszakup-id (не внутренний bigint).
func TestExport_RoundTripAndNeutral(t *testing.T) {
	f := export.FlagRecord{
		FlagType:           "price_per_km",
		SubjectType:        "contract",
		SubjectRef:         "DEMO-0001",
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
	if back["subject_ref"] != "DEMO-0001" {
		t.Errorf("subject_ref должен быть публичным goszakup-id, got %v", back["subject_ref"])
	}
	if _, hasInternal := back["subject_id"]; hasInternal {
		t.Errorf("внутренний subject_id НЕ должен попадать в экспорт (wire-конвенция): %v", back)
	}
	if back["evidence"] == nil {
		t.Errorf("evidence не экспортирован")
	}

	if !strings.Contains(printable, "сигнал, требующий проверки") {
		t.Errorf("печатный без нейтральной рамки: %q", printable)
	}
	for _, fact := range []string{"price_per_km", "v1.0", "contract", "DEMO-0001"} {
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

// TestExport_EmptyOrInvalidEvidence_HonestError — честная деградация (§7.4): экспортировать нечего, если
// evidence пуст или невалиден → error, НЕ фабрикованный документ. Negative-control defensive json.Valid (defer 4.6).
func TestExport_EmptyOrInvalidEvidence_HonestError(t *testing.T) {
	base := export.FlagRecord{FlagType: "price_per_km", SubjectType: "contract", SubjectRef: "DEMO-0001", MethodologyVersion: "v1.0"}

	// Вырожденный/непустой-НЕ-объект evidence → error (ловит `{}`/`null`/массив/скаляр/битый/пустой).
	for _, ev := range []json.RawMessage{
		nil, json.RawMessage(``), json.RawMessage(`{не json`),
		json.RawMessage(`{}`), json.RawMessage(`null`), json.RawMessage(`[]`), json.RawMessage(`5`),
	} {
		f := base
		f.Evidence = ev
		if _, _, err := export.Export(f, "сигнал, требующий проверки"); err == nil {
			t.Errorf("вырожденный evidence %q должен дать error (нечего цитировать), got nil", string(ev))
		}
	}
	// Пустая methodology_version → error (без версии пересчёт невозможен; согласовано с X-гейтом).
	noVer := base
	noVer.MethodologyVersion = ""
	noVer.Evidence = json.RawMessage(`{"x":1}`)
	if _, _, err := export.Export(noVer, "сигнал, требующий проверки"); err == nil {
		t.Error("пустая methodology_version должна дать error (не пересчитываемо), got nil")
	}
	// negative-control: НЕПУСТОЙ объект + версия НЕ отвергаются (страж краснеет ТОЛЬКО на вырожденном).
	good := base
	good.Evidence = json.RawMessage(`{"x":1}`)
	if _, _, err := export.Export(good, "сигнал, требующий проверки"); err != nil {
		t.Errorf("валидный evidence+версия не должны отвергаться: %v", err)
	}
}
