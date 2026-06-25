// Package export — экспорт evidence флага для цитирования третьим лицом/СМИ (AR-29, Story 4.6). Чистый слой
// (без store/IO): берёт уже прочитанный флаг, отдаёт канонический JSON (машиночитаемый, пересчитываемый) и
// печатный НЕЙТРАЛЬНЫЙ текст. Числа — внутри evidence (не дублируются прозой; нейтральная рамка — из registry,
// передаётся вызывающим, чтобы пакет не дублировал лексикон).
package export

import (
	"encoding/json"
	"fmt"
)

// FlagRecord — минимальное представление флага для экспорта (из risk_flags). Без store-зависимостей.
type FlagRecord struct {
	FlagType           string
	SubjectType        string
	SubjectID          int64
	MethodologyVersion string
	Evidence           json.RawMessage // как хранится в risk_flags.evidence (jsonb)
}

// exportDoc — каноническая форма JSON-экспорта (стабильные ключи для пересчёта третьим лицом).
type exportDoc struct {
	FlagType           string          `json:"flag_type"`
	SubjectType        string          `json:"subject_type"`
	SubjectID          int64           `json:"subject_id"`
	MethodologyVersion string          `json:"methodology_version"`
	Evidence           json.RawMessage `json:"evidence"`
}

// Export — AR-29: (а) канонический JSON (evidence + контекст; пересчитываемо третьим лицом) + (б) печатный
// НЕЙТРАЛЬНЫЙ текст (факты + сигнал-рамка, без оценочных слов). Чистая функция. neutralFrame — нейтральная
// рамка из registry (напр. «сигнал, требующий проверки»), передаётся вызывающим (пакет не тянет лексикон).
func Export(f FlagRecord, neutralFrame string) (jsonBytes []byte, printable string, err error) {
	doc := exportDoc{
		FlagType:           f.FlagType,
		SubjectType:        f.SubjectType,
		SubjectID:          f.SubjectID,
		MethodologyVersion: f.MethodologyVersion,
		Evidence:           f.Evidence,
	}
	jsonBytes, err = json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, "", fmt.Errorf("export: marshal evidence (%s #%d): %w", f.FlagType, f.SubjectID, err)
	}
	// Печатный: только ФАКТЫ + нейтральная рамка. Никаких оценочных слов; числа — внутри evidence (не дублируем
	// прозой → нет числовых литералов-порогов в тексте).
	printable = fmt.Sprintf("%s\nТип сигнала: %s\nСубъект: %s #%d\nВерсия методики: %s\nEvidence (пересчитываемо): %s",
		neutralFrame, f.FlagType, f.SubjectType, f.SubjectID, f.MethodologyVersion, string(f.Evidence))
	return jsonBytes, printable, nil
}
