package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"ashyqqala/server/internal/apierr"
	"ashyqqala/server/internal/benchmark"
	"ashyqqala/server/internal/clock"
	"ashyqqala/server/internal/district"
	"ashyqqala/server/internal/registry"
	"ashyqqala/server/internal/store/gen"
)

// districtObjectsLimit — bounded-срез списка объектов района на странице. Агрегаты считаются по ВСЕМ
// объектам (отдельные запросы), список — лишь отображаемый кусок (FR-17 AC1, прецедент ContractorCard).
const districtObjectsLimit = 50

// container_state — ТРЕТЬЯ ось честности (AR-17): закрытый набор для района/списка. НЕ в Go-реестре
// honest_states (там value_state/flag_state, оси 1–2); канон значений — architecture (Story 0.8 прецедент:
// ввести как закрытый набор на проводе). Значения co-occur (например no_flags_raised + not_geocoded) → массив.
//   - no_contracts  — в районе нет объектов;
//   - no_flags_raised — объекты есть, активных сигналов нет (НЕ «всё чисто» — отдельной строкой);
//   - not_geocoded  — объекты учтены по КАТО, но точек на карте пока нет (гео контракта — Epic 3).
const (
	containerNoContracts = "no_contracts"
	containerNoFlags     = "no_flags_raised"
	containerNotGeocoded = "not_geocoded"
)

// DistrictFlagCountDTO — один тип активного флага района и его количество (разбивка «Сигналы района»).
type DistrictFlagCountDTO struct {
	FlagType string `json:"flag_type"`
	Count    string `json:"count"` // целое строкой
}

// DistrictObjectDTO — объект района в bounded-списке (форма ≈ ContractListItem). Деньги/nullable — конверт.
type DistrictObjectDTO struct {
	GoszakupContractID string        `json:"goszakup_contract_id"`
	SubjectRu          Field[string] `json:"subject_ru"`
	SubjectKk          Field[string] `json:"subject_kk"`
	AmountTng          Field[string] `json:"amount_tng"`
	KatoCode           Field[string] `json:"kato_code"`
	Direction          Field[string] `json:"direction"`
	HasActiveFlag      bool          `json:"has_active_flag"`
}

// DistrictDirectionMedianDTO — медиана ₸/км «район vs город» по ОДНОМУ направлению (FR-18, Story 6.4).
// Медианы — честный конверт: ok с целым ₸/км строкой, иначе состояние (insufficient_sample/not_comparable;
// сейчас всё not_comparable — нет length_km, Epic 3). ComparisonPct ok ТОЛЬКО при обеих медианах ok.
// *SampleSize — размер сопоставимой выборки целым строкой; *Key — ключ пересчёта (evidence на проводе, AC1).
type DistrictDirectionMedianDTO struct {
	Direction          string        `json:"direction"`
	DistrictMedianTng  Field[string] `json:"district_median_tng"`
	CityMedianTng      Field[string] `json:"city_median_tng"`
	ComparisonPct      Field[string] `json:"comparison_pct"`
	DistrictSampleSize string        `json:"district_sample_size"`
	CitySampleSize     string        `json:"city_sample_size"`
	DistrictKey        string        `json:"district_comparability_key"`
	CityKey            string        `json:"city_comparability_key"`
}

// DistrictDTO — wire-форма сводки района (FR-17). name_* честно no_data, пока КАТО-код района не подтверждён
// (Story 0.1; коды не выдумываем). Счётчики строкой; 0 — честный ноль (отражён в container_state, не «нет данных»).
type DistrictDTO struct {
	Kato                     string                       `json:"kato"`
	NameRu                   Field[string]                `json:"name_ru"`
	NameKk                   Field[string]                `json:"name_kk"`
	ContractCount            string                       `json:"contract_count"`
	TotalAmountTng           Field[string]                `json:"total_amount_tng"`
	ActiveFlagsCount         string                       `json:"active_flags_count"`
	FlagsByType              []DistrictFlagCountDTO       `json:"flags_by_type"`
	ContainerState           []string                     `json:"container_state"`
	Objects                  []DistrictObjectDTO          `json:"objects"`
	Medians                  []DistrictDirectionMedianDTO `json:"medians"`
	MedianMethodologyVersion string                       `json:"median_methodology_version"`
}

