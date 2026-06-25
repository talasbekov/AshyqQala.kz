package methodology

import (
	"strconv"
	"strings"

	"ashyqqala/server/internal/flags"
)

// ParamKV — плоский порог методики (key/value строкой, как в таблице methodology_params). Story 4.6:
// наполнение DB-реестра из вложенного YAML + перекрёстный инвариант YAML↔DB.
type ParamKV struct {
	Key   string
	Value string
}

// Flatten разворачивает типизированные flags.Params (из Load) в ПЛОСКИЙ детерминированный список порогов
// (как на проводе/в DB-реестре methodology_params). Ключи стабильны (точечная нотация YAML), значения — строкой.
// Порядок фиксирован (для детерминизма seed/инварианта). Не включает methodology_version (это колонка version
// строки, а не порог). exclude_methods → CSV (пустой список → пустая строка).
func Flatten(p flags.Params) []ParamKV {
	return []ParamKV{
		{"median.min_sample", strconv.Itoa(p.MinSample)},
		{"median.comparability_window_months", strconv.Itoa(p.ComparabilityWindowMonths)},
		{"flag.price_per_km.deviation_factor", strconv.FormatFloat(p.PricePerKMDeviationFactor, 'g', -1, 64)},
		{"flag.monopoly.concentration_share", strconv.FormatFloat(p.MonopolyConcentrationShare, 'g', -1, 64)},
		{"flag.monopoly.min_group_contracts", strconv.Itoa(p.MonopolyMinGroupContracts)},
		{"flag.single_participant.enabled", strconv.FormatBool(p.SingleParticipantEnabled)},
		{"flag.single_participant.exclude_methods", strings.Join(p.SingleParticipantExcludeMethods, ",")},
		{"flag.rnu.enabled", strconv.FormatBool(p.RNUEnabled)},
	}
}
