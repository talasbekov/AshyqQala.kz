package projection_test

import (
	"context"
	"errors"
	"testing"

	"ashyqqala/server/internal/clock"
	"ashyqqala/server/internal/flags"
	"ashyqqala/server/internal/recalc"
	"ashyqqala/server/internal/store/projection"
)

// Compile-time: адаптеры реализуют контракт порядка recalc.FlagRecalculator (флаги после benchmark, AC4).
var (
	_ recalc.FlagRecalculator = projection.SingleParticipantRecalculator{}
	_ recalc.FlagRecalculator = projection.PricePerKMRecalculator{}
	_ recalc.FlagRecalculator = projection.MonopolyRecalculator{}
	_ recalc.FlagRecalculator = projection.RNURecalculator{}
)

// TestPricePerKMRecalculator_OrderInRecalcRun — AC4 (4.3): price-флаг пересчитывается ПОСЛЕ benchmark в
// recalc.Run (медиана из свежего снапшота). Provider пуст → DB-free (нет обращения к пулу).
func TestPricePerKMRecalculator_OrderInRecalcRun(t *testing.T) {
	var order []string
	bench := fakeBench{order: &order}
	priceR := projection.PricePerKMRecalculator{
		Store: projection.NewRiskFlagStore(nil, flags.Params{MethodologyVersion: "v1.0"}),
		Bench: projection.NewBenchmarkStore(nil, flags.Params{MethodologyVersion: "v1.0"}),
		Provider: func(context.Context) ([]projection.PricePerKMInput, error) {
			order = append(order, "flags")
			return nil, nil // пусто → без обращения к БД
		},
	}
	if err := recalc.Run(context.Background(), bench, priceR); err != nil {
		t.Fatalf("recalc.Run: %v", err)
	}
	if len(order) != 2 || order[0] != "benchmark" || order[1] != "flags" {
		t.Fatalf("порядок = %v, ожидалось [benchmark flags]", order)
	}
}

// TestPricePerKMRecalculator_Guards — nil Store/Bench/Provider → честная ошибка (не паника).
func TestPricePerKMRecalculator_Guards(t *testing.T) {
	prov := func(context.Context) ([]projection.PricePerKMInput, error) { return nil, nil }
	cases := []projection.PricePerKMRecalculator{
		{Bench: projection.NewBenchmarkStore(nil, flags.Params{}), Provider: prov},                                          // nil Store
		{Store: projection.NewRiskFlagStore(nil, flags.Params{}), Provider: prov},                                           // nil Bench
		{Store: projection.NewRiskFlagStore(nil, flags.Params{}), Bench: projection.NewBenchmarkStore(nil, flags.Params{})}, // nil Provider
	}
	for i, r := range cases {
		if err := r.RecalcFlags(context.Background()); err == nil {
			t.Errorf("case %d: ожидалась ошибка (nil-поле), got nil", i)
		}
	}
}

// fakeBench — фейковый BenchmarkRecalculator, фиксирующий порядок.
type fakeBench struct{ order *[]string }

func (f fakeBench) RecalcBenchmarks(context.Context) error {
	*f.order = append(*f.order, "benchmark")
	return nil
}

// TestSingleParticipantRecalculator_NilStore — Store не задан → честная ошибка (а не nil-паника).
func TestSingleParticipantRecalculator_NilStore(t *testing.T) {
	r := projection.SingleParticipantRecalculator{
		Provider: func(context.Context) ([]projection.SingleParticipantInput, error) { return nil, nil },
	}
	if err := r.RecalcFlags(context.Background()); err == nil {
		t.Fatal("ожидалась ошибка при nil Store, got nil")
	}
}

// TestSingleParticipantRecalculator_NilProvider — provider не задан → честная ошибка (живой источник — Epic 2).
func TestSingleParticipantRecalculator_NilProvider(t *testing.T) {
	r := projection.SingleParticipantRecalculator{Store: projection.NewRiskFlagStore(nil, flags.Params{})}
	if err := r.RecalcFlags(context.Background()); err == nil {
		t.Fatal("ожидалась ошибка при nil Provider, got nil")
	}
}

// TestSingleParticipantRecalculator_ProviderError — ошибка provider прокидывается.
func TestSingleParticipantRecalculator_ProviderError(t *testing.T) {
	r := projection.SingleParticipantRecalculator{
		Store:    projection.NewRiskFlagStore(nil, flags.Params{}),
		Provider: func(context.Context) ([]projection.SingleParticipantInput, error) { return nil, errors.New("boom") },
	}
	if err := r.RecalcFlags(context.Background()); err == nil {
		t.Fatal("ожидалась ошибка provider, got nil")
	}
}

