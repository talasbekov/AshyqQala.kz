//go:build integration

// Интеграционные тесты транзакционного outbox (Story 2.7, O-1..O-4) на РЕАЛЬНОЙ БД. Конвенция репо:
// //go:build integration + DATABASE_URL + t.Skip; миграции применяются внешне; -p 1 (общая БД). Каждый страж
// краснеет (negative-control): O-1 без tx — бизнес-строка пережила бы откат; O-3 без UNIQUE — 2 строки;
// SKIP LOCKED без него — tx2 взял бы залоченную строку (двойная доставка); O-4 без сдвига — строка видна сразу.
// Гоняется: `go test -tags=integration ./internal/outbox/...` с DATABASE_URL (миграции 0001-0016). [[guards-must-prove-red]]
package outbox_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/clock"
	"ashyqqala/server/internal/outbox"
	"ashyqqala/server/internal/store/gen"
)

func integrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL не задан — пропуск интеграционного теста")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("подключение к БД: %v", err)
	}
	return pool
}

// resetOutbox — пустой outbox перед тестом (перезапускаемость; -p 1 → без гонок между тестами).
func resetOutbox(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), "TRUNCATE notifications_outbox"); err != nil {
		t.Fatalf("TRUNCATE notifications_outbox (применена ли миграция 0016?): %v", err)
	}
}

// countingDispatcher — учитывает вызовы Send (O-2 / SKIP LOCKED / «доставлено ровно 1×»).
type countingDispatcher struct{ n int }

func (d *countingDispatcher) Send(context.Context, outbox.Event) error { d.n++; return nil }

// failingDispatcher — всегда ошибается (O-4: растим attempts, сдвигаем видимость на backoff).
type failingDispatcher struct{ n int }

func (d *failingDispatcher) Send(context.Context, outbox.Event) error {
	d.n++
	return errors.New("boom: доставка не удалась")
}

func uid(e outbox.Event) pgtype.UUID { return pgtype.UUID{Bytes: e.EventID, Valid: true} }

// TestEnqueue_Atomic_O1 — O-1: бизнес-вставка (organizations) + Enqueue в ОДНОЙ tx. Rollback ⇒ НЕТ ни той,
// ни другой строки; commit ⇒ обе есть. КОНТРОЛЬ: без транзакции вызывающего бизнес-строка пережила бы откат.
func TestEnqueue_Atomic_O1(t *testing.T) {
	pool := integrationPool(t)
	defer pool.Close()
	ctx := context.Background()
	resetOutbox(t, pool)

	const bin = "999999999901" // фейковый БИН (не пересекается с фикстурами/seed)
	clean := func() {
		_, _ = pool.Exec(ctx, "DELETE FROM organizations WHERE bin = $1", bin)
		_, _ = pool.Exec(ctx, "TRUNCATE notifications_outbox")
	}
	clean()
	t.Cleanup(clean)

	ev := outbox.Event{
		EventID:    outbox.NewEventID("contract", bin),
		Type:       outbox.TypeContractCreated,
		OccurredAt: time.Unix(1_700_000_000, 0).UTC(),
		SubjectRef: outbox.SubjectURN("contract", bin),
		V:          1,
	}

	// --- ROLLBACK: обе записи в одной tx, затем откат ⇒ ни организации, ни outbox-строки.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := gen.New(tx).UpsertOrganization(ctx, gen.UpsertOrganizationParams{Bin: bin, IsSupplier: true}); err != nil {
		t.Fatalf("бизнес-вставка организации: %v", err)
	}
	if err := outbox.Enqueue(ctx, tx, ev); err != nil {
		t.Fatalf("Enqueue в tx: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if _, err := gen.New(pool).GetOrganizationByBIN(ctx, bin); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("после rollback организация присутствует — НЕ атомарно (err=%v)", err)
	}
	if _, err := gen.New(pool).GetOutboxByEventID(ctx, uid(ev)); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("после rollback outbox-строка присутствует — НЕ атомарно (err=%v)", err)
	}

	// --- COMMIT: те же записи в одной tx с COMMIT ⇒ ОБЕ присутствуют.
	tx2, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin2: %v", err)
	}
	if err := gen.New(tx2).UpsertOrganization(ctx, gen.UpsertOrganizationParams{Bin: bin, IsSupplier: true}); err != nil {
		t.Fatalf("бизнес-вставка (commit): %v", err)
	}
	if err := outbox.Enqueue(ctx, tx2, ev); err != nil {
		t.Fatalf("Enqueue в tx2: %v", err)
	}
	if err := tx2.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if _, err := gen.New(pool).GetOrganizationByBIN(ctx, bin); err != nil {
		t.Fatalf("после commit организации нет: %v", err)
	}
	row, err := gen.New(pool).GetOutboxByEventID(ctx, uid(ev))
	if err != nil {
		t.Fatalf("после commit outbox-строки нет: %v", err)
	}
	if row.SentAt.Valid {
		t.Error("свежая outbox-строка помечена sent_at — не должна быть доставлена")
	}
	if row.Attempts != 0 {
		t.Errorf("свежая outbox-строка attempts=%d, ожидалось 0", row.Attempts)
	}
}

