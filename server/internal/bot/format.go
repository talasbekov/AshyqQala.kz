package bot

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	"ashyqqala/server/internal/outbox"
	"ashyqqala/server/internal/registry"
	"ashyqqala/server/internal/render"
)

// telegramMaxLen — лимит длины сообщения Telegram (символов/рун). Финальная строка обрезается по ГРАНИЦЕ
// руны (UTF-8 кириллица/казахский — не байтами) [Telegram Bot API: sendMessage text ≤ 4096].
const telegramMaxLen = 4096

// notifyEmoji — НЕЙТРАЛЬНЫЙ эмодзи-префикс уведомления (колокольчик = «уведомление», НЕ оценочный знак).
// Эмодзи не входит в taboo-лексикон; нейтральность проверяется на ФИНАЛЬНОЙ строке (FindTaboo).
const notifyEmoji = "\U0001F514" // 🔔

// markdownV2Reserved — зарезервированные символы MarkdownV2 (Telegram Bot API) — экранируются '\'. Сам '\'
// ВКЛЮЧЁН первым: литеральный backslash в прозе должен удваиваться ('\\'), иначе он съест экранирование
// следующего reserved-символа.
const markdownV2Reserved = "\\_*[]()~`>#+-=|{}.!"

// escapeMarkdownV2 экранирует зарезервированные символы MarkdownV2 (Telegram): каждый reserved → '\X'.
// Экранирование НЕ должно протащить taboo или сломать рамку — нейтральность проверяется ПОСЛЕ (на финале).
func escapeMarkdownV2(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 16)
	for _, r := range s {
		if r < utf8.RuneSelf && strings.ContainsRune(markdownV2Reserved, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// truncateUTF16 обрезает строку так, чтобы её длина в UTF-16 кодовых единицах (как считает лимит Telegram,
// НЕ руны/байты) была ≤ max, не разрывая руну. Астральные руны (эмодзи 🔔) занимают 2 UTF-16-юнита.
func truncateUTF16(s string, max int) string {
	if max <= 0 {
		return ""
	}
	units := 0
	for i, r := range s {
		w := 1
		if r > 0xFFFF { // вне BMP → суррогатная пара
			w = 2
		}
		if units+w > max {
			return s[:i]
		}
		units += w
	}
	return s
}

// trimDanglingEscape убирает ВИСЯЧИЙ '\' (нечётный хвост обратных слешей): обрезка по лимиту могла разорвать
// пару '\X', оставив одиночный '\' в конце → Telegram отверг бы MarkdownV2. Чётный хвост (\\)= экранированный
// слеш — оставляем.
func trimDanglingEscape(s string) string {
	n := 0
	for i := len(s) - 1; i >= 0 && s[i] == '\\'; i-- {
		n++
	}
	if n%2 == 1 {
		return s[:len(s)-1]
	}
	return s
}

// FormatTelegram превращает НЕЙТРАЛЬНУЮ прозу (из render) в финальную Telegram-строку (AC3): MarkdownV2-
// экранирование → нейтральный эмодзи-префикс → обрезка по 4096 рун → срез висячего '\'. ЧИСТАЯ функция
// (вход→финал, без IO): per-surface neutrality-тест ассертит FindTaboo==∅ именно на её выводе.
func FormatTelegram(neutral string) string {
	out := notifyEmoji + " " + escapeMarkdownV2(neutral)
	out = truncateUTF16(out, telegramMaxLen)
	return trimDanglingEscape(out)
}

// flagPayload — нейтральный конверт события flag.raised (id/состояние). Полный контент уведомления
// (число, deep-link, «как посчитано») — Story 7.2; 7.1 = транспорт + нейтральная проза через render.
type flagPayload struct {
	FlagID    string `json:"flag_id"`
	FlagState string `json:"flag_state"`
}

// knownFlagState — принадлежит ли значение закрытому enum FlagState (анти-мислейбл неизвестного состояния).
func knownFlagState(fs registry.FlagState) bool {
	for _, s := range registry.AllFlagStates() {
		if s == fs {
			return true
		}
	}
	return false
}

// renderEvent выбирает НЕЙТРАЛЬНУЮ прозу события ТОЛЬКО через render (AC2: ноль текстовых литералов в боте).
// flag.raised → FlagLine (из payload id/состояния; неизвестный flag_id → честный fallback render). Прочие
// типы (contract.created и т.п.) → нейтральная рамка (богатый контент — Story 7.2). НИКОГДА пусто/taboo.
func renderEvent(rd render.Renderer, e outbox.Event, loc registry.Locale) string {
	switch e.Type {
	case outbox.TypeFlagRaised:
		var p flagPayload
		_ = json.Unmarshal(e.Payload, &p) // битый/пустой payload → нулевые поля → честный fallback ниже
		fs := registry.FlagState(p.FlagState)
		if !knownFlagState(fs) {
			// Пусто/неизвестный мусор → тип события flag.raised авторитетен (НЕ мислейбл «недостаточно данных»,
			// который `FlagLine` выдал бы для любого не-raised значения).
			fs = registry.FlagRaised
		}
		return rd.FlagLine(p.FlagID, fs, loc)
	default:
		// Нейтральная рамка как минимально-честный текст (контент уведомления — Story 7.2).
		return rd.Text(loc, "frame.signal")
	}
}

// formatEvent = renderEvent (нейтральная проза через render) → FormatTelegram (MarkdownV2/4096/эмодзи).
func formatEvent(rd render.Renderer, e outbox.Event, loc registry.Locale) string {
	return FormatTelegram(renderEvent(rd, e, loc))
}
