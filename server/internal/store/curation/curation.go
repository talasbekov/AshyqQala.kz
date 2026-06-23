// Package curation — доступ к КУРАТОРСКИМ данным + механика «правка переживает перезапись» (Story 1.10,
// курация-stub). Граница AR-4/AR-10: проекционные таблицы импортёр перестраивает; кураторские правки
// хранятся ОТДЕЛЬНО, импортёр их только читает → правка переживает ре-импорт. Полная S-0-приёмка
// (реальные 2 импорта + Directus + БД) — Story 2.6; здесь — детерминированная механика-stub.
package curation

import "maps"

// Override — ручная кураторская правка одного поля проекционной строки. Хранится ВНЕ проекции.
type Override struct {
	Field string
	Value string
}

// Apply накладывает кураторские правки поверх (пере)построенной проекционной строки. Поскольку
// overrides хранятся отдельно от проекции, правка переживает любую перезапись проекции импортёром
// (AR-10). Вход не мутируется.
func Apply(projectionRow map[string]string, overrides []Override) map[string]string {
	out := make(map[string]string, len(projectionRow))
	maps.Copy(out, projectionRow)
	for _, o := range overrides {
		out[o.Field] = o.Value
	}
	return out
}
