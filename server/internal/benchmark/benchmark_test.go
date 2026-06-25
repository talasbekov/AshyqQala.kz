package benchmark_test

import (
	"testing"
	"time"

	"ashyqqala/server/internal/benchmark"
	"ashyqqala/server/internal/clock"
	"ashyqqala/server/internal/median"
	"ashyqqala/server/internal/registry"
)

// refNow — опорное «сейчас» для тестов (Fixed clock; детерминизм). Окно 24 мес → cutoff 2024-06-01.
var refNow = clock.Fixed{T: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)}

const windowMonths = 24

// unixAt — unix-секунды даты (помощник: строит SignDateUnix выборки).
func unixAt(y int, m time.Month, d int) int64 {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix()
}

// sampleSpread — n сопоставимых выборок ВНУТРИ окна (2025 г.), цены 1..n*100 (медиана предсказуема).
func sampleSpread(n int) []benchmark.Sample {
	out := make([]benchmark.Sample, n)
	for i := range out {
		out[i] = benchmark.Sample{PricePerKM: int64((i + 1) * 100), SignDateUnix: unixAt(2025, time.January, 1)}
	}
	return out
}

// TestGroupMedian_InsufficientExclusive — property (AC2/AC5г): n<MinSample → (nil, insufficient_sample);
// иначе (value, ok). Однозначность: нельзя медиану при insufficient и наоборот. Зеркалит median_test.
func TestGroupMedian_InsufficientExclusive(t *testing.T) {
	for n := 0; n <= 10; n++ {
		v, st, size := benchmark.GroupMedian(sampleSpread(n), windowMonths, refNow)
		if size != n {
			t.Errorf("n=%d: sample_size=%d, ожидалось %d", n, size, n)
		}
		if n < median.MinSample {
			if v != nil || st != registry.StateInsufficientSample {
				t.Errorf("n=%d: ожидалось (nil, insufficient_sample), получено (%v, %s)", n, v, st)
			}
		} else {
			if v == nil || st != registry.StateOK {
				t.Errorf("n=%d: ожидалось (value, ok), получено (%v, %s)", n, v, st)
			}
		}
	}
}

// TestGroupMedian_OKValue — OK-путь (≥MinSample): корректная целая медиана.
func TestGroupMedian_OKValue(t *testing.T) {
	// 5 цен: 100,200,300,400,500 → медиана 300.
	v, st, size := benchmark.GroupMedian(sampleSpread(5), windowMonths, refNow)
	if st != registry.StateOK || v == nil || *v != 300 || size != 5 {
		t.Fatalf("ожидалось (300, ok, 5), получено (%v, %s, %d)", v, st, size)
	}
}

// TestGroupMedian_EvenSampleRounding — прозрачность ручного пересчёта: при ЧЁТНОЙ выборке медиана = floor
// среднего двух центральных (целые ₸; median.Median: lo + (hi-lo)/2 округляет вниз). [10,20,30,41,50,60] →
// центральные 30,41 → 35 (не 35.5). Документируем правило, чтобы пересчёт третьим лицом совпал бит-в-бит.
func TestGroupMedian_EvenSampleRounding(t *testing.T) {
	prices := []int64{10, 20, 30, 41, 50, 60}
	samples := make([]benchmark.Sample, len(prices))
	for i, p := range prices {
		samples[i] = benchmark.Sample{PricePerKM: p, SignDateUnix: unixAt(2025, time.January, 1)}
	}
	v, st, size := benchmark.GroupMedian(samples, windowMonths, refNow)
	if st != registry.StateOK || v == nil || *v != 35 || size != 6 {
		t.Fatalf("чётная выборка: ожидалось (35=floor(35.5), ok, 6), получено (%v, %s, %d)", v, st, size)
	}
}

