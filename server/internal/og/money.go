package og

import (
	"fmt"
	"regexp"
	"strings"
)

// reInteger — каноничная целая строка тенге (как web formatMoney, web/src/shared/i18n/format.ts): ^-?\d+$.
var reInteger = regexp.MustCompile(`^-?\d+$`)

// groupSep — разделитель разрядов U+00A0 (NO-BREAK SPACE): Intl.NumberFormat('ru-KZ'|'kk-KZ') группирует
// ИМЕННО им (проверено в Node — оба KZ-локали идентичны), поэтому серверный вывод совпадает с SPA
// байт-в-байт (AC-1 «цифры не расходятся с SPA»). tenge — обычный пробел U+0020 + знак тенге U+20B8, как " ₸".
const (
	groupSep = " "
	tenge    = " ₸"
)

// formatMoney — серверный эквивалент web formatMoney: целые тенге приходят СТРОКОЙ (точность >2^53 не
// теряем — работаем со строкой цифр, без float/int64) → группировка по 3 разряда + " ₸". Локаль на
// разделитель не влияет (ru-KZ == kk-KZ). Честный fail: не каноничная целая строка → error, НИКОГДА
// молчаливый «0 ₸» (как и web: пустая/hex-строка недопустима — это фабрикация значения, §7.4).
func formatMoney(amountTng string) (string, error) {
	if !reInteger.MatchString(amountTng) {
		return "", fmt.Errorf("og: ожидалась строка целых тенге (^-?\\d+$), получено %q", amountTng)
	}
	neg := strings.HasPrefix(amountTng, "-")
	digits := strings.TrimPrefix(amountTng, "-")

	var b strings.Builder
	n := len(digits) // только ASCII-цифры → длина в байтах == число разрядов
	for i := range n {
		if i > 0 && (n-i)%3 == 0 {
			b.WriteString(groupSep)
		}
		b.WriteByte(digits[i])
	}
	grouped := b.String()
	if neg {
		grouped = "-" + grouped
	}
	return grouped + tenge, nil
}
