package goszakup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// FileSource — токен-независимый источник: локальные JSON-дампы `<dir>/<resource>.json` (массив объектов).
// Зеркалит file-источник stage0-audit. Для разработки/фикстур и трека «Парсер-мост» (Story 0.6) без токена.
type FileSource struct{ Dir string }

func NewFileSource(dir string) FileSource { return FileSource{Dir: dir} }

func (s FileSource) Name() string { return "file:" + s.Dir }

// Fetch читает `<Dir>/<resource>.json` (массив объектов) и отдаёт в handle. Отсутствующий/битый файл —
// ЧЕСТНАЯ ошибка (не тихо пусто). scopeBINs игнорируется (фильтрация по БИН — забота ows-источника).
func (s FileSource) Fetch(resource string, _ []string, max int, handle func(items []map[string]any) error) error {
	fname := filepath.Join(s.Dir, resource+".json")
	data, err := os.ReadFile(fname)
	if err != nil {
		return fmt.Errorf("goszakup file-источник: чтение %s: %w", fname, err)
	}
	var items []map[string]any
	if err := json.Unmarshal(data, &items); err != nil {
		return fmt.Errorf("goszakup file-источник: разбор %s: %w", fname, err)
	}
	if max > 0 && len(items) > max {
		items = items[:max]
	}
	return handle(items)
}

// Компайл-тайм проверка: FileSource удовлетворяет Source.
var _ Source = FileSource{}
