// Package normalize — нормализация наименований Организаций к каноническому БИН (FR-2, Story 2.3).
// ЧИСТОЕ ЯДРО (AR-13/AR-27): без store/IO/времени — принимает готовый Lexicon (грузит loader, см.
// internal/lexicon). Детерминированно и воспроизводимо вручную (гардрейл §4.1: простые формулы, не статистика).
// NFKC намеренно АППРОКСИМИРОВАН (casefold + фолд гомоглифов + лексикон, без golang.org/x/text) — домовое
// решение, как registry/neutrality.go. Неуверенность ВСЕГДА → manual/conflict (честность над домыслом).
package normalize

import "strings"

// BIN — канонический бизнес-идентификатор организации (12 цифр в проде). Хранится как строка: ведущие
// нули значимы, матч — ТОЧНЫЙ. Тип-контракт ядра.
type BIN string

// Normalize — детерминированное приведение сырой строки: тримминг + схлопывание внутренних пробелов.
// Регистр СОХРАНЯЕТСЯ (для отображаемых наименований). Низкоуровневый помощник; для МАТЧА — CanonicalKey.
func Normalize(raw string) BIN {
	return BIN(strings.Join(strings.Fields(raw), " "))
}

// NormalizeBIN извлекает цифры из сырого значения (убирает пробелы/дефисы/префиксы). Это НИЗКОуровневая
// экстракция; для канонического БИН с проверкой валидности — CanonicalBIN.
func NormalizeBIN(raw string) BIN {
	var b strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return BIN(b.String())
}

// ValidBIN — БИН валиден ⇔ ровно 12 десятичных цифр (data-model: «12 цифр в проде»). NormalizeBIN уже
// оставляет только цифры, так что проверяется по сути длина.
func ValidBIN(b BIN) bool {
	if len(b) != 12 {
		return false
	}
	for _, r := range b {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// CanonicalBIN — канонический БИН из сырого значения: цифры NormalizeBIN, но "" если результат НЕ валиден
// (≠ 12 цифр). Так мусорный/неполный «БИН» (адрес, № договора, обрезок) НЕ порождает фантомную организацию:
// резолв уходит в ветку «имя без БИН» → manual/conflict, а не выдумывает сущность (честность над домыслом).
func CanonicalBIN(raw string) BIN {
	b := NormalizeBIN(raw)
	if !ValidBIN(b) {
		return ""
	}
	return b
}

// Lexicon — кураторский словарь нормализации (FR-2): гомоглифы (лат→кир, анти-обфускация) и синонимы
// (орг-формы/аббревиатуры/падежи/kk-морфология — СЛОВАРЁМ, детерминированно, без морфоанализатора).
// Грузится loader-ом из registry; ядро принимает готовый Lexicon и остаётся чистым.
type Lexicon struct {
	Version    string
	Homoglyphs map[rune]rune     // символ → канон (напр. лат 'o' → кир 'о')
	Synonyms   map[string]string // нормализованное слово/фраза → канон (напр. "товарищество..." → "тоо")
}

// CanonicalKey — нормализованный ключ наименования для МАТЧА (НЕ для отображения): casefold + фолд
// гомоглифов + схлопывание пробелов/пунктуации + лексикон-синонимы (пофразово и пословно).
// Детерминирован и воспроизводим вручную.
func CanonicalKey(rawName string, lex Lexicon) string {
	s := strings.ToLower(strings.TrimSpace(rawName))

	if len(lex.Homoglyphs) > 0 {
		var b strings.Builder
		b.Grow(len(s))
		for _, r := range s {
			if c, ok := lex.Homoglyphs[r]; ok {
				b.WriteRune(c)
			} else {
				b.WriteRune(r)
			}
		}
		s = b.String()
	}

	// Схлопывание пробелов и пунктуации (кавычки/запятые/точки/дефисы) → одиночный пробел.
	fields := strings.FieldsFunc(s, func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', '\r', '"', '«', '»', ',', '.', '-':
			return true
		}
		return false
	})

	// Пофразовый синоним (вся нормализованная фраза целиком) → канон.
	phrase := strings.Join(fields, " ")
	if canon, ok := lex.Synonyms[phrase]; ok {
		phrase = canon
		fields = strings.Fields(phrase)
	}

	// Пословный синоним (аббревиатуры/орг-формы/падежи).
	for i, w := range fields {
		if canon, ok := lex.Synonyms[w]; ok {
			fields[i] = canon
		}
	}
	return strings.Join(fields, " ")
}

