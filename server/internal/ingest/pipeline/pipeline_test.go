package pipeline_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"ashyqqala/server/internal/clock"
	"ashyqqala/server/internal/ingest/pipeline"
	"ashyqqala/server/internal/recalc"
)

// countingHook — хук, считающий вызовы Recalc (для проверки «РОВНО 1 раз на публикацию», AC1).
type countingHook struct {
	calls  int
	order  *[]string
	recErr error
}

func (h *countingHook) Recalc(context.Context) error {
	h.calls++
	if h.order != nil {
		*h.order = append(*h.order, "recalc")
	}
	return h.recErr
}

// recordingNorm — шаг нормализации, фиксирующий вызов/порядок и опц. ошибку.
type recordingNorm struct {
	called  bool
	order   *[]string
	normErr error
}

func (n *recordingNorm) Normalize(context.Context) error {
	n.called = true
	if n.order != nil {
		*n.order = append(*n.order, "normalize")
	}
	return n.normErr
}

// TestRunPostImport_HookCalledExactlyOnce — AC1: хук пересчёта вызывается РОВНО один раз за публикацию.
func TestRunPostImport_HookCalledExactlyOnce(t *testing.T) {
	hook := &countingHook{}
	if err := pipeline.RunPostImport(context.Background(), &recordingNorm{}, hook); err != nil {
		t.Fatalf("RunPostImport: %v", err)
	}
	if hook.calls != 1 {
		t.Fatalf("хук вызван %d раз, ожидалось РОВНО 1 (AC1)", hook.calls)
	}
}

// TestCountingHook_ActuallyCounts — КОНТРОЛЬ (guard-reddens): счётчик реально считает вызовы, значит
// проверка «==1» осмысленна и поймала бы и 0 (пропуск), и 2 (двойной пересчёт). См. [[guards-must-prove-red]].
func TestCountingHook_ActuallyCounts(t *testing.T) {
	hook := &countingHook{}
	if hook.calls != 0 {
		t.Fatalf("до вызовов счётчик = %d, ожидалось 0", hook.calls)
	}
	_ = hook.Recalc(context.Background())
	_ = hook.Recalc(context.Background())
	if hook.calls != 2 {
		t.Fatalf("после 2 вызовов счётчик = %d, ожидалось 2 (иначе ==1-проверка тавтологична)", hook.calls)
	}
}

// TestRunPostImport_OrderNormalizeThenRecalc — AC1: порядок зафиксирован нормализация → пересчёт.
func TestRunPostImport_OrderNormalizeThenRecalc(t *testing.T) {
	var order []string
	norm := &recordingNorm{order: &order}
	hook := &countingHook{order: &order}
	if err := pipeline.RunPostImport(context.Background(), norm, hook); err != nil {
		t.Fatalf("RunPostImport: %v", err)
	}
	if len(order) != 2 || order[0] != "normalize" || order[1] != "recalc" {
		t.Fatalf("порядок = %v, ожидалось [normalize recalc] (AC1)", order)
	}
}

// TestRunPostImport_NormalizeErrorSkipsRecalc — нормализация упала → пересчёт НЕ запускается (не публикуем
// снапшот на неполностью нормализованных данных). Negative-control порядка.
func TestRunPostImport_NormalizeErrorSkipsRecalc(t *testing.T) {
	norm := &recordingNorm{normErr: errors.New("norm boom")}
	hook := &countingHook{}
	if err := pipeline.RunPostImport(context.Background(), norm, hook); err == nil {
		t.Fatal("ожидалась ошибка при падении нормализации, got nil")
	}
	if hook.calls != 0 {
		t.Fatalf("пересчёт вызван (%d) несмотря на падение нормализации — нарушение порядка", hook.calls)
	}
}

// TestRunPostImport_HookErrorPropagates — ошибка пересчёта прокидывается.
func TestRunPostImport_HookErrorPropagates(t *testing.T) {
	hook := &countingHook{recErr: errors.New("recalc boom")}
	if err := pipeline.RunPostImport(context.Background(), &recordingNorm{}, hook); err == nil {
		t.Fatal("ожидалась ошибка пересчёта, got nil")
	}
}

