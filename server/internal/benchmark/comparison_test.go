package benchmark_test

import (
	"math/rand"
	"testing"

	"ashyqqala/server/internal/benchmark"
	"ashyqqala/server/internal/median"
	"ashyqqala/server/internal/registry"
)

// ptr — указатель на int64 (хелпер для ожидаемых медиан).
func ptr(v int64) *int64 { return &v }

// TestEvaluate_NotComputable — computable=false (нет geo_objects.length_km, Epic 3) → ₸/км СТРУКТУРНО
// невычислима → not_comparable (нечего сравнивать), НЕ insufficient_sample, БЕЗ обращения к выборке.
// Это текущее токен-независимое состояние медианы района/города (прецедент 4.3: честно-пусто).
func TestEvaluate_NotComputable(t *testing.T) {
	// Даже при «достаточной» по размеру выборке: computable=false перебивает (нет измеримой базы ₸/км).
	out := benchmark.Evaluate(sampleSpread(8), false, windowMonths, refNow)
	if out.Median != nil {
		t.Errorf("not_comparable: медиана должна быть nil, получено %v", *out.Median)
	}
	if out.State != registry.StateNotComparable {
		t.Errorf("state = %s, ожидалось not_comparable", out.State)
	}
	if out.N != 0 {
		t.Errorf("N = %d, ожидалось 0 (выборка не считалась)", out.N)
	}
}

// TestEvaluate_DelegatesToGroupMedian — computable=true → исход совпадает с GroupMedian (окно × порог):
// ok с медианой при ≥MinSample, insufficient_sample иначе. Negative control относительно not_comparable.
func TestEvaluate_DelegatesToGroupMedian(t *testing.T) {
	okOut := benchmark.Evaluate(sampleSpread(5), true, windowMonths, refNow) // 100..500 → 300
	if okOut.State != registry.StateOK || okOut.Median == nil || *okOut.Median != 300 || okOut.N != 5 {
		t.Fatalf("computable ≥MinSample: ожидалось (300, ok, 5), получено (%v, %s, %d)", okOut.Median, okOut.State, okOut.N)
	}
	insOut := benchmark.Evaluate(sampleSpread(4), true, windowMonths, refNow)
	if insOut.Median != nil || insOut.State != registry.StateInsufficientSample || insOut.N != 4 {
		t.Fatalf("computable <MinSample: ожидалось (nil, insufficient_sample, 4), получено (%v, %s, %d)", insOut.Median, insOut.State, insOut.N)
	}
}

// TestEvaluate_MedianIffMinSample — НЕСУЩИЙ property-инвариант AC3 (arch:596-598): медиана показывается
// ⟺ comparable_n ≥ MinSample ⟺ state==ok. НЕВОЗМОЖНО одновременно (медиана + insufficient). Случайные
// выборки внутри окна; computable=true (живой путь). + negative control порога n=4/n=5.
func TestEvaluate_MedianIffMinSample(t *testing.T) {
	rng := rand.New(rand.NewSource(64064)) // фикс-сид → детерминизм property-теста
	for range 500 {
		n := rng.Intn(2*median.MinSample + 3) // 0..12, плотно вокруг порога 5
		samples := make([]benchmark.Sample, n)
		for i := range samples {
			samples[i] = benchmark.Sample{
				PricePerKM:   int64(rng.Intn(1_000_000) + 1),
				SignDateUnix: unixAt(2025, 1, 1), // внутри окна 24 мес от refNow
			}
		}
		out := benchmark.Evaluate(samples, true, windowMonths, refNow)

		shown := out.Median != nil
		enough := out.N >= median.MinSample
		if shown != enough {
			t.Fatalf("n=%d N=%d: median-показана=%v, но N≥MinSample=%v (нарушена однозначность)", n, out.N, shown, enough)
		}
		if shown != (out.State == registry.StateOK) {
			t.Fatalf("n=%d: median-показана=%v, но state==ok=%v (рассинхрон значения и состояния)", n, shown, out.State == registry.StateOK)
		}
		// Категорически нельзя: одновременно медиана И insufficient_sample.
		if out.Median != nil && out.State == registry.StateInsufficientSample {
			t.Fatalf("n=%d: показаны И медиана, И insufficient_sample — запрещено инвариантом", n)
		}
	}
	// Negative control порога: краснеет, если кто-то сдвинет MinSample.
	if benchmark.Evaluate(sampleSpread(median.MinSample-1), true, windowMonths, refNow).Median != nil {
		t.Errorf("n=MinSample-1 не должна давать медиану (insufficient)")
	}
	if benchmark.Evaluate(sampleSpread(median.MinSample), true, windowMonths, refNow).Median == nil {
		t.Errorf("n=MinSample должна давать медиану (ok)")
	}
}

