//go:build scrape

package main

import (
	"strings"
	"testing"
)

// TestInterimEnabled_GateRequiresFlag — AC1: без ASHYQQALA_INTERIM_SCRAPE=1 команда отказывает.
func TestInterimEnabled_GateRequiresFlag(t *testing.T) {
	off := func(string) string { return "" }
	if interimEnabled(off) {
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
	// Строгий «=1»: любое иное значение (true/0/yes) → выключено.
	other := func(k string) string {
		if k == interimFlag {
			return "true"
		}
		return ""
	}
	if interimEnabled(other) {
		t.Error("ASHYQQALA_INTERIM_SCRAPE=true (не =1) → выключено")
	}
}

// TestLoudWarning_MentionsDeviationAndDeletion — AC1/Task 5: warning несёт §6.1/§6.4 и критерий удаления.
func TestLoudWarning_MentionsDeviationAndDeletion(t *testing.T) {
	var b strings.Builder
	loudWarning(&b)
	out := b.String()
	for _, want := range []string{"§6.1/§6.4", "GOSZAKUP_TOKEN", "scrape→ows", "ВРЕМЕННЫЙ", "КРИТЕРИЙ УДАЛЕНИЯ"} {
		if !strings.Contains(out, want) {
			t.Errorf("warning не содержит %q:\n%s", want, out)
		}
	}
}
