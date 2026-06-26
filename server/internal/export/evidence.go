// Package export — экспорт evidence флага для цитирования третьим лицом/СМИ (AR-29, Story 4.6). Чистый слой
// (без store/IO): берёт уже прочитанный флаг, отдаёт канонический JSON (машиночитаемый, пересчитываемый) и
// печатный НЕЙТРАЛЬНЫЙ текст. Числа — внутри evidence (не дублируются прозой; нейтральная рамка — из registry,
// передаётся вызывающим, чтобы пакет не дублировал лексикон).
package export

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FlagRecord — минимальное представление флага для экспорта (из risk_flags). Без store-зависимостей.
// SubjectRef — ПУБЛИЧНЫЙ стабильный ключ субъекта (goszakup_contract_id), НЕ внутренний bigint: экспорт
// для цитирования третьим лицом должен ссылаться на natural id (wire-конвенция; OQ#2 Story 5.6).
type FlagRecord struct {
	FlagType           string
	SubjectType        string
	SubjectRef         string
	MethodologyVersion string
	Evidence           json.RawMessage // как хранится в risk_flags.evidence (jsonb)
}

// exportDoc — каноническая форма JSON-экспорта (стабильные ключи для пересчёта третьим лицом).
type exportDoc struct {
	FlagType           string          `json:"flag_type"`
	SubjectType        string          `json:"subject_type"`
	SubjectRef         string          `json:"subject_ref"`
	MethodologyVersion string          `json:"methodology_version"`
	Evidence           json.RawMessage `json:"evidence"`
}

// Export — AR-29: (а) канонический JSON (evidence + контекст; пересчитываемо третьим лицом) + (б) печатный
// НЕЙТРАЛЬНЫЙ текст (факты + сигнал-рамка, без оценочных слов). Чистая функция. neutralFrame — нейтральная
// рамка из registry (напр. «сигнал, требующий проверки»), передаётся вызывающим (пакет не тянет лексикон).
// Честная деградация: пустой/невалидный evidence → error (экспортировать нечего; не фабрикуем документ, §7.4).
func Export(f FlagRecord, neutralFrame string) (jsonBytes []byte, printable string, err error) {
	// Честная деградация (§7.4): экспортировать нечего, если evidence — не НЕПУСТОЙ JSON-объект (ловит ``,
	// `{}`, `null`, массив, скаляр, битый JSON) ИЛИ нет версии методики. Без объекта-входов/версии пересчёт
	// третьим лицом невозможен → не выдаём «пересчитываемый» документ. Согласовано с X-гейтом one-pager
	// (`evidence <> '{}' AND methodology_version <> ''`).
	var obj map[string]json.RawMessage
	if len(f.Evidence) == 0 || json.Unmarshal(f.Evidence, &obj) != nil || len(obj) == 0 {
		return nil, "", fmt.Errorf("export: пустой/вырожденный evidence (%s %s) — цитировать нечего", f.FlagType, f.SubjectRef)
	}
	if strings.TrimSpace(f.MethodologyVersion) == "" {
		return nil, "", fmt.Errorf("export: пустая methodology_version (%s %s) — не пересчитываемо", f.FlagType, f.SubjectRef)
	}
	doc := exportDoc{
		FlagType:           f.FlagType,
		SubjectType:        f.SubjectType,
		SubjectRef:         f.SubjectRef,
		MethodologyVersion: f.MethodologyVersion,
		Evidence:           f.Evidence,
	}
	jsonBytes, err = json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, "", fmt.Errorf("export: marshal evidence (%s %s): %w", f.FlagType, f.SubjectRef, err)
	}
	// Печатный: только ФАКТЫ + нейтральная рамка. Никаких оценочных слов; числа — внутри evidence (не дублируем
	// прозой → нет числовых литералов-порогов в тексте).
	printable = fmt.Sprintf("%s\nТип сигнала: %s\nСубъект: %s %s\nВерсия методики: %s\nEvidence (пересчитываемо): %s",
		neutralFrame, f.FlagType, f.SubjectType, f.SubjectRef, f.MethodologyVersion, string(f.Evidence))
	return jsonBytes, printable, nil
}