// TestSingleParticipantRecalculator_OrderInRecalcRun — AC4: в recalc.Run флаги пересчитываются ПОСЛЕ
// benchmark. Provider пуст → RecomputeSingleParticipant не трогает БД (цикл по нулю входов) → тест DB-free.
func TestSingleParticipantRecalculator_OrderInRecalcRun(t *testing.T) {
	var order []string
	bench := fakeBench{order: &order}
	flagsR := projection.SingleParticipantRecalculator{
		Store: projection.NewRiskFlagStore(nil, flags.Params{MethodologyVersion: "v1.0"}),
		Provider: func(context.Context) ([]projection.SingleParticipantInput, error) {
			order = append(order, "flags")
			return nil, nil // пусто → без обращения к БД
		},
	}
	if err := recalc.Run(context.Background(), bench, flagsR); err != nil {
		t.Fatalf("recalc.Run: %v", err)
	}
	if len(order) != 2 || order[0] != "benchmark" || order[1] != "flags" {
		t.Fatalf("порядок = %v, ожидалось [benchmark flags]", order)
	}
}

// TestMonopolyRecalculator_OrderInRecalcRun — AC4 (4.4): monopoly-флаг пересчитывается ПОСЛЕ benchmark в
// recalc.Run. Монополия benchmark не использует (доля по сумме), но порядок recalc единый. Provider пуст → DB-free.
func TestMonopolyRecalculator_OrderInRecalcRun(t *testing.T) {
	var order []string
	bench := fakeBench{order: &order}
	monR := projection.MonopolyRecalculator{
		Store: projection.NewRiskFlagStore(nil, flags.Params{MethodologyVersion: "v1.0"}),
		Provider: func(context.Context) ([]projection.MonopolyInput, error) {
			order = append(order, "flags")
			return nil, nil // пусто → без обращения к БД
		},
	}
	if err := recalc.Run(context.Background(), bench, monR); err != nil {
		t.Fatalf("recalc.Run: %v", err)
	}
	if len(order) != 2 || order[0] != "benchmark" || order[1] != "flags" {
		t.Fatalf("порядок = %v, ожидалось [benchmark flags]", order)
	}
}

// TestMonopolyRecalculator_Guards — nil Store/Provider → честная ошибка (не паника); ошибка provider прокидывается.
func TestMonopolyRecalculator_Guards(t *testing.T) {
	prov := func(context.Context) ([]projection.MonopolyInput, error) { return nil, nil }
	provErr := func(context.Context) ([]projection.MonopolyInput, error) { return nil, errors.New("boom") }
	cases := []projection.MonopolyRecalculator{
		{Provider: prov}, // nil Store
		{Store: projection.NewRiskFlagStore(nil, flags.Params{})},                    // nil Provider
		{Store: projection.NewRiskFlagStore(nil, flags.Params{}), Provider: provErr}, // provider error
	}
	for i, r := range cases {
		if err := r.RecalcFlags(context.Background()); err == nil {
			t.Errorf("case %d: ожидалась ошибка, got nil", i)
		}
	}
}

// TestRNURecalculator_OrderInRecalcRun — AC4 (4.5): rnu-флаг пересчитывается ПОСЛЕ benchmark в recalc.Run. РНУ
// benchmark не использует (дата-зависимый), но порядок recalc единый. Provider пуст → DB-free.
func TestRNURecalculator_OrderInRecalcRun(t *testing.T) {
	var order []string
	bench := fakeBench{order: &order}
	rnuR := projection.RNURecalculator{
		Store: projection.NewRiskFlagStore(nil, flags.Params{MethodologyVersion: "v1.0", RNUEnabled: true}),
		Clock: clock.Fixed{},
		Provider: func(context.Context) ([]projection.RNUInput, error) {
			order = append(order, "flags")
			return nil, nil // пусто → без обращения к БД
		},
	}
	if err := recalc.Run(context.Background(), bench, rnuR); err != nil {
		t.Fatalf("recalc.Run: %v", err)
	}
	if len(order) != 2 || order[0] != "benchmark" || order[1] != "flags" {
		t.Fatalf("порядок = %v, ожидалось [benchmark flags]", order)
	}
}

// TestRNURecalculator_Guards — nil Store/Clock/Provider → честная ошибка (не паника); ошибка provider прокидывается.
func TestRNURecalculator_Guards(t *testing.T) {
	store := projection.NewRiskFlagStore(nil, flags.Params{})
	prov := func(context.Context) ([]projection.RNUInput, error) { return nil, nil }
	provErr := func(context.Context) ([]projection.RNUInput, error) { return nil, errors.New("boom") }
	cases := []projection.RNURecalculator{
		{Clock: clock.Fixed{}, Provider: prov},                  // nil Store
		{Store: store, Provider: prov},                          // nil Clock (дата-зависимый — обязателен)
		{Store: store, Clock: clock.Fixed{}},                    // nil Provider
		{Store: store, Clock: clock.Fixed{}, Provider: provErr}, // provider error
	}
	for i, r := range cases {
		if err := r.RecalcFlags(context.Background()); err == nil {
			t.Errorf("case %d: ожидалась ошибка, got nil", i)
		}
	}
}
