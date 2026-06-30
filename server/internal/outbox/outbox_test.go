package outbox

import (
	"bytes"
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"ashyqqala/server/internal/registry"
)

// loadRegistry загружает КАНОНИЧЕСКИЙ реестр из корня репо (как render_test/registry_test): outbox →
// internal → server → repo-root.
func loadRegistry(t *testing.T) *registry.Registry {
	t.Helper()
	reg, err := registry.Load(filepath.Join("..", "..", "..", "registry"))
	if err != nil {
		t.Fatalf("registry.Load: %v", err)
	}
	return reg
}

// assertNoTaboo: текст не несёт НИ ОДНОГО табуированного корня НИ В ОДНОЙ локали — через КАНОНИЧЕСКИЙ
// матчер реестра (casefold + homoglyph-fold), а не локальный хардкод-список. Так страж нейтральности не
// дрейфует ниже канона при пополнении taboo_lexicon.json (этос guards-must-prove-red).
func assertNoTaboo(t *testing.T, text string) {
	t.Helper()
	reg := loadRegistry(t)
	for _, loc := range registry.AllLocales() {
		if hits := reg.FindTaboo(loc, text); len(hits) > 0 {
			t.Errorf("текст содержит табуированные корни %v (локаль %s) — нарушение нейтральности: %s", hits, loc, text)
		}
	}
}

// noopDBTX — фейковая gen.DBTX для unit-проверки валидации Enqueue БЕЗ БД (Exec — no-op успех).
type noopDBTX struct{}

func (noopDBTX) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (noopDBTX) Query(context.Context, string, ...interface{}) (pgx.Rows, error) { return nil, nil }
func (noopDBTX) QueryRow(context.Context, string, ...interface{}) pgx.Row        { return nil }

func TestNewEventID_Deterministic(t *testing.T) {
	a := NewEventID("contract", "44071234")
	b := NewEventID("contract", "44071234")
	if a != b {
		t.Fatalf("NewEventID не детерминирован: %s != %s", a, b)
	}
	// negative control: другой вход → другой id (иначе «всегда одинаковый» прошёл бы мимо → дедуп склеил бы разное).
	if c := NewEventID("contract", "99999999"); a == c {
		t.Fatalf("разный вход дал тот же id (%s) — разные события дедуплицировались бы ошибочно", a)
	}
	// границы частей не спутываются: {a,bc} != {ab,c}.
	if NewEventID("a", "bc") == NewEventID("ab", "c") {
		t.Fatal("границы частей спутаны: {a,bc} == {ab,c}")
	}
	if a.IsZero() {
		t.Fatal("NewEventID вернул нулевой id")
	}
	// UUIDv5: версия 5, вариант RFC 4122 (10b).
	if v := a[6] >> 4; v != 5 {
		t.Errorf("версия UUID = %d, ожидалось 5", v)
	}
	if variant := a[8] >> 6; variant != 0b10 {
		t.Errorf("вариант UUID = %#b, ожидалось 0b10 (RFC 4122)", variant)
	}
}

func TestEventID_String(t *testing.T) {
	id := EventID{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef}
	if got, want := id.String(), "01234567-89ab-cdef-0123-456789abcdef"; got != want {
		t.Fatalf("String() = %q, ожидалось %q", got, want)
	}
	if got := (EventID{}).String(); got != "00000000-0000-0000-0000-000000000000" {
		t.Errorf("нулевой id: %q", got)
	}
}

func TestSubjectURN(t *testing.T) {
	if got := SubjectURN("contract", "44071234"); got != "contract:44071234" {
		t.Fatalf("SubjectURN = %q, ожидалось contract:44071234", got)
	}
}

func TestBackoff_Deterministic(t *testing.T) {
	if got := backoff(1); got != 30*time.Second {
		t.Errorf("backoff(1) = %v, ожидалось 30s", got)
	}
	if got := backoff(2); got != 60*time.Second {
		t.Errorf("backoff(2) = %v, ожидалось 60s", got)
	}
	// кламп attempts<1 → как attempts=1 (страховка от нулевого/отрицательного счётчика).
	if backoff(0) != backoff(1) || backoff(-5) != backoff(1) {
		t.Error("backoff(<1) должен клампиться к backoff(1)")
	}
	// negative control: монотонно растёт с attempts (иначе «константа» прошла бы тест мимо O-4).
	if !(backoff(1) < backoff(2) && backoff(2) < backoff(3)) {
		t.Error("backoff не монотонно растёт с attempts")
	}
	// потолок (иначе линейная формула ушла бы в бесконечность).
	if got := backoff(100000); got != backoffMax {
		t.Errorf("backoff(огромное) = %v, ожидался потолок %v", got, backoffMax)
	}
	// детерминизм повтора.
	if backoff(7) != backoff(7) {
		t.Error("backoff недетерминирован")
	}
}

