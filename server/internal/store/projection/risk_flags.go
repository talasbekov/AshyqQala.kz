package projection

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"ashyqqala/server/internal/clock"
	"ashyqqala/server/internal/flags"
	"ashyqqala/server/internal/registry"
	"ashyqqala/server/internal/store/gen"
)

// Значения risk_flags.flag_type (разделяют флаги 4.2–4.5; одна таблица/store).
const (
	SingleParticipantFlagType = "single_participant" // FR-19 (Story 4.2)
	PricePerKMFlagType        = "price_per_km"       // FR-20 (Story 4.3)
	MonopolyFlagType          = "monopoly"           // FR-21 (Story 4.4, contractor-субъект)
	RNUFlagType               = "rnu"                // FR-22 (Story 4.5, contractor-субъект, дата-зависимый)
)

// RiskFlagStore — доступ к ПРОИЗВОДНОЙ таблице risk_flags. Держит загруженные methodology_params (источник
// порогов + каноническая methodology_version: версия в данных == params, AC5б). Пересчёт идемпотентен и
// атомарен (весь батч raise/clear в одной транзакции).
type RiskFlagStore struct {
	pool   *pgxpool.Pool
	params flags.Params
}

// NewRiskFlagStore — конструктор. params — загруженные methodology_params.
func NewRiskFlagStore(pool *pgxpool.Pool, params flags.Params) *RiskFlagStore {
	return &RiskFlagStore{pool: pool, params: params}
}

// flagOutcome — исход оценки флага для одного субъекта (контракт ИЛИ подрядчик; для идемпотентной публикации).
// subjectID — contract_id (contract-флаги 4.2/4.3) ИЛИ organization_id (contractor-флаги 4.4) по контексту вызова.
type flagOutcome struct {
	subjectID int64
	raised    bool
	evidence  []byte // непусто только при raised
}