// DistrictStore — что хендлеру нужно от слоя данных. Запросы 6.3 реализует *gen.Queries; ₸/км-шов 6.4
// (PricePerKMSamples) — адаптер NewDistrictStore (gen не выражает ₸/км — нужен geo_objects.length_km, Epic 3).
// Мокается в тестах.
type DistrictStore interface {
	DistrictAggregates(ctx context.Context, katoPrefix string) (gen.DistrictAggregatesRow, error)
	DistrictActiveFlagsByType(ctx context.Context, katoPrefix string) ([]gen.DistrictActiveFlagsByTypeRow, error)
	ListContractsByDistrict(ctx context.Context, arg gen.ListContractsByDistrictParams) ([]gen.ListContractsByDistrictRow, error)
	// PricePerKMSamples — ₸/км-выборка группы сопоставимости (направление × КАТО-префикс) для медианы
	// района/города (FR-18). computable=false → ₸/км СТРУКТУРНО невычислима (нет length_km) → not_comparable.
	// Окно (24 мес) режет ЧИСТОЕ ядро по дате — метод отдаёт ВСЕ samples группы (детерминизм/тестируемость).
	PricePerKMSamples(ctx context.Context, direction, katoPrefix string) (samples []benchmark.Sample, computable bool, err error)
}

// districtStore — адаптер слоя данных страницы района: промотанные запросы 6.3 (*gen.Queries) + ₸/км-шов 6.4.
type districtStore struct {
	*gen.Queries
}

// NewDistrictStore оборачивает gen.Queries адаптером района (запросы 6.3 + ₸/км-шов 6.4 PricePerKMSamples).
func NewDistrictStore(q *gen.Queries) DistrictStore { return districtStore{Queries: q} }

// PricePerKMSamples — ШОВ сбора ₸/км-выборки группы (Story 6.4, FR-18), НАПОЛНЕН Story 3.1. Теперь
// `geo_objects.length_km` СУЩЕСТВУЕТ (миграция 0020) → цена/км = `amount_tng / length_km` СТРУКТУРНО вычислима.
// Запрос JOIN'ит geo_objects (direction × КАТО-префикс, length_km > 0) → возвращает (samples, true): база
// измерима (computable=true), а `benchmark.GroupMedian` решает ok-с-медианой vs insufficient_sample по размеру
// выборки. not_comparable теперь только на ошибке запроса — структурно база есть. Форма DTO/хендлера/тестов
// НЕ изменилась (это и есть шов 6.4). [Source: queries/districts.sql PricePerKMSamplesByDirection; benchmark.Sample]
func (s districtStore) PricePerKMSamples(ctx context.Context, direction, katoPrefix string) ([]benchmark.Sample, bool, error) {
	rows, err := s.PricePerKMSamplesByDirection(ctx, gen.PricePerKMSamplesByDirectionParams{
		Direction:  direction,
		KatoPrefix: katoPrefix,
	})
	if err != nil {
		return nil, false, err
	}
	samples := make([]benchmark.Sample, 0, len(rows))
	for _, r := range rows {
		samples = append(samples, benchmark.Sample{PricePerKM: r.PricePerKm, SignDateUnix: r.SignDateUnix})
	}
	return samples, true, nil
}

// medianDirections — направления MVP сравнения медиан (FR-18 scope: дорога и водоснабжение; «other» вне
// сравнения). Детерминированный набор → медиана показывается по каждому направлению независимо, всегда.
var medianDirections = []string{"road", "water"}

// cityKato — КАТО-префикс города Астаны для городской медианы (D2; architecture.md:175, astana_districts.json).
const cityKato = "71"

