package flags_test

import (
	"testing"
	"time"

	"ashyqqala/server/internal/clock"
	"ashyqqala/server/internal/flags"
	"ashyqqala/server/internal/registry"
)

// Каркас (формулы 4 флагов — Epic 4): ядро НЕ может оценить флаг честно → insufficient_data всегда,
// НИКОГДА ложное not_raised («всё чисто»). Гардрейл честности над домыслом.
func TestFlag_Skeleton_AlwaysInsufficient(t *testing.T) {
	if got := flags.Flag(flags.Inputs{}, flags.Params{}); got != registry.FlagInsufficientData {
		t.Fatalf("без clock: ожидалось insufficient_data, получено %s", got)
	}
	in := flags.Inputs{Now: clock.Fixed{T: time.Date(2026, 6, 23, 0, 0, 0, 0, time.UTC)}}
	if got := flags.Flag(in, flags.Params{}); got != registry.FlagInsufficientData {
		t.Fatalf("с clock (каркас): ожидалось insufficient_data, получено %s", got)
	}
}