// Status — статус резолва наименования к БИН (data-model org_name_aliases.resolve_status).
type Status string

const (
	StatusAuto     Status = "auto"     // однозначно → объединено под один БИН автоматически
	StatusManual   Status = "manual"   // неуверенность → очередь оператору
	StatusConflict Status = "conflict" // «БИН↔наименование» противоречие → изоляция, «требует проверки»
)

// Candidate — известная (уже канонизированная) организация: БИН + нормализованный ключ наименования.
// Строится вызывающим из проекции organizations (CanonicalKey её name_ru/name_kk).
type Candidate struct {
	BIN     BIN
	NameKey string // CanonicalKey известного наименования
}

// Input — сырое появление организации в источнике (БИН + наименование как в дампе).
type Input struct {
	RawBIN  string
	RawName string
}

// Decision — решение резолва: канонический БИН + статус + причина (для evidence/«требует проверки»).
type Decision struct {
	BIN    BIN
	Status Status
	Reason string
}

// Resolve — ЧИСТАЯ детерминированная нормализация одного появления к каноническому БИН против известных
// кандидатов. БИН — точный матч ВАЛИДНОГО (12-значного) БИН; наименование — CanonicalKey. Решение:
//   - auto:     валидный БИН без конфликта наименования (объединение вариантов написания под один БИН).
//   - conflict: «БИН↔наименование» — наименование уже закреплено за ДРУГИМ БИН; ИЛИ без БИН → ≥2 разных БИН.
//   - manual:   наименование без валидного БИН (совпало с ≤1 кандидатом) — оператор подтверждает; без БИН
//     авто-объединение = домысел (даже единственное совпадение по имени может быть другой орг
//     с генеричным/гомоглиф-именем), поэтому в очередь, а не auto. [decision code-review 2026-06-27]
//
// Никакого автослияния-домысла: неуверенность всегда → manual/conflict (честность над домыслом, §4.1).
// Выход детерминирован (не зависит от порядка обхода existing).
func Resolve(in Input, existing []Candidate, lex Lexicon) Decision {
	bin := CanonicalBIN(in.RawBIN) // "" если не валидный 12-значный БИН → ветка «без БИН»
	nameKey := CanonicalKey(in.RawName, lex)

	// Множество БИН, под которыми уже встречалось это нормализованное наименование.
	binsForName := map[BIN]bool{}
	if nameKey != "" {
		for _, c := range existing {
			if c.NameKey == nameKey {
				binsForName[c.BIN] = true
			}
		}
	}

	if bin != "" {
		// Конфликт, если это наименование уже закреплено за ДРУГИМ БИН.
		for b := range binsForName {
			if b != bin {
				return Decision{BIN: bin, Status: StatusConflict, Reason: "наименование уже закреплено за другим БИН"}
			}
		}
		return Decision{BIN: bin, Status: StatusAuto, Reason: "точный БИН без конфликта наименования"}
	}

	// БИН отсутствует/невалиден — резолв по наименованию. Авто-объединение БЕЗ БИН не делаем (домысел):
	// ≥2 разных БИН → conflict; иначе (0 или 1 совпадение) → manual-очередь оператору.
	if len(binsForName) >= 2 {
		return Decision{Status: StatusConflict, Reason: "наименование без БИН совпало с несколькими БИН"}
	}
	if len(binsForName) == 1 {
		return Decision{Status: StatusManual, Reason: "наименование без БИН совпало с одним кандидатом — требует подтверждения оператором"}
	}
	return Decision{Status: StatusManual, Reason: "наименование без БИН не совпало ни с одной известной организацией"}
}
