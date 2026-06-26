package og

import "testing"

// TestFormatMoney_GroupingMatchesIntl — серверный формат денег совпадает с web Intl.NumberFormat('ru-KZ'|
// 'kk-KZ') байт-в-байт (разделитель U+00A0, знак минус U+002D — проверено в Node), AC-1 «цифры не расходятся».
func TestFormatMoney_GroupingMatchesIntl(t *testing.T) {
	const nb = " " // NO-BREAK SPACE — разделитель Intl ru-KZ/kk-KZ (см. money.go)
	cases := map[string]string{
		"123456789": "123" + nb + "456" + nb + "789 ₸",
		"0":         "0 ₸",
		"999":       "999 ₸",
		"1000":      "1" + nb + "000 ₸",
		"-1234567":  "-1" + nb + "234" + nb + "567 ₸",
	}
	for in, want := range cases {
		got, err := formatMoney(in)
		if err != nil {
			t.Fatalf("formatMoney(%q) error: %v", in, err)
		}
		if got != want {
			t.Errorf("formatMoney(%q) = %q; want %q", in, got, want)
		}
	}
}

// TestFormatMoney_RejectsNonCanonical — честный fail на не-каноничной строке (НЕ молчаливый «0 ₸»):
// пустая, дробь, hex, с пробелами, буквы, плюс, запятая. Зеркало web formatMoney (^-?\d+$).
func TestFormatMoney_RejectsNonCanonical(t *testing.T) {
	for _, in := range []string{"", "12.5", "0x10", "1 000", "abc", "+5", "12,5", " 5"} {
		if _, err := formatMoney(in); err == nil {
			t.Errorf("formatMoney(%q): ожидалась ошибка (не молчаливый «0 ₸»), got nil", in)
		}
	}
}
