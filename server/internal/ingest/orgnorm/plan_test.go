package orgnorm_test

import (
	"testing"

	"ashyqqala/server/internal/ingest/decode"
	"ashyqqala/server/internal/ingest/orgnorm"
	"ashyqqala/server/internal/normalize"
)

func testLex() normalize.Lexicon {
	return normalize.Lexicon{
		Version:    "v1",
		Homoglyphs: map[rune]rune{'o': 'о', 'a': 'а', 'e': 'е'},
		Synonyms:   map[string]string{},
	}
}

// TestPlanNormalization_AutoManualConflict — ЧИСТОЕ планирование батча покрывает все три исхода:
//   - вариативное написание того же БИН → auto (объединение под один БИН);
//   - одно имя под двумя разными БИН → conflict;
//   - имя без БИН без совпадений → manual.
func TestPlanNormalization_AutoManualConflict(t *testing.T) {
	apps := []orgnorm.Appearance{
		{Org: decode.Organization{BIN: "222222222222", NameRu: "ТОО Астана Жол", IsSupplier: true}, Source: "contract"},
		{Org: decode.Organization{BIN: "222222222222", NameRu: "ТОО \"Астана-Жoл\"", IsSupplier: true}, Source: "trd-buy"}, // вариант (гомоглиф/кавычки) того же БИН
		{Org: decode.Organization{BIN: "333333333333", NameRu: "ТОО СуВодоканал", IsSupplier: true}, Source: "contract"},
		{Org: decode.Organization{BIN: "444444444444", NameRu: "ТОО СуВодоканал", IsSupplier: true}, Source: "contract"}, // то же имя, другой БИН → conflict
		{Org: decode.Organization{BIN: "", NameRu: "ТОО Призрак"}, Source: "trd-buy"},                                    // нет БИН, нет совпадения → manual
	}

	plan := orgnorm.PlanNormalization(apps, nil, testLex())

	// Организации: дедуп по БИН (222/333/444); появление без БИН не регистрируется.
	if len(plan.Orgs) != 3 {
		t.Fatalf("ожидалось 3 организации (222/333/444), получено %d", len(plan.Orgs))
	}

	cnt := map[normalize.Status]int{}
	autoBINs := map[normalize.BIN]bool{}
	for _, a := range plan.Aliases {
		cnt[a.Status]++
		if a.Status == normalize.StatusAuto {
			autoBINs[a.BIN] = true
		}
	}
	if cnt[normalize.StatusAuto] != 2 {
		t.Errorf("auto = %d, ожидалось 2 (оба написания 222)", cnt[normalize.StatusAuto])
	}
	if cnt[normalize.StatusConflict] != 1 {
		t.Errorf("conflict = %d, ожидалось 1 (СуВодоканал под 333 и 444)", cnt[normalize.StatusConflict])
	}
	if cnt[normalize.StatusManual] != 1 {
		t.Errorf("manual = %d, ожидалось 1 (Призрак без БИН)", cnt[normalize.StatusManual])
	}
	if len(autoBINs) != 1 || !autoBINs["222222222222"] {
		t.Errorf("оба auto-псевдонима должны вести к БИН 222, got %v", autoBINs)
	}
}

// TestPlanNormalization_ExistingCandidate — имя без БИН, совпавшее с УЖЕ известной организацией (existing),
// → manual (НЕ auto): без БИН авто-объединение = домысел, оператор подтверждает. [decision code-review 2026-06-27]
func TestPlanNormalization_ExistingCandidate(t *testing.T) {
	lex := testLex()
	existing := []normalize.Candidate{{BIN: "222222222222", NameKey: normalize.CanonicalKey("ТОО Астана Жол", lex)}}
	apps := []orgnorm.Appearance{
		{Org: decode.Organization{BIN: "", NameRu: "ТОО Астана Жол"}, Source: "trd-buy"},
	}
	plan := orgnorm.PlanNormalization(apps, existing, lex)
	if len(plan.Aliases) != 1 || plan.Aliases[0].Status != normalize.StatusManual {
		t.Fatalf("ожидался 1 manual-псевдоним (name-only без БИН), got %+v", plan.Aliases)
	}
}

// TestPlanNormalization_KkOnly — появление без БИН и без ru-имени, только kk → НЕ теряется: псевдоним в
// очередь (manual). [patch code-review 2026-06-27]
func TestPlanNormalization_KkOnly(t *testing.T) {
	lex := testLex()
	apps := []orgnorm.Appearance{
		{Org: decode.Organization{BIN: "", NameRu: "", NameKk: "Белгісіз ЖШС"}, Source: "trd-buy"},
	}
	plan := orgnorm.PlanNormalization(apps, nil, lex)
	if len(plan.Aliases) != 1 || plan.Aliases[0].RawName != "Белгісіз ЖШС" || plan.Aliases[0].Status != normalize.StatusManual {
		t.Fatalf("kk-only появление должно дать 1 manual-псевдоним, got %+v", plan.Aliases)
	}
}
