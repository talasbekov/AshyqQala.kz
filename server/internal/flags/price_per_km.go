package flags

import (
	"math"

	"ashyqqala/server/internal/registry"
)

// PricePerKMEvidence — ИММУТАБЕЛЬНЫЕ входы расчёта флага «аномальная цена за км» (FR-20) для пересчитываемости
// третьим лицом. Сериализуется в `risk_flags.evidence` (jsonb). methodology_version пинит порог.
type PricePerKMEvidence struct {
	PricePerKM         *int64  `json:"price_per_km"`
	Median             *int64  `json:"median"`
	SampleSize         int     `json:"sample_size"`
	DeviationFactor    float64 `json:"deviation_factor"`
	ComparabilityKey   string  `json:"comparability_key"`
	MethodologyVersion string  `json:"methodology_version"`
}

// factorScale — масштаб для ЦЕЛОЧИСЛЕННОГО (детерминированного) сравнения с дробным порогом. deviation_factor
// (1.5) → целое threshold = round(factor × scale) (1500), сравнение price×scale > median×threshold идёт в
// int64 БЕЗ плавающей точки → пересчёт третьим лицом бит-в-бит (1.5 = 3/2: price×1000 > median×1500).
const factorScale = 1000

// PricePerKM — ЧИСТАЯ оценка флага «аномальная цена за км» (FR-20, Story 4.3): по цене/км контракта, медиане
// сопоставимой группы и размеру выборки → честное состояние + evidence. Порог (deviation_factor) — ТОЛЬКО из
// methodology_params (params). Без store/IO/времени.
//
//   - цена/км nil (не вычислима, напр. нет geo_objects.length_km) ИЛИ медиана nil → `insufficient_data` (флаг
//     НЕ строится — НЕ выдуманная аномалия);
//   - медиана ≤ 0 (база сравнения недостоверна) ИЛИ цена/км < 0 (грязные данные) → `insufficient_data`: иначе
//     выродившийся порог ≤ 0 / отрицательный вход фабриковали бы аномалию (honesty-over-inference);
//   - размер выборки < params.MinSample → `insufficient_data` (медиана недостоверна);
//   - цена/км > медиана × deviation_factor → `raised` (сигнал, требующий проверки);
//   - иначе → `not_raised`.
//
// Сравнение детерминировано (целочисленно, см. factorScale); формула простая (×1.5, НЕ MAD) — публичная
// объяснимость. Граница «ровно ×медиана×factor» → not_raised (строгое >).
func PricePerKM(in Inputs, params Params) (registry.FlagState, PricePerKMEvidence) {
	ev := PricePerKMEvidence{
		PricePerKM:         in.PricePerKM,
		Median:             in.GroupMedian,
		SampleSize:         in.GroupSampleSize,
		DeviationFactor:    params.PricePerKMDeviationFactor,
		ComparabilityKey:   in.ComparabilityKey,
		MethodologyVersion: params.MethodologyVersion,
	}

	if in.PricePerKM == nil || in.GroupMedian == nil {
		return registry.FlagInsufficientData, ev
	}
	// Непозитивная медиана (≤0) недостоверна как база сравнения — порог выродился бы в ≤0 и любая положительная
	// цена/км дала бы фабрикацию аномалии; отрицательная цена/км — грязные данные. Оба → честный insufficient.
	if *in.GroupMedian <= 0 || *in.PricePerKM < 0 {
		return registry.FlagInsufficientData, ev
	}
	if in.GroupSampleSize < params.MinSample {
		return registry.FlagInsufficientData, ev
	}
	// threshold = round(factor × scale) — целое (для 1.5 при scale=1000 → 1500), считается один раз; сам факт
	// «фактор дробный» НЕ протекает в сравнение (плавающей точки в сравнении нет). price×scale > median×threshold.
	threshold := int64(math.Round(params.PricePerKMDeviationFactor * factorScale))
	if *in.PricePerKM*factorScale > *in.GroupMedian*threshold {
		return registry.FlagRaised, ev
	}
	return registry.FlagNotRaised, ev
}
