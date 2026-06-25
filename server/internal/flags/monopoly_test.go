package flags_test

import (
	"testing"

	"ashyqqala/server/internal/flags"
	"ashyqqala/server/internal/registry"
)

// monParams — methodology-пороги для теста монополии: доля 0.5, минимум 5 контрактов в группе.
var monParams = flags.Params{
	MethodologyVersion:         "v1.0",
	MonopolyConcentrationShare: 0.5,
	MonopolyMinGroupContracts:  5,
}

func monIn(supplier, total *int64, contracts int, resolved bool) flags.Inputs {
	return flags.Inputs{
		SupplierSum:         supplier,
		GroupTotalSum:       total,
		GroupContracts:      contracts,
		SupplierBINResolved: resolved,
		SupplierBIN:         "123456789012",
		ComparabilityKey:    "direction=road|kato=710000000",
	}
}

// TestMonopoly_States — таблица FR-21 (AC1/AC2/AC3): raised только при доле топ-БИН ≥ 0.5 при ≥ 5 контрактах,
// знаменателе > 0 и разрешённом БИН; иначе честные состояния. Граница ровно 0.5 → raised (инклюзивно ≥).
func TestMonopoly_States(t *testing.T) {
	cases := []struct {
		name      string
		supplier  *int64
		total     *int64
		contracts int
		resolved  bool
		want      registry.FlagState
	}{
		{"0.6 ≥ 0.5, 5 контрактов, resolved → raised", pi64(600_000), pi64(1_000_000), 5, true, registry.FlagRaised},
		{"ровно 0.5 → raised (инклюзивно ≥)", pi64(500_000), pi64(1_000_000), 5, true, registry.FlagRaised},
		{"чуть ниже 0.5 → not_raised", pi64(499_999), pi64(1_000_000), 5, true, registry.FlagNotRaised},
		{"доля мала → not_raised", pi64(200_000), pi64(1_000_000), 8, true, registry.FlagNotRaised},
		{"контрактов < min (4) → insufficient", pi64(900_000), pi64(1_000_000), 4, true, registry.FlagInsufficientData},
		{"нет суммы поставщика → insufficient", nil, pi64(1_000_000), 5, true, registry.FlagInsufficientData},
		{"нет знаменателя → insufficient", pi64(600_000), nil, 5, true, registry.FlagInsufficientData},
		{"знаменатель 0 → insufficient (база недостоверна)", pi64(600_000), pi64(0), 5, true, registry.FlagInsufficientData},
		{"знаменатель < 0 → insufficient (грязные данные)", pi64(600_000), pi64(-1_000_000), 5, true, registry.FlagInsufficientData},
		{"сумма поставщика < 0 → insufficient (грязные данные)", pi64(-100), pi64(1_000_000), 5, true, registry.FlagInsufficientData},
		{"БИН не разрешён (manual/conflict) → insufficient (профиль уточняется)", pi64(600_000), pi64(1_000_000), 5, false, registry.FlagInsufficientData},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st, _ := flags.Monopoly(monIn(c.supplier, c.total, c.contracts, c.resolved), monParams)
			if st != c.want {
				t.Fatalf("состояние = %s, ожидалось %s", st, c.want)
			}
		})
	}
}

// TestMonopoly_ThresholdDiscriminates — negative/positive control: порог РАЗЛИЧАЕТ (способен покраснеть) —
// при том же знаменателе доля ровно на пороге → raised (инклюзивно), строго ниже → not_raised. Литералы
// ОТЛИЧНЫ от таблицы (независимый контроль). [[guards-must-prove-red]]
func TestMonopoly_ThresholdDiscriminates(t *testing.T) {
	total := pi64(2_000_000) // порог доли 0.5 → ровно 1_000_000
	if st, _ := flags.Monopoly(monIn(pi64(1_000_000), total, 5, true), monParams); st != registry.FlagRaised {
		t.Fatalf("ровно на пороге (0.5) → ожидалось raised (инклюзивно), получено %s", st)
	}
	if st, _ := flags.Monopoly(monIn(pi64(999_999), total, 5, true), monParams); st != registry.FlagNotRaised {
		t.Fatalf("строго ниже порога → ожидалось not_raised, получено %s", st)
	}
}

// TestMonopoly_Evidence — evidence хранит все входы (raised); methodology_version из params. Также проверяем
// evidence на insufficient-пути (контрактов < min) — заполняется до early-return (урок ревью 4.3).
func TestMonopoly_Evidence(t *testing.T) {
	st, ev := flags.Monopoly(monIn(pi64(600_000), pi64(1_000_000), 7, true), monParams)
	if st != registry.FlagRaised {
		t.Fatalf("ожидалось raised, получено %s", st)
	}
	if ev.SupplierSum == nil || *ev.SupplierSum != 600_000 || ev.GroupTotalSum == nil || *ev.GroupTotalSum != 1_000_000 {
		t.Errorf("evidence суммы неполны: %+v", ev)
	}
	if ev.GroupContracts != 7 || ev.ConcentrationShare != 0.5 || ev.MinGroupContracts != 5 {
		t.Errorf("evidence порог/контракты неполны: %+v", ev)
	}
	if ev.SupplierBIN == "" || ev.ComparabilityKey == "" || !ev.SupplierResolved {
		t.Errorf("evidence бин/ключ/resolved неполны: %+v", ev)
	}
	if ev.MethodologyVersion != "v1.0" {
		t.Errorf("evidence.methodology_version = %q, ожидалось v1.0 (==params)", ev.MethodologyVersion)
	}

	// insufficient-путь: evidence всё равно заполнен (пересчитываемость честного состояния).
	_, evIns := flags.Monopoly(monIn(pi64(900_000), pi64(1_000_000), 4, true), monParams)
	if evIns.GroupContracts != 4 || evIns.GroupTotalSum == nil || evIns.MethodologyVersion != "v1.0" {
		t.Errorf("evidence на insufficient-пути неполон: %+v", evIns)
	}
}
