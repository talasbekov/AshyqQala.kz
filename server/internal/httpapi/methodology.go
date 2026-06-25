package httpapi

import (
	"net/http"

	"ashyqqala/server/internal/flags"
)

// MethodologyHandler — read-only пороги методики (Story 5.3, FR-23/NFR-5). Пороги ВСЕГДА отдаются из ЕДИНОГО
// источника `methodology_params` (registry/values, рантайм-загрузка) — экран методики показывает их даже когда
// сигнал НЕ выставлен (нет evidence), без литералов на фронте. Конфиг иммутабелен в рантайме → params статичны.
type MethodologyHandler struct {
	Params flags.Params
}

// MethodologyThresholds — пороги методики на проводе (snake_case). Доли/факторы — JSON-числа (малые десятичные,
// точность не теряется); целые — int. Имена синхронны с methodology_params.v1.yaml.
type MethodologyThresholds struct {
	MinSample                       int      `json:"min_sample"`
	ComparabilityWindowMonths       int      `json:"comparability_window_months"`
	PricePerKMDeviationFactor       float64  `json:"price_per_km_deviation_factor"`
	MonopolyConcentrationShare      float64  `json:"monopoly_concentration_share"`
	MonopolyMinGroupContracts       int      `json:"monopoly_min_group_contracts"`
	SingleParticipantExcludeMethods []string `json:"single_participant_exclude_methods"`
}

// MethodologyDTO — wire-форма экрана методики: каноническая версия + пороги (для рендера формулы/порогов ВСЕГДА).
type MethodologyDTO struct {
	Version    string                `json:"version"`
	Thresholds MethodologyThresholds `json:"thresholds"`
}

// Get обслуживает GET /api/methodology — статичные пороги из methodology_params (без БД).
func (h MethodologyHandler) Get(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, MethodologyDTO{
		Version: h.Params.MethodologyVersion,
		Thresholds: MethodologyThresholds{
			MinSample:                       h.Params.MinSample,
			ComparabilityWindowMonths:       h.Params.ComparabilityWindowMonths,
			PricePerKMDeviationFactor:       h.Params.PricePerKMDeviationFactor,
			MonopolyConcentrationShare:      h.Params.MonopolyConcentrationShare,
			MonopolyMinGroupContracts:       h.Params.MonopolyMinGroupContracts,
			SingleParticipantExcludeMethods: h.Params.SingleParticipantExcludeMethods,
		},
	})
}
