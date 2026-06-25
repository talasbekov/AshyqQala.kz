package flags_test

import (
	"testing"

	"ashyqqala/server/internal/flags"
	"ashyqqala/server/internal/registry"
)

func pi(n int) *int { return &n }

// spParams — methodology-пороги single-participant для тестов (enabled + один исключённый способ).
var spParams = flags.Params{
	MethodologyVersion:              "v1.0",
	SingleParticipantEnabled:        true,
	SingleParticipantExcludeMethods: []string{"из_одного_источника"},
}

// TestSingleParticipant_States — таблица состояний FR-19 (AC1/AC2): raised только при enabled && count==1 &&
// способ ∉ exclude; нет данных → insufficient_data (НЕ ложный not_raised); disabled → not_published.
func TestSingleParticipant_States(t *testing.T) {
	cases := []struct {
		name   string
		count  *int
		method string
		params flags.Params
		want   registry.FlagState
	}{
		{"1 участник, способ не исключён → raised", pi(1), "конкурс", spParams, registry.FlagRaised},
		{"1 участник, способ исключён → not_raised", pi(1), "из_одного_источника", spParams, registry.FlagNotRaised},
		{"1 участник, исключён (регистр/пробелы) → not_raised", pi(1), "  Из_Одного_Источника ", spParams, registry.FlagNotRaised},
		{"2 участника → not_raised", pi(2), "конкурс", spParams, registry.FlagNotRaised},
		{"0 участников → insufficient (нет данных)", pi(0), "конкурс", spParams, registry.FlagInsufficientData},
		{"nil участников → insufficient (нет данных)", nil, "конкурс", spParams, registry.FlagInsufficientData},
		{"disabled → not_published", pi(1), "конкурс", flags.Params{MethodologyVersion: "v1.0", SingleParticipantEnabled: false, SingleParticipantExcludeMethods: []string{"из_одного_источника"}}, registry.FlagNotPublished},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st, _ := flags.SingleParticipant(flags.Inputs{ParticipantCount: c.count, ProcurementMethod: c.method}, c.params)
			if st != c.want {
				t.Fatalf("состояние = %s, ожидалось %s", st, c.want)
			}
		})
	}
}

// TestSingleParticipant_ExcludeDiscriminates — negative/positive control: предикат исключения РАЗЛИЧАЕТ
// (способен покраснеть) — при одном и том же count==1 способ ∈ exclude даёт not_raised, ∉ exclude → raised.
// См. [[guards-must-prove-red]].
func TestSingleParticipant_ExcludeDiscriminates(t *testing.T) {
	in := flags.Inputs{ParticipantCount: pi(1), ProcurementMethod: "из_одного_источника"}
	if st, _ := flags.SingleParticipant(in, spParams); st != registry.FlagNotRaised {
		t.Fatalf("исключённый способ должен дать not_raised, получено %s", st)
	}
	in.ProcurementMethod = "открытый_конкурс"
	if st, _ := flags.SingleParticipant(in, spParams); st != registry.FlagRaised {
		t.Fatalf("неисключённый способ должен дать raised, получено %s", st)
	}
}

// TestSingleParticipant_Evidence — evidence хранит ВСЕ входы (пересчитываемость); methodology_version из params.
func TestSingleParticipant_Evidence(t *testing.T) {
	in := flags.Inputs{ParticipantCount: pi(1), ProcurementMethod: "конкурс"}
	st, ev := flags.SingleParticipant(in, spParams)
	if st != registry.FlagRaised {
		t.Fatalf("ожидалось raised, получено %s", st)
	}
	if ev.ParticipantCount == nil || *ev.ParticipantCount != 1 {
		t.Errorf("evidence.participant_count = %v, ожидалось 1", ev.ParticipantCount)
	}
	if ev.ProcurementMethod != "конкурс" {
		t.Errorf("evidence.procurement_method = %q", ev.ProcurementMethod)
	}
	if ev.Enabled != true || len(ev.ExcludeMethods) != 1 {
		t.Errorf("evidence.enabled/exclude_methods неполны: %+v", ev)
	}
	if ev.MethodologyVersion != "v1.0" {
		t.Errorf("evidence.methodology_version = %q, ожидалось v1.0 (== params)", ev.MethodologyVersion)
	}
}

// TestSingleParticipant_Property — однозначность: raised ⟺ enabled && count==1 && способ ∉ exclude.
func TestSingleParticipant_Property(t *testing.T) {
	methods := []string{"конкурс", "из_одного_источника", "аукцион"}
	for _, enabled := range []bool{true, false} {
		for c := 0; c <= 3; c++ {
			for _, m := range methods {
				p := flags.Params{MethodologyVersion: "v1.0", SingleParticipantEnabled: enabled, SingleParticipantExcludeMethods: []string{"из_одного_источника"}}
				st, _ := flags.SingleParticipant(flags.Inputs{ParticipantCount: pi(c), ProcurementMethod: m}, p)
				wantRaised := enabled && c == 1 && m != "из_одного_источника"
				if (st == registry.FlagRaised) != wantRaised {
					t.Errorf("enabled=%v count=%d method=%q: raised=%v, ожидалось %v (state=%s)", enabled, c, m, st == registry.FlagRaised, wantRaised, st)
				}
			}
		}
	}
}
