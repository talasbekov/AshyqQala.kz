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
	"ashyqqala/server/internal/export"
	"ashyqqala/server/internal/registry"
	"ashyqqala/server/internal/render"
)

// EvidenceExportHandler — публичный экспорт evidence raised-флага контракта (AR-29, SM-5): JSON (канонический,
// пересчитываемо третьим лицом) + печатный НЕЙТРАЛЬНЫЙ текст (для цитирования в СМИ). Тот же путь данных, что
// карточка (GetContractByID → ListContractFlags), поэтому цифры не расходятся. Рамка прозы — из render/registry.
type EvidenceExportHandler struct {
	Store    ContractStore
	Renderer render.Renderer
	Log      *slog.Logger
}

// lookupRaisedFlag — резолвит raised-флаг контракта (по ПУБЛИЧНОМУ goszakup-id + flag_type) в export.FlagRecord.
// (rec, true, nil) — raised с evidence; (zero, false, nil) — контракта/raised-флага нет (нечего цитировать → 404);
// (zero, false, err) — DB-ошибка (→ 500). subject_ref — публичный goszakup_contract_id (НЕ внутренний bigint).
func (h EvidenceExportHandler) lookupRaisedFlag(ctx context.Context, goszakupID, flagType string) (export.FlagRecord, bool, error) {
	c, err := h.Store.GetContractByID(ctx, goszakupID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return export.FlagRecord{}, false, nil
	case err != nil:
		return export.FlagRecord{}, false, err
	}
	rows, err := h.Store.ListContractFlags(ctx, pgtype.Int8{Int64: c.ID, Valid: true})
	if err != nil {
		return export.FlagRecord{}, false, err
	}
	for _, r := range rows {
		if r.FlagType == flagType && r.IsActive { // только raised: evidence есть лишь у активной строки
			return export.FlagRecord{
				FlagType:           r.FlagType,
				SubjectType:        r.SubjectType,
				SubjectRef:         c.GoszakupContractID,
				MethodologyVersion: r.MethodologyVersion,
				Evidence:           json.RawMessage(r.Evidence),
			}, true, nil
		}
	}
	return export.FlagRecord{}, false, nil // нет raised-строки этого типа → нечего цитировать (честный 404)
}

// exportLocale — локаль нейтральной рамки печатного экспорта: ?lang=ru|kk > дефолт KK (NFR-6). JSON-экспорт
// локаль-агностичен (только данные), рамка нужна лишь печатному тексту.
func exportLocale(r *http.Request) registry.Locale {
	if r.URL.Query().Get("lang") == "ru" {
		return registry.RU
	}
	return registry.KK
}

// serve — общий путь обоих форматов: lookup → export → запись (export.Export маршалит в буфер ДО записи статуса,
// поэтому частичного/битого ответа нет). format: "text" → text/plain (печатный); иначе → application/json.
func (h EvidenceExportHandler) serve(w http.ResponseWriter, r *http.Request, format string) {
	goszakupID := chi.URLParam(r, "goszakup_id")
	flagType := chi.URLParam(r, "flag_type")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	rec, ok, err := h.lookupRaisedFlag(ctx, goszakupID, flagType)
	if err != nil {
		if h.Log != nil {
			h.Log.Error("evidence_lookup_failed", "error", err.Error(), "goszakup_id", goszakupID)
		}
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, "internal error")
		return
	}
	if !ok {
		// Нет контракта / нет raised-флага этого типа → честно «цитировать нечего» (НЕ фабрикуем пустой экспорт).
		apierr.Write(w, http.StatusNotFound, apierr.CodeNotFound, "no raised flag to export")
		return
	}
	jb, printable, eerr := export.Export(rec, h.Renderer.Text(exportLocale(r), "frame.signal"))
	if eerr != nil {
		// Пустой/битый evidence (defensive) → честный 404, не фабрикованный документ.
		if h.Log != nil {
			h.Log.Warn("evidence_export_failed", "error", eerr.Error(), "goszakup_id", goszakupID)
		}
		apierr.Write(w, http.StatusNotFound, apierr.CodeNotFound, "no evidence to export")
		return
	}
	if format == "text" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(printable))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(jb)
}

// GetJSON — GET /api/contracts/{goszakup_id}/flags/{flag_type}/evidence.json (канонический JSON для пересчёта).
func (h EvidenceExportHandler) GetJSON(w http.ResponseWriter, r *http.Request) { h.serve(w, r, "json") }

// GetText — GET /api/contracts/{goszakup_id}/flags/{flag_type}/evidence.txt (печатный нейтральный текст для СМИ).
func (h EvidenceExportHandler) GetText(w http.ResponseWriter, r *http.Request) { h.serve(w, r, "text") }
