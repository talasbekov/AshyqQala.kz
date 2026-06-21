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

	"ashyqqala/server/internal/apierr"
	"ashyqqala/server/internal/store/gen"
)

// ContractStore — что хендлеру нужно от слоя данных (реализует *gen.Queries; мокается в тестах).
type ContractStore interface {
	GetContractByID(ctx context.Context, goszakupContractID string) (gen.Contract, error)
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
	SourceURL          Field[string] `json:"source_url"`
	ImportedAt         Field[string] `json:"imported_at"`
	UpdatedAt          Field[string] `json:"updated_at"`
}

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
		SourceURL:          fromText(c.SourceUrl),
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
	writeJSON(w, http.StatusOK, toContractDTO(c))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
