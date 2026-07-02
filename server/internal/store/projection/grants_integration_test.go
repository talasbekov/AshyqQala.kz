//go:build integration

// Страж границы ПРОЕКЦИОННЫЕ ⊥ КУРАТОРСКИЕ (Story 2.6, AC1 + миграция 0015; скорректирован code-review).
// ВЕРНАЯ табличная граница: app_curator (Directus) НЕ пишет ПРОЕКЦИОННЫЕ таблицы (запрет 42501). app_importer
// пишет ОБА вида (проекции И org_name_aliases — он сам материализует AUTO-псевдонимы через orgnorm.Apply),
// поэтому табличного запрета importer на курат. таблицу НЕТ. Выживание КУРИРУЕМЫХ строк (manual/conflict)
// держит SQL-гейт `WHERE resolve_status='auto'` (row-level) — это проверяет S-0 тест, НЕ грант.
// Проверка через SET LOCAL ROLE в транзакции + ROLLBACK (без мутаций). Требует подключения superuser/члена
// роли (в CI — postgis `ashyqqala`-superuser; реальная разводка ролей — ops/Story 2.2).
// Гоняется: `go test -tags=integration ./internal/store/...` с DATABASE_URL (миграции 0001-0015).
package projection_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// underRole выполняет sql под ролью role в транзакции и ОТКАТЫВАЕТ её (без мутаций). Возвращает ошибку Exec.
func underRole(t *testing.T, pool interface {
	Begin(context.Context) (pgx.Tx, error)
}, role, sql string) error {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	// SET LOCAL ROLE требует, чтобы подключающийся был superuser ИЛИ членом роли (миграция 0015 НЕ выдаёт
	// membership — реальная разводка ops/Story 2.2). В CI/локально коннект — superuser, поэтому работает.
	if _, err := tx.Exec(ctx, "SET LOCAL ROLE "+role); err != nil {
		t.Fatalf("SET LOCAL ROLE %s (нужен superuser/член роли; создана ли миграция 0015?): %v", role, err)
	}
	_, execErr := tx.Exec(ctx, sql)
	return execErr
}

func isPermissionDenied(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "42501" // insufficient_privilege
}

func TestRoleGrants_ProjectionCurationBoundary(t *testing.T) {
	pool := benchmarkPool(t) // helper из price_benchmarks_integration_test.go (DATABASE_URL + skip)
	defer pool.Close()

	// ГРАНИЦА: curator (Directus) НЕ пишет ПРОЕКЦИОННУЮ таблицу (organizations) — запрет 42501.
	if err := underRole(t, pool, "app_curator",
		"INSERT INTO organizations (bin) VALUES ('999999000001')"); !isPermissionDenied(err) {
		t.Fatalf("app_curator записал в проекционную organizations (ожидался 42501): err=%v", err)
	}

	// Positive control: importer ПИШЕТ проекцию (organizations) — норма (запрет не «всё подряд»). Откат в underRole.
	if err := underRole(t, pool, "app_importer",
		"INSERT INTO organizations (bin) VALUES ('999999000002')"); err != nil {
		t.Fatalf("app_importer НЕ смог писать проекцию organizations (грант слишком строг?): %v", err)
	}

	// importer ПИШЕТ org_name_aliases — НОРМА (он материализует AUTO-псевдонимы). Выживание курируемых строк
	// (manual/conflict) при ре-импорте обеспечивает SQL-гейт `WHERE resolve_status='auto'` (см. S-0 тест), не грант.
	if err := underRole(t, pool, "app_importer",
		"INSERT INTO org_name_aliases (raw_name, source, resolve_status) VALUES ('страж-importer-auto','contract','auto')"); err != nil {
		t.Fatalf("app_importer НЕ смог писать org_name_aliases (нужен для AUTO-псевдонимов): %v", err)
	}

	// curator ПИШЕТ org_name_aliases (manual-разрешения) — норма.
	if err := underRole(t, pool, "app_curator",
		"INSERT INTO org_name_aliases (raw_name, source, resolve_status) VALUES ('страж-curator','contract','manual')"); err != nil {
		t.Fatalf("app_curator НЕ смог писать кураторскую org_name_aliases: %v", err)
	}
}

// TestRoleGrants_GeoCurationBoundary — Story 3.2: гео-сторона границы AR-4 (0020). geo_objects/districts —
// КУРАТОРСКИЕ: curator (Directus/геокодер) пишет; importer ТОЛЬКО читает (запрет 42501 на запись). Гранты
// сейчас инертны в рантайме (app коннектится owner-ролью, 0015) — граница ДОКАЗЫВАЕТСЯ этим стражем;
// реальная role-DSN разводка (LOGIN/membership) — ops/Story 2.2.
func TestRoleGrants_GeoCurationBoundary(t *testing.T) {
	pool := benchmarkPool(t)
	defer pool.Close()

	// importer НЕ пишет кураторскую geo_objects (unmatched без geom — CHECK доволен, падать должен ГРАНТ).
	if err := underRole(t, pool, "app_importer",
		"INSERT INTO geo_objects (geocode_status) VALUES ('unmatched')"); !isPermissionDenied(err) {
		t.Fatalf("app_importer записал в кураторскую geo_objects (ожидался 42501): err=%v", err)
	}
	// curator ПИШЕТ geo_objects — путь Directus 3.2 (и batch-геокодера).
	if err := underRole(t, pool, "app_curator",
		"INSERT INTO geo_objects (geocode_status) VALUES ('unmatched')"); err != nil {
		t.Fatalf("app_curator НЕ смог писать кураторскую geo_objects (Directus 3.2): %v", err)
	}
	// importer НЕ пишет кураторскую districts.
	if err := underRole(t, pool, "app_importer",
		"INSERT INTO districts (name_ru, name_kk) VALUES ('страж-importer', 'страж-importer')"); !isPermissionDenied(err) {
		t.Fatalf("app_importer записал в кураторскую districts (ожидался 42501): err=%v", err)
	}
	// curator ПИШЕТ districts (справочник районов курируем).
	if err := underRole(t, pool, "app_curator",
		"INSERT INTO districts (name_ru, name_kk) VALUES ('страж-curator', 'страж-curator')"); err != nil {
		t.Fatalf("app_curator НЕ смог писать кураторскую districts: %v", err)
	}
}
