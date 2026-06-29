//go:build integration

// Интеграционный тест нормализации (Story 2.3): file-Source → decode → PlanNormalization → Apply → БД.
// Проверяет: auto/manual/conflict материализуются в org_name_aliases; проекция organizations наполнена;
// ручное разрешение оператора переживает ре-импорт (AR-4/AR-10). Гоняется: `go test -tags=integration
// ./internal/ingest/...` с DATABASE_URL (миграции 0001-0013). CI без тега этот файл не компилирует.
package orgnorm_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/goszakup"
	"ashyqqala/server/internal/ingest/decode"
	"ashyqqala/server/internal/ingest/orgnorm"
	"ashyqqala/server/internal/lexicon"
	"ashyqqala/server/internal/store/curation"
	"ashyqqala/server/internal/store/gen"
	"ashyqqala/server/internal/store/projection"
)

// Golden schema_hash фикстур testdata (= хеши из decode/organizations_test.go; те же наборы ключей).
const (
	contractHash = "9e6463f619991ba9d7944a3968006dcab3d2adb9bd05f679871112efcec0737f"
	trdBuyHash   = "554289cda264df36bf44036ef640ef060be561e33d9864aa50e1cd13a62e78f8"
)

func buildAppearances(t *testing.T) []orgnorm.Appearance {
	t.Helper()
	src := goszakup.NewFileSource("testdata")
	custOrgs, err := decode.DecodeOrgsFromContracts(src, contractHash, 0)
	if err != nil {
		t.Fatalf("decode contracts: %v", err)
	}
	trdOrgs, err := decode.DecodeOrgsFromTrdBuy(src, trdBuyHash, 0)
	if err != nil {
		t.Fatalf("decode trd-buy: %v", err)
	}
	var apps []orgnorm.Appearance
	for _, o := range custOrgs {
		apps = append(apps, orgnorm.Appearance{Org: o, Source: "contract"})
	}
	for _, o := range trdOrgs {
		apps = append(apps, orgnorm.Appearance{Org: o, Source: "trd-buy"})
	}
	return apps
}

func containsRaw(rows []gen.OrgNameAlias, raw string) bool {
	for _, r := range rows {
		if r.RawName == raw {
			return true
		}
	}
	return false
}

func rawNames(rows []gen.OrgNameAlias) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.RawName)
	}
	return out
}

func TestOrgNorm_Integration(t *testing.T) {
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

	// Чистый старт (тест владеет этими таблицами): детерминированные счётчики.
	if _, err := pool.Exec(ctx, "TRUNCATE org_name_aliases, organizations RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("TRUNCATE (применены ли миграции 0012/0013?): %v", err)
	}

	lex, err := lexicon.Load("../../../../registry")
	if err != nil {
		t.Fatalf("lexicon.Load: %v", err)
	}
	orgStore := projection.NewOrgStore(pool)
	aliasStore := curation.NewAliasStore(pool)

	// --- Импорт #1 ---
	existing, err := orgStore.ListOrganizations(ctx)
	if err != nil {
		t.Fatalf("ListOrganizations: %v", err)
	}
	plan := orgnorm.PlanNormalization(buildAppearances(t), orgnorm.BuildCandidates(existing, lex), lex)
	if err := orgnorm.Apply(ctx, pool, plan); err != nil {
		t.Fatalf("Apply #1: %v", err)
	}

	// Проекция: организация с БИН заведена; роль подрядчика выставлена.
	org222, err := orgStore.GetOrganizationByBIN(ctx, "222222222222")
	if err != nil {
		t.Fatalf("GetOrganizationByBIN(222): %v", err)
	}
	if !org222.IsSupplier {
		t.Errorf("222 должна быть is_supplier=true")
	}

	// auto: вариативное написание 222 объединено (псевдоним trd-buy ведёт к 222).
	autoAlias, err := aliasStore.GetAlias(ctx, gen.GetAliasParams{RawName: "ТОО \"Астана-Жoл\"", Source: "trd-buy"})
	if err != nil {
		t.Fatalf("GetAlias(вариант 222): %v", err)
	}
	if autoAlias.ResolveStatus != "auto" || !autoAlias.OrganizationID.Valid || autoAlias.OrganizationID.Int64 != org222.ID {
		t.Errorf("вариант 222: status=%s org_id=%+v, ожидалось auto→%d", autoAlias.ResolveStatus, autoAlias.OrganizationID, org222.ID)
	}

	// conflict: «ТОО СуВодоканал» под 333 и 444 → изолировано.
	conflicts, err := aliasStore.ListAliasesByStatus(ctx, "conflict")
	if err != nil {
		t.Fatalf("ListAliasesByStatus(conflict): %v", err)
	}
	if !containsRaw(conflicts, "ТОО СуВодоканал") {
		t.Errorf("ожидался conflict для «ТОО СуВодоканал», got %v", rawNames(conflicts))
	}
	// Инвариант 0013 + честность: conflict-псевдонимы не привязаны к организации (organization_id NULL).
	for _, c := range conflicts {
		if c.OrganizationID.Valid {
			t.Errorf("conflict «%s» должен иметь organization_id NULL, got %d", c.RawName, c.OrganizationID.Int64)
		}
	}

	// manual: имя без БИН (пропущенный БИН в источнике) → очередь оператору, organization_id NULL.
	manuals, err := aliasStore.ListAliasesByStatus(ctx, "manual")
	if err != nil {
		t.Fatalf("ListAliasesByStatus(manual): %v", err)
	}
	if !containsRaw(manuals, "ТОО Безбинная Фирма") {
		t.Fatalf("ожидался manual для «ТОО Безбинная Фирма», got %v", rawNames(manuals))
	}

	// --- Оператор разрешает manual-запись (имитация Directus): привязывает к 555 (Дала) ---
	org555, err := orgStore.GetOrganizationByBIN(ctx, "555555555555")
	if err != nil {
		t.Fatalf("GetOrganizationByBIN(555): %v", err)
	}
	if err := aliasStore.ResolveAliasManually(ctx, gen.ResolveAliasManuallyParams{
		RawName:        "ТОО Безбинная Фирма",
		Source:         "trd-buy",
		OrganizationID: pgtype.Int8{Int64: org555.ID, Valid: true},
	}); err != nil {
		t.Fatalf("ResolveAliasManually: %v", err)
	}

	// --- Импорт #2 (ре-импорт): ручное разрешение должно ПЕРЕЖИТЬ ---
	existing2, _ := orgStore.ListOrganizations(ctx)
	plan2 := orgnorm.PlanNormalization(buildAppearances(t), orgnorm.BuildCandidates(existing2, lex), lex)
	if err := orgnorm.Apply(ctx, pool, plan2); err != nil {
		t.Fatalf("Apply #2: %v", err)
	}

	resolved, err := aliasStore.GetAlias(ctx, gen.GetAliasParams{RawName: "ТОО Безбинная Фирма", Source: "trd-buy"})
	if err != nil {
		t.Fatalf("GetAlias(manual после ре-импорта): %v", err)
	}
	if !resolved.OrganizationID.Valid || resolved.OrganizationID.Int64 != org555.ID {
		t.Errorf("ручное разрешение НЕ пережило ре-импорт: org_id=%+v, ожидалось %d (UpsertAlias не должен затирать non-auto)", resolved.OrganizationID, org555.ID)
	}
	t.Logf("OK: auto/manual/conflict материализованы; ручное разрешение пережило ре-импорт (org_id=%d)", resolved.OrganizationID.Int64)
}
