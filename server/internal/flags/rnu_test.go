package flags_test

import (
	"testing"
	"time"

	"ashyqqala/server/internal/clock"
	"ashyqqala/server/internal/flags"
	"ashyqqala/server/internal/registry"
)

// rnuNow — фиксированное «сейчас» для дата-зависимых тестов РНУ (детерминизм через clock.Fixed).
var rnuNow = time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)

func rnuClock() clock.Clock { return clock.Fixed{T: rnuNow} }

// dayUnix — unix-секунды начала суток (даты РНУ — *int64 unix; ядро time-free).
func dayUnix(y int, m time.Month, d int) *int64 {
	u := time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix()
	return &u
}

func u64(v int64) *int64 { return &v }

var rnuParams = flags.Params{MethodologyVersion: "v1.0", RNUEnabled: true}

func rnuIn(start, end *int64) flags.Inputs {
	return flags.Inputs{
		Now:           rnuClock(),
		RNUStartUnix:  start,
		RNUEndUnix:    end,
		RNUGoszakupID: "RNU-12345",
		RNUSourceURL:  "https://goszakup.gov.kz/ru/egzrnu/index",
		RNUReasonRef:  "приказ №7",
	}
}

// TestRNU_States — таблица FR-22 (AC1/AC2/AC3) с clock.Fixed: raised при активной записи (start ≤ now, end
// пуст/будущее); not_raised при истёкшей/будущей; not_published при kill-switch; insufficient при битой записи.
func TestRNU_States(t *testing.T) {
	cases := []struct {
		name   string
		start  *int64
		end    *int64
		params flags.Params
		want   registry.FlagState
	}{
		{"активна (start прошлое, end nil) → raised", dayUnix(2026, 1, 1), nil, rnuParams, registry.FlagRaised},
		{"активна (start прошлое, end будущее) → raised", dayUnix(2026, 1, 1), dayUnix(2027, 1, 1), rnuParams, registry.FlagRaised},
		{"истекла (end в прошлом) → not_raised (авто-снятие)", dayUnix(2026, 1, 1), dayUnix(2026, 3, 1), rnuParams, registry.FlagNotRaised},
		{"ещё не активна (start в будущем) → not_raised", dayUnix(2026, 12, 1), nil, rnuParams, registry.FlagNotRaised},
		{"граница: start == now → raised (≤)", u64(rnuNow.Unix()), nil, rnuParams, registry.FlagRaised},
		{"граница: end == now → not_raised (end > now строго)", dayUnix(2026, 1, 1), u64(rnuNow.Unix()), rnuParams, registry.FlagNotRaised},
		{"kill-switch enabled=false → not_published", dayUnix(2026, 1, 1), nil, flags.Params{MethodologyVersion: "v1.0", RNUEnabled: false}, registry.FlagNotPublished},
		{"битая запись (нет start) → insufficient", nil, nil, rnuParams, registry.FlagInsufficientData},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st, _ := flags.RNU(rnuIn(c.start, c.end), c.params)
			if st != c.want {
				t.Fatalf("состояние = %s, ожидалось %s", st, c.want)
			}
		})
	}
}

// TestRNU_AutoClearDiscriminates — negative/positive control авто-снятия: ОДНА запись (start прошлое, end T),
// clock ДО end → raised, clock ПОСЛЕ end → not_raised. Доказывает, что флаг реально снимается по end_date
// (страж способен покраснеть). [[guards-must-prove-red]]
func TestRNU_AutoClearDiscriminates(t *testing.T) {
	start := dayUnix(2026, 1, 1)
	end := dayUnix(2026, 6, 20) // запись истекает 2026-06-20
	in := flags.Inputs{RNUStartUnix: start, RNUEndUnix: end, RNUGoszakupID: "RNU-1"}

	in.Now = clock.Fixed{T: time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)} // ДО end → активна
	if st, _ := flags.RNU(in, rnuParams); st != registry.FlagRaised {
		t.Fatalf("clock до end → ожидалось raised, получено %s", st)
	}
	in.Now = clock.Fixed{T: time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)} // ПОСЛЕ end → снята
	if st, _ := flags.RNU(in, rnuParams); st != registry.FlagNotRaised {
		t.Fatalf("clock после end → ожидалось not_raised (авто-снятие), получено %s", st)
	}
}

// TestRNU_NilClock — дата-зависимый флаг без часов (nil Now) → insufficient_data, БЕЗ паники (ядро не паникует;
// P2 ревью: экспортируемая RNU не должна разваливаться при nil clock от прямого вызова).
func TestRNU_NilClock(t *testing.T) {
	in := flags.Inputs{RNUStartUnix: dayUnix(2026, 1, 1)} // Now не задан → nil
	if st, _ := flags.RNU(in, rnuParams); st != registry.FlagInsufficientData {
		t.Fatalf("nil clock → ожидалось insufficient_data (без паники), получено %s", st)
	}
}

// TestRNU_Evidence — evidence хранит ссылку реестра + даты (атрибуция государству); версия из params. Также
// проверяем evidence на not_published-пути (kill-switch) — заполняется до early-return (урок ревью 4.3).
func TestRNU_Evidence(t *testing.T) {
	st, ev := flags.RNU(rnuIn(dayUnix(2026, 1, 1), dayUnix(2027, 1, 1)), rnuParams)
	if st != registry.FlagRaised {
		t.Fatalf("ожидалось raised, получено %s", st)
	}
	if ev.GoszakupRNUID != "RNU-12345" || ev.SourceURL == "" || ev.ReasonRef == "" {
		t.Errorf("evidence ссылки реестра неполны: %+v", ev)
	}
	if ev.StartDateUnix == nil || ev.EndDateUnix == nil || !ev.Enabled {
		t.Errorf("evidence даты/enabled неполны: %+v", ev)
	}
	if ev.MethodologyVersion != "v1.0" {
		t.Errorf("evidence.methodology_version = %q, ожидалось v1.0 (==params)", ev.MethodologyVersion)
	}

	// not_published-путь (kill-switch): evidence всё равно заполнен.
	_, evOff := flags.RNU(rnuIn(dayUnix(2026, 1, 1), nil), flags.Params{MethodologyVersion: "v1.0", RNUEnabled: false})
	if evOff.GoszakupRNUID != "RNU-12345" || evOff.Enabled || evOff.MethodologyVersion != "v1.0" {
		t.Errorf("evidence на not_published-пути неполон: %+v", evOff)
	}
}
