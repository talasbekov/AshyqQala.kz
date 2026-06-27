package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"ashyqqala/server/internal/apierr"
	"ashyqqala/server/internal/normalize"
	"ashyqqala/server/internal/registry"
	"ashyqqala/server/internal/store/gen"
)

// contractorFlagTypes — flag_type'ы карточки ПОДРЯДЧИКА (contractor-субъект): монополия (FR-21) и РНУ-флаг
// (FR-22). Контракт-субъектные (single_participant/price_per_km) — на карточке контракта (5.1), сюда не входят.
// Строки совпадают с projection.MonopolyFlagType/RNUFlagType (литералами, без импорта store-слоя). Порядок
// детерминирован (стабильность wire).
var contractorFlagTypes = []string{
	"monopoly", // FR-21, Story 4.4
	"rnu",      // FR-22, Story 4.5
}

// ContractorProfileDTO — честное состояние профиля (FR-13: «строится при неполном профиле»). В S-0:
//   - "unverified" — идентичность БИН спорна (manual/conflict-псевдоним по FK ИЛИ по совпадению имени): «профиль
//     уточняется», флаги НЕ основание (перекрывает всё);
//   - "partial"    — есть связанные контракты (показаны доступные данные), но полнота — после живого импорта;
//   - "incomplete" — связь контракт↔организация ещё не наполнена (Story 2.2) → «профиль неполный».
//
// "complete" в S-0 НЕ выдаём (нет живого импорта — не заявляем полноту; вводится в Story 2.2).
type ContractorProfileDTO struct {
	State string `json:"state"`
}

// ContractorFlagDTO — wire-форма агрегированного флага подрядчика. Та же ось flag_state, что у контракта.
type ContractorFlagDTO struct {
	FlagID             string             `json:"flag_id"`
	State              registry.FlagState `json:"state"`
	MethodologyVersion Field[string]      `json:"methodology_version"`
	DetectedAt         Field[string]      `json:"detected_at"`
	Evidence           json.RawMessage    `json:"evidence"`
}

// RNUMarkDTO — метка РНУ (FR-14). active реконструируется НА ЧТЕНИИ из дат (авто-снятие по end_date),
// а не из булева «активна» в сторе. Каждая метка несёт СВОЙ source_url (AC-3: защитная ссылка на КАЖДУЮ метку).
// «недобросовестный/жосықсыз» допустим на фронте ТОЛЬКО как цитата названия реестра (нейтральность); здесь —
// только нейтральные данные (даты/ссылка/идентификатор реестра/причина-референс).
type RNUMarkDTO struct {
	Active     bool          `json:"active"`
	StartDate  Field[string] `json:"start_date"`
	EndDate    Field[string] `json:"end_date"`
	ReasonRef  Field[string] `json:"reason_ref"`
	SourceURL  Field[string] `json:"source_url"`
	RegistryID Field[string] `json:"registry_id"`
}

// ContractorContractDTO — элемент списка контрактов подрядчика (FR-13, AC-1). Публичный goszakup_contract_id
// (ссылка на карточку контракта). Деньги строкой; nullable — честный конверт.
type ContractorContractDTO struct {
	GoszakupContractID string        `json:"goszakup_contract_id"`
	SubjectRu          Field[string] `json:"subject_ru"`
	SubjectKk          Field[string] `json:"subject_kk"`
	AmountTng          Field[string] `json:"amount_tng"`
	KatoCode           Field[string] `json:"kato_code"`
	Direction          Field[string] `json:"direction"`
}