// TestGroupMedian_WindowFilter — скользящее окно: выборки СТАРШЕ 24 мес от опорного «сейчас» отбрасываются;
// при их отсечении группа честно падает в insufficient_sample (а не считает медиану по устаревшему).
func TestGroupMedian_WindowFilter(t *testing.T) {
	samples := []benchmark.Sample{
		{PricePerKM: 100, SignDateUnix: unixAt(2025, time.March, 1)},     // в окне
		{PricePerKM: 200, SignDateUnix: unixAt(2025, time.June, 1)},      // в окне
		{PricePerKM: 300, SignDateUnix: unixAt(2025, time.September, 1)}, // в окне
		{PricePerKM: 9000, SignDateUnix: unixAt(2020, time.January, 1)},  // вне окна (старее 24 мес)
		{PricePerKM: 9100, SignDateUnix: unixAt(2019, time.January, 1)},  // вне окна
	}
	v, st, size := benchmark.GroupMedian(samples, windowMonths, refNow)
	if size != 3 {
		t.Fatalf("в окно должно попасть 3 выборки, получено %d", size)
	}
	if v != nil || st != registry.StateInsufficientSample {
		t.Fatalf("3 < MinSample → ожидалось insufficient_sample, получено (%v, %s)", v, st)
	}
}

// TestGroupMedian_BoundaryCutoff — выборка РОВНО на границе окна (cutoff) включается (>=), на день раньше — нет.
func TestGroupMedian_BoundaryCutoff(t *testing.T) {
	cutoff := refNow.T.AddDate(0, -windowMonths, 0) // 2024-06-01
	onEdge := benchmark.Sample{PricePerKM: 100, SignDateUnix: cutoff.Unix()}
	justBefore := benchmark.Sample{PricePerKM: 200, SignDateUnix: cutoff.AddDate(0, 0, -1).Unix()}
	_, _, sizeEdge := benchmark.GroupMedian([]benchmark.Sample{onEdge}, windowMonths, refNow)
	_, _, sizeBefore := benchmark.GroupMedian([]benchmark.Sample{justBefore}, windowMonths, refNow)
	if sizeEdge != 1 {
		t.Errorf("выборка на границе окна должна включаться, size=%d", sizeEdge)
	}
	if sizeBefore != 0 {
		t.Errorf("выборка на день старше границы должна отсекаться, size=%d", sizeBefore)
	}
}

// TestGroupMedian_DoesNotMutateInput — чистота: вход не мутируется (порядок/значения сохранены).
func TestGroupMedian_DoesNotMutateInput(t *testing.T) {
	in := sampleSpread(6)
	first := in[0]
	_, _, _ = benchmark.GroupMedian(in, windowMonths, refNow)
	if in[0] != first || len(in) != 6 {
		t.Fatalf("GroupMedian мутировал вход: %+v", in)
	}
}

// TestGroupMedian_Deterministic — детерминизм: тот же вход + тот же clock → тот же результат бит-в-бит.
func TestGroupMedian_Deterministic(t *testing.T) {
	in := sampleSpread(7)
	v1, st1, sz1 := benchmark.GroupMedian(in, windowMonths, refNow)
	v2, st2, sz2 := benchmark.GroupMedian(in, windowMonths, refNow)
	if st1 != st2 || sz1 != sz2 || (v1 == nil) != (v2 == nil) || (v1 != nil && *v1 != *v2) {
		t.Fatalf("недетерминизм: (%v,%s,%d) vs (%v,%s,%d)", v1, st1, sz1, v2, st2, sz2)
	}
}

// TestGroupKey_Stable — ключ сопоставимости: пин golden-литералом (детерминизм формата); разные
// direction/kato → разные ключи (страж различает группы). См. [[guards-must-prove-red]] (пин литералом).
func TestGroupKey_Stable(t *testing.T) {
	g := benchmark.Group{Direction: "road", Kato: "710000000"}
	const want = "direction=road|kato=710000000"
	if got := g.Key(); got != want {
		t.Fatalf("Key() = %q, ожидалось golden %q", got, want)
	}
	byDirection := benchmark.Group{Direction: "water", Kato: "710000000"}
	if g.Key() == byDirection.Key() {
		t.Fatalf("разные direction дали одинаковый ключ: %q", g.Key())
	}
	byKato := benchmark.Group{Direction: "road", Kato: "750000000"}
	if g.Key() == byKato.Key() {
		t.Fatalf("разные kato дали одинаковый ключ: %q", g.Key())
	}
}
