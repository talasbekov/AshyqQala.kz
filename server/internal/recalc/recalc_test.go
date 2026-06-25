package recalc_test

import (
	"context"
	"errors"
	"testing"

	"ashyqqala/server/internal/recalc"
)

// recorder — фейковые шаги, записывающие порядок вызовов (для контракта порядка benchmark→flags).
type recorder struct {
	order      *[]string
	benchErr   error
	flagErr    error
	flagCalled *bool
}

func (r recorder) RecalcBenchmarks(context.Context) error {
	*r.order = append(*r.order, "benchmark")
	return r.benchErr
}

func (r recorder) RecalcFlags(context.Context) error {
	*r.order = append(*r.order, "flags")
	*r.flagCalled = true
	return r.flagErr
}

// TestRun_OrderBenchmarkBeforeFlags — AC4: benchmark пересчитывается СТРОГО перед флагами.
func TestRun_OrderBenchmarkBeforeFlags(t *testing.T) {
	var order []string
	called := false
	rec := recorder{order: &order, flagCalled: &called}
	if err := recalc.Run(context.Background(), rec, rec); err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := []string{"benchmark", "flags"}
	if len(order) != 2 || order[0] != want[0] || order[1] != want[1] {
		t.Fatalf("порядок = %v, ожидалось %v (benchmark ПЕРЕД flags)", order, want)
	}
}

// TestRun_BenchmarkFailure_SkipsFlags — если benchmark упал, флаги НЕ считаются (negative-control порядка:
// флаг не строится против устаревшего/полупересчитанного benchmark). См. [[guards-must-prove-red]].
func TestRun_BenchmarkFailure_SkipsFlags(t *testing.T) {
	var order []string
	called := false
	rec := recorder{order: &order, benchErr: errors.New("boom"), flagCalled: &called}
	if err := recalc.Run(context.Background(), rec, rec); err == nil {
		t.Fatal("ожидалась ошибка при падении benchmark, got nil")
	}
	if called {
		t.Fatal("флаги пересчитаны несмотря на падение benchmark — нарушение порядка (флаг против устаревшего кэша)")
	}
	if len(order) != 1 || order[0] != "benchmark" {
		t.Fatalf("порядок = %v, ожидалось только [benchmark]", order)
	}
}

// TestRun_FlagFailurePropagates — ошибка пересчёта флагов прокидывается (после успешного benchmark).
func TestRun_FlagFailurePropagates(t *testing.T) {
	var order []string
	called := false
	rec := recorder{order: &order, flagErr: errors.New("flag boom"), flagCalled: &called}
	if err := recalc.Run(context.Background(), rec, rec); err == nil {
		t.Fatal("ожидалась ошибка при падении флагов, got nil")
	}
	if !called {
		t.Fatal("флаги должны были вызваться после успешного benchmark")
	}
}
