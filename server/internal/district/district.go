// Package district — рантайм-загрузчик реестра районов Астаны (FR-17, Story 6.3). Источник истины —
// registry/values/astana_districts.json (человек правит ТОЛЬКО там; читается в рантайме — принцип AR-12,
// как registry/lexicon/methodology). Честно падает (честность над домыслом): отсутствующий/битый файл,
// невалидная версия, пустой slug/имя, дубль slug, нецифровой kato → явная ошибка на старте, НЕ тихий
// пустой реестр.
//
// КАТО-коды районов в S-0 = null (их подтверждает Story 0.1 из живого API/классификатора; гардрейл
// stage0-access.md §3 «коды не выдумываем»). Пока kato=null — NameByKATO не находит имя (честный no_data),
// но агрегация по КАТО-префиксу работает (район определяется КАТО из URL, не реестром).
package district

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// DistrictsFile — имя текущей версии реестра в registry/values. Новая версия = НОВЫЙ файл (vN+1).
const DistrictsFile = "astana_districts.json"

// versionRe — формат version реестра: vN (например v1).
var versionRe = regexp.MustCompile(`^v\d+$`)

// katoRe — КАТО-код/префикс из URL: только цифры, 2..11 знаков. Цифры-онли гарантируют отсутствие
// LIKE-метасимволов (%/_/\) → безопасный префикс-матч в SQL без экранирования.
var katoRe = regexp.MustCompile(`^[0-9]{2,11}$`)

// District — один район Астаны. Kato=nil пока код не подтверждён (Story 0.1); коды НЕ выдумываем.
type District struct {
	Slug   string  `json:"slug"`
	NameKk string  `json:"name_kk"`
	NameRu string  `json:"name_ru"`
	Kato   *string `json:"kato"`
}

// fileShape — форма JSON-источника. `_comment` объявлен явно: DisallowUnknownFields ловит опечатки в
// реальных ключах, но не ругается на комментарий.
type fileShape struct {
	Comment   string     `json:"_comment"`
	Version   string     `json:"version"`
	Districts []District `json:"districts"`
}

// Catalog — загруженный реестр районов.
type Catalog struct {
	districts []District
}

// Load читает реестр из <registryRoot>/values/astana_districts.json. registryRoot — корень registry
// (как у registry.Load/lexicon.Load/methodology.Load).
func Load(registryRoot string) (*Catalog, error) {
	path := filepath.Join(registryRoot, "values", DistrictsFile)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("district: чтение %s: %w", path, err)
	}
	var f fileShape
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields() // неизвестный/опечатанный ключ → честная ошибка (не тихий пропуск)
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("district: парс %s: %w", path, err)
	}
	if !versionRe.MatchString(f.Version) {
		return nil, fmt.Errorf("district: %s: невалидная version %q (формат ^v\\d+$)", path, f.Version)
	}
	if len(f.Districts) == 0 {
		return nil, fmt.Errorf("district: %s: пустой список районов — honest-fail", path)
	}

	seen := map[string]bool{}
	katoSeen := map[string]string{} // kato → slug (для детекта точного дубля кода)
	for i, d := range f.Districts {
		if strings.TrimSpace(d.Slug) == "" || strings.TrimSpace(d.NameKk) == "" || strings.TrimSpace(d.NameRu) == "" {
			return nil, fmt.Errorf("district: %s: район #%d — пустой slug/name_kk/name_ru (honest-fail)", path, i)
		}
		if seen[d.Slug] {
			return nil, fmt.Errorf("district: %s: дубль slug %q", path, d.Slug)
		}
		seen[d.Slug] = true
		if d.Kato != nil {
			if !katoRe.MatchString(*d.Kato) {
				return nil, fmt.Errorf("district: %s: район %q — нецифровой/некорректный kato %q (формат ^[0-9]{2,11}$ или null)", path, d.Slug, *d.Kato)
			}
			// Точный дубль kato → honest-fail: два района с одним кодом сделали бы NameByKATO неоднозначным
			// (молча выбрал бы один → ложная атрибуция имени). Иерархическая вложенность (город «710» ⊃ район
			// «710512») ЛЕГИТИМНА — её разрешает longest-prefix; ловим только ТОЧНОЕ совпадение кодов.
			if prev, ok := katoSeen[*d.Kato]; ok {
				return nil, fmt.Errorf("district: %s: дубль kato %q у районов %q и %q", path, *d.Kato, prev, d.Slug)
			}
			katoSeen[*d.Kato] = d.Slug
		}
	}
	return &Catalog{districts: f.Districts}, nil
}

// ValidKATO — допустим ли КАТО-код/префикс из URL (цифры, 2..11). false → хендлер отвечает 400.
func ValidKATO(s string) bool { return katoRe.MatchString(s) }

// NameByKATO возвращает имена района, чей подтверждённый kato является префиксом запрошенного kato
// (самое длинное совпадение — самый специфичный район). ok=false, если ни один код не подтверждён
// (kato=null) или нет совпадения → вызывающий честно показывает no_data («название уточняется»),
// НЕ выдумывает имя.
func (c *Catalog) NameByKATO(kato string) (nameKk, nameRu string, ok bool) {
	best := -1
	for i, d := range c.districts {
		if d.Kato == nil || *d.Kato == "" {
			continue
		}
		if strings.HasPrefix(kato, *d.Kato) {
			if best == -1 || len(*d.Kato) > len(*c.districts[best].Kato) {
				best = i
			}
		}
	}
	if best == -1 {
		return "", "", false
	}
	return c.districts[best].NameKk, c.districts[best].NameRu, true
}

// All — все районы реестра (для тестов/будущего списка районов).
func (c *Catalog) All() []District { return append([]District(nil), c.districts...) }
