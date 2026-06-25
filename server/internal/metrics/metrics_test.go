package metrics

import (
	"context"
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// gatherValue — значение метрики name из реестра (ok=false, если метрика отсутствует в выдаче).
func gatherValue(t *testing.T, reg *prometheus.Registry, name string) (float64, bool) {
	t.Helper()
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() == name {
			m := mf.GetMetric()
			if len(m) == 0 {
				return 0, false
			}
			return m[0].GetCounter().GetValue(), true
		}
	}
	return 0, false
}

// TestSMC1Collector_DBDerived — SM-C1 derived из источника (flag_disputes): reviewed = знаменатель
// (confirmed|withdrawn), incorrect = числитель (withdrawn). Значения берутся из источника на scrape (не
// императивный инкремент → race-free/restart-safe). SM-C1 = incorrect/reviewed.
func TestSMC1Collector_DBDerived(t *testing.T) {
	src := func(ctx context.Context) (SMC1Counts, error) {
		return SMC1Counts{Reviewed: 2, Incorrect: 1}, nil
	}
	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(smc1Collector{src: src})

	if v, ok := gatherValue(t, reg, "ashyqqala_flags_reviewed_total"); !ok || v != 2 {
		t.Errorf("reviewed = %v (ok=%v), ожидалось 2", v, ok)
	}
	if v, ok := gatherValue(t, reg, "ashyqqala_flags_incorrect_total"); !ok || v != 1 {
		t.Errorf("incorrect = %v (ok=%v), ожидалось 1", v, ok)
	}
}

// TestSMC1Collector_ErrorHonest — при ошибке источника коллектор НЕ отдаёт ложный 0 (честность над домыслом):
// метрики отсутствуют в выдаче, scrape не падает и не паникует.
func TestSMC1Collector_ErrorHonest(t *testing.T) {
	src := func(ctx context.Context) (SMC1Counts, error) {
		return SMC1Counts{}, errors.New("db down")
	}
	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(smc1Collector{src: src})

	if _, ok := gatherValue(t, reg, "ashyqqala_flags_reviewed_total"); ok {
		t.Error("reviewed присутствует при ошибке источника — ожидалось отсутствие (не ложный 0)")
	}
	if _, ok := gatherValue(t, reg, "ashyqqala_flags_incorrect_total"); ok {
		t.Error("incorrect присутствует при ошибке источника — ожидалось отсутствие")
	}
}
