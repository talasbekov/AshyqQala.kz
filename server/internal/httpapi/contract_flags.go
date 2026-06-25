package httpapi

import (
	"encoding/json"

	"ashyqqala/server/internal/registry"
	"ashyqqala/server/internal/store/gen"
)

// contractFlagTypes — flag_type'ы, применимые к КАРТОЧКЕ КОНТРАКТА (контракт-субъект: FR-19 «единственный
// участник», FR-20 «цена за км»). Монополия (FR-21) и РНУ (FR-22) — contractor-субъект (organization_id) и
// уходят на карточку подрядчика (Story 5.2): на ЭТОЙ поверхности они вне охвата (AC5) и здесь не появляются
// (а не как «not_applicable»-шум в каждой карточке). Порядок — детерминированный (стабильность wire/golden).
var contractFlagTypes = []string{
	"single_participant", // FR-19, Story 4.2
	"price_per_km",       // FR-20, Story 4.3
}

// ContractFlagDTO — wire-форма флага на карточке контракта. state — ось flag_state двухосевого honest-enum
// (registry, Story 1.4): raised | not_raised | insufficient_data | not_published. evidence — СЫРОЙ jsonb
// пересчитываемости (FR-23: все входы расчёта); непусто ТОЛЬКО при raised (иначе null). methodology_version —
// value лишь когда строка существует (флаг оценивался), иначе честный no_data.
type ContractFlagDTO struct {
	FlagID             string             `json:"flag_id"`
	State              registry.FlagState `json:"state"`
	MethodologyVersion Field[string]      `json:"methodology_version"`
	DetectedAt         Field[string]      `json:"detected_at"` // когда флаг оценён (ISO8601 Z); no_data если строки нет
	Evidence           json.RawMessage    `json:"evidence"`
}

// resolveContractFlags — ЧЕСТНАЯ реконструкция состояний флагов карточки на ЧТЕНИИ (Story 5.1, AC4; несущее
// наследие ретро Epic 4: «store схлопывает честные состояния → Epic 5 обязан реконструировать на чтении»).
// risk_flags хранит ТОЛЬКО строки raised (is_active=true) и снятые (is_active=false); «нет строки» само по себе
// НЕ доказывает «всё чисто». Правило (гардрейл честности над домыслом, §7.4):
//
//   - активная строка        → raised      (+ evidence + methodology_version, FR-23);
//   - снятая строка (!active) → not_raised  (ДОКАЗАНО: флаг оценивался и не сработал);
//   - строки НЕТ вовсе        → insufficient_data (НЕТ доказательства оценки — НЕ ложный not_raised «чисто»).
//
// Возвращает дескриптор на КАЖДЫЙ contractFlagTypes в детерминированном порядке, поэтому «проверено, сигнала
// нет» отличимо от «нет данных» (карточка никогда молча не подразумевает «всё чисто»). rows —
// ListContractFlags(contract_id). Монополия/РНУ (contractor-субъект) сюда НЕ попадают (AC5).
func resolveContractFlags(rows []gen.RiskFlag) []ContractFlagDTO {
	byType := make(map[string]gen.RiskFlag, len(rows))
	for _, r := range rows {
		// Карточка контракта рассматривает только contract-субъектные строки (страховка: ListContractFlags уже
		// фильтрует по contract_id, но контрактор-строка с тем же contract_id невозможна по схеме).
		byType[r.FlagType] = r
	}
	out := make([]ContractFlagDTO, 0, len(contractFlagTypes))
	for _, ft := range contractFlagTypes {
		dto := ContractFlagDTO{FlagID: ft, MethodologyVersion: noData[string](), DetectedAt: noData[string](), Evidence: nil}
		switch r, ok := byType[ft]; {
		case !ok:
			dto.State = registry.FlagInsufficientData // нет строки → НЕТ доказательства оценки (честно)
		case r.IsActive:
			dto.State = registry.FlagRaised
			dto.MethodologyVersion = okField(r.MethodologyVersion)
			dto.DetectedAt = fromTimestamptz(r.DetectedAt)
			dto.Evidence = json.RawMessage(r.Evidence) // пересчитываемость FR-23
		default:
			dto.State = registry.FlagNotRaised // снятая строка → оценивалось, сигнал не выставлен
			dto.MethodologyVersion = okField(r.MethodologyVersion)
			dto.DetectedAt = fromTimestamptz(r.DetectedAt)
		}
		out = append(out, dto)
	}
	return out
}
