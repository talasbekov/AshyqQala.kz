// Package flags — чистые сигнатуры-контракты флагов риска (Story 1.10). Без store/IO; время — ТОЛЬКО
// через clock (не time.Now). РЕАЛЬНЫЕ формулы 4 флагов (FR-19…FR-23) — Epic 4; здесь — каркас контракта,
// чтобы потребители (Epic 4) не приватизировали ядро и не создавали обратных зависимостей.
package flags

import (
	"ashyqqala/server/internal/clock"
	"ashyqqala/server/internal/registry"
)

// Inputs — входы расчёта флага (КАРКАС). Полная модель (число участников, цена/км, доля БИН, РНУ-даты) —
// Epic 4. Now — источник «сейчас» для дата-зависимых флагов (напр. РНУ авто-снятие по end_date),
// инъектируется через clock (детерминизм; go-list-граница «без реального времени»).
type Inputs struct {
	Now clock.Clock
}

// Params — параметры методики (КАРКАС, согласован с render.Params). Иммутабельный движок версий — Story 4.1.
type Params struct{}

// Flag — чистая сигнатура-контракт: входы + параметры → состояние флага (registry.FlagState).
// Каркас: РЕАЛЬНЫЕ формулы 4 флагов — Epic 4. До этого ядро НЕ может оценить флаг → единственное
// честное состояние «недостаточно для оценки» (insufficient_data), а НЕ ложное not_raised («всё чисто»):
// гардрейл честности над домыслом. in.Now/params — задел контракта (время через clock,
// methodology_params), потребляются Epic 4. Без store/IO; время — только через in.Now.
func Flag(in Inputs, params Params) registry.FlagState {
	_ = in     // задел: дата-зависимые флаги берут «сейчас» через in.Now (clock) — Epic 4
	_ = params // задел: methodology_params протекают в сигнатуру; движок — Story 4.1
	return registry.FlagInsufficientData
}
