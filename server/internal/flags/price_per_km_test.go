package flags_test

import (
	"testing"

	"ashyqqala/server/internal/flags"
	"ashyqqala/server/internal/registry"
)

func pi64(n int64) *int64 { return &n }

// ppkmParams — methodology-пороги для теста цены/км: фактор 1.5, min_sample 5.
var ppkmParams = flags.Params{
	MethodologyVersion:        "v1.0",
	MinSample:                 5,
	PricePerKMDeviationFactor: 1.5,
}

func ppkmIn(price, median *int64, sample int) flags.Inputs {
	return flags.Inputs{PricePerKM: price, GroupMedian: median, GroupSampleSize: sample, ComparabilityKey: "direction=road|kato=710000000"}
}

// TestPricePerKM_States — таблица FR-20 (AC1/AC2): raised только при цене/км > медиана×1.5 при достаточной
// выборке; нет цены/медианы или sample<min → insufficient (флаг не строится); ровно ×1.5 → not_raised.
func TestPricePerKM_States(t *testing.T) {
	cases := []struct {
		name   string
		price  *int64
		median *int64
		sample int
		want   registry.FlagState
	}{
		{"2M > 1M×1.5=1.5M → raised", pi64(2_000_000), pi64(1_000_000), 5, registry.FlagRaised},
		{"ровно ×1.5 (1.5M) → not_raised", pi64(1_500_000), pi64(1_000_000), 5, registry.FlagNotRaised},
		{"чуть выше порога → raised", pi64(1_500_001), pi64(1_000_000), 5, registry.FlagRaised},
		{"чуть ниже порога → not_raised", pi64(1_499_999), pi64(1_000_000), 5, registry.FlagNotRaised},
		{"цена ниже медианы → not_raised", pi64(800_000), pi64(1_000_000), 6, registry.FlagNotRaised},
		{"sample<min → insufficient", pi64(2_000_000), pi64(1_000_000), 4, registry.FlagInsufficientData},
		{"нет медианы → insufficient", pi64(2_000_000), nil, 5, registry.FlagInsufficientData},
		{"нет цены/км (нет длины) → insufficient", nil, pi64(1_000_000), 5, registry.FlagInsufficientData},
		{"медиана 0 → insufficient (база недостоверна, не фабрикация)", pi64(2_000_000), pi64(0), 5, registry.FlagInsufficientData},
		{"медиана < 0 → insufficient (грязные данные)", pi64(2_000_000), pi64(-1_000_000), 5, registry.FlagInsufficientData},
		{"отрицательная цена/км → insufficient (грязные данные)", pi64(-100), pi64(1_000_000), 5, registry.FlagInsufficientData},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st, _ := flags.PricePerKM(ppkmIn(c.price, c.median, c.sample), ppkmParams)
			if st != c.want {
				t.Fatalf("состояние = %s, ожидалось %s", st, c.want)
			}
		})
	}
}

// TestPricePerKM_ThresholdDiscriminates — negative/positive control: порог РАЗЛИЧАЕТ (способен покраснеть) —
// при той же медиане цена строго выше порога → raised, ровно на пороге → not_raised. [[guards-must-prove-red]]
func TestPricePerKM_ThresholdDiscriminates(t *testing.T) {
	median := pi64(2_000_000) // порог = 3_000_000 (×1.5)
	if st, _ := flags.PricePerKM(ppkmIn(pi64(3_000_001), median, 5), ppkmParams); st != registry.FlagRaised {
		t.Fatalf("выше порога → ожидалось raised, получено %s", st)
	}
	if st, _ := flags.PricePerKM(ppkmIn(pi64(3_000_000), median, 5), ppkmParams); st != registry.FlagNotRaised {
		t.Fatalf("ровно на пороге → ожидалось not_raised, получено %s", st)
	}
}

// TestPricePerKM_Evidence — evidence хранит все входы; methodology_version из params.
func TestPricePerKM_Evidence(t *testing.T) {
	st, ev := flags.PricePerKM(ppkmIn(pi64(2_000_000), pi64(1_000_000), 7), ppkmParams)
	if st != registry.FlagRaised {
		t.Fatalf("ожидалось raised, получено %s", st)
	}
	if ev.PricePerKM == nil || *ev.PricePerKM != 2_000_000 || ev.Median == nil || *ev.Median != 1_000_000 {
		t.Errorf("evidence цена/медиана неполны: %+v", ev)
	}
	if ev.SampleSize != 7 || ev.DeviationFactor != 1.5 || ev.ComparabilityKey == "" {
		t.Errorf("evidence sample/factor/key неполны: %+v", ev)
	}
	if ev.MethodologyVersion != "v1.0" {
		t.Errorf("evidence.methodology_version = %q, ожидалось v1.0 (==params)", ev.MethodologyVersion)
	}
}
