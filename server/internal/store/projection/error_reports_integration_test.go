//go:build integration

// Интеграционный тест error_reports (Story 5.4, FR-28): вставка обращения через sqlc InsertErrorReport +
// чтение + DB-уровневые CHECK-страхи (kind/subject_type enum, message непустой, resolved ⟺ финальный статус).
// Гоняется: `go test -tags=integration ./internal/store/...` с DATABASE_URL (миграции 0001-0011).
package projection_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/store/gen"
)

func errReportPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL не задан — пропуск интеграционного теста")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	return pool
}

func TestErrorReports_InsertAndRead(t *testing.T) {
	pool := errReportPool(t)
	defer pool.Close()
	ctx := context.Background()
	q := gen.New(pool)

	clean := func() { _, _ = pool.Exec(ctx, "DELETE FROM error_reports WHERE subject_ref LIKE 'ITEST-%'") }
	clean()
	defer clean()

	// Вставка валидного обращения: contact пуст → NULL.
	row, err := q.InsertErrorReport(ctx, gen.InsertErrorReportParams{
		Kind:        "flag_error",
		SubjectType: "contract",
		SubjectRef:  "ITEST-0002",
		Message:     "цена за км выглядит завышенной",
	})
	if err != nil {
		t.Fatalf("InsertErrorReport: %v", err)
	}
	if row.ID <= 0 || !row.CreatedAt.Valid {
		t.Fatalf("ожидался id>0 и created_at, получено %+v", row)
	}

	got, err := q.GetErrorReport(ctx, row.ID)
	if err != nil {
		t.Fatalf("GetErrorReport: %v", err)
	}
	if got.Status != "new" {
		t.Fatalf("status по умолчанию = %q, ожидалось new", got.Status)
	}
	if got.Contact.Valid {
		t.Fatalf("пустой contact должен быть NULL, получено %+v", got.Contact)
	}
	if got.ResolvedAt.Valid {
		t.Fatalf("new-обращение не должно иметь resolved_at")
	}
}

// DB-уровневые страхи (negative-control): CHECK-констрейнты отбивают мусор даже в обход серверной валидации.
func TestErrorReports_DBConstraints(t *testing.T) {
	pool := errReportPool(t)
	defer pool.Close()
	ctx := context.Background()

	exec := func(sql string, args ...any) error {
		_, err := pool.Exec(ctx, sql, args...)
		return err
	}

	// kind вне enum → CHECK error.
	if err := exec(`INSERT INTO error_reports (kind, subject_type, subject_ref, message) VALUES ('nope','contract','ITEST-x','m')`); err == nil {
		t.Fatal("kind вне enum должен отбиваться CHECK error_reports_kind_chk")
	}
	// subject_type вне enum → CHECK error.
	if err := exec(`INSERT INTO error_reports (kind, subject_type, subject_ref, message) VALUES ('data_error','nope','ITEST-x','m')`); err == nil {
		t.Fatal("subject_type вне enum должен отбиваться CHECK")
	}
	// пустой message → CHECK error.
	if err := exec(`INSERT INTO error_reports (kind, subject_type, subject_ref, message) VALUES ('data_error','contract','ITEST-x','   ')`); err == nil {
		t.Fatal("пустой message должен отбиваться CHECK error_reports_message_nonempty_chk")
	}
	// resolved-статус без resolved_at → CHECK error (инвариант честной очереди).
	err := exec(`INSERT INTO error_reports (kind, subject_type, subject_ref, message, status) VALUES ('data_error','contract','ITEST-x','m','resolved')`)
	if err == nil {
		t.Fatal("status=resolved без resolved_at должен отбиваться CHECK error_reports_resolved_chk")
	}
	_, _ = pool.Exec(ctx, "DELETE FROM error_reports WHERE subject_ref = 'ITEST-x'")
}
