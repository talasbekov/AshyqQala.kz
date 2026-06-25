package flags

import (
	"ashyqqala/server/internal/registry"
)

// RNUEvidence — ИММУТАБЕЛЬНЫЕ входы расчёта флага «наличие в РНУ» (FR-22) для пересчитываемости третьим лицом.
// Сериализуется в `risk_flags.evidence` (jsonb). Атрибуция ГОСУДАРСТВУ: ссылка на реестр (source_url,
// goszakup_rnu_id) + даты записи — НЕ оценка платформы. methodology_version пинит kill-switch.
type RNUEvidence struct {
	GoszakupRNUID      string `json:"goszakup_rnu_id"`
	StartDateUnix      *int64 `json:"start_date"`
	EndDateUnix        *int64 `json:"end_date"`
	SourceURL          string `json:"source_url"`
	ReasonRef          string `json:"reason_ref"`
	Enabled            bool   `json:"enabled"`
	MethodologyVersion string `json:"methodology_version"`
}

// RNU — ЧИСТАЯ ДАТА-ЗАВИСИМАЯ оценка флага «наличие в РНУ» (FR-22, Story 4.5): по датам записи РНУ Подрядчика и
// «сейчас» (in.Now, clock.Clock) → честное состояние + evidence. Kill-switch (enabled) — ТОЛЬКО из
// methodology_params. Время берётся ТОЛЬКО через in.Now (НЕ time.Now) → детерминизм; пакет НЕ импортирует `time`
// (как benchmark.GroupMedian) → go-list-граница чистоты ядра.
//
//   - `!enabled` → `not_published` (методика отключила флаг — это не «всё чисто»);
//   - `start_date` nil (битая запись) → `insufficient_data` (честно — НЕ выдуманный факт);
//   - `start_date > now` (запись ещё не активна) → `not_raised`;
//   - `end_date` задан И `end_date ≤ now` (запись истекла) → `not_raised` (АВТО-СНЯТИЕ, не «навечно»);
//   - иначе (start ≤ now, end пуст/в будущем) → `raised` (активная запись в реестре — государственный факт).
//
// Реестр АВТОРИТЕТЕН: отсутствие/истечение записи → not_raised (определённый отрицательный факт), НЕ insufficient.
// Граница: `start == now` → активна (≤); `end == now` → снята (для активности end > now строго).
func RNU(in Inputs, params Params) (registry.FlagState, RNUEvidence) {
	ev := RNUEvidence{
		GoszakupRNUID:      in.RNUGoszakupID,
		StartDateUnix:      in.RNUStartUnix,
		EndDateUnix:        in.RNUEndUnix,
		SourceURL:          in.RNUSourceURL,
		ReasonRef:          in.RNUReasonRef,
		Enabled:            params.RNUEnabled,
		MethodologyVersion: params.MethodologyVersion,
	}

	if !params.RNUEnabled {
		return registry.FlagNotPublished, ev
	}
	if in.RNUStartUnix == nil {
		return registry.FlagInsufficientData, ev
	}
	if in.Now == nil {
		return registry.FlagInsufficientData, ev // дата-зависимый флаг без часов оценить нельзя (ядро не паникует)
	}
	now := in.Now.Now().Unix() // время только через clock; .Unix() — метод time.Time, пакет `time` не импортируется
	if *in.RNUStartUnix > now {
		return registry.FlagNotRaised, ev
	}
	if in.RNUEndUnix != nil && *in.RNUEndUnix <= now {
		return registry.FlagNotRaised, ev
	}
	return registry.FlagRaised, ev
}
