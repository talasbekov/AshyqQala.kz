package flags

import (
	"math"

	"ashyqqala/server/internal/registry"
)

// MonopolyEvidence — ИММУТАБЕЛЬНЫЕ входы расчёта флага «монополия в регионе» (FR-21) для пересчитываемости
// третьим лицом. Сериализуется в `risk_flags.evidence` (jsonb). methodology_version пинит порог.
type MonopolyEvidence struct {
	SupplierBIN        string  `json:"supplier_bin"`
	SupplierSum        *int64  `json:"supplier_sum"`
	GroupTotalSum      *int64  `json:"group_total_sum"`
	GroupContracts     int     `json:"group_contracts"`
	SupplierResolved   bool    `json:"supplier_resolved"`
	ConcentrationShare float64 `json:"concentration_share"`
	MinGroupContracts  int     `json:"min_group_contracts"`
	ComparabilityKey   string  `json:"comparability_key"`
	MethodologyVersion string  `json:"methodology_version"`
}

// shareScale — масштаб для ЦЕЛОЧИСЛЕННОГО (детерминированного) сравнения доли с дробным порогом. concentration_share
// (0.5) → целое threshold = round(share × scale) (500), сравнение supplier×scale >= total×threshold идёт в int64
// БЕЗ плавающего деления → пересчёт третьим лицом бит-в-бит (0.5 = 1/2: supplier×1000 >= total×500, т.е. 2·supplier >= total).
const shareScale = 1000

// Monopoly — ЧИСТАЯ оценка флага «монополия в регионе» (FR-21, Story 4.4): по сумме ₸ топ-поставщика в группе
// (КАТО × направление), сумме ₸ всей группы (знаменатель), числу контрактов группы и разрешённости БИН →
// честное состояние + evidence. Пороги (concentration_share, min_group_contracts) — ТОЛЬКО из methodology_params
// (params). Без store/IO/времени.
//
//   - число контрактов группы < params.MonopolyMinGroupContracts → `insufficient_data` (выборка мала);
//   - сумма поставщика nil ИЛИ знаменатель nil/≤0 ИЛИ сумма поставщика < 0 → `insufficient_data` (нет/недостоверна
//     база — НЕ выдуманная монополия);
//   - БИН не разрешён (resolve_status manual/conflict) → `insufficient_data` («профиль уточняется» — не флагать
//     неразрешённый БИН; reason различается в evidence.supplier_resolved=false);
//   - доля топ-БИН (supplier/total) ≥ concentration_share → `raised` (сигнал, требующий проверки);
//   - иначе → `not_raised`.
//
// Сравнение детерминировано (целочисленно, см. shareScale); формула простая (доля по сумме, НЕ HHI/Джини) —
// публичная объяснимость. Граница «ровно × concentration_share» → raised (инклюзивное ≥, FR-21 «≥»).
func Monopoly(in Inputs, params Params) (registry.FlagState, MonopolyEvidence) {
	ev := MonopolyEvidence{
		SupplierBIN:        in.SupplierBIN,
		SupplierSum:        in.SupplierSum,
		GroupTotalSum:      in.GroupTotalSum,
		GroupContracts:     in.GroupContracts,
		SupplierResolved:   in.SupplierBINResolved,
		ConcentrationShare: params.MonopolyConcentrationShare,
		MinGroupContracts:  params.MonopolyMinGroupContracts,
		ComparabilityKey:   in.ComparabilityKey,
		MethodologyVersion: params.MethodologyVersion,
	}

	if in.GroupContracts < params.MonopolyMinGroupContracts {
		return registry.FlagInsufficientData, ev
	}
	if in.SupplierSum == nil || in.GroupTotalSum == nil {
		return registry.FlagInsufficientData, ev
	}
	// Непозитивный знаменатель (≤0) недостоверен как база доли; отрицательная сумма поставщика — грязные данные.
	// Оба → честный insufficient (база доли невычислима — НЕ фабрикация монополии; урок ревью 4.3).
	if *in.GroupTotalSum <= 0 || *in.SupplierSum < 0 {
		return registry.FlagInsufficientData, ev
	}
	// Неразрешённый БИН (нормализация manual/conflict) НЕ флагается («профиль уточняется»). Honest: не оцениваем
	// долю на неразрешённом профиле (prd.md:185; epics.md:1461-1463). Reason — в evidence.supplier_resolved.
	if !in.SupplierBINResolved {
		return registry.FlagInsufficientData, ev
	}
	// threshold = round(share × scale) — целое (для 0.5 при scale=1000 → 500), считается один раз; сам факт «доля
	// дробная» НЕ протекает в сравнение (плавающего деления нет). supplier×scale >= total×threshold, ≥ инклюзивно.
	threshold := int64(math.Round(params.MonopolyConcentrationShare * shareScale))
	if *in.SupplierSum*shareScale >= *in.GroupTotalSum*threshold {
		return registry.FlagRaised, ev
	}
	return registry.FlagNotRaised, ev
}
