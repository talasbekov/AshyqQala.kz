package httpapi

import (
	"context"
	"log/slog"
	"math"
	"net/http"
	"time"

	"ashyqqala/server/internal/apierr"
	"ashyqqala/server/internal/registry"
	"ashyqqala/server/internal/store/gen"
)

// LotsStore — что хендлеру ранней карты нужно от слоя данных (реализует *gen.Queries; мокается в тестах).
type LotsStore interface {
	ListLotsWithGeo(ctx context.Context) ([]gen.ListLotsWithGeoRow, error)
}

// MapLot — ⏳ ИНТЕРИМ (Story 0.8, трек «Парсер-мост»): wire-форма лота для ранней карты.
// Наименование/сумма — честный конверт {value,state}; координаты — bare [lon,lat] (nullable,
// null ⇒ нет точки, НИКОГДА 0,0). geocode_state — подмножество value_state (почему лот на карте/вне):
// ok (есть точка) | geocode_pending (ещё не геокодирован) | geocode_failed (unmatched, без точки).
type MapLot struct {
	GoszakupLotID string              `json:"goszakup_lot_id"`
	SubjectRu     Field[string]       `json:"subject_ru"`
	SubjectKk     Field[string]       `json:"subject_kk"`
	AmountTng     Field[string]       `json:"amount_tng"`
	Lon           *float64            `json:"lon"`
	Lat           *float64            `json:"lat"`
	GeocodeState  registry.ValueState `json:"geocode_state"`
}

// toMapLot переводит строку LEFT JOIN lots↔interim_geo_lots в честный wire-DTO.
// Гардрейл честности: координата отдаётся ТОЛЬКО для matched (auto + обе координаты валидны);
// иначе lon/lat = nil (никогда 0,0). Различаем «нет гео-строки» (geocode_pending) от «unmatched».
// log (может быть nil) служит для WARN-сигнала о нарушении инварианта пайплайна (см. ветку auto-без-точки).
func toMapLot(row gen.ListLotsWithGeoRow, log *slog.Logger) MapLot {
	m := MapLot{
		GoszakupLotID: row.GoszakupLotID,
		SubjectRu:     fromText(row.TitleRu),
		SubjectKk:     fromText(row.TitleKk),
		AmountTng:     fromInt8String(row.Amount),
	}
	validCoords := row.Lat.Valid && row.Lon.Valid && isFinite(row.Lat.Float64) && isFinite(row.Lon.Float64)
	switch {
	case !row.GeocodeStatus.Valid:
		// LEFT JOIN не нашёл строки в interim_geo_lots → лот ещё не геокодирован
		m.GeocodeState = registry.StateGeocodePending
	case row.GeocodeStatus.String == "auto" && validCoords:
		lat := row.Lat.Float64
		lon := row.Lon.Float64
		m.Lat = &lat
		m.Lon = &lon
		m.GeocodeState = registry.StateOK
	case row.GeocodeStatus.String == "auto" && !validCoords:
		// АНОМАЛИЯ: status=auto обязан нести координаты (инвариант Story 0.7: auto ⇒ coordinates present).
		// Это не честный unmatched — это нарушение инварианта пайплайна геокодера, его НЕЛЬЗЯ молча
		// маскировать под geocode_failed. На проводе всё равно отдаём geocode_failed (без точки, НЕ 0,0),
		// но сигнал должен быть НАБЛЮДАЕМ через WARN-лог, чтобы аномалию заметили в проде.
		if log != nil {
			log.Warn("map_lot_invariant_violation",
				"goszakup_lot_id", row.GoszakupLotID,
				"reason", "geocode_status=auto без валидных конечных координат (нарушен инвариант auto⇒coords)")
		}
		m.GeocodeState = registry.StateGeocodeFailed
	default:
		// unmatched (или иной не-auto статус без точки) → честно без точки (НЕ 0,0 и не NaN/Inf).
		// БД CHECK гарантирует «обе координаты или ни одной», но НЕ конечность и НЕ «auto ⇒ not-null».
		m.GeocodeState = registry.StateGeocodeFailed
	}
	return m
}

// isFinite — координата пригодна для карты только если конечна. NaN/Inf допустимы в типе double precision
// и CHECK в БД их НЕ ловит; json.Encode на них падает уже после WriteHeader(200) → обрезанный ответ ломает
// ВЕСЬ /api/lots (а не один лот). Честнее отдать geocode_failed (defense-in-depth к гарду геокодера 0.7).
func isFinite(f float64) bool { return !math.IsNaN(f) && !math.IsInf(f, 0) }

// MapLotsHandler — хендлер ранней карты лотов (Story 0.8). Читает interim_geo_lots через store
// (НЕ импортирует tools/scrape — изоляция трека сохранена, go-list-страж зелёный).
type MapLotsHandler struct {
	Store LotsStore
	Log   *slog.Logger
}

// List обслуживает GET /api/lots.
func (h MapLotsHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second) // per-request таймаут к БД
	defer cancel()
	rows, err := h.Store.ListLotsWithGeo(ctx)
	if err != nil {
		if h.Log != nil {
			h.Log.Error("list_lots_with_geo_failed", "error", err.Error())
		}
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, "internal error")
		return
	}
	out := make([]MapLot, 0, len(rows)) // [] (не null) при пустоте — честная деградация контейнера на фронте
	for _, row := range rows {
		out = append(out, toMapLot(row, h.Log))
	}
	writeJSON(w, http.StatusOK, out)
}
