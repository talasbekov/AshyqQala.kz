package flags

import (
	"strings"

	"ashyqqala/server/internal/registry"
)

// SingleParticipantEvidence — ИММУТАБЕЛЬНЫЕ входы расчёта флага «единственный участник» (FR-19) для
// пересчитываемости третьим лицом (architecture.md:122, 591). Сериализуется в `risk_flags.evidence` (jsonb).
// methodology_version пинит пороги (перекрёстный инвариант: версия в evidence == в params).
type SingleParticipantEvidence struct {
	ParticipantCount   *int     `json:"participant_count"`
	ProcurementMethod  string   `json:"procurement_method"`
	ExcludeMethods     []string `json:"exclude_methods"`
	Enabled            bool     `json:"enabled"`
	MethodologyVersion string   `json:"methodology_version"`
}

// SingleParticipant — ЧИСТАЯ оценка флага «единственный участник» (FR-19, Story 4.2): по числу участников
// объявления и способу закупки → честное состояние флага + evidence. Пороги — ТОЛЬКО из methodology_params
// (params), литералов нет. Без store/IO/времени (ядро остаётся чистым).
//
//   - `!enabled` → `not_published` (методика отключила флаг — это не «всё чисто»);
//   - число участников nil или < 1 → `insufficient_data` (нет данных → флаг НЕ строится, НЕ ложный not_raised);
//   - участников == 1 И способ закупки ∉ exclude_methods → `raised` (сигнал, требующий проверки);
//   - иначе (участников > 1, либо способ ∈ exclude_methods) → `not_raised`.
//
// Способ сравнивается без учёта регистра и окружающих пробелов (устойчивость к вариативности источника).
func SingleParticipant(in Inputs, params Params) (registry.FlagState, SingleParticipantEvidence) {
	ev := SingleParticipantEvidence{
		ParticipantCount:   in.ParticipantCount,
		ProcurementMethod:  in.ProcurementMethod,
		ExcludeMethods:     params.SingleParticipantExcludeMethods,
		Enabled:            params.SingleParticipantEnabled,
		MethodologyVersion: params.MethodologyVersion,
	}

	if !params.SingleParticipantEnabled {
		return registry.FlagNotPublished, ev
	}
	if in.ParticipantCount == nil || *in.ParticipantCount < 1 {
		return registry.FlagInsufficientData, ev
	}
	if *in.ParticipantCount == 1 && !methodExcluded(in.ProcurementMethod, params.SingleParticipantExcludeMethods) {
		return registry.FlagRaised, ev
	}
	return registry.FlagNotRaised, ev
}

// methodExcluded — способ закупки входит в список законных исключений (без учёта регистра/пробелов).
// Вынесен отдельным предикатом, чтобы поведение «исключён ⇄ не исключён» доказуемо РАЗЛИЧАЛОСЬ тестом
// (страж способен покраснеть). См. [[guards-must-prove-red]].
func methodExcluded(method string, exclude []string) bool {
	m := strings.TrimSpace(method)
	for _, e := range exclude {
		if strings.EqualFold(m, strings.TrimSpace(e)) {
			return true
		}
	}
	return false
}
