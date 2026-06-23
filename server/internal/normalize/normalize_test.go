package normalize_test

import (
	"testing"

	"ashyqqala/server/internal/normalize"
)

func TestNormalize_CollapsesWhitespace(t *testing.T) {
	cases := []struct {
		in   string
		want normalize.BIN
	}{
		{"  ТОО   Астана  Жол  ", "ТОО Астана Жол"},
		{"123456789012", "123456789012"},
		{"", ""},
		{"\tx\n y ", "x y"},
	}
	for _, c := range cases {
		if got := normalize.Normalize(c.in); got != c.want {
			t.Errorf("Normalize(%q) = %q, ожидалось %q", c.in, got, c.want)
		}
	}
}