// TestRunPostImport_NilHookUsesNoop — анти-фиктивность (AC1): шов наблюдаем БЕЗ реального потребителя —
// nil hook → noopRecalc, конвейер выполняется без паники.
func TestRunPostImport_NilHookUsesNoop(t *testing.T) {
	norm := &recordingNorm{}
	if err := pipeline.RunPostImport(context.Background(), norm, nil); err != nil {
		t.Fatalf("RunPostImport с nil-хуком (noop) должен пройти, got %v", err)
	}
	if !norm.called {
		t.Fatal("нормализация должна была выполниться даже при noop-хуке")
	}
}

// TestRunPostImport_NilNormalizerSkipsNormalize — нормализатор не задан → только пересчёт (хук всё равно 1 раз).
func TestRunPostImport_NilNormalizerSkipsNormalize(t *testing.T) {
	hook := &countingHook{}
	if err := pipeline.RunPostImport(context.Background(), nil, hook); err != nil {
		t.Fatalf("RunPostImport с nil-нормализатором: %v", err)
	}
	if hook.calls != 1 {
		t.Fatalf("хук вызван %d раз, ожидалось 1", hook.calls)
	}
}

// fakeBench / fakeFlags — фиксируют порядок benchmark→flags для проверки делегирования RecalcRunHook.
type fakeBench struct{ order *[]string }

func (f fakeBench) RecalcBenchmarks(context.Context) error {
	*f.order = append(*f.order, "benchmark")
	return nil
}

type fakeFlags struct {
	order *[]string
	label string // различимая метка (для проверки ПОРЯДКА в композите); пусто → "flags"
	err   error
}

func (f fakeFlags) RecalcFlags(context.Context) error {
	lbl := f.label
	if lbl == "" {
		lbl = "flags"
	}
	*f.order = append(*f.order, lbl)
	return f.err
}

// TestRecalcRunHook_DelegatesBenchmarkThenFlags — реальный хук оборачивает recalc.Run: benchmark ПЕРЕД flags.
func TestRecalcRunHook_DelegatesBenchmarkThenFlags(t *testing.T) {
	var order []string
	hook := pipeline.RecalcRunHook{Bench: fakeBench{order: &order}, Flags: fakeFlags{order: &order}}
	if err := hook.Recalc(context.Background()); err != nil {
		t.Fatalf("Recalc: %v", err)
	}
	if len(order) != 2 || order[0] != "benchmark" || order[1] != "flags" {
		t.Fatalf("порядок = %v, ожидалось [benchmark flags]", order)
	}
}

// TestRecalcRunHook_NilDepsHonestError — nil Bench/Flags → честная ошибка (не nil-паника).
func TestRecalcRunHook_NilDepsHonestError(t *testing.T) {
	var order []string
	cases := []pipeline.RecalcRunHook{
		{Flags: fakeFlags{order: &order}}, // nil Bench
		{Bench: fakeBench{order: &order}}, // nil Flags
	}
	for i, h := range cases {
		if err := h.Recalc(context.Background()); err == nil {
			t.Errorf("case %d: ожидалась ошибка при nil-зависимости, got nil", i)
		}
	}
}

// TestCompositeFlags_RunsAllInOrder — композит вызывает суб-рекалькуляторы по порядку (один FlagRecalculator
// из четырёх флагов Epic 4).
func TestCompositeFlags_RunsAllInOrder(t *testing.T) {
	var order []string
	comp := pipeline.CompositeFlags{
		fakeFlags{order: &order, label: "a"}, fakeFlags{order: &order, label: "b"}, fakeFlags{order: &order, label: "c"},
	}
	if err := comp.RecalcFlags(context.Background()); err != nil {
		t.Fatalf("CompositeFlags: %v", err)
	}
	if len(order) != 3 || order[0] != "a" || order[1] != "b" || order[2] != "c" {
		t.Fatalf("порядок суб-рекалькуляторов = %v, ожидалось [a b c]", order)
	}
}

