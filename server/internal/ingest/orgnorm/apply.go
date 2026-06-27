package orgnorm

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"ashyqqala/server/internal/ingest/decode"
	"ashyqqala/server/internal/normalize"
	"ashyqqala/server/internal/store/curation"
	"ashyqqala/server/internal/store/gen"
	"ashyqqala/server/internal/store/projection"
)

// Apply применяет план к БД: UPSERT организаций (проекция), затем UPSERT псевдонимов (кураторская таблица;
// organization_id ищется по каноническому БИН). Авто-строки пере-выводятся при ре-импорте; ручные
// разрешения оператора (manual/conflict) переживают (UpsertAlias гейтит по resolve_status='auto').
func Apply(ctx context.Context, plan Plan, orgStore *projection.OrgStore, aliasStore *curation.AliasStore) error {
	for _, o := range plan.Orgs {
		if err := orgStore.UpsertOrganization(ctx, toOrgParams(o)); err != nil {
			return err
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
				return err
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
			return err
		}
	}
	return nil
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
