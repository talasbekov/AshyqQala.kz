// Package orgnorm — оркестрация нормализации наименований→БИН (FR-2, Story 2.3): декодированные организации
// → ЧИСТОЕ планирование (normalize.Resolve: auto/manual/conflict) → запись в проекцию organizations +
// кураторские org_name_aliases. НЕ импортирует goszakup напрямую (AR-27: только ingest/decode) — работает
// с уже декодированными организациями; шаг file/ows→decode делает вызывающий (cmd/importer или тест).
package orgnorm

import (
	"ashyqqala/server/internal/ingest/decode"
	"ashyqqala/server/internal/normalize"
	"ashyqqala/server/internal/store/gen"
)

// Appearance — одно появление организации в источнике с пометкой источника (contract/trd-buy).
type Appearance struct {
	Org    decode.Organization
	Source string
}

// AliasDecision — решение по одному вариативному написанию: канонический БИН ("" если не разрешён) + статус.
type AliasDecision struct {
	RawName string
	Source  string
	BIN     normalize.BIN
	Status  normalize.Status
}

// Plan — результат ЧИСТОГО планирования (без БД): организации к UPSERT (дедуп по БИН) + решения по псевдонимам.
type Plan struct {
	Orgs    []decode.Organization
	Aliases []AliasDecision
}

// BuildCandidates строит кандидатов матча из проекции organizations (CanonicalKey её name_ru/name_kk).
func BuildCandidates(orgs []gen.Organization, lex normalize.Lexicon) []normalize.Candidate {
	var out []normalize.Candidate
	for _, o := range orgs {
		bin := normalize.BIN(o.Bin)
		if o.NameRu.Valid && o.NameRu.String != "" {
			out = append(out, normalize.Candidate{BIN: bin, NameKey: normalize.CanonicalKey(o.NameRu.String, lex)})
		}
		if o.NameKk.Valid && o.NameKk.String != "" {
			out = append(out, normalize.Candidate{BIN: bin, NameKey: normalize.CanonicalKey(o.NameKk.String, lex)})
		}
	}
	return out
}

// candidatesFromBatch — кандидаты из самого батча: организация с БИН+именем устанавливает связь имя↔БИН
// (так конфликт «одно имя → разные БИН» детектируется и внутри одного импорта).
func candidatesFromBatch(apps []Appearance, lex normalize.Lexicon) []normalize.Candidate {
	var out []normalize.Candidate
	for _, a := range apps {
		bin := normalize.CanonicalBIN(a.Org.BIN) // "" если не валидный 12-значный БИН
		if bin == "" {
			continue
		}
		if a.Org.NameRu != "" {
			out = append(out, normalize.Candidate{BIN: bin, NameKey: normalize.CanonicalKey(a.Org.NameRu, lex)})
		}
		if a.Org.NameKk != "" {
			out = append(out, normalize.Candidate{BIN: bin, NameKey: normalize.CanonicalKey(a.Org.NameKk, lex)})
		}
	}
	return out
}

// PlanNormalization — ЧИСТОЕ детерминированное планирование: дедуп организаций по БИН (роли OR-ятся, имена —
// первое непустое) + решение по каждому вариативному написанию (resolve против existing ∪ батч).
func PlanNormalization(apps []Appearance, existing []normalize.Candidate, lex normalize.Lexicon) Plan {
	cands := append(append([]normalize.Candidate(nil), existing...), candidatesFromBatch(apps, lex)...)

	// Дедуп организаций по каноническому БИН.
	byBIN := map[string]*decode.Organization{}
	var order []string
	for _, a := range apps {
		bin := string(normalize.CanonicalBIN(a.Org.BIN)) // "" если не валидный 12-значный БИН
		if bin == "" {
			continue // появление без валидного БИН не регистрирует организацию (только псевдоним → manual/conflict)
		}
		if cur, ok := byBIN[bin]; ok {
			cur.IsCustomer = cur.IsCustomer || a.Org.IsCustomer
			cur.IsSupplier = cur.IsSupplier || a.Org.IsSupplier
			if cur.NameRu == "" {
				cur.NameRu = a.Org.NameRu
			}
			if cur.NameKk == "" {
				cur.NameKk = a.Org.NameKk
			}
			if cur.RegKato == "" {
				cur.RegKato = a.Org.RegKato
			}
			if cur.SourceURL == "" {
				cur.SourceURL = a.Org.SourceURL
			}
		} else {
			o := a.Org
			o.BIN = bin // канонический БИН (только цифры)
			byBIN[bin] = &o
			order = append(order, bin)
		}
	}
	plan := Plan{}
	for _, bin := range order {
		plan.Orgs = append(plan.Orgs, *byBIN[bin])
	}

	// Решение по каждому вариативному написанию (дедуп по (raw_name, source)). Имя берём ru, иначе kk —
	// kk-only появление (пропущен ru) тоже попадает в очередь нормализации, а не теряется молча.
	seen := map[[2]string]bool{}
	for _, a := range apps {
		name := a.Org.NameRu
		if name == "" {
			name = a.Org.NameKk
		}
		raw := string(normalize.Normalize(name)) // схлопнутые пробелы, регистр сохранён
		if raw == "" {
			continue // нет наименования вовсе — нечего нормализовать (организация заведётся по БИН)
		}
		key := [2]string{raw, a.Source}
		if seen[key] {
			continue
		}
		seen[key] = true
		dec := normalize.Resolve(normalize.Input{RawBIN: a.Org.BIN, RawName: name}, cands, lex)
		plan.Aliases = append(plan.Aliases, AliasDecision{RawName: raw, Source: a.Source, BIN: dec.BIN, Status: dec.Status})
	}
	return plan
}