// TestCompositeFlags_NilElementHonestError — nil суб-рекалькулятор → честная ошибка (не nil-паника);
// элементы до nil выполняются, после — нет.
func TestCompositeFlags_NilElementHonestError(t *testing.T) {
	var order []string
	comp := pipeline.CompositeFlags{fakeFlags{order: &order, label: "a"}, nil, fakeFlags{order: &order, label: "c"}}
	if err := comp.RecalcFlags(context.Background()); err == nil {
		t.Fatal("ожидалась честная ошибка на nil-элементе, got nil")
	}
	if len(order) != 1 || order[0] != "a" {
		t.Fatalf("до nil должен выполниться только 'a', got %v", order)
	}
}

// TestCompositeFlags_EmptyIsNoop — пустой композит — no-op (без паники).
func TestCompositeFlags_EmptyIsNoop(t *testing.T) {
	if err := pipeline.CompositeFlags(nil).RecalcFlags(context.Background()); err != nil {
		t.Fatalf("пустой композит должен быть no-op, got %v", err)
	}
}

// TestCompositeFlags_ErrorStopsAndPropagates — ошибка суб-рекалькулятора прокидывается и останавливает дальнейшие.
func TestCompositeFlags_ErrorStopsAndPropagates(t *testing.T) {
	var order []string
	comp := pipeline.CompositeFlags{
		fakeFlags{order: &order},
		fakeFlags{order: &order, err: errors.New("boom")},
		fakeFlags{order: &order}, // не должен вызваться
	}
	if err := comp.RecalcFlags(context.Background()); err == nil {
		t.Fatal("ожидалась ошибка суб-рекалькулятора, got nil")
	}
	if len(order) != 2 {
		t.Fatalf("после ошибки на 2-м должно быть 2 вызова, got %d (%v)", len(order), order)
	}
}

// incClock — фейковые часы, продвигающиеся на 1 час при каждом Now() (для проверки FreezeClock).
type incClock struct{ n int }

func (c *incClock) Now() time.Time {
	c.n++
	return time.Unix(int64(c.n)*3600, 0).UTC()
}

// TestFreezeClock_PinsSingleNow — долг RNU (deferred-work:218): один `Now()` на весь проход пересчёта.
// FreezeClock снимает время ОДИН раз; замороженные часы возвращают то же значение, сколько ни спрашивай
// (длинный батч не пересечёт границу суток).
func TestFreezeClock_PinsSingleNow(t *testing.T) {
	underlying := &incClock{}
	frozen := pipeline.FreezeClock(underlying)
	first := frozen.Now()
	second := frozen.Now()
	if !first.Equal(second) {
		t.Fatalf("замороженные часы вернули разное время: %v != %v", first, second)
	}
	if underlying.n != 1 {
		t.Fatalf("underlying.Now() вызван %d раз, ожидалось 1 (снято единожды)", underlying.n)
	}
	if !first.Equal(time.Unix(3600, 0).UTC()) {
		t.Fatalf("заморожено %v, ожидалось первое значение underlying (1ч)", first)
	}
}

// TestFreezeClock_NilDefaultsToReal — FreezeClock(nil) не паникует (дефолт — реальные часы) и возвращает
// стабильные замороженные часы (а не nil-дереференс).
func TestFreezeClock_NilDefaultsToReal(t *testing.T) {
	frozen := pipeline.FreezeClock(nil)
	a := frozen.Now()
	b := frozen.Now()
	if !a.Equal(b) {
		t.Fatalf("FreezeClock(nil) вернул нестабильные часы: %v != %v", a, b)
	}
	if a.IsZero() {
		t.Fatal("FreezeClock(nil) вернул нулевое время — ожидалось реальное «сейчас»")
	}
}

// Compile-time: типы реализуют ожидаемые контракты.
var (
	_ pipeline.RecalcHook     = pipeline.RecalcRunHook{}
	_ recalc.FlagRecalculator = pipeline.CompositeFlags(nil)
	_ clock.Clock             = pipeline.FreezeClock(clock.Fixed{})
)