// TestEnqueue_Dedup_O3 — O-3: 2× Enqueue одного event_id ⇒ 1 строка (ON CONFLICT DO NOTHING). КОНТРОЛЬ:
// другое событие добавляет строку (таблица не «всегда 1»; дедуп по event_id, а не по факту вставки).
func TestEnqueue_Dedup_O3(t *testing.T) {
	pool := integrationPool(t)
	defer pool.Close()
	ctx := context.Background()
	resetOutbox(t, pool)

	ev := outbox.Event{
		EventID:    outbox.NewEventID("flag", "44071234", "single_participant"),
		Type:       outbox.TypeFlagRaised,
		OccurredAt: time.Unix(1_700_000_000, 0).UTC(),
		SubjectRef: outbox.SubjectURN("contract", "44071234"),
	}
	if err := outbox.Enqueue(ctx, pool, ev); err != nil {
		t.Fatalf("Enqueue #1: %v", err)
	}
	if err := outbox.Enqueue(ctx, pool, ev); err != nil {
		t.Fatalf("Enqueue #2 (должен быть no-op): %v", err)
	}
	if n, err := gen.New(pool).CountOutbox(ctx); err != nil || n != 1 {
		t.Fatalf("2× Enqueue одного события: CountOutbox=%d err=%v, ожидалось 1 (дедуп O-3)", n, err)
	}

	// negative control: ДРУГОЙ event_id не дедуплицируется.
	ev2 := ev
	ev2.EventID = outbox.NewEventID("flag", "55555555", "single_participant")
	if err := outbox.Enqueue(ctx, pool, ev2); err != nil {
		t.Fatalf("Enqueue другого события: %v", err)
	}
	if n, err := gen.New(pool).CountOutbox(ctx); err != nil || n != 2 {
		t.Fatalf("CountOutbox=%d err=%v, ожидалось 2 (разные event_id не склеиваются)", n, err)
	}
}

