//go:build scrape

package main

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"ashyqqala/server/internal/store/gen"
)

func TestInterimEnabled_GateRequiresFlag(t *testing.T) {
	if interimEnabled(func(string) string { return "" }) {
		t.Error("без ASHYQQALA_INTERIM_SCRAPE команда должна быть выключена")
	}
	on := func(k string) string {
		if k == interimFlag {
			return "1"
		}
		return ""
	}
	if !interimEnabled(on) {
		t.Error("с ASHYQQALA_INTERIM_SCRAPE=1 команда должна быть включена")
	}
}

func TestLoudWarning_MentionsDeviationAndDeletion(t *testing.T) {
	var b strings.Builder
	loudWarning(&b)
	out := b.String()
	for _, want := range []string{"§6.1/§6.4", "GOSZAKUP_TOKEN", "keyword-bias", "КРИТЕРИЙ УДАЛЕНИЯ", "ВРЕМЕННЫЙ"} {
		if !strings.Contains(out, want) {
			t.Errorf("warning не содержит %q:\n%s", want, out)
		}
	}
}

// TestToGeoParams_UnmatchedIsNull — гардрейл честности (AC2): unmatched → lat/lon NULL (Valid=false) +
// status unmatched, НИКОГДА не 0,0; matched → Valid + status auto.
func TestToGeoParams_UnmatchedIsNull(t *testing.T) {
	lot := gen.Lot{GoszakupLotID: "scrape-x", TitleRu: pgtype.Text{String: "ул. Абая", Valid: true}}

	m := toGeoParams(lot, "ул. Абая, Астана", 51.16, 71.47, true)
	if m.GeocodeStatus != "auto" || !m.Lat.Valid || !m.Lon.Valid || m.Lat.Float64 != 51.16 {
		t.Errorf("matched: status=%q lat=%+v lon=%+v, ожидалось auto + Valid", m.GeocodeStatus, m.Lat, m.Lon)
	}
	if !m.AddressText.Valid || m.AddressText.String != "ул. Абая, Астана" {
		t.Errorf("address_text=%+v, ожидался РЕАЛЬНЫЙ запрос геокодеру (воспроизводимость)", m.AddressText)
	}
	u := toGeoParams(lot, "ул. Абая, Астана", 0, 0, false)
	if u.GeocodeStatus != "unmatched" {
		t.Errorf("unmatched status=%q, ожидалось unmatched", u.GeocodeStatus)
	}
	if u.Lat.Valid || u.Lon.Valid {
		t.Errorf("unmatched: lat/lon должны быть NULL (Valid=false), получено lat=%+v lon=%+v (нельзя 0,0)", u.Lat, u.Lon)
	}
	if u.Confidence.Valid {
		t.Error("confidence в интериме должен быть NULL (importance не парсится)")
	}
}

type fakeGeocoder struct {
	fn func(q string) (float64, float64, bool, error)
}

func (f fakeGeocoder) Geocode(_ context.Context, q string) (float64, float64, bool, error) {
	return f.fn(q)
}

type fakeStore struct{ got []gen.UpsertGeoLotParams }

func (s *fakeStore) UpsertGeoLot(_ context.Context, p gen.UpsertGeoLotParams) error {
	s.got = append(s.got, p)
	return nil
}

// TestGeocodeLots_CoverageAndHonesty — цикл считает автопокрытие как факт и сохраняет честные состояния:
// matched→auto+координаты, no-match→unmatched+NULL, error→считается, без точки нет 0,0, пустой title не геокодится.
func TestGeocodeLots_CoverageAndHonesty(t *testing.T) {
	lots := []gen.Lot{
		{GoszakupLotID: "A", TitleRu: pgtype.Text{String: "ул. Абая", Valid: true}},       // matched
		{GoszakupLotID: "B", TitleRu: pgtype.Text{String: "нет в OSM", Valid: true}},      // empty result
		{GoszakupLotID: "C", TitleRu: pgtype.Text{Valid: false}},                          // нет title → не геокодим
		{GoszakupLotID: "D", TitleRu: pgtype.Text{String: "сетевая ошибка", Valid: true}}, // geocoder error
	}
	gc := fakeGeocoder{fn: func(q string) (float64, float64, bool, error) {
		switch {
		case strings.HasPrefix(q, "ул. Абая"):
			return 51.16, 71.47, true, nil
		case strings.HasPrefix(q, "сетевая ошибка"):
			return 0, 0, false, io.ErrUnexpectedEOF
		default:
			return 0, 0, false, nil
		}
	}}
	store := &fakeStore{}

	st, err := geocodeLots(context.Background(), lots, gc, store, 0, io.Discard)
	if err != nil {
		t.Fatalf("geocodeLots: %v", err)
	}
	if st.Total != 4 || st.Matched != 1 || st.Unmatched != 3 || st.Errors != 1 {
		t.Errorf("stats = %+v, ожидалось Total4 Matched1 Unmatched3 Errors1", st)
	}
	if len(store.got) != 4 {
		t.Fatalf("записей = %d, ожидалось 4 (даже unmatched пишутся честным состоянием)", len(store.got))
	}
	by := map[string]gen.UpsertGeoLotParams{}
	for _, p := range store.got {
		by[p.GoszakupLotID] = p
	}
	if a := by["A"]; a.GeocodeStatus != "auto" || !a.Lat.Valid {
		t.Errorf("A должен быть auto+координата, got %+v", a)
	}
	for _, id := range []string{"B", "C", "D"} {
		if u := by[id]; u.GeocodeStatus != "unmatched" || u.Lat.Valid || u.Lon.Valid {
			t.Errorf("%s должен быть unmatched+NULL (не 0,0), got %+v", id, u)
		}
	}
}
