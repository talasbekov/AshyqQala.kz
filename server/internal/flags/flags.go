// Package flags — чистые сигнатуры-контракты флагов риска (Story 1.10). Без store/IO; время — ТОЛЬКО
// через clock (не time.Now). РЕАЛЬНЫЕ формулы 4 флагов (FR-19…FR-23) — Epic 4; здесь — каркас контракта,
// чтобы потребители (Epic 4) не приватизировали ядро и не создавали обратных зависимостей.
package flags

import (
	"ashyqqala/server/internal/clock"
	"ashyqqala/server/internal/registry"
)

// Inputs — входы расчёта флага. Now — источник «сейчас» для дата-зависимых флагов (напр. РНУ авто-снятие
// по end_date), инъектируется через clock (детерминизм; go-list-граница «без реального времени»).
// Per-flag поля добавляются по мере реализации флагов (4.2–4.5); zero-value безопасен для каркаса.
type Inputs struct {
	Now clock.Clock

	// FR-19 (Story 4.2): число участников объявления (nil = нет данных → insufficient_data, флаг не строится)
	// и способ закупки. На синтетике (golden-фикстуры); живой источник /v2/trd-buy — Epic 2.
	ParticipantCount  *int
	ProcurementMethod string

	// FR-20 (Story 4.3): цена/км контракта (nil = не вычислима, напр. нет geo_objects.length_km — Epic 3),
	// медиана группы и размер выборки из кэша price_benchmarks (nil median = недостаточно), ключ группы.
	// На синтетике (golden-фикстуры); живые суммы (/v2/contract, Epic 2) + длина (Epic 3).
	PricePerKM       *int64
	GroupMedian      *int64
	GroupSampleSize  int
	ComparabilityKey string // также ключ группы (direction×kato) для монополии FR-21

	// FR-21 (Story 4.4): групповой агрегат монополии. Суммы ₸ топ-поставщика и всей группы (КАТО × направление;
	// nil = нет данных → insufficient_data), число контрактов группы, разрешённость БИН (resolve_status auto →
	// true; manual/conflict → false → флаг по БИН не строится) и сам БИН. На синтетике (golden); живые суммы
	// (/v2/contract) + нормализация (org_name_aliases.resolve_status) — Epic 2.
	SupplierSum         *int64
	GroupTotalSum       *int64
	GroupContracts      int
	SupplierBINResolved bool
	SupplierBIN         string

	// FR-22 (Story 4.5): запись РНУ Подрядчика — даты в unix-секундах (*int64; nil start = битая запись →
	// insufficient_data), ссылки реестра для evidence/атрибуции государству. «Сейчас» берётся через Now (clock)
	// → активность/авто-снятие детерминированы. На синтетике (rnu_entries seed); живой /v2/rnu — Epic 2.
	RNUStartUnix  *int64
	RNUEndUnix    *int64 // nil = открытая запись (без даты окончания)
	RNUGoszakupID string
	RNUSourceURL  string
	RNUReasonRef  string
}

// Params — типизированные пороги методики (Story 4.1), согласованы с render.Params. Наполняются ТОЛЬКО
// из methodology_params (рантайм-загрузчик internal/methodology поверх registry/values/*.yaml) — числовых
// литералов-порогов в коде/прозе быть НЕ должно (AC5а, единственный источник дефолтов — YAML). Эти поля —
// контракт для формул 4 флагов (4.2–4.5) и медиан района (FR-18); сами формулы здесь НЕ реализуются.
type Params struct {
	// MethodologyVersion — каноническая версия методики (формат ^v\d+\.\d+$). Протекает в render.Params/
	// render.Evidence; перекрёстный инвариант: версия в evidence == версия в params (AC5б).
	MethodologyVersion string

	// MinSample — минимальный размер сопоставимой выборки для медианы (data-model §2: 5). ДОЛЖЕН совпадать
	// с median.MinSample (чистое ядро) — рассинхрон ловит перекрёстный тест methodology (single-source).
	MinSample int
	// ComparabilityWindowMonths — скользящее окно сопоставимости (data-model §2: 24 мес) для группы
	// direction×kato. Окно применяется при сборе выборки (фильтр по дате подписания относительно clock).
	ComparabilityWindowMonths int

	// PricePerKMDeviationFactor — порог флага «аномальная цена за км» (FR-20, data-model §2: 1.5).
	// Намеренно простой множитель медианы (×1.5, НЕ MAD) — публичная пересчитываемость. Потребитель — 4.3.
	PricePerKMDeviationFactor float64

	// MonopolyConcentrationShare — доля топ-поставщика для флага монополии (FR-21, data-model §2: 0.5).
	MonopolyConcentrationShare float64
	// MonopolyMinGroupContracts — минимум контрактов в группе для оценки монополии (data-model §2: 5).
	MonopolyMinGroupContracts int

	// SingleParticipantEnabled — kill-switch флага «единственный участник» (FR-19, data-model §2: true).
	SingleParticipantEnabled bool
	// SingleParticipantExcludeMethods — способы закупки, где единственный участник ЗАКОНЕН (НЕ сигналить),
	// напр. закупка из одного источника (FR-19, data-model §2). Потребитель — Story 4.2.
	SingleParticipantExcludeMethods []string

	// RNUEnabled — kill-switch флага «наличие в РНУ» (FR-22, data-model §2: true). Потребитель — Story 4.5.
	RNUEnabled bool
}

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
