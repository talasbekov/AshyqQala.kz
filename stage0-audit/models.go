package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// decodeItems разбирает массив "items" в обобщённые записи (устойчиво к схеме).
func decodeItems(raw json.RawMessage) ([]map[string]any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	return items, nil
}

// fieldCandidates — логическое поле → кандидаты реальных имён API (первое непустое побеждает).
// VERIFY против вывода режима -probe и поправить здесь, если имена другие.
var fieldCandidates = map[string][]string{
	"contract_sum":  {"contract_sum_wnds", "contract_sum", "faktSumWnds", "summ", "sum"},
	"supplier_biin": {"supplier_biin", "supplier_bin", "biin", "bin"},
	"customer_bin":  {"customer_bin", "cust_bin", "org_bin"},
	"sign_date":     {"sign_date", "signed", "crdate", "date_signed", "publish_date"},
	"status":        {"ref_contract_status_id", "ref_buy_status_id", "status"},
	"name":          {"name_ru", "lot_name_ru", "descr_ru", "name"},
	"amount":        {"amount", "summ", "sum", "total_sum"},
	"kato":          {"ref_kato", "kato", "kato_code", "delivery_kato", "ref_kato_id"},
	"anno":          {"trd_buy_number_anno", "number_anno", "ref_trd_buy_number_anno"},
	"rnu_bin":       {"supplier_biin", "biin", "bin", "pid_biin"},
	"rnu_start":     {"start_date", "date_start", "rnu_start"},
	"rnu_end":       {"end_date", "date_end", "rnu_end"},
	"participants":  {"count", "cnt_supplier", "offers_count", "participant_count"},
}

func getField(rec map[string]any, logical string) (any, string, bool) {
	for _, k := range fieldCandidates[logical] {
		if v, ok := rec[k]; ok && v != nil {
			return v, k, true
		}
	}
	return nil, "", false
}

func getString(rec map[string]any, logical string) string {
	v, _, ok := getField(rec, logical)
	if !ok {
		return ""
	}
	return toString(v)
}

func getFloat(rec map[string]any, logical string) (float64, bool) {
	v, _, ok := getField(rec, logical)
	if !ok {
		return 0, false
	}
	return toFloat(v)
}

func toFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f, err == nil
	}
	return 0, false
}

func toString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	case bool:
		return strconv.FormatBool(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// recordKeys — отсортированные ключи записи (для schema-probe).
func recordKeys(rec map[string]any) []string {
	keys := make([]string, 0, len(rec))
	for k := range rec {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
