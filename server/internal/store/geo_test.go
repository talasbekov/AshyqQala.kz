package store

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

// TestValidateBBox — код-ревью (Edge Case Hunter) нашёл: NaN/Inf/инвертированный/вне-диапазона bbox
// уходил бы в ST_MakeEnvelope без проверки. Чистая функция — юнит-тест без БД.
func TestValidateBBox(t *testing.T) {
	nan := 0.0
	nan = nan / nan
	inf := 1.0
	inf = inf / (inf - inf)

	cases := []struct {
		name                           string
		minLon, minLat, maxLon, maxLat float64
		wantErr                        bool
	}{
		{"валидный bbox", 71.20, 51.00, 71.78, 51.30, false},
		{"NaN minLon", nan, 51.00, 71.78, 51.30, true},
		{"Inf maxLat", 71.20, 51.00, 71.78, inf, true},
		{"инвертированный lon (min>=max)", 71.78, 51.00, 71.20, 51.30, true},
		{"инвертированный lat (min>=max)", 71.20, 51.30, 71.78, 51.00, true},
		{"вырожденный (min==max)", 71.20, 51.00, 71.20, 51.30, true},
		{"lon вне диапазона WGS84", -200, 51.00, 71.78, 51.30, true},
		{"lat вне диапазона WGS84", 71.20, 51.00, 71.78, 200, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateBBox(c.minLon, c.minLat, c.maxLon, c.maxLat)
			if (err != nil) != c.wantErr {
				t.Errorf("validateBBox(%v,%v,%v,%v) err=%v, wantErr=%v", c.minLon, c.minLat, c.maxLon, c.maxLat, err, c.wantErr)
			}
		})
	}
}

// TestFormatUUID — невалидный pgtype.UUID → "" (честно, не выдуманный id); валидный → канонический
// 8-4-4-4-12 hex-формат (RFC 4122).
func TestFormatUUID(t *testing.T) {
	if got := formatUUID(pgtype.UUID{}); got != "" {
		t.Errorf("formatUUID(невалидный) = %q, ожидалось «»", got)
	}
	valid := pgtype.UUID{
		Bytes: [16]byte{0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0, 0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0},
		Valid: true,
	}
	want := "12345678-9abc-def0-1234-56789abcdef0"
	if got := formatUUID(valid); got != want {
		t.Errorf("formatUUID = %q, ожидалось %q", got, want)
	}
}
