//go:build integration

// Интеграционный тест атомарности применения нормализации (Story 2.4, AC3 / закрытие долга
// deferred-work.md:108,229: orgnorm.Apply нетранзакционен). Доказывает: при падении на UPSERT псевдонима
// организации, записанные раньше в том же Apply, ОТКАТЫВАЮТСЯ (не остаётся орг без псевдонимов).
// Гоняется: `go test -tags=integration ./internal/ingest/...` с DATABASE_URL (миграции 0001-0014).
package orgnorm_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/ingest/decode"
	"ashyqqala/server/internal/ingest/orgnorm"
	"ashyqqala/server/internal/normalize"
	"ashyqqala/server/internal/store/projection"
)

// TestApply_AtomicOnAliasFailure — AC3: Apply атомарен. Псевдоним с НЕВАЛИДНЫМ resolve_status нарушает CHECK
// миграции 0013 → UPSERT падает; организация, записанная раньше в том же Apply, не должна закоммититься
// (откат всей транзакции). КОНТРОЛЬ: без транзакции (прежний код) орг бы осталась — тест бы покраснел.
func TestApply_AtomicOnAliasFailure(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL не задан — пропуск интеграционного теста")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("подключение к БД: %v", err)
	}
	defer pool.Close()

	const bin = "999999999999" // уникальный БИН для теста атомарности (не пересекается с фикстурами/seed)
	// Пречистка (перезапускаемость): удалить возможный остаток прошлого прогона.
	if _, err := pool.Exec(ctx, "DELETE FROM org_name_aliases WHERE raw_name = $1", "ТОО Тест-Атомарность"); err != nil {
		t.Fatalf("пречистка псевдонимов: %v", err)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM organizations WHERE bin = $1", bin); err != nil {
		t.Fatalf("пречистка организаций: %v", err)
	}

	plan := orgnorm.Plan{
		Orgs: []decode.Organization{{BIN: bin, NameRu: "ТОО Тест-Атомарность", IsSupplier: true}},
		Aliases: []orgnorm.AliasDecision{
			// resolve_status = "bogus" нарушает CHECK 0013 → UpsertAlias упадёт ПОСЛЕ UpsertOrganization.
			{RawName: "ТОО Тест-Атомарность", Source: "contract", BIN: "", Status: normalize.Status("bogus")},
		},
	}

	err = orgnorm.Apply(ctx, pool, plan)
	if err == nil {
		t.Fatal("ожидалась ошибка UPSERT псевдонима (CHECK resolve_status), got nil")
	}

	// Организация НЕ должна закоммититься: весь Apply откатился (атомарность). Без транзакции орг осталась бы.
	orgStore := projection.NewOrgStore(pool)
	if _, err := orgStore.GetOrganizationByBIN(ctx, bin); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("организация %s присутствует после сбоя Apply — НЕ атомарно (ожидался откат/ErrNoRows), err=%v", bin, err)
	}
}