// TestProcessBatch_DeliversAndMarksSent_O2 — O-2: enqueued событие → ProcessBatch вызывает Dispatcher.Send 1×
// и ставит sent_at. КОНТРОЛЬ: повторный проход НЕ передоставляет (строка sent → выпала из поллинга).
func TestProcessBatch_DeliversAndMarksSent_O2(t *testing.T) {
	pool := integrationPool(t)
	defer pool.Close()
	ctx := context.Background()
	resetOutbox(t, pool)

	ev := outbox.Event{
		EventID:    outbox.NewEventID("contract", "100500"),
		Type:       outbox.TypeContractCreated,
		OccurredAt: time.Unix(1_700_000_000, 0).UTC(),
		SubjectRef: outbox.SubjectURN("contract", "100500"),
	}
	if err := outbox.Enqueue(ctx, pool, ev); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	// «сейчас» выводим из РЕАЛЬНОГО available_at строки (+1s) → строка гарантированно видна (без skew host/БД).
	row0, err := gen.New(pool).GetOutboxByEventID(ctx, uid(ev))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	now := row0.AvailableAt.Time.Add(time.Second)

	disp := &countingDispatcher{}
	sent, err := outbox.ProcessBatch(ctx, pool, disp, clock.Fixed{T: now}, 10)
	if err != nil {
		t.Fatalf("ProcessBatch: %v", err)
	}
	if sent != 1 || disp.n != 1 {
		t.Fatalf("первый проход: sent=%d dispatched=%d, ожидалось 1/1", sent, disp.n)
	}
	row1, err := gen.New(pool).GetOutboxByEventID(ctx, uid(ev))
	if err != nil {
		t.Fatalf("read после доставки: %v", err)
	}
	if !row1.SentAt.Valid {
		t.Error("sent_at не проставлен после успешной доставки (O-2)")
	}

	// negative control: повторный проход НЕ передоставляет (counting dispatcher не растёт).
	sent2, err := outbox.ProcessBatch(ctx, pool, disp, clock.Fixed{T: now}, 10)
	if err != nil {
		t.Fatalf("ProcessBatch #2: %v", err)
	}
	if sent2 != 0 || disp.n != 1 {
		t.Fatalf("повторный проход передоставил: sent=%d dispatched(total)=%d, ожидалось 0/1", sent2, disp.n)
	}
}

// TestPollUnsent_SkipLocked — O-2 конкурентность: пока tx1 держит FOR UPDATE-лок строки, tx2 (другое
// соединение) её ПРОПУСКАЕТ (SKIP LOCKED) → два воркера не двоят доставку. КОНТРОЛЬ: после освобождения
// лока строка снова видна (иначе проверка вакуумна). Модель — pipeline/lock_integration_test.
func TestPollUnsent_SkipLocked(t *testing.T) {
	pool := integrationPool(t)
	defer pool.Close()
	ctx := context.Background()
	resetOutbox(t, pool)

	ev := outbox.Event{
		EventID:    outbox.NewEventID("contract", "777"),
		Type:       outbox.TypeContractCreated,
		OccurredAt: time.Unix(1_700_000_000, 0).UTC(),
		SubjectRef: outbox.SubjectURN("contract", "777"),
	}
	if err := outbox.Enqueue(ctx, pool, ev); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	row0, err := gen.New(pool).GetOutboxByEventID(ctx, uid(ev))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	now := pgtype.Timestamptz{Time: row0.AvailableAt.Time.Add(time.Second), Valid: true}
	params := gen.PollUnsentParams{AvailableAt: now, Limit: 10}

	// tx1 берёт строку под FOR UPDATE (SKIP LOCKED) и ДЕРЖИТ лок.
	tx1, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx1: %v", err)
	}
	defer tx1.Rollback(ctx) //nolint:errcheck // безопасный no-op, если уже откатили ниже
	rows1, err := gen.New(tx1).PollUnsent(ctx, params)
	if err != nil {
		t.Fatalf("poll tx1: %v", err)
	}
	if len(rows1) != 1 {
		t.Fatalf("tx1 должен взять 1 строку, взял %d", len(rows1))
	}

	// tx2 (другое соединение) поллит под занятым локом → SKIP LOCKED ⇒ 0 строк.
	tx2, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx2: %v", err)
	}
	rows2, err := gen.New(tx2).PollUnsent(ctx, params)
	if err != nil {
		t.Fatalf("poll tx2: %v", err)
	}
	_ = tx2.Rollback(ctx)
	if len(rows2) != 0 {
		t.Fatalf("tx2 взял %d строк под занятым локом — SKIP LOCKED НЕ работает (двойная доставка)", len(rows2))
	}

	// negative control: освобождаем лок → строка снова видна другому воркеру.
	if err := tx1.Rollback(ctx); err != nil {
		t.Fatalf("rollback tx1: %v", err)
	}
	tx3, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx3: %v", err)
	}
	defer tx3.Rollback(ctx) //nolint:errcheck
	rows3, err := gen.New(tx3).PollUnsent(ctx, params)
	if err != nil {
		t.Fatalf("poll tx3: %v", err)
	}
	if len(rows3) != 1 {
		t.Fatalf("после освобождения лока строка не видна (взял %d) — тест SKIP LOCKED был бы вакуумен", len(rows3))
	}
}

