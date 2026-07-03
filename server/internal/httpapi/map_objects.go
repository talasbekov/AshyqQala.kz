package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ashyqqala/server/internal/apierr"
	"ashyqqala/server/internal/store"
	"ashyqqala/server/internal/store/gen"
)

// mapObjectsLimit — cap выдачи bbox-окна (Story 3.4 D2, NFR-1): при превышении ответ несёт truncated=true
// (честная плашка «показаны не все объекты», не тихое обрезание). Пилотная плотность — сотни–тысячи.
const mapObjectsLimit = 2000

// MapObjectsStore — что хендлеру канонической карты нужно от слоя данных (мокается в тестах).
type MapObjectsStore interface {
	ListGeoObjectsInBBox(ctx context.Context, minLon, minLat, maxLon, maxLat float64, limit int) ([]store.GeoObjectPoint, bool, error)
	CountContractsWithoutPoint(ctx context.Context) (int64, error)
}

// mapObjectsStore — адаптер pgx-raw функций store поверх пула/tx (gen.DBTX); прецедент NewDistrictStore.
type mapObjectsStore struct{ db gen.DBTX }

// NewMapObjectsStore оборачивает соединение адаптером карты (bbox-ридер 3.1 + счётчик «без точки» 3.4).
func NewMapObjectsStore(db gen.DBTX) MapObjectsStore { return mapObjectsStore{db: db} }

func (s mapObjectsStore) ListGeoObjectsInBBox(ctx context.Context, minLon, minLat, maxLon, maxLat float64, limit int) ([]store.GeoObjectPoint, bool, error) {
	return store.ListGeoObjectsInBBox(ctx, s.db, minLon, minLat, maxLon, maxLat, limit)
}

func (s mapObjectsStore) CountContractsWithoutPoint(ctx context.Context) (int64, error) {
	return store.CountContractsWithoutPoint(ctx, s.db)
}

// MapObject — wire-форма объекта канонической карты (Story 3.4, FR-7). geom — GeoJSON RFC7946
// (Point|LineString, порядок [lon,lat], AR-19) как отдал ST_AsGeoJSON — сервер его не пересобирает.
// has_active_flag питает глиф «!» маркера и амбер-кольцо кластера (AC2); geocode_status — глиф «✓»
// (verified) и default-ветку рендера (AR-16: закрытый union, фронт обязан переживать неизвестное значение).
// Контента превью (предмет/сумма/подрядчик) тут НЕТ — это Story 3.5 (FR-8).
type MapObject struct {
	PublicID           string          `json:"public_id"`
	GoszakupContractID *string         `json:"goszakup_contract_id"` // null — гео-строка без контракта (честно)
	GeocodeStatus      string          `json:"geocode_status"`
	HasActiveFlag      bool            `json:"has_active_flag"`
	LengthKm           *float64        `json:"length_km"` // только LINESTRING; null у точки — у точки нет длины
	Geom               json.RawMessage `json:"geom"`
}

// MapObjectsResponse — ответ bbox-окна: объекты + честная truncated-детекция cap'а + счётчик
// «без точки на карте» (AC3: unmatched и контракты без гео-строки на карте структурно невидимы —
// счётчик и есть их видимость, {data-state-ungeocoded}).
type MapObjectsResponse struct {
	Items           []MapObject `json:"items"`
	Truncated       bool        `json:"truncated"`
	UngeocodedCount int64       `json:"ungeocoded_count"`
}

// MapObjectsHandler — хендлер канонической карты (Story 3.4): geo_objects (канон 3.1/3.2) в bbox.
// НЕ читает interim_geo_lots — интерим-карта лотов (/api/lots, Story 0.8) живёт отдельно до сноса моста.
type MapObjectsHandler struct {
	Store MapObjectsStore
	Log   *slog.Logger
}

// List обслуживает GET /api/map/objects?bbox=minLon,minLat,maxLon,maxLat.
func (h MapObjectsHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second) // per-request таймаут к БД
	defer cancel()

	minLon, minLat, maxLon, maxLat, err := parseBBoxParam(r.URL.Query().Get("bbox"))
	if err != nil {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidationFailed, err.Error())
		return
	}

	items, truncated, err := h.Store.ListGeoObjectsInBBox(ctx, minLon, minLat, maxLon, maxLat, mapObjectsLimit)
	if err != nil {
		if h.Log != nil {
			h.Log.Error("list_geo_objects_in_bbox_failed", "error", err.Error())
		}
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, "internal error")
		return
	}
	count, err := h.Store.CountContractsWithoutPoint(ctx)
	if err != nil {
		if h.Log != nil {
			h.Log.Error("count_contracts_without_point_failed", "error", err.Error())
		}
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, "internal error")
		return
	}

	resp := MapObjectsResponse{
		Items:           make([]MapObject, 0, len(items)), // [] не null — честная пустая коллекция
		Truncated:       truncated,
		UngeocodedCount: count,
	}
	for _, p := range items {
		resp.Items = append(resp.Items, MapObject{
			PublicID:           p.PublicID,
			GoszakupContractID: p.GoszakupContractID,
			GeocodeStatus:      p.GeocodeStatus,
			HasActiveFlag:      p.HasActiveFlag,
			LengthKm:           p.LengthKm,
			Geom:               p.Geom,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

// parseBBoxParam — синтаксический разбор query-параметра bbox («minLon,minLat,maxLon,maxLat») +
// семантическая валидация store.ValidateBBox (NaN/Inf/диапазон WGS84/инверсия). Любая ошибка —
// валидационная (400), НЕ 500: мусорный bbox — это ввод клиента, а не сбой сервера.
func parseBBoxParam(raw string) (minLon, minLat, maxLon, maxLat float64, err error) {
	if raw == "" {
		return 0, 0, 0, 0, errBBox("параметр bbox обязателен (minLon,minLat,maxLon,maxLat)")
	}
	parts := strings.Split(raw, ",")
	if len(parts) != 4 {
		return 0, 0, 0, 0, errBBox("bbox: ожидалось 4 координаты через запятую")
	}
	vals := make([]float64, 4)
	for i, p := range parts {
		v, perr := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if perr != nil {
			return 0, 0, 0, 0, errBBox("bbox: координата не число")
		}
		vals[i] = v // NaN/Inf парсятся ParseFloat без ошибки — их отсечёт ValidateBBox ниже
	}
	if verr := store.ValidateBBox(vals[0], vals[1], vals[2], vals[3]); verr != nil {
		// Пакетный префикс «store: » — внутренняя номенклатура, публичному 400-detail она не нужна
		// (500-ветка аналогично отдаёт нейтральное "internal error", а не прозу store).
		return 0, 0, 0, 0, errBBox(strings.TrimPrefix(verr.Error(), "store: "))
	}
	return vals[0], vals[1], vals[2], vals[3], nil
}

type errBBox string

func (e errBBox) Error() string { return string(e) }
