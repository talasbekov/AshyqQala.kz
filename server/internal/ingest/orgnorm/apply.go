package orgnorm

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/ingest/decode"
	"ashyqqala/server/internal/normalize"
	"ashyqqala/server/internal/store/curation"
	"ashyqqala/server/internal/store/gen"
	"ashyqqala/server/internal/store/projection"
)

// Apply применяет план к БД АТОМАРНО (Story 2.4: закрытие долга нетранзакционности, deferred-work.md:108,229):
// все UPSERT организаций (проекция) и псевдонимов (кураторская таблица) идут в ОДНОЙ транзакции — сбой
// посреди батча откатывает всё (не остаётся орг без псевдонимов). organization_id для auto-резолва ищется
// по каноническому БИН в той же транзакции (read-your-writes: орг, записанная выше в этом же Apply, видна).
// Авто-строки пере-выводятся при ре-импорте; ручные разрешения оператора (manual/conflict) переживают
// (UpsertAlias гейтит по resolve_status='auto').
func Apply(ctx context.Context, pool *pgxpool.Pool, plan Plan) error {
	if pool == nil {
		return fmt.Errorf("orgnorm apply: pool не задан")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("orgnorm apply: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback после успешного commit — штатный no-op

	orgStore := projection.NewOrgStore(tx)
	aliasStore := curation.NewAliasStore(tx)

	for _, o := range plan.Orgs {
		if err := orgStore.UpsertOrganization(ctx, toOrgParams(o)); err != nil {
			return fmt.Errorf("orgnorm apply: upsert организации %s: %w", o.BIN, err)
		}
	}
	for _, a := range plan.Aliases {
		var orgID pgtype.Int8
		// organization_id проставляем ТОЛЬКО для auto-резолва. manual/conflict остаются NULL «пока БИН не
		// разрешён» (инвариант миграции 0013 + честность: спорное/неуверенное имя НЕ привязываем к орг,
		// иначе ListResolvedAliases выдал бы его как подтверждённое написание). Оператор проставит позже.
		if a.Status == normalize.StatusAuto && a.BIN != "" {
			org, err := orgStore.GetOrganizationByBIN(ctx, string(a.BIN))
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("orgnorm apply: поиск организации по БИН %s: %w", a.BIN, err)
			}
			if err == nil {
				orgID = pgtype.Int8{Int64: org.ID, Valid: true}
			}
		}
		p := gen.UpsertAliasParams{
			OrganizationID: orgID,
			RawName:        a.RawName,
			Source:         a.Source,
			ResolveStatus:  string(a.Status),
		}
		if err := aliasStore.UpsertAlias(ctx, p); err != nil {
			return fmt.Errorf("orgnorm apply: upsert псевдонима %q (%s): %w", a.RawName, a.Source, err)
		}
	}
	return tx.Commit(ctx)
}

// toOrgParams — маппинг доменной decode.Organization → sqlc-параметры (NULL для пустых: честное «нет данных»).
func toOrgParams(o decode.Organization) gen.UpsertOrganizationParams {
	p := gen.UpsertOrganizationParams{Bin: o.BIN, IsCustomer: o.IsCustomer, IsSupplier: o.IsSupplier}
	if o.NameRu != "" {
		p.NameRu = pgtype.Text{String: o.NameRu, Valid: true}
	}
	if o.NameKk != "" {
		p.NameKk = pgtype.Text{String: o.NameKk, Valid: true}
	}
	if o.RegKato != "" {
		p.RegKato = pgtype.Text{String: o.RegKato, Valid: true}
	}
	if o.SourceURL != "" {
		p.SourceUrl = pgtype.Text{String: o.SourceURL, Valid: true}
	}
	return p
}
