package bot

import (
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"ashyqqala/server/internal/registry"
	"ashyqqala/server/internal/render"
)

func newRenderer(t *testing.T) render.Renderer {
	t.Helper()
	reg, err := registry.Load(filepath.Join("../../../", "registry"))
	if err != nil {
		t.Fatalf("registry.Load: %v", err)
	}
	return render.Renderer{Reg: reg}
}

// TestEscapeMarkdownV2 — каждый зарезервированный символ MarkdownV2 экранируется '\', обычные — нет.
func TestEscapeMarkdownV2(t *testing.T) {
	for _, r := range markdownV2Reserved {
		in := string(r)
		got := escapeMarkdownV2(in)
		want := "\\" + in
		if got != want {
			t.Errorf("escapeMarkdownV2(%q) = %q; want %q", in, got, want)
		}
	}
	// Кириллица/буквы не экранируются.
	if got := escapeMarkdownV2("Цена за км"); got != "Цена за км" {
		t.Errorf("буквы не должны экранироваться: %q", got)
	}
	// Смешанное: точка и дефис экранируются, буквы нет.
	if got := escapeMarkdownV2("a-b.c"); got != "a\\-b\\.c" {
		t.Errorf("escapeMarkdownV2(a-b.c) = %q; want a\\-b\\.c", got)
	}
}

// TestTruncateUTF16 — обрезка по UTF-16-юнитам (как лимит Telegram), не разрывая руну: BMP-кириллица 1:1,
// астральный эмодзи = 2 юнита.
func TestTruncateUTF16(t *testing.T) {
	s := "абвгд" // 5 BMP-рун = 5 UTF-16-юнитов
	if got := truncateUTF16(s, 3); got != "абв" {
		t.Errorf("truncateUTF16(%q,3) = %q; want абв", s, got)
	}
	if got := truncateUTF16(s, 99); got != s {
		t.Errorf("truncateUTF16 без обрезки изменил строку: %q", got)
	}
	if got := truncateUTF16(s, 0); got != "" {
		t.Errorf("truncateUTF16(_,0) = %q; want пусто", got)
	}
	// Астральная руна = 2 юнита: 🔔🔔 (2 руны, 4 юнита). max=3 → влезает только первая (вторая не лезет целиком).
	two := "\U0001F514\U0001F514"
	if got := truncateUTF16(two, 3); got != "\U0001F514" {
		t.Errorf("truncateUTF16(2×эмодзи,3) = %q; want один эмодзи", got)
	}
}

// TestTrimDanglingEscape — нечётный хвост '\' срезается (висячий escape после обрезки), чётный остаётся.
func TestTrimDanglingEscape(t *testing.T) {
	cases := map[string]string{
		"ab\\":   "ab",     // 1 слеш (нечёт) → срез
		"ab\\\\": "ab\\\\", // 2 слеша (чёт) → оставить
		"abc":    "abc",
	}
	for in, want := range cases {
		if got := trimDanglingEscape(in); got != want {
			t.Errorf("trimDanglingEscape(%q) = %q; want %q", in, got, want)
		}
	}
}

// TestFormatTelegram — финальная строка: эмодзи-префикс + экранированная проза, ≤4096, без висячего '\'.
func TestFormatTelegram(t *testing.T) {
	got := FormatTelegram("Цена за км выше медианы — сигнал, требующий проверки")
	if !strings.HasPrefix(got, notifyEmoji+" ") {
		t.Errorf("нет нейтрального эмодзи-префикса: %q", got)
	}
	// «—» не reserved, но запятая? нет. Проверим, что точка-эквиваленты экранированы при наличии.
	if strings.Count(got, "\\") != 0 {
		// в этой фразе нет reserved-символов → ноль экранирований
		t.Errorf("неожиданное экранирование: %q", got)
	}
	// Длинная строка обрезается до лимита Telegram в UTF-16-юнитах (с учётом 2-юнитного эмодзи-префикса).
	long := strings.Repeat("я", 5000)
	if u := len(utf16.Encode([]rune(FormatTelegram(long)))); u > telegramMaxLen {
		t.Errorf("финал длиннее лимита Telegram: %d UTF-16-юнитов", u)
	}
}