// TestShouldDeadLetter — ЧИСТАЯ политика потолка попыток (Story 7.1, dead-letter): под потолком → false; на
// потолке (attempts+1 ≥ max) → true; без потолка (max ≤ 0) → никогда (случайный 0 не убивает очередь молча).
func TestShouldDeadLetter(t *testing.T) {
	cases := []struct {
		attempts, max int
		want          bool
	}{
		{0, 10, false}, // 1-я попытка
		{8, 10, false}, // 9-я < 10
		{9, 10, true},  // 10-я == потолок → dead
		{10, 10, true}, // сверх потолка
		{0, 2, false},  // граница для интеграционного теста
		{1, 2, true},   // потолок 2 достигнут
		{100, 0, false},
		{100, -1, false},
	}
	for _, c := range cases {
		if got := shouldDeadLetter(c.attempts, c.max); got != c.want {
			t.Errorf("shouldDeadLetter(%d,%d) = %v; want %v", c.attempts, c.max, got, c.want)
		}
	}
}

func TestLoggingDispatcher_Send_NeutralAndSucceeds(t *testing.T) {
	var buf bytes.Buffer
	d := LoggingDispatcher{Log: slog.New(slog.NewJSONHandler(&buf, nil))}
	e := Event{
		EventID:    NewEventID("contract", "44071234"),
		Type:       TypeFlagRaised,
		OccurredAt: time.Unix(1_700_000_000, 0).UTC(),
		SubjectRef: SubjectURN("contract", "44071234"),
		V:          1,
	}
	if err := d.Send(context.Background(), e); err != nil {
		t.Fatalf("Send вернул ошибку: %v", err)
	}
	out := buf.String()
	// negative control: реально залогировал нейтральный конверт (id/type/subject_ref) — иначе проверка
	// нейтральности ниже была бы вакуумной (пустой лог тривиально «нейтрален»).
	for _, want := range []string{"outbox_dispatch", e.EventID.String(), TypeFlagRaised, "contract:44071234"} {
		if !strings.Contains(out, want) {
			t.Errorf("лог не содержит ожидаемое %q: %s", want, out)
		}
	}
	// нейтральность (CLAUDE.md#guardrails): ни одного табуированного корня — конверт не несёт прозу флага.
	// Источник табу — КАНОНИЧЕСКИЙ реестр (taboo_lexicon.json) + его матчер (casefold + homoglyph-fold),
	// чтобы страж не дрейфовал ниже канона при пополнении лексикона (см. assertNoTaboo).
	assertNoTaboo(t, out)
}

func TestLoggingDispatcher_DoesNotLogPayloadProse(t *testing.T) {
	var buf bytes.Buffer
	d := LoggingDispatcher{Log: slog.New(slog.NewJSONHandler(&buf, nil))}
	// payload С табуированной прозой: диспетчер НЕ должен её логировать (нейтральность — через render на
	// поверхности bot, Epic 7; здесь конверт нейтрален). Этот страж покраснел бы, если payload попал в лог.
	e := Event{
		EventID:    NewEventID("flag", "1"),
		Type:       TypeFlagRaised,
		OccurredAt: time.Unix(1_700_000_000, 0).UTC(),
		SubjectRef: SubjectURN("contract", "1"),
		Payload:    []byte(`{"note":"коррупция и нарушение, виновен"}`),
		V:          1,
	}
	if err := d.Send(context.Background(), e); err != nil {
		t.Fatalf("Send: %v", err)
	}
	out := buf.String()
	// табу — из канонического реестра, не из хардкод-подмножества (анти-дрейф; см. assertNoTaboo).
	assertNoTaboo(t, out)
	// не вакуумно: событие всё же залогировано (id присутствует) — иначе «нет прозы» выполнялось бы тривиально.
	if !strings.Contains(buf.String(), e.EventID.String()) {
		t.Errorf("событие не залогировано вовсе (тест вакуумен): %s", buf.String())
	}
}