// DistrictsHandler — хендлер страницы района (Story 6.3 FR-17 + 6.4 FR-18 медиана). Window/MethodologyVersion —
// из methodology_params (НЕ хардкод порога/окна/версии). Clock — источник «сейчас» для окна медианы (тесты — Fixed).
type DistrictsHandler struct {
	Store              DistrictStore
	Districts          *district.Catalog // имя района по подтверждённому КАТО-коду; nil → имя всегда no_data
	Window             int               // ComparabilityWindowMonths из methodology_params (окно медианы FR-18)
	MethodologyVersion string            // каноническая methodology_version (фиксация на поверхности, AC4)
	Clock              clock.Clock       // «сейчас» для окна медианы; nil → clock.Real (прод)
	Log                *slog.Logger
}

// clk — clock с дефолтом на реальные часы (прод), Fixed подставляют тесты.
func (h DistrictsHandler) clk() clock.Clock {
	if h.Clock != nil {
		return h.Clock
	}
	return clock.Real{}
}

func (h DistrictsHandler) logErr(event string, err error, kato string) {
	if h.Log != nil {
		h.Log.Error(event, "error", err.Error(), "kato", kato)
	}
}

// Get обслуживает GET /api/districts/{kato}.
func (h DistrictsHandler) Get(w http.ResponseWriter, r *http.Request) {
	kato := chi.URLParam(r, "kato")
	// Невалидный КАТО (не цифры 2..11) → 400. Неизвестный-но-валидный КАТО НЕ 404: нет авторитетного списка
	// районов для отказа (коды не подтверждены) → честные пустые агрегаты (container_state no_contracts).
	if !district.ValidKATO(kato) {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidationFailed, "invalid kato code")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// kato — только цифры (валидировано) → префикс безопасен (нет LIKE-метасимволов %/_/\).
	prefix := kato + "%"

	agg, err := h.Store.DistrictAggregates(ctx, prefix)
	if err != nil {
		h.logErr("district_aggregates_failed", err, kato)
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, "internal error")
		return
	}
	flagRows, err := h.Store.DistrictActiveFlagsByType(ctx, prefix)
	if err != nil {
		h.logErr("district_flags_failed", err, kato)
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, "internal error")
		return
	}
	objRows, err := h.Store.ListContractsByDistrict(ctx, gen.ListContractsByDistrictParams{KatoPrefix: prefix, Lim: districtObjectsLimit})
	if err != nil {
		h.logErr("district_objects_failed", err, kato)
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, "internal error")
		return
	}
	medians, err := h.medianBlock(ctx, kato)
	if err != nil {
		h.logErr("district_median_failed", err, kato)
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, h.project(kato, agg, flagRows, objRows, medians))
}

// medianBlock считает медиану ₸/км «район vs город» по каждому направлению MVP (FR-18, Story 6.4).
// compute-on-read (D1): выборки собираются по группе (направление × КАТО-префикс), окно × порог применяет
// ЧИСТОЕ ядро benchmark.Evaluate. Район = префикс из URL; город = префикс «71» (D2). Дельта показывается
// только когда обе медианы ok. Сейчас всё not_comparable (нет length_km, Epic 3) — честный skip.
func (h DistrictsHandler) medianBlock(ctx context.Context, kato string) ([]DistrictDirectionMedianDTO, error) {
	districtPrefix := kato + "%"
	cityPrefix := cityKato + "%"
	out := make([]DistrictDirectionMedianDTO, 0, len(medianDirections))
	for _, dir := range medianDirections {
		dSamples, dComputable, err := h.Store.PricePerKMSamples(ctx, dir, districtPrefix)
		if err != nil {
			return nil, err
		}
		cSamples, cComputable, err := h.Store.PricePerKMSamples(ctx, dir, cityPrefix)
		if err != nil {
			return nil, err
		}
		dOut := benchmark.Evaluate(dSamples, dComputable, h.Window, h.clk())
		cOut := benchmark.Evaluate(cSamples, cComputable, h.Window, h.clk())
		cmp := benchmark.CompareGroups(dOut, cOut)
		out = append(out, DistrictDirectionMedianDTO{
			Direction:          dir,
			DistrictMedianTng:  medianField(dOut),
			CityMedianTng:      medianField(cOut),
			ComparisonPct:      pctField(cmp),
			DistrictSampleSize: strconv.Itoa(dOut.N),
			CitySampleSize:     strconv.Itoa(cOut.N),
			DistrictKey:        benchmark.Group{Direction: dir, Kato: kato}.Key(),
			CityKey:            benchmark.Group{Direction: dir, Kato: cityKato}.Key(),
		})
	}
	return out, nil
}