// ContractorDTO — wire-форма карточки подрядчика. id (внутр.) не отдаём; публичный id = natural bin.
type ContractorDTO struct {
	Bin            string                  `json:"bin"`
	NameRu         Field[string]           `json:"name_ru"`
	NameKk         Field[string]           `json:"name_kk"`
	RegKato        Field[string]           `json:"reg_kato"`
	IsCustomer     bool                    `json:"is_customer"`
	IsSupplier     bool                    `json:"is_supplier"`
	Profile        ContractorProfileDTO    `json:"profile"`
	ContractCount  Field[string]           `json:"contract_count"`   // целое строкой; no_data при неполном профиле (не «0»)
	TotalAmountTng Field[string]           `json:"total_amount_tng"` // деньги строкой; no_data при неполном профиле
	Regions        []string                `json:"regions"`          // КАТО (distinct); пусто при неполном профиле
	Contracts      []ContractorContractDTO `json:"contracts"`        // FR-13 список контрактов (по supplier_org_id); пусто до наполнения 2.2
	Flags          []ContractorFlagDTO     `json:"flags"`            // monopoly/rnu — честная реконструкция
	RnuMarks       []RNUMarkDTO            `json:"rnu_marks"`        // FR-14
	SourceURL      Field[string]           `json:"source_url"`
	ImportedAt     Field[string]           `json:"imported_at"`
	UpdatedAt      Field[string]           `json:"updated_at"`
}

// resolveContractorFlags — ЧЕСТНАЯ реконструкция состояний агрегированных флагов на ЧТЕНИИ (как
// resolveContractFlags для контракта). Дополнительно: при unverified-профиле (resolve_status manual/conflict)
// флаги НЕ используются как основание — все принудительно insufficient_data («профиль уточняется», не оцениваем),
// чтобы карточка не делала выводов о подрядчике при неустойчивой идентичности (честность над домыслом, §7.4).
func resolveContractorFlags(rows []gen.RiskFlag, unverified bool) []ContractorFlagDTO {
	byType := make(map[string]gen.RiskFlag, len(rows))
	for _, r := range rows {
		byType[r.FlagType] = r
	}
	out := make([]ContractorFlagDTO, 0, len(contractorFlagTypes))
	for _, ft := range contractorFlagTypes {
		dto := ContractorFlagDTO{FlagID: ft, MethodologyVersion: noData[string](), DetectedAt: noData[string](), Evidence: nil}
		switch r, ok := byType[ft]; {
		case unverified:
			dto.State = registry.FlagInsufficientData // профиль уточняется → флаг не основание
		case !ok:
			dto.State = registry.FlagInsufficientData // нет строки → нет доказательства оценки
		case r.IsActive:
			dto.State = registry.FlagRaised
			dto.MethodologyVersion = versionField(r.MethodologyVersion)
			dto.DetectedAt = fromTimestamptz(r.DetectedAt)
			dto.Evidence = json.RawMessage(r.Evidence)
		default:
			dto.State = registry.FlagNotRaised
			dto.MethodologyVersion = versionField(r.MethodologyVersion)
			dto.DetectedAt = fromTimestamptz(r.DetectedAt)
		}
		out = append(out, dto)
	}
	return out
}

// resolveRNUMarks — метки РНУ на ЧТЕНИИ (FR-14). active = нет end_date ЛИБО end_date СТРОГО позже asOf-дня
// (авто-снятие по end_date: `end_date <= today` → снята — как RecomputeRNU `flags/rnu.go` «end > now строго»
// и migration 0008; AC-2 «активна iff end_date в будущем»). asOf инъектируется (тестируемость).
func resolveRNUMarks(entries []gen.RnuEntry, asOf time.Time) []RNUMarkDTO {
	day := asOf.UTC().Truncate(24 * time.Hour)
	out := make([]RNUMarkDTO, 0, len(entries))
	for _, e := range entries {
		active := !e.EndDate.Valid || e.EndDate.Time.UTC().Truncate(24*time.Hour).After(day)
		out = append(out, RNUMarkDTO{
			Active:     active,
			StartDate:  fromDate(e.StartDate),
			EndDate:    fromDate(e.EndDate),
			ReasonRef:  fromText(e.ReasonRef),
			SourceURL:  fromText(e.SourceUrl),
			RegistryID: fromText(e.GoszakupRnuID),
		})
	}
	return out
}