func TestLoggingDispatcher_NilLog_NoPanic(t *testing.T) {
	d := LoggingDispatcher{} // Log == nil → безопасный no-op
	if err := d.Send(context.Background(), Event{Type: TypeContractCreated}); err != nil {
		t.Fatalf("Send с nil-логгером: %v", err)
	}
}

func TestEnqueue_ValidatesEnvelope(t *testing.T) {
	ctx := context.Background()
	valid := Event{
		EventID:    NewEventID("x"),
		Type:       "a.b",
		SubjectRef: SubjectURN("contract", "1"),
		OccurredAt: time.Unix(1_700_000_000, 0).UTC(),
	}
	// Каждый обязательный элемент конверта валидируется (negative control по одному полю за раз).
	cases := []struct {
		name   string
		mutate func(*Event)
	}{
		{"нулевой event_id (нужен id эмиттера для дедупа O-3)", func(e *Event) { e.EventID = EventID{} }},
		{"пустой type", func(e *Event) { e.Type = "" }},
		{"пустой subject_ref (load-bearing поле AC1)", func(e *Event) { e.SubjectRef = "" }},
		{"нулевой occurred_at (честность, не 0001-01-01)", func(e *Event) { e.OccurredAt = time.Time{} }},
		{"невалидный JSON payload (колонка JSONB)", func(e *Event) { e.Payload = []byte("{не json") }},
		{"не-объектный payload (валидный JSON, но не {...} — конверт обязан быть объектом)", func(e *Event) { e.Payload = []byte("[1,2,3]") }},
	}
	for _, c := range cases {
		bad := valid
		c.mutate(&bad)
		if err := Enqueue(ctx, noopDBTX{}, bad); err == nil {
			t.Errorf("Enqueue должен ошибаться: %s", c.name)
		}
	}
	// nil tx → ошибка (нельзя писать вне транзакции вызывающего, O-1).
	if err := Enqueue(ctx, nil, valid); err == nil {
		t.Error("Enqueue с nil tx должен ошибаться")
	}
	// positive control: валидный конверт проходит валидацию (доходит до EnqueueEvent → no-op Exec).
	if err := Enqueue(ctx, noopDBTX{}, valid); err != nil {
		t.Errorf("валидный Enqueue не должен ошибаться: %v", err)
	}
	// пустой slice payload — НЕ ошибка (нормализуется в {}, как nil).
	empty := valid
	empty.Payload = []byte{}
	if err := Enqueue(ctx, noopDBTX{}, empty); err != nil {
		t.Errorf("пустой payload должен нормализоваться в {}, а не падать: %v", err)
	}
	// объектный payload проходит (конверт по контракту — объект {...}).
	obj := valid
	obj.Payload = []byte(`{"k":"v"}`)
	if err := Enqueue(ctx, noopDBTX{}, obj); err != nil {
		t.Errorf("объектный payload не должен падать: %v", err)
	}
}

// TestNeutralityGuard_CanonicalMatcher_ProvesRed — negative control для assertNoTaboo: канонический матчер
// РЕАЛЬНО ловит табу (иначе «нет табу» выполнялось бы тривиально при пустом/сломанном матчере) и сильнее
// прежнего хардкода — ловит гомоглиф-обфускацию (лат→кир).
func TestNeutralityGuard_CanonicalMatcher_ProvesRed(t *testing.T) {
	reg := loadRegistry(t)
	if hits := reg.FindTaboo(registry.RU, "в акте обнаружена коррупция"); len(hits) == 0 {
		t.Fatal("канонический матчер НЕ поймал явное табу — страж нейтральности был бы вакуумным")
	}
	// гомоглиф: латиница 'a' в «нарушение» (пример из neutrality.go) — хардкод-substring это пропускал бы.
	if hits := reg.FindTaboo(registry.RU, "нaрушение акта"); len(hits) == 0 {
		t.Error("матчер не ловит гомоглиф-обфускацию табу (лат. 'a' в «нарушение»)")
	}
	// kk-локаль покрыта (прежний хардкод был только ru).
	if hits := reg.FindTaboo(registry.KK, "сыбайлас жемқорлық"); len(hits) == 0 {
		t.Error("матчер не ловит kk-табу (жемқор) — kk-локаль вне покрытия")
	}
}