// TestCompareGroups_DeltaShownOnlyWhenBothOK — % дельта показывается ТОЛЬКО когда ОБЕ медианы ok.
// Любая не-ok сторона → DeltaShown=false (нечего сравнивать; дельты нет, не 0).
func TestCompareGroups_DeltaShownOnlyWhenBothOK(t *testing.T) {
	okD := benchmark.Outcome{Median: ptr(384), State: registry.StateOK, N: 9}
	okC := benchmark.Outcome{Median: ptr(331), State: registry.StateOK, N: 40}
	ins := benchmark.Outcome{State: registry.StateInsufficientSample, N: 3}
	notc := benchmark.Outcome{State: registry.StateNotComparable}

	cases := []struct {
		name      string
		d, c      benchmark.Outcome
		wantShown bool
	}{
		{"обе ok", okD, okC, true},
		{"район insufficient", ins, okC, false},
		{"город insufficient", okD, ins, false},
		{"район not_comparable", notc, okC, false},
		{"город not_comparable", okD, notc, false},
		{"обе not_comparable", notc, notc, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmp := benchmark.CompareGroups(c.d, c.c)
			if cmp.DeltaShown != c.wantShown {
				t.Errorf("DeltaShown = %v, ожидалось %v", cmp.DeltaShown, c.wantShown)
			}
		})
	}
}

// TestCompareGroups_DeltaInteger — дельта целочисленная (без float-дрейфа): район 384 vs город 331 →
// (384−331)×100/331 = 16% (как +16% в UX-макете). Знак: район дороже → положительная.
func TestCompareGroups_DeltaInteger(t *testing.T) {
	higher := benchmark.CompareGroups(
		benchmark.Outcome{Median: ptr(384), State: registry.StateOK, N: 9},
		benchmark.Outcome{Median: ptr(331), State: registry.StateOK, N: 40},
	)
	if !higher.DeltaShown || higher.DeltaPercent != 16 {
		t.Errorf("дельта = %d (shown=%v), ожидалось 16", higher.DeltaPercent, higher.DeltaShown)
	}
	// Район дешевле города → отрицательная дельта (усечение к нулю).
	lower := benchmark.CompareGroups(
		benchmark.Outcome{Median: ptr(331), State: registry.StateOK, N: 9},
		benchmark.Outcome{Median: ptr(384), State: registry.StateOK, N: 40},
	)
	if !lower.DeltaShown || lower.DeltaPercent != -13 { // -5300/384 = -13.8 → -13
		t.Errorf("дельта = %d, ожидалось -13", lower.DeltaPercent)
	}
}

// TestCompareGroups_CityNonPositiveNoDelta — городская медиана ≤0 недостоверна как база → дельты НЕТ
// (защита от деления на ноль / фабрикации %), даже если состояние ok.
func TestCompareGroups_CityNonPositiveNoDelta(t *testing.T) {
	cmp := benchmark.CompareGroups(
		benchmark.Outcome{Median: ptr(384), State: registry.StateOK, N: 9},
		benchmark.Outcome{Median: ptr(0), State: registry.StateOK, N: 40},
	)
	if cmp.DeltaShown {
		t.Errorf("город median=0: дельта не должна показываться (деление на ноль)")
	}
}