// ContractorStore — что хендлеру нужно от слоя данных (реализует *gen.Queries; мокается в тестах).
type ContractorStore interface {
	GetOrganizationByBIN(ctx context.Context, bin string) (gen.Organization, error)
	CountUnresolvedAliasesByOrg(ctx context.Context, organizationID pgtype.Int8) (int64, error)
	ListUnresolvedAliasNames(ctx context.Context) ([]string, error)
	ContractorAggregates(ctx context.Context, supplierOrgID pgtype.Int8) (gen.ContractorAggregatesRow, error)
	ListContractsBySupplierOrg(ctx context.Context, supplierOrgID pgtype.Int8) ([]gen.ListContractsBySupplierOrgRow, error)
	ListContractorFlags(ctx context.Context, organizationID pgtype.Int8) ([]gen.RiskFlag, error)
	ListRNUByOrg(ctx context.Context, organizationID int64) ([]gen.RnuEntry, error)
}

// ContractorsHandler — хендлер карточки подрядчика (Story 5.2).
type ContractorsHandler struct {
	Store   ContractorStore
	Log     *slog.Logger
	Lexicon normalize.Lexicon // для детекта «профиль уточняется» по совпадению имени (conflict/manual псевдонимы)
	Now     func() time.Time  // инъекция времени для РНУ-меток (тесты пиннят); nil → time.Now
}

// contestedByName — спорная ли идентичность организации: совпадает ли её каноническое имя (ru/kk) с
// каноническим написанием какого-либо псевдонима из очереди (manual/conflict). Закрывает дыру F1: conflict-
// псевдонимы из Story 2.3 имеют organization_id=NULL, поэтому привязка по FK их не видит — матчим по имени.
func (h ContractorsHandler) contestedByName(org gen.Organization, unresolved []string) bool {
	keys := map[string]bool{}
	if org.NameRu.Valid && org.NameRu.String != "" {
		keys[normalize.CanonicalKey(org.NameRu.String, h.Lexicon)] = true
	}
	if org.NameKk.Valid && org.NameKk.String != "" {
		keys[normalize.CanonicalKey(org.NameKk.String, h.Lexicon)] = true
	}
	for _, raw := range unresolved {
		if keys[normalize.CanonicalKey(raw, h.Lexicon)] {
			return true
		}
	}
	return false
}

