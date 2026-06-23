package curation_test

import (
	"testing"

	"ashyqqala/server/internal/store/curation"
)

// Курация-stub (AC3): ручная правка переживает перезапись проекции импортёром (AR-10).
func TestApply_EditSurvivesProjectionOverwrite(t *testing.T) {
	overrides := []curation.Override{{Field: "subject", Value: "Курированное наименование"}}

	// Импорт №1: проекция собрана из снапшота.
	p1 := map[string]string{"subject": "Сырое наименование v1", "amount": "100"}
	v1 := curation.Apply(p1, overrides)
	if v1["subject"] != "Курированное наименование" {
		t.Fatalf("курация не применилась к импорту №1: %q", v1["subject"])
	}

	// Импорт №2: импортёр ПЕРЕЗАПИСАЛ проекцию (другое сырое значение). Кураторская правка хранится
	// отдельно → переживает перезапись.
	p2 := map[string]string{"subject": "Сырое наименование v2", "amount": "200"}
	v2 := curation.Apply(p2, overrides)
	if v2["subject"] != "Курированное наименование" {
		t.Fatalf("правка НЕ пережила перезапись проекции: %q", v2["subject"])
	}
	if v2["amount"] != "200" {
		t.Fatalf("некурированное поле должно отражать новую проекцию: %q", v2["amount"])
	}
}

func TestApply_DoesNotMutateInput(t *testing.T) {
	p := map[string]string{"subject": "orig"}
	_ = curation.Apply(p, []curation.Override{{Field: "subject", Value: "new"}})
	if p["subject"] != "orig" {
		t.Fatalf("Apply мутировал вход: %q", p["subject"])
	}
}
