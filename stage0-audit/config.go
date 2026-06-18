package main

import "time"

// Config — параметры аудита. Значения с пометкой VERIFY/ASSUMPTION уточняются
// против живого API ows_v2 на первом запуске (см. режим -probe и README).
type Config struct {
	BaseURL           string
	Token             string
	WindowMonths      int
	Sample            int           // максимум записей на эндпоинт (0 = без лимита)
	MaxPages          int           // предохранитель пагинации
	RequestDelay      time.Duration // вежливая пауза между запросами
	MinSample         int           // median.min_sample (дефолт PRD: 5)
	MinGroupContracts int           // flag.monopoly.min_group_contracts (5)
	GeoGate           float64       // 0.70 — Go/No-Go гейт геопривязки
	MonopolyShare     float64       // flag.monopoly.concentration_share (0.5)
	DeviationFactor   float64       // flag.price_per_km.deviation_factor (1.5) — справочно

	// Геокодер (опционально, для реального замера автопокрытия — OQ-4)
	GeocoderKind  string        // "none" | "nominatim"
	GeocoderURL   string        // база геокодера (пусто = публичный Nominatim)
	GeocoderUA    string        // User-Agent (политика Nominatim требует идентификации)
	AstanaViewbox string        // viewbox lon/lat для ограничения по Астане
	GeoSample     int           // сколько объектов геокодировать (лимит)
	GeoDelay      time.Duration // пауза между запросами к геокодеру (политика ~1/сек)
}

// DefaultAstanaViewbox — приблизительный bbox Астаны: lon_left,lat_top,lon_right,lat_bottom.
var DefaultAstanaViewbox = "71.20,51.30,71.78,51.00"

// AstanaKATOPrefixes — VERIFY: префикс КАТО Астаны. Код города Астаны начинается на "71".
var AstanaKATOPrefixes = []string{"71"}

// Ключевые слова направлений (в нижнем регистре) для классификации текста лота.
var RoadKeywords = []string{
	"дорог", "автодорог", "асфальт", "улиц", "тротуар", "магистрал", "проезж",
	"дорожн", "жол", "көше", "автомобиль дорог",
}
var WaterKeywords = []string{
	"водоснаб", "водопровод", "канализац", "водовод", "питьев", "водозабор",
	"сети водоснабжения", "сумен жабдық", "су құбыр", "кәріз",
}

// Маркеры адреса/локации в тексте — прокси для оценки геокодируемости.
var AddressMarkers = []string{
	"ул.", "улиц", "проспект", "пр-т", "пр.", "мкр", "микрорайон", "шоссе",
	"дом ", " д.", "район", "переулок", "квартал", "көше", "даңғыл", "ауданы", "шағын",
}
