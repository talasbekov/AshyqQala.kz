// Package decode — единая граница декодирования ows_v2→домен + schema_hash (Story 1.10, AR-11).
// Политика инвертирована относительно stage0-audit: аудит=тихо → прод=стоп+алерт при дрейфе
// зафиксированного контракта схемы (НЕ тихий 0). Концепт fieldCandidates перенесён из stage0
// (другой Go-модуль — импорт запрещён). Живой ows_v2-декод и реальный snapshot B-1 — Story 2.1.
package decode

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
)

// SchemaHash — стабильный хеш НАБОРА имён полей объекта (key-sorted): дрейф схемы детектируется,
// а перестановка полей — нет («иначе хеш мигает», architecture.md:349).
func SchemaHash(fields []string) string {
	s := append([]string(nil), fields...)
	slices.Sort(s)
	h := sha256.New()
	for _, f := range s {
		h.Write([]byte(f))
		h.Write([]byte{0}) // разделитель: ["ab","c"] != ["a","bc"]
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Contract — доменная проекция объекта ows_v2 (КАРКАС S-0; полная доменная модель — Epic 2).
type Contract struct {
	GoszakupContractID string
	AmountTng          *int64
}

// fieldCandidates — логическое поле → кандидаты реальных имён ows_v2 (первый присутствующий побеждает).
// Перенос концепта stage0-audit/models.go (импорт того модуля запрещён).
var fieldCandidates = map[string][]string{
	"goszakup_contract_id": {"contract_id", "id"},
	"amount_tng":           {"contract_sum_wnds", "contract_sum", "sum"},
}

// ErrSchemaDrift — дрейф зафиксированного контракта схемы: detect-and-halt (стоп+алерт), НЕ тихий 0.
type ErrSchemaDrift struct {
	Expected string
	Got      string
}

func (e ErrSchemaDrift) Error() string {
	return fmt.Sprintf("decode: дрейф схемы — schema_hash %s ≠ зафиксированный %s; стоп+алерт (не тихий 0)", e.Got, e.Expected)
}

// Decode декодирует объект ows_v2 в домен ТОЛЬКО при совпадении schema_hash с зафиксированным
// контрактом. При дрейфе (набор полей изменился) → ErrSchemaDrift, а не молчаливый 0/"".
func Decode(raw map[string]any, pinnedSchemaHash string) (Contract, error) {
	got := SchemaHash(keysOf(raw))
	if got != pinnedSchemaHash {
		return Contract{}, ErrSchemaDrift{Expected: pinnedSchemaHash, Got: got}
	}
	var c Contract
	c.GoszakupContractID = pickString(raw, "goszakup_contract_id")
	if n, ok := pickInt(raw, "amount_tng"); ok {
		c.AmountTng = &n
	}
	return c, nil
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func pick(m map[string]any, logical string) (any, bool) {
	for _, k := range fieldCandidates[logical] {
		if v, ok := m[k]; ok && v != nil {
			return v, true
		}
	}
	return nil, false
}

func pickString(m map[string]any, logical string) string {
	if v, ok := pick(m, logical); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func pickInt(m map[string]any, logical string) (int64, bool) {
	if v, ok := pick(m, logical); ok {
		switch n := v.(type) {
		case float64: // JSON-числа после json.Unmarshal
			return int64(n), true
		case int64:
			return n, true
		case int:
			return int64(n), true
		}
	}
	return 0, false
}