// applyFlagOutcomes ИДЕМПОТЕНТНО публикует исходы одного flag_type в ОДНОЙ транзакции (all-or-nothing):
// raised → raise(...) с evidence (повтор не плодит дубли — UPSERT); иначе → clear(...) активного (is_active=false
// + cleared_at, история строки сохраняется). Сбой посреди батча → rollback (нет полупересчёта). Субъект-агностичен:
// конкретные raise/clear (contract|contractor) передаются замыканиями. Пустой батч — no-op (и DB-free путь для
// тестов порядка с nil-пулом). [Story 4.4: обобщение applyContractFlags 4.2/4.3 на subject-kind, без дублей tx.]
func (s *RiskFlagStore) applyFlagOutcomes(
	ctx context.Context, flagType string, outcomes []flagOutcome,
	raise func(q *gen.Queries, o flagOutcome) error,
	clear func(q *gen.Queries, o flagOutcome) error,
) error {
	if len(outcomes) == 0 {
		return nil // пустой пересчёт — no-op (и DB-free путь для тестов порядка с nil-пулом)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("risk_flags: begin пересчёта (%s): %w", flagType, err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback после commit — штатный no-op
	q := gen.New(tx)
	for _, o := range outcomes {
		if o.raised {
			if err := raise(q, o); err != nil {
				return fmt.Errorf("risk_flags: raise %s (subject %d): %w", flagType, o.subjectID, err)
			}
			continue
		}
		if err := clear(q, o); err != nil {
			return fmt.Errorf("risk_flags: clear %s (subject %d): %w", flagType, o.subjectID, err)
		}
	}
	return tx.Commit(ctx)
}

// applyContractFlags — публикация contract-флагов (subject_type='contract', contract_id). Запросы 4.2/4.3.
func (s *RiskFlagStore) applyContractFlags(ctx context.Context, flagType string, outcomes []flagOutcome) error {
	return s.applyFlagOutcomes(ctx, flagType, outcomes,
		func(q *gen.Queries, o flagOutcome) error {
			return q.RaiseContractFlag(ctx, gen.RaiseContractFlagParams{
				FlagType:           flagType,
				ContractID:         pgtype.Int8{Int64: o.subjectID, Valid: true},
				Evidence:           o.evidence,
				MethodologyVersion: s.params.MethodologyVersion,
			})
		},
		func(q *gen.Queries, o flagOutcome) error {
			return q.ClearContractFlag(ctx, gen.ClearContractFlagParams{
				FlagType: flagType, ContractID: pgtype.Int8{Int64: o.subjectID, Valid: true},
			})
		},
	)
}

// applyContractorFlags — публикация contractor-флагов (subject_type='contractor', organization_id). Запросы 4.4.
func (s *RiskFlagStore) applyContractorFlags(ctx context.Context, flagType string, outcomes []flagOutcome) error {
	return s.applyFlagOutcomes(ctx, flagType, outcomes,
		func(q *gen.Queries, o flagOutcome) error {
			return q.RaiseContractorFlag(ctx, gen.RaiseContractorFlagParams{
				FlagType:           flagType,
				OrganizationID:     pgtype.Int8{Int64: o.subjectID, Valid: true},
				Evidence:           o.evidence,
				MethodologyVersion: s.params.MethodologyVersion,
			})
		},
		func(q *gen.Queries, o flagOutcome) error {
			return q.ClearContractorFlag(ctx, gen.ClearContractorFlagParams{
				FlagType: flagType, OrganizationID: pgtype.Int8{Int64: o.subjectID, Valid: true},
			})
		},
	)
}

// getByType / countActiveByType — общее чтение по flag_type+contract (флаги 4.2/4.3 переиспользуют).
func (s *RiskFlagStore) getByType(ctx context.Context, flagType string, contractID int64) (gen.RiskFlag, bool, error) {
	row, err := gen.New(s.pool).GetContractFlag(ctx, gen.GetContractFlagParams{
		FlagType:   flagType,
		ContractID: pgtype.Int8{Int64: contractID, Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.RiskFlag{}, false, nil
	}
	if err != nil {
		return gen.RiskFlag{}, false, fmt.Errorf("risk_flags get %s (contract %d): %w", flagType, contractID, err)
	}
	return row, true, nil
}

func (s *RiskFlagStore) countActiveByType(ctx context.Context, flagType string) (int64, error) {
	return gen.New(s.pool).CountActiveContractFlags(ctx, flagType)
}

// getByOrgType — чтение contractor-флага по flag_type+organization_id (флаг 4.4).
func (s *RiskFlagStore) getByOrgType(ctx context.Context, flagType string, organizationID int64) (gen.RiskFlag, bool, error) {
	row, err := gen.New(s.pool).GetContractorFlag(ctx, gen.GetContractorFlagParams{
		FlagType:       flagType,
		OrganizationID: pgtype.Int8{Int64: organizationID, Valid: true},
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.RiskFlag{}, false, nil
	}
	if err != nil {
		return gen.RiskFlag{}, false, fmt.Errorf("risk_flags get %s (org %d): %w", flagType, organizationID, err)
	}
	return row, true, nil
}

// ---- FR-19: единственный участник (Story 4.2) ----

// SingleParticipantInput — вход пересчёта флага «единственный участник». На 4.2 синтетический (golden);
// ЖИВОЙ источник (проекции announcements/participants из /v2/trd-buy) — ДЕСКОУП на Epic 2.
type SingleParticipantInput struct {
	ContractID        int64
	ParticipantCount  *int
	ProcurementMethod string
}

// RecomputeSingleParticipant ИДЕМПОТЕНТНО пересчитывает флаг FR-19 (чистая flags.SingleParticipant) и
// публикует в risk_flags. Детерминизм: тот же вход + та же methodology_version → тот же результат.
func (s *RiskFlagStore) RecomputeSingleParticipant(ctx context.Context, inputs []SingleParticipantInput) error {
	outcomes := make([]flagOutcome, 0, len(inputs))
	for _, in := range inputs {
		state, ev := flags.SingleParticipant(flags.Inputs{
			ParticipantCount:  in.ParticipantCount,
			ProcurementMethod: in.ProcurementMethod,
		}, s.params)
		o := flagOutcome{subjectID: in.ContractID, raised: state == registry.FlagRaised}
		if o.raised {
			evJSON, err := json.Marshal(ev)
			if err != nil {
				return fmt.Errorf("risk_flags: marshal single_participant evidence (contract %d): %w", in.ContractID, err)
			}
			o.evidence = evJSON
		}
		outcomes = append(outcomes, o)
	}
	return s.applyContractFlags(ctx, SingleParticipantFlagType, outcomes)
}

// Get — single-participant флаг по контракту (активный или снятый): строка + признак наличия.
func (s *RiskFlagStore) Get(ctx context.Context, contractID int64) (gen.RiskFlag, bool, error) {
	return s.getByType(ctx, SingleParticipantFlagType, contractID)
}

// CountActive — число активных single-participant флагов.
func (s *RiskFlagStore) CountActive(ctx context.Context) (int64, error) {
	return s.countActiveByType(ctx, SingleParticipantFlagType)
}

// SingleParticipantRecalculator адаптирует пересчёт FR-19 к recalc.FlagRecalculator (флаги ПОСЛЕ benchmark,
// Story 4.1). Provider на 4.2 синтетический; живой /v2/trd-buy — Epic 2.
type SingleParticipantRecalculator struct {
	Store    *RiskFlagStore
	Provider func(context.Context) ([]SingleParticipantInput, error)
}

// RecalcFlags — пересчёт single-participant флагов (после benchmark). Provider/Store не заданы → честная ошибка.
func (r SingleParticipantRecalculator) RecalcFlags(ctx context.Context) error {
	if r.Store == nil {
		return fmt.Errorf("risk_flags: Store не задан")
	}
	if r.Provider == nil {
		return fmt.Errorf("risk_flags: provider участников не задан (живой источник /v2/trd-buy — Epic 2)")
	}
	inputs, err := r.Provider(ctx)
	if err != nil {
		return fmt.Errorf("risk_flags: provider участников: %w", err)
	}
	return r.Store.RecomputeSingleParticipant(ctx, inputs)
}

// ---- FR-20: аномальная цена за км (Story 4.3) ----

// PricePerKMInput — вход пересчёта флага цены/км. На 4.3 цена/км синтетическая (golden); медиана читается из
// кэша price_benchmarks по ComparabilityKey. ЖИВЫЕ суммы (/v2/contract, Epic 2) + length_km (Epic 3) — ДЕСКОУП.
type PricePerKMInput struct {
	ContractID       int64
	PricePerKM       *int64 // nil = не вычислима (нет длины) → флаг не строится
	ComparabilityKey string
}

// RecomputePricePerKM ИДЕМПОТЕНТНО пересчитывает флаг FR-20: медиана группы читается из кэша price_benchmarks
// (bench, пересчитанного ПЕРЕД флагами — порядок recalc.Run), затем чистая flags.PricePerKM. Без медианы/цены
// (нет length_km) или sample<min → флаг не строится (insufficient, честно).
func (s *RiskFlagStore) RecomputePricePerKM(ctx context.Context, bench *BenchmarkStore, inputs []PricePerKMInput) error {
	if bench == nil {
		return fmt.Errorf("risk_flags: BenchmarkStore (источник медиан) не задан")
	}
	outcomes := make([]flagOutcome, 0, len(inputs))
	for _, in := range inputs {
		median, sample, err := bench.MedianForKey(ctx, in.ComparabilityKey)
		if err != nil {
			return fmt.Errorf("risk_flags: медиана для %q (contract %d): %w", in.ComparabilityKey, in.ContractID, err)
		}
		state, ev := flags.PricePerKM(flags.Inputs{
			PricePerKM:       in.PricePerKM,
			GroupMedian:      median,
			GroupSampleSize:  sample,
			ComparabilityKey: in.ComparabilityKey,
		}, s.params)
		o := flagOutcome{subjectID: in.ContractID, raised: state == registry.FlagRaised}
		if o.raised {
			evJSON, err := json.Marshal(ev)
			if err != nil {
				return fmt.Errorf("risk_flags: marshal price_per_km evidence (contract %d): %w", in.ContractID, err)
			}
			o.evidence = evJSON
		}
		outcomes = append(outcomes, o)
	}
	return s.applyContractFlags(ctx, PricePerKMFlagType, outcomes)
}

// GetPricePerKM — price_per_km флаг по контракту.
func (s *RiskFlagStore) GetPricePerKM(ctx context.Context, contractID int64) (gen.RiskFlag, bool, error) {
	return s.getByType(ctx, PricePerKMFlagType, contractID)
}

// CountActivePricePerKM — число активных price_per_km флагов.
func (s *RiskFlagStore) CountActivePricePerKM(ctx context.Context) (int64, error) {
	return s.countActiveByType(ctx, PricePerKMFlagType)
}

// PricePerKMRecalculator адаптирует пересчёт FR-20 к recalc.FlagRecalculator (после benchmark swap — медиана
// свежая). Provider цен/км на 4.3 синтетический; живые суммы (Epic 2) + length_km (Epic 3) — дескоуп.
type PricePerKMRecalculator struct {
	Store    *RiskFlagStore
	Bench    *BenchmarkStore
	Provider func(context.Context) ([]PricePerKMInput, error)
}

// RecalcFlags — пересчёт price_per_km флагов ПОСЛЕ benchmark (медиана из свежего снапшота).
func (r PricePerKMRecalculator) RecalcFlags(ctx context.Context) error {
	if r.Store == nil || r.Bench == nil {
		return fmt.Errorf("risk_flags: Store/Bench не заданы")
	}
	if r.Provider == nil {
		return fmt.Errorf("risk_flags: provider цен/км не задан (живой источник суммы/длина — Epic 2/3)")
	}
	inputs, err := r.Provider(ctx)
	if err != nil {
		return fmt.Errorf("risk_flags: provider цен/км: %w", err)
	}
	return r.Store.RecomputePricePerKM(ctx, r.Bench, inputs)
}

// ---- FR-21: монополия в регионе (Story 4.4, contractor-субъект) ----

// MonopolyInput — вход пересчёта флага монополии: групповой агрегат на ПОДРЯДЧИКА (organization_id). На 4.4
// суммы синтетические (golden); живые SUM(amount) по БИН/группе (/v2/contract, Epic 2) + нормализация
// (org_name_aliases.resolve_status, Epic 2) — ДЕСКОУП. SupplierBINResolved: auto → true; manual/conflict → false.
type MonopolyInput struct {
	OrganizationID      int64
	SupplierBIN         string
	SupplierSum         *int64 // nil = нет данных → флаг не строится
	GroupTotalSum       *int64 // nil/≤0 = знаменатель недостоверен → флаг не строится
	GroupContracts      int
	SupplierBINResolved bool
	ComparabilityKey    string // direction×kato (benchmark.Group.Key)
}

// RecomputeMonopoly ИДЕМПОТЕНТНО пересчитывает флаг FR-21 (чистая flags.Monopoly) и публикует в risk_flags по
// CONTRACTOR-субъекту (organization_id). Малая выборка / нулевой знаменатель / неразрешённый БИН → флаг не
// строится (insufficient/clear). Детерминизм: тот же вход + та же methodology_version → тот же результат.
func (s *RiskFlagStore) RecomputeMonopoly(ctx context.Context, inputs []MonopolyInput) error {
	outcomes := make([]flagOutcome, 0, len(inputs))
	for _, in := range inputs {
		state, ev := flags.Monopoly(flags.Inputs{
			SupplierSum:         in.SupplierSum,
			GroupTotalSum:       in.GroupTotalSum,
			GroupContracts:      in.GroupContracts,
			SupplierBINResolved: in.SupplierBINResolved,
			SupplierBIN:         in.SupplierBIN,
			ComparabilityKey:    in.ComparabilityKey,
		}, s.params)
		o := flagOutcome{subjectID: in.OrganizationID, raised: state == registry.FlagRaised}
		if o.raised {
			evJSON, err := json.Marshal(ev)
			if err != nil {
				return fmt.Errorf("risk_flags: marshal monopoly evidence (org %d): %w", in.OrganizationID, err)
			}
			o.evidence = evJSON
		}
		outcomes = append(outcomes, o)
	}
	return s.applyContractorFlags(ctx, MonopolyFlagType, outcomes)
}

// GetMonopoly — monopoly флаг по подрядчику (organization_id).
func (s *RiskFlagStore) GetMonopoly(ctx context.Context, organizationID int64) (gen.RiskFlag, bool, error) {
	return s.getByOrgType(ctx, MonopolyFlagType, organizationID)
}

// CountActiveMonopoly — число активных monopoly флагов (flag_type-scoped; все строки monopoly — contractor).
func (s *RiskFlagStore) CountActiveMonopoly(ctx context.Context) (int64, error) {
	return s.countActiveByType(ctx, MonopolyFlagType)
}

// MonopolyRecalculator адаптирует пересчёт FR-21 к recalc.FlagRecalculator (флаги ПОСЛЕ benchmark, Story 4.1).
// БЕЗ Bench: монополия — доля по сумме, медиана не нужна (в отличие от PricePerKMRecalculator). Provider
// агрегатов на 4.4 синтетический; живые суммы (/v2/contract) + нормализация (org_name_aliases) — Epic 2.
type MonopolyRecalculator struct {
	Store    *RiskFlagStore
	Provider func(context.Context) ([]MonopolyInput, error)
}

// RecalcFlags — пересчёт monopoly флагов (после benchmark). Provider/Store не заданы → честная ошибка.
func (r MonopolyRecalculator) RecalcFlags(ctx context.Context) error {
	if r.Store == nil {
		return fmt.Errorf("risk_flags: Store не задан")
	}
	if r.Provider == nil {
		return fmt.Errorf("risk_flags: provider агрегатов монополии не задан (живые суммы/нормализация — Epic 2)")
	}
	inputs, err := r.Provider(ctx)
	if err != nil {
		return fmt.Errorf("risk_flags: provider монополии: %w", err)
	}
	return r.Store.RecomputeMonopoly(ctx, inputs)
}

// ---- FR-22: наличие в РНУ (Story 4.5, contractor-субъект, ДАТА-ЗАВИСИМЫЙ) ----

// dateToUnix — pgtype.Date → *int64 unix (nil если NULL/невалидно). Граница дат — полночь UTC (как seed/импорт);
// чистое ядро сравнивает unix с now из clock.
func dateToUnix(d pgtype.Date) *int64 {
	if !d.Valid {
		return nil
	}
	u := d.Time.Unix()
	return &u
}

// RNUInput — вход пересчёта флага РНУ: запись реестра на Подрядчика (organization_id). На 4.5 из `rnu_entries`
// (seed, через `ListRNUInputs`); живой /v2/rnu-импорт → Epic 2. Даты — *int64 unix (nil end = открытая запись).
type RNUInput struct {
	OrganizationID int64
	StartUnix      *int64
	EndUnix        *int64
	GoszakupID     string
	SourceURL      string
	ReasonRef      string
}

// RecomputeRNU ИДЕМПОТЕНТНО пересчитывает флаг FR-22 (дата-зависимый): АГРЕГИРУЕТ записи РНУ ПО ПОДРЯДЧИКУ
// (organization_id) — орг «в РНУ» ⟺ есть ХОТЯ БЫ ОДНА активная запись (чистая flags.RNU с `now` из clock).
// Один исход на org (НЕ на строку: иначе активная запись затиралась бы истёкшей в той же транзакции при
// нескольких записях на org — Epic 2). evidence — последней активной записи в детерминированном порядке
// ListRNUEntries (organization_id, start_date, id → запись с наибольшим start_date). idempotent raise/clear по
// organization_id (`flag_type='rnu'`, contractor-путь 4.4). Истёкшие/будущие/нет-активных → not_raised (снятие).
func (s *RiskFlagStore) RecomputeRNU(ctx context.Context, now clock.Clock, inputs []RNUInput) error {
	if now == nil {
		return fmt.Errorf("risk_flags: Clock не задан (флаг РНУ дата-зависим — активность/авто-снятие по end_date)")
	}
	type orgAgg struct {
		raised   bool
		evidence []byte
	}
	order := make([]int64, 0, len(inputs)) // порядок первого появления org → детерминизм исходов
	byOrg := make(map[int64]*orgAgg, len(inputs))
	for _, in := range inputs {
		agg, ok := byOrg[in.OrganizationID]
		if !ok {
			agg = &orgAgg{}
			byOrg[in.OrganizationID] = agg
			order = append(order, in.OrganizationID)
		}
		state, ev := flags.RNU(flags.Inputs{
			Now:           now,
			RNUStartUnix:  in.StartUnix,
			RNUEndUnix:    in.EndUnix,
			RNUGoszakupID: in.GoszakupID,
			RNUSourceURL:  in.SourceURL,
			RNUReasonRef:  in.ReasonRef,
		}, s.params)
		if state == registry.FlagRaised {
			evJSON, err := json.Marshal(ev)
			if err != nil {
				return fmt.Errorf("risk_flags: marshal rnu evidence (org %d): %w", in.OrganizationID, err)
			}
			agg.raised = true     // ХОТЯ БЫ одна активная запись → орг в РНУ
			agg.evidence = evJSON // последняя активная (наибольший start_date по ORDER BY) — детерминировано
		}
	}
	outcomes := make([]flagOutcome, 0, len(order))
	for _, org := range order {
		agg := byOrg[org]
		outcomes = append(outcomes, flagOutcome{subjectID: org, raised: agg.raised, evidence: agg.evidence})
	}
	return s.applyContractorFlags(ctx, RNUFlagType, outcomes)
}

// ListRNUInputs читает `rnu_entries` → []RNUInput (Provider для RNURecalculator; источник наполнения — живой
// /v2/rnu-импорт Epic 2, на синтетике seed). Даты DATE → *int64 unix (ядро time-free).
func (s *RiskFlagStore) ListRNUInputs(ctx context.Context) ([]RNUInput, error) {
	rows, err := gen.New(s.pool).ListRNUEntries(ctx)
	if err != nil {
		return nil, fmt.Errorf("risk_flags: чтение rnu_entries: %w", err)
	}
	out := make([]RNUInput, 0, len(rows))
	for _, r := range rows {
		out = append(out, RNUInput{
			OrganizationID: r.OrganizationID,
			StartUnix:      dateToUnix(r.StartDate),
			EndUnix:        dateToUnix(r.EndDate),
			GoszakupID:     r.GoszakupRnuID.String,
			SourceURL:      r.SourceUrl.String,
			ReasonRef:      r.ReasonRef.String,
		})
	}
	return out, nil
}

// GetRNU — rnu флаг по подрядчику (organization_id).
func (s *RiskFlagStore) GetRNU(ctx context.Context, organizationID int64) (gen.RiskFlag, bool, error) {
	return s.getByOrgType(ctx, RNUFlagType, organizationID)
}

// CountActiveRNU — число активных rnu флагов.
func (s *RiskFlagStore) CountActiveRNU(ctx context.Context) (int64, error) {
	return s.countActiveByType(ctx, RNUFlagType)
}

// RNURecalculator адаптирует пересчёт FR-22 к recalc.FlagRecalculator (флаги после benchmark; РНУ benchmark не
// использует). Держит `Clock` (ПЕРВЫЙ дата-зависимый флаг) — «сейчас» для активности/авто-снятия по end_date.
// Provider — источник записей (`Store.ListRNUInputs` из `rnu_entries`; живой /v2/rnu → Epic 2). БЕЗ Bench.
type RNURecalculator struct {
	Store    *RiskFlagStore
	Clock    clock.Clock
	Provider func(context.Context) ([]RNUInput, error)
}

// RecalcFlags — пересчёт rnu флагов (после benchmark). Store/Clock/Provider не заданы → честная ошибка.
func (r RNURecalculator) RecalcFlags(ctx context.Context) error {
	if r.Store == nil {
		return fmt.Errorf("risk_flags: Store не задан")
	}
	if r.Clock == nil {
		return fmt.Errorf("risk_flags: Clock не задан (флаг РНУ дата-зависим — активность/авто-снятие по end_date)")
	}
	if r.Provider == nil {
		return fmt.Errorf("risk_flags: provider записей РНУ не задан (Store.ListRNUInputs; живой /v2/rnu — Epic 2)")
	}
	inputs, err := r.Provider(ctx)
	if err != nil {
		return fmt.Errorf("risk_flags: provider РНУ: %w", err)
	}
	return r.Store.RecomputeRNU(ctx, r.Clock, inputs)
}