// medianField — Outcome → честный конверт ₸/км: ok с целым строкой (медиана достоверна), иначе value:null +
// честное состояние (insufficient_sample <5 / not_comparable — нет измеримой базы ₸/км). Никогда 0/NaN.
func medianField(o benchmark.Outcome) Field[string] {
	if o.State == registry.StateOK && o.Median != nil {
		return okField(strconv.FormatInt(*o.Median, 10))
	}
	return Field[string]{State: o.State}
}

// pctField — % дельта на проводе: ok с целым (со знаком) строкой ТОЛЬКО когда обе медианы ok; иначе
// not_comparable (нечего сравнивать — value:null, не «0%»).
func pctField(c benchmark.Comparison) Field[string] {
	if c.DeltaShown {
		return okField(strconv.FormatInt(c.DeltaPercent, 10))
	}
	return Field[string]{State: registry.StateNotComparable}
}

// project собирает честный DistrictDTO из строк стора + предрасчитанного median-блока (FR-18, 6.4).
func (h DistrictsHandler) project(kato string, agg gen.DistrictAggregatesRow, flagRows []gen.DistrictActiveFlagsByTypeRow, objRows []gen.ListContractsByDistrictRow, medians []DistrictDirectionMedianDTO) DistrictDTO {
	dto := DistrictDTO{
		Kato:                     kato,
		NameRu:                   noData[string](),
		NameKk:                   noData[string](),
		ContractCount:            strconv.FormatInt(agg.ContractCount, 10),
		Medians:                  medians,
		MedianMethodologyVersion: h.MethodologyVersion,
	}

	// Имя района: ТОЛЬКО если КАТО-код района подтверждён (Story 0.1). Иначе честный no_data — не выдумываем.
	if h.Districts != nil {
		if kk, ru, ok := h.Districts.NameByKATO(kato); ok {
			dto.NameKk = okField(kk)
			dto.NameRu = okField(ru)
		}
	}

	// Сумма: честный no_data, если нет ни одной известной суммы (контракты могут быть, а сумм нет) — НЕ «0 ₸».
	if agg.AmountKnownCount > 0 {
		dto.TotalAmountTng = okField(strconv.FormatInt(agg.TotalAmountTng, 10))
	} else {
		dto.TotalAmountTng = noData[string]()
	}

	// Активные флаги: разбивка по типам + total. Порядок из SQL (по flag_type) сохраняется.
	dto.FlagsByType = make([]DistrictFlagCountDTO, 0, len(flagRows))
	var activeTotal int64
	for _, fr := range flagRows {
		dto.FlagsByType = append(dto.FlagsByType, DistrictFlagCountDTO{
			FlagType: fr.FlagType,
			Count:    strconv.FormatInt(fr.N, 10),
		})
		activeTotal += fr.N
	}
	dto.ActiveFlagsCount = strconv.FormatInt(activeTotal, 10)

	// container_state (AR-17), отдельной строкой; значения co-occur → массив (никогда nil).
	cs := make([]string, 0, 2)
	if agg.ContractCount == 0 {
		cs = append(cs, containerNoContracts)
	} else {
		if activeTotal == 0 {
			cs = append(cs, containerNoFlags)
		}
		// Контракт-уровневой геопривязки нет до Epic 3 → объекты учтены по КАТО, но точек на карте нет.
		cs = append(cs, containerNotGeocoded)
	}
	dto.ContainerState = cs

	dto.Objects = make([]DistrictObjectDTO, 0, len(objRows))
	for _, o := range objRows {
		dto.Objects = append(dto.Objects, DistrictObjectDTO{
			GoszakupContractID: o.GoszakupContractID,
			SubjectRu:          fromText(o.SubjectRu),
			SubjectKk:          fromText(o.SubjectKk),
			AmountTng:          fromInt8String(o.AmountTng),
			KatoCode:           fromText(o.KatoCode),
			Direction:          fromText(o.Direction),
			HasActiveFlag:      o.HasActiveFlag,
		})
	}

	return dto
}