// TestProcessBatch_RetryVisibility_O4 — O-4: неудачная доставка растит attempts и сдвигает available_at на
// backoff; строка СКРЫТА до now+backoff (детерминизм через clock.Fixed). КОНТРОЛЬ: тем же «сейчас» строка
// не доставляется (скрыта); со сдвинутым «сейчас» — доставляется. Без сдвига видимости строка шла бы сразу.
func TestProcessBatch_RetryVisibility_O4(t *testing.T) {
	pool := integrationPool(t)
	defer pool.Close()
	ctx := context.Background()
	resetOutbox(t, pool)

	ev := outbox.Event{
		EventID:    outbox.NewEventID("contract", "424242"),
		Type:       outbox.TypeContractCreated,
		OccurredAt: time.Unix(1_700_000_000, 0).UTC(),
		SubjectRef: outbox.SubjectURN("contract", "424242"),
	}
	if err := outbox.Enqueue(ctx, pool, ev); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	row0, err := gen.New(pool).GetOutboxByEventID(ctx, uid(ev))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	now := row0.AvailableAt.Time.Add(time.Second)

	// Падающая доставка: attempts++ и сдвиг видимости; ошибка Send НЕ валит батч.
	fail := &failingDispatcher{}
	sent, err := outbox.ProcessBatch(ctx, pool, fail, clock.Fixed{T: now}, 10)
	if err != nil {
		t.Fatalf("ProcessBatch(fail): %v", err)
	}
	if sent != 0 || fail.n != 1 {
		t.Fatalf("неудачная доставка: sent=%d dispatched=%d, ожидалось 0/1", sent, fail.n)
	}
	row1, err := gen.New(pool).GetOutboxByEventID(ctx, uid(ev))
	if err != nil {
		t.Fatalf("read после неудачи: %v", err)
	}
	if row1.Attempts != 1 {
		t.Errorf("attempts=%d после одной неудачи, ожидалось 1", row1.Attempts)
	}
	if row1.SentAt.Valid {
		t.Error("sent_at проставлен при неудачной доставке — не должен")
	}
	if shift := row1.AvailableAt.Time.Sub(now); shift < 29*time.Second || shift > 31*time.Second {
		t.Errorf("сдвиг видимости = %v, ожидалось ~30s (backoff(1)) [O-4]", shift)
	}

	// negative control 1: тем же «сейчас» строка СКРЫТА (available_at в будущем) → НЕ доставляется.
	ok := &countingDispatcher{}
	sentHidden, err := outbox.ProcessBatch(ctx, pool, ok, clock.Fixed{T: now}, 10)
	if err != nil {
		t.Fatalf("ProcessBatch(hidden): %v", err)
	}
	if sentHidden != 0 || ok.n != 0 {
		t.Fatalf("скрытая строка доставлена тем же now: sent=%d dispatched=%d, ожидалось 0/0 (O-4 visibility)", sentHidden, ok.n)
	}

	// сдвигаем «сейчас» за backoff → строка снова видна → доставляется (детерминированный ретрай).
	later := now.Add(31 * time.Second)
	sentLater, err := outbox.ProcessBatch(ctx, pool, ok, clock.Fixed{T: later}, 10)
	if err != nil {
		t.Fatalf("ProcessBatch(later): %v", err)
	}
	if sentLater != 1 || ok.n != 1 {
		t.Fatalf("после backoff строка не доставлена: sent=%d dispatched=%d, ожидалось 1/1", sentLater, ok.n)
	}
	row2, err := gen.New(pool).GetOutboxByEventID(ctx, uid(ev))
	if err != nil {
		t.Fatalf("read финал: %v", err)
	}
	if !row2.SentAt.Valid {
		t.Error("sent_at не проставлен после успешного ретрая")
	}
}
