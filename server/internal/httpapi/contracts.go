package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"ashyqqala/server/internal/apierr"
	"ashyqqala/server/internal/store/gen"
)

// ContractStore — что хендлеру нужно от слоя данных (реализует *gen.Queries; мокается в тестах).
// Story 5.1: карточка композирует контракт + последний акт (FR-11) + контракт-флаги (FR-12/FR-23).
type ContractStore interface {
	GetContractByID(ctx context.Context, goszakupContractID string) (gen.Contract, error)
	GetLatestActByContractID(ctx context.Context, contractID int64) (gen.Act, error)
	ListContractFlags(ctx context.Context, contractID pgtype.Int8) ([]gen.RiskFlag, error)
}

// ContractDTO — wire-форма карточки контракта. snake_case; деньги строкой; даты ISO/`date`;
// nullable-поля — честный конверт {value,state}. id (внутр. bigint) на проводе НЕ отдаём —
// публичный id = natural goszakup_contract_id.
type ContractDTO struct {
	GoszakupContractID string        `json:"goszakup_contract_id"`
	SubjectRu          Field[string] `json:"subject_ru"`
	SubjectKk          Field[string] `json:"subject_kk"`
	AmountTng          Field[string] `json:"amount_tng"`
	SignDate           Field[string] `json:"sign_date"`
	PlanStart          Field[string] `json:"plan_start"`
	PlanEnd            Field[string] `json:"plan_end"`
	Status             Field[string] `json:"status"`
	Direction          Field[string] `json:"direction"`
	KatoCode           Field[string] `json:"kato_code"`
	// Заказчик/Подрядчик (FR-10): колонок customer_org_id/supplier_org_id + таблицы organizations ещё нет
	// (Epic 2). Токен-готово: пока честный no_data (карточка показывает «нет данных», не выдумывает), при
	// появлении organizations запрос join'ит и поля наполняются БЕЗ изменения wire-формы/карточки.
	Customer   Field[string]     `json:"customer"`
	Supplier   Field[string]     `json:"supplier"`
	SourceURL  Field[string]     `json:"source_url"`
	Act        ActDTO            `json:"act"`   // FR-11: акт приёмки (дата/подписант), за честным состоянием
	Flags      []ContractFlagDTO `json:"flags"` // FR-12/FR-23: активные флаги + честная реконструкция (AC4)
	ImportedAt Field[string]     `json:"imported_at"`
	UpdatedAt  Field[string]     `json:"updated_at"`
}

// ActDTO — wire-форма акта на карточке (FR-11). present — есть ли акт вообще; при отсутствии все поля no_data
// (не выдумываем дату/подписанта). signer — служебная информация должностного лица (§6.2.4), не приватные данные.
type ActDTO struct {
	Present   bool          `json:"present"`
	ActDate   Field[string] `json:"act_date"`
	Signer    Field[string] `json:"signer"`
	SourceURL Field[string] `json:"source_url"`
}

// emptyActDTO — честное «акта нет»: present=false, все поля no_data (НИКОГДА фейковая дата/подписант).
func emptyActDTO() ActDTO {
	return ActDTO{Present: false, ActDate: noData[string](), Signer: noData[string](), SourceURL: noData[string]()}
}

func toActDTO(a gen.Act) ActDTO {
	return ActDTO{
		Present:   true,
		ActDate:   fromDate(a.ActDate),
		Signer:    fromText(a.SignerInfo),
		SourceURL: fromText(a.SourceUrl),
	}
}

// toContractDTO — контракт-поля карточки. Act/Flags инициализируются честными дефолтами (нет акта / нет
// доказательства оценки флагов) и ПЕРЕЗАПИСЫВАЮТСЯ хендлером по фактическим чтениям (Get).
func toContractDTO(c gen.Contract) ContractDTO {
	return ContractDTO{
		GoszakupContractID: c.GoszakupContractID,
		SubjectRu:          fromText(c.SubjectRu),
		SubjectKk:          fromText(c.SubjectKk),
		AmountTng:          fromInt8String(c.AmountTng),
		SignDate:           fromDate(c.SignDate),
		PlanStart:          fromDate(c.PlanStart),
		PlanEnd:            fromDate(c.PlanEnd),
		Status:             fromText(c.Status),
		Direction:          fromText(c.Direction),
		KatoCode:           fromText(c.KatoCode),
		Customer:           noData[string](), // Epic 2 (organizations) — токен-готово
		Supplier:           noData[string](),
		SourceURL:          fromText(c.SourceUrl),
		Act:                emptyActDTO(),                 // перезапишет хендлер при наличии акта
		Flags:              resolveContractFlags(nil),     // дефолт: insufficient_data; перезапишет хендлер
		ImportedAt:         fromTimestamptz(c.ImportedAt), // ISO8601 Z; NULL→no_data (не фабрикуем дату)
		UpdatedAt:          fromTimestamptz(c.UpdatedAt),
	}
}

// ContractsHandler — хендлер карточки контракта.
type ContractsHandler struct {
	Store ContractStore
	Log   *slog.Logger
}

// Get обслуживает GET /api/contracts/{goszakup_id}.
func (h ContractsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "goszakup_id")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second) // per-request таймаут к БД
	defer cancel()
	c, err := h.Store.GetContractByID(ctx, id)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		apierr.Write(w, http.StatusNotFound, apierr.CodeNotFound, "contract not found")
		return
	case err != nil:
		if h.Log != nil {
			h.Log.Error("get_contract_failed", "error", err.Error(), "goszakup_id", id)
		}
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, "internal error")
		return
	}
	dto := toContractDTO(c)

	// Акт (FR-11). Нет акта (ErrNoRows) → честный no_data-блок (дефолт из toContractDTO). DB-ошибка ≠ «нет
	// акта»: маскировать сбой под «акта нет» = домысел (§7.4) → честная 500.
	act, aerr := h.Store.GetLatestActByContractID(ctx, c.ID)
	switch {
	case errors.Is(aerr, pgx.ErrNoRows):
		// dto.Act уже emptyActDTO()
	case aerr != nil:
		if h.Log != nil {
			h.Log.Error("get_act_failed", "error", aerr.Error(), "goszakup_id", id)
		}
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, "internal error")
		return
	default:
		dto.Act = toActDTO(act)
	}

	// Активные флаги + ЧЕСТНАЯ реконструкция состояний на чтении (FR-12/FR-23, AC3/AC4).
	rows, ferr := h.Store.ListContractFlags(ctx, pgtype.Int8{Int64: c.ID, Valid: true})
	if ferr != nil {
		if h.Log != nil {
			h.Log.Error("list_contract_flags_failed", "error", ferr.Error(), "goszakup_id", id)
		}
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, "internal error")
		return
	}
	dto.Flags = resolveContractFlags(rows)

	writeJSON(w, http.StatusOK, dto)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
