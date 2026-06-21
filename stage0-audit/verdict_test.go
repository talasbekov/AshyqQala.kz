package main

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

// baseCfg — пороги, как в main() (config.go дефолты). GeoGate — несущий гейт (AC2).
func baseCfg() Config {
	return Config{GeoGate: 0.70, MinSample: 5, MinGroupContracts: 5, WindowMonths: 24}
}

func baseOpts() verdictOpts {
	return verdictOpts{generatedAt: "2026-06-20T00:00:00+05:00", dataSource: "file:test"}
}

// TestDeriveVerdict — table-driven: гео-гейт определяет verdict+exit (AC1, AC2).
func TestDeriveVerdict(t *testing.T) {
	cases := []struct {
		name          string
		attempted     int
		success       int
		allowFallback bool
		wantVerdict   string
		wantExit      int
		wantOQ4State  string
		wantCoverageN bool // true → coverage ожидается null (state != ok)
	}{
		{"coverage 0.85 → go", 200, 170, false, "go", 0, "ok", false},
		{"граница 0.70 → go (>=)", 200, 140, false, "go", 0, "ok", false},
		{"coverage 0.55 → no_go", 200, 110, false, "no_go", 1, "ok", false},
		{"геокодер не прогонялся (attempted=0) → no_go", 0, 0, false, "no_go", 1, "insufficient_sample", true},
		{"n=0/1/2 → недостаточно (attempted=2)", 2, 2, false, "no_go", 1, "insufficient_sample", true},
		{"fallback при 0.55 → go_with_fallback", 200, 110, true, "go_with_fallback", 0, "ok", false},
		{"fallback не спасает insufficient", 0, 0, true, "no_go", 1, "insufficient_sample", true},
	}
	cfg := baseCfg()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := Report{
				geocodeEnabled:   c.attempted > 0,
				geocodeAttempted: c.attempted,
				geocodeSuccess:   c.success,
			}
			opts := baseOpts()
			opts.allowFallback = c.allowFallback
			v, exit := deriveVerdict(r, cfg, opts)

			if v.Verdict != c.wantVerdict {
				t.Errorf("verdict = %q, хотим %q", v.Verdict, c.wantVerdict)
			}
			if exit != c.wantExit {
				t.Errorf("exit = %d, хотим %d", exit, c.wantExit)
			}
			if v.OQ4GeoCoverage.State != c.wantOQ4State {
				t.Errorf("oq4.state = %q, хотим %q", v.OQ4GeoCoverage.State, c.wantOQ4State)
			}
			if c.wantCoverageN && v.OQ4GeoCoverage.Coverage != nil {
				t.Errorf("coverage = %v, хотим null при state != ok", *v.OQ4GeoCoverage.Coverage)
			}
			if !c.wantCoverageN && v.OQ4GeoCoverage.Coverage == nil {
				t.Errorf("coverage = null, хотим число при state ok")
			}
			// Несущий инвариант AC2: verdict != go ⟺ exit 1 (go и go_with_fallback → 0).
			if (v.Verdict == "no_go") != (exit == 1) {
				t.Errorf("инвариант exit: verdict=%q exit=%d", v.Verdict, exit)
			}
		})
	}
}

// TestDeriveVerdict_honestyNoFabricated0 — гардрейл: «нет замера» НЕ выдаётся за 0%.
func TestDeriveVerdict_honestyNoFabricated0(t *testing.T) {
	r := Report{geocodeEnabled: false, geocodeAttempted: 0, geocodeSuccess: 0}
	v, exit := deriveVerdict(r, baseCfg(), baseOpts())
	if v.OQ4GeoCoverage.Coverage != nil {
		t.Fatalf("coverage должен быть null (нет замера), а не 0%%")
	}
	if v.Verdict == "go" || exit == 0 {
		t.Fatalf("без замера verdict не может быть go (got %q/exit %d)", v.Verdict, exit)
	}
	if v.OQ4GeoCoverage.State != "insufficient_sample" {
		t.Fatalf("state = %q, хотим insufficient_sample", v.OQ4GeoCoverage.State)
	}
}

// TestDeriveVerdict_requiredKeys — AC1: 5 обязательных ключей верхнего уровня присутствуют в JSON.
func TestDeriveVerdict_requiredKeys(t *testing.T) {
	r := Report{geocodeEnabled: true, geocodeAttempted: 200, geocodeSuccess: 170}
	v, _ := deriveVerdict(r, baseCfg(), baseOpts())
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"oq1_volume", "oq4_geo_coverage", "oq6_sample", "verdict", "methodology_version"} {
		if _, ok := m[k]; !ok {
			t.Errorf("в JSON нет обязательного ключа %q", k)
		}
	}
	if v.MethodologyVersion != MethodologyVersion {
		t.Errorf("methodology_version = %q, хотим %q", v.MethodologyVersion, MethodologyVersion)
	}
	// oq4 несёт CI и размер выборки (AC1).
	var oq4 map[string]json.RawMessage
	_ = json.Unmarshal(m["oq4_geo_coverage"], &oq4)
	for _, k := range []string{"ci_lower", "ci_upper", "sample_size"} {
		if _, ok := oq4[k]; !ok {
			t.Errorf("в oq4_geo_coverage нет ключа %q", k)
		}
	}
}

// TestWilsonInterval — Wilson score 95% (z=1.96).
func TestWilsonInterval(t *testing.T) {
	// Известное значение: 124/200 = 0.62 → CI ≈ [0.551, 0.684].
	lo, hi := wilsonInterval(124, 200, 1.96)
	if math.Abs(lo-0.551) > 0.01 {
		t.Errorf("lo = %.4f, хотим ≈0.551", lo)
	}
	if math.Abs(hi-0.684) > 0.01 {
		t.Errorf("hi = %.4f, хотим ≈0.684", hi)
	}
	// Инварианты: 0 ≤ lo ≤ p ≤ hi ≤ 1.
	p := 124.0 / 200.0
	if !(lo >= 0 && lo <= p && p <= hi && hi <= 1) {
		t.Errorf("нарушен инвариант 0≤lo≤p≤hi≤1: lo=%.4f p=%.4f hi=%.4f", lo, p, hi)
	}
	// n=0 → честное состояние полной неопределённости [0,1], НЕ [0,0].
	lo0, hi0 := wilsonInterval(0, 0, 1.96)
	if lo0 != 0 || hi0 != 1 {
		t.Errorf("n=0 → хотим [0,1], получили [%.4f,%.4f]", lo0, hi0)
	}
}

// TestDeriveVerdict_deterministicKeys — топ-уровень struct → стабильный порядок ключей (AC3).
func TestDeriveVerdict_deterministicKeys(t *testing.T) {
	r := Report{geocodeEnabled: true, geocodeAttempted: 200, geocodeSuccess: 124}
	v, _ := deriveVerdict(r, baseCfg(), baseOpts())
	b1, _ := json.MarshalIndent(v, "", "  ")
	b2, _ := json.MarshalIndent(v, "", "  ")
	if string(b1) != string(b2) {
		t.Fatal("сериализация недетерминирована")
	}
	// methodology_version идёт первым ключом (стабильный порядок из определения struct).
	if !strings.HasPrefix(strings.TrimSpace(string(b1)), "{\n  \"methodology_version\"") {
		t.Errorf("ожидали methodology_version первым ключом, получили:\n%s", b1[:80])
	}
}
