package decode

import "ashyqqala/server/internal/goszakup"

// Organization — доменная проекция организации (заказчик/подрядчик/участник). В ows_v2 НЕТ отдельного
// ресурса организаций: они извлекаются из contract (заказчик+поставщик) и trd-buy (участники). БИН —
// строка (ведущие нули значимы; точный матч). Каркас S-0: имена реальных полей ows VERIFY-маркированы
// (file-фикстура задаёт форму) — value-robustness для живого ows (числа строками и т.п.) → Story 2.1/2.2.
type Organization struct {
	BIN        string
	NameRu     string
	NameKk     string
	RegKato    string
	IsCustomer bool
	IsSupplier bool
	SourceURL  string
}

// orgFieldCandidates — логическое поле → кандидаты реальных имён ows_v2 (первый присутствующий побеждает).
// Префиксы customer_/supplier_/participant_ — две организации в одной записи contract и одна в trd-buy.
var orgFieldCandidates = map[string][]string{
	"customer_bin":     {"customer_bin", "customer_biniin"},
	"customer_name_ru": {"customer_name_ru", "customer_name"},
	"customer_name_kk": {"customer_name_kk"},
	"customer_kato":    {"customer_kato", "ref_kato"},

	"supplier_bin":     {"supplier_bin", "supplier_biniin"},
	"supplier_name_ru": {"supplier_name_ru", "supplier_name"},
	"supplier_name_kk": {"supplier_name_kk"},
	"supplier_kato":    {"supplier_kato"},

	"participant_bin":     {"participant_bin", "bin", "biniin"},
	"participant_name_ru": {"participant_name_ru", "name_ru", "name"},
	"participant_name_kk": {"participant_name_kk", "name_kk"},
	"participant_kato":    {"participant_kato", "kato"},
	"is_winner":           {"is_winner", "winner"},
}

// DecodeOrgsFromContracts тянет записи contract из источника (file/ows) и извлекает по ДВЕ организации
// (заказчик + подрядчик) из каждой при совпадении schema_hash; дрейф → ErrSchemaDrift (стоп, не тихий 0).
// `decode` — ЕДИНСТВЕННЫЙ импортёр goszakup (AR-27). Источник file — без токена; ows — Story 2.1/2.2.
func DecodeOrgsFromContracts(src goszakup.Source, pinnedSchemaHash string, max int) ([]Organization, error) {
	var out []Organization
	err := src.Fetch(string(goszakup.ResourceContract), nil, max, func(items []map[string]any) error {
		for _, raw := range items {
			if got := SchemaHash(keysOf(raw)); got != pinnedSchemaHash {
				return ErrSchemaDrift{Expected: pinnedSchemaHash, Got: got}
			}
			if cust := orgFromContract(raw, "customer", true, false); hasOrg(cust) {
				out = append(out, cust)
			}
			if supp := orgFromContract(raw, "supplier", false, true); hasOrg(supp) {
				out = append(out, supp)
			}
		}
		return nil
	})
	return out, err
}

// DecodeOrgsFromTrdBuy тянет записи trd-buy (участники объявлений) и извлекает организацию-участника из
// каждой при совпадении schema_hash; победитель помечается подрядчиком (is_supplier). Дрейф → ErrSchemaDrift.
func DecodeOrgsFromTrdBuy(src goszakup.Source, pinnedSchemaHash string, max int) ([]Organization, error) {
	var out []Organization
	err := src.Fetch(string(goszakup.ResourceTrdBuy), nil, max, func(items []map[string]any) error {
		for _, raw := range items {
			if got := SchemaHash(keysOf(raw)); got != pinnedSchemaHash {
				return ErrSchemaDrift{Expected: pinnedSchemaHash, Got: got}
			}
			o := Organization{
				BIN:        orgString(raw, "participant_bin"),
				NameRu:     orgString(raw, "participant_name_ru"),
				NameKk:     orgString(raw, "participant_name_kk"),
				RegKato:    orgString(raw, "participant_kato"),
				IsSupplier: orgBool(raw, "is_winner"), // победитель = подрядчик; прочие участники — просто известная орг
			}
			if hasOrg(o) {
				out = append(out, o)
			}
		}
		return nil
	})
	return out, err
}

// hasOrg — появление пригодно для нормализации, если есть БИН ЛИБО наименование. Появление БЕЗ БИН, но
// с именем (пропущенный БИН в источнике) НЕ регистрирует проекционную организацию, но порождает псевдоним
// в очередь оператору (manual) — это и есть кейс FR-2 «неоднозначно/не уверены → ручная разметка».
func hasOrg(o Organization) bool {
	return o.BIN != "" || o.NameRu != "" || o.NameKk != ""
}

// orgFromContract извлекает одну организацию (заказчик ИЛИ подрядчик) из записи contract по префиксу.
func orgFromContract(raw map[string]any, prefix string, isCustomer, isSupplier bool) Organization {
	return Organization{
		BIN:        orgString(raw, prefix+"_bin"),
		NameRu:     orgString(raw, prefix+"_name_ru"),
		NameKk:     orgString(raw, prefix+"_name_kk"),
		RegKato:    orgString(raw, prefix+"_kato"),
		IsCustomer: isCustomer,
		IsSupplier: isSupplier,
	}
}

func orgPick(raw map[string]any, logical string) (any, bool) {
	for _, k := range orgFieldCandidates[logical] {
		if v, ok := raw[k]; ok && v != nil {
			return v, true
		}
	}
	return nil, false
}

func orgString(raw map[string]any, logical string) string {
	if v, ok := orgPick(raw, logical); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func orgBool(raw map[string]any, logical string) bool {
	if v, ok := orgPick(raw, logical); ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}