func (h ContractorsHandler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

// Projection собирает ContractorDTO. (zero, pgx.ErrNoRows) если организации нет → вызывающий → 404.
func (h ContractorsHandler) Projection(ctx context.Context, bin string) (ContractorDTO, error) {
	org, err := h.Store.GetOrganizationByBIN(ctx, bin)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return ContractorDTO{}, err
	case err != nil:
		if h.Log != nil {
			h.Log.Error("get_contractor_failed", "error", err.Error(), "bin", bin)
		}
		return ContractorDTO{}, err
	}
	orgPK := pgtype.Int8{Int64: org.ID, Valid: true}

	// «профиль уточняется» — есть НЕ-auto псевдонимы у организации (manual/conflict).
	unresolved, err := h.Store.CountUnresolvedAliasesByOrg(ctx, orgPK)
	if err != nil {
		if h.Log != nil {
			h.Log.Error("count_unresolved_aliases_failed", "error", err.Error(), "bin", bin)
		}
		return ContractorDTO{}, err
	}
	unverified := unresolved > 0

	// F1: conflict/неразрешённые-manual псевдонимы имеют organization_id=NULL (Story 2.3 P1) → CountUnresolvedAliasesByOrg
	// их НЕ видит. Доп. детект спорной идентичности по СОВПАДЕНИЮ ИМЕНИ (per-БИН реконструкция, отложенная из 2.3).
	if !unverified {
		names, nerr := h.Store.ListUnresolvedAliasNames(ctx)
		if nerr != nil {
			if h.Log != nil {
				h.Log.Error("list_unresolved_aliases_failed", "error", nerr.Error(), "bin", bin)
			}
			return ContractorDTO{}, nerr
		}
		unverified = h.contestedByName(org, names)
	}

	agg, err := h.Store.ContractorAggregates(ctx, orgPK)
	if err != nil {
		if h.Log != nil {
			h.Log.Error("contractor_aggregates_failed", "error", err.Error(), "bin", bin)
		}
		return ContractorDTO{}, err
	}
	flagRows, err := h.Store.ListContractorFlags(ctx, orgPK)
	if err != nil {
		if h.Log != nil {
			h.Log.Error("list_contractor_flags_failed", "error", err.Error(), "bin", bin)
		}
		return ContractorDTO{}, err
	}
	rnuRows, err := h.Store.ListRNUByOrg(ctx, org.ID)
	if err != nil {
		if h.Log != nil {
			h.Log.Error("list_rnu_failed", "error", err.Error(), "bin", bin)
		}
		return ContractorDTO{}, err
	}
	contractRows, err := h.Store.ListContractsBySupplierOrg(ctx, orgPK)
	if err != nil {
		if h.Log != nil {
			h.Log.Error("list_contractor_contracts_failed", "error", err.Error(), "bin", bin)
		}
		return ContractorDTO{}, err
	}

	contracts := make([]ContractorContractDTO, 0, len(contractRows))
	for _, c := range contractRows {
		contracts = append(contracts, ContractorContractDTO{
			GoszakupContractID: c.GoszakupContractID,
			SubjectRu:          fromText(c.SubjectRu),
			SubjectKk:          fromText(c.SubjectKk),
			AmountTng:          fromInt8String(c.AmountTng),
			KatoCode:           fromText(c.KatoCode),
			Direction:          fromText(c.Direction),
		})
	}

	dto := ContractorDTO{
		Bin:        org.Bin,
		NameRu:     fromText(org.NameRu),
		NameKk:     fromText(org.NameKk),
		RegKato:    fromText(org.RegKato),
		IsCustomer: org.IsCustomer,
		IsSupplier: org.IsSupplier,
		Regions:    agg.Regions,
		Contracts:  contracts,
		Flags:      resolveContractorFlags(flagRows, unverified),
		RnuMarks:   resolveRNUMarks(rnuRows, h.now()),
		SourceURL:  fromText(org.SourceUrl),
		ImportedAt: fromTimestamptz(org.ImportedAt),
		UpdatedAt:  fromTimestamptz(org.UpdatedAt),
	}

	// F6: честное состояние профиля. unverified («профиль уточняется») перекрывает всё (идентичность спорна →
	// флаги не основание). Иначе: есть связанные контракты → partial («показаны доступные данные»); нет → incomplete
	// («данные появятся после импорта»). "complete" в S-0 не выдаём (нет живого импорта).
	switch {
	case unverified:
		dto.Profile.State = "unverified"
	case agg.ContractCount > 0:
		dto.Profile.State = "partial"
	default:
		dto.Profile.State = "incomplete"
	}

	// Агрегаты: нет связанных контрактов (linkage наполняется в 2.2) → честный no_data «профиль неполный»,
	// НЕ «0 контрактов подтверждено». Есть связи → реальные значения (деньги/число строкой).
	if agg.ContractCount > 0 {
		dto.ContractCount = okField(strconv.FormatInt(agg.ContractCount, 10))
		dto.TotalAmountTng = okField(strconv.FormatInt(agg.TotalAmountTng, 10))
	} else {
		dto.ContractCount = noData[string]()
		dto.TotalAmountTng = noData[string]()
	}
	if dto.Regions == nil {
		dto.Regions = []string{}
	}
	return dto, nil
}

// Get обслуживает GET /api/contractors/{bin}.
func (h ContractorsHandler) Get(w http.ResponseWriter, r *http.Request) {
	bin := chi.URLParam(r, "bin")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	dto, err := h.Projection(ctx, bin)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		apierr.Write(w, http.StatusNotFound, apierr.CodeNotFound, "contractor not found")
		return
	case err != nil:
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, dto)
}
