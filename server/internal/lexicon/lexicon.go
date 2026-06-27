// Package lexicon — рантайм-загрузчик КУРАТОРСКОГО лексикона нормализации (FR-2, Story 2.3). Источник
// истины — registry/values/normalize_lexicon.v1.json (человек правит ТОЛЬКО там; читается в рантайме —
// принцип AR-12, как registry/methodology). НЕ в чистом ядре (os/json) → не нарушает страж чистоты
// internal/normalize. Честно падает (честность над домыслом): отсутствующий/битый файл, невалидная версия,
// многосимвольный гомоглиф, пустой синоним — явная ошибка на старте, а НЕ тихий пустой лексикон.
package lexicon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"ashyqqala/server/internal/normalize"
)

// LexiconFile — имя текущей версии лексикона в registry/values. Новая версия = НОВЫЙ файл (vN+1).
const LexiconFile = "normalize_lexicon.v1.json"

// versionRe — формат version лексикона: vN (например v1).
var versionRe = regexp.MustCompile(`^v\d+$`)

// fileShape — форма JSON-источника. `_comment` объявлен явно: DisallowUnknownFields ловит опечатки в
// реальных ключах, но не ругается на комментарий.
type fileShape struct {
	Comment    string            `json:"_comment"`
	Version    string            `json:"version"`
	Homoglyphs map[string]string `json:"homoglyphs"`
	Synonyms   map[string]string `json:"synonyms"`
}

// Load читает кураторский лексикон из <registryRoot>/values/normalize_lexicon.v1.json и отдаёт готовый
// normalize.Lexicon. registryRoot — корень registry (как у registry.Load/methodology.Load).
func Load(registryRoot string) (normalize.Lexicon, error) {
	path := filepath.Join(registryRoot, "values", LexiconFile)
	b, err := os.ReadFile(path)
	if err != nil {
		return normalize.Lexicon{}, fmt.Errorf("lexicon: чтение %s: %w", path, err)
	}
	var f fileShape
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields() // неизвестный/опечатанный ключ → честная ошибка (не тихий пропуск)
	if err := dec.Decode(&f); err != nil {
		return normalize.Lexicon{}, fmt.Errorf("lexicon: парс %s: %w", path, err)
	}
	if !versionRe.MatchString(f.Version) {
		return normalize.Lexicon{}, fmt.Errorf("lexicon: %s: невалидная version %q (формат ^v\\d+$)", path, f.Version)
	}

	homo := make(map[rune]rune, len(f.Homoglyphs))
	for k, v := range f.Homoglyphs {
		kr, err1 := singleRune(k)
		vr, err2 := singleRune(v)
		if err1 != nil || err2 != nil {
			return normalize.Lexicon{}, fmt.Errorf("lexicon: %s: гомоглиф %q→%q должен быть одиночными рунами", path, k, v)
		}
		homo[kr] = vr
	}

	for k, v := range f.Synonyms {
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
			return normalize.Lexicon{}, fmt.Errorf("lexicon: %s: пустой синоним (%q→%q) — honest-fail (тихая пустая строка молча сломала бы матч)", path, k, v)
		}
	}

	return normalize.Lexicon{Version: f.Version, Homoglyphs: homo, Synonyms: f.Synonyms}, nil
}

// singleRune — строка должна быть ровно одной руной (для гомоглифа символ→символ).
func singleRune(s string) (rune, error) {
	rs := []rune(s)
	if len(rs) != 1 {
		return 0, fmt.Errorf("ожидалась одна руна, got %q", s)
	}
	return rs[0], nil
}
