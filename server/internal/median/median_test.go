package median_test

import (
	"testing"

	"ashyqqala/server/internal/median"
	"ashyqqala/server/internal/registry"
)

// Property: insufficient ОДНОЗНАЧЕН — при n<MinSample (nil, insufficient_sample); иначе (value, ok).
// Нельзя показать медиану при insufficient и наоборот (AR-26). [architecture.md:598]
func TestMedian_InsufficientExclusive(t *testing.T) {
	for n := 0; n <= 10; n++ {
		samples := make([]int64, n)
		for i := range samples {
			samples[i] = int64(i + 1)
		}
		v, st := median.Median(samples)
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

func TestMedian_Values(t *testing.T) {
	cases := []struct {
		name string
		in   []int64
		want int64
	}{
		{"нечётная 1..5", []int64{5, 1, 3, 2, 4}, 3},
		{"чётная 1..6", []int64{6, 1, 3, 2, 4, 5}, 3}, // (3+4)/2 = 3 (целочисленно)
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v, st := median.Median(c.in)
			if st != registry.StateOK || v == nil || *v != c.want {
				t.Fatalf("%s: ожидалось (%d, ok), получено (%v, %s)", c.name, c.want, v, st)
			}
		})
	}
}

func TestMedian_DoesNotMutateInput(t *testing.T) {
	in := []int64{3, 1, 2, 5, 4}
	_, _ = median.Median(in)
	if in[0] != 3 || in[1] != 1 {
		t.Fatalf("Median мутировал вход: %v", in)
	}
}
