package main

import (
	"strings"
	"testing"
)

// TestRenderOnePager_Populated — непустая проекция: таблица N/M/X + вердикт; нейтрально; источник честно
// помечен (синтетика). Ключевые строки запинены ЛИТЕРАЛОМ ([[guards-must-prove-red]]).
func TestRenderOnePager_Populated(t *testing.T) {
	m := Metrics{Marked: 300, ActiveFlags: 12, Recomputable: 10}
	v := VerdictSummary{
		Verdict: "no_go", Reason: "oq4_geo_coverage: недостаточная выборка",
		GeoGateThreshold: 0.7, GeoCoverage: nil, GeoState: "insufficient_sample",
		ContractsInWindow: 300, DataSource: "file:./data", MethodologyVersion: "stage0-1.0",
	}
	out := renderOnePager(m, v, "2026-06-26")

	for _, want := range []string{
		"# Pilot one-pager — AshyqQala.kz (этап 5)",
		"Источник данных: file:./data (синтетика/демо)",
		"| Контрактов размечено (N) | 300 |",
		"| Сигналов поднято (M) | 12 |",
		"| Из них пересчитываемых по методике (X) | 10 |",
		"сигнал, требующий проверки",
		"/api/contracts/{goszakup_id}/flags/{flag_type}/evidence.json",
		"**Вердикт:** no_go — данные пока недостаточны (требуется дозамер)",
		"**Гео-покрытие (OQ4, гейт ≥ 70%):** недостаточно сопоставимых данных (состояние: insufficient_sample)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("one-pager не содержит литерал %q\n---\n%s", want, out)
		}
	}
	assertNeutral(t, out)
}

// TestRenderOnePager_EmptyProjection_HonestNoData — пустая проекция (Marked==0) → честный «данные не загружены»,
// НЕ таблица фабрикованных нулей (negative-control honesty над домыслом, §7.4).
func TestRenderOnePager_EmptyProjection_HonestNoData(t *testing.T) {
	out := renderOnePager(
		Metrics{},
		VerdictSummary{Verdict: "no_go", GeoGateThreshold: 0.7, DataSource: "file:./data"},
		"2026-06-26",
	)
	if !strings.Contains(out, "Недостаточно данных для пилотных метрик") {
		t.Errorf("пустая проекция → ожидался честный «данные не загружены», got:\n%s", out)
	}
	if strings.Contains(out, "размечено (N)") || strings.Contains(out, "| --- | --- |") {
		t.Errorf("пустая проекция НЕ должна показывать таблицу нулей как факт:\n%s", out)
	}
	assertNeutral(t, out)
}

// TestHonestCount — 0 → «нет данных» (не «0» как факт); >0 → число.
func TestHonestCount(t *testing.T) {
	if honestCount(0) != "нет данных" {
		t.Errorf("honestCount(0) = %q; want «нет данных»", honestCount(0))
	}
	if honestCount(42) != "42" {
		t.Errorf("honestCount(42) = %q; want 42", honestCount(42))
	}
}

// tabooRoots — оценочная лексика, запрещённая на любой поверхности (ru-подмножество).
var tabooRoots = []string{"нарушение", "коррупц", "виновен", "преступл"}

func tabooHits(s string) []string {
	lower := strings.ToLower(s)
	var hits []string
	for _, root := range tabooRoots {
		if strings.Contains(lower, root) {
			hits = append(hits, root)
		}
	}
	return hits
}

func assertNeutral(t *testing.T, out string) {
	t.Helper()
	if h := tabooHits(out); len(h) > 0 {
		t.Errorf("one-pager содержит оценочные слова %v", h)
	}
}

// TestTabooScan_RedsOnRealOutput — negative-control: скан ловит taboo, ВПЛЕТЁННОЕ в реальный рендер
// (verdict_reason печатается verbatim) → доказывает, что страж нейтральности краснеет по ПРИЧИНЕ, а не на
// примитиве `strings.Contains` ([[guards-must-prove-red]]).
func TestTabooScan_RedsOnRealOutput(t *testing.T) {
	out := renderOnePager(
		Metrics{Marked: 1, ActiveFlags: 1, Recomputable: 1},
		VerdictSummary{Verdict: "no_go", Reason: "это нарушение закона", GeoGateThreshold: 0.7, DataSource: "file:x"},
		"2026-06-26",
	)
	if hits := tabooHits(out); len(hits) == 0 {
		t.Error("negative-control: скан НЕ покраснел на taboo, впаянном в реальный one-pager — страж фиктивен")
	}
}

// TestDataSourceLabel_HonestNonLive — живой только `ows`; интерим/`scrape:`/нераспознанный помечается как НЕ живой (P3).
func TestDataSourceLabel_HonestNonLive(t *testing.T) {
	cases := map[string]string{
		"ows:v2":          "ows:v2 (живой источник)",
		"file:./data":     "file:./data (синтетика/демо)",
		"scrape:goszakup": "scrape:goszakup (интерим-парсер, не живой)",
		"weird":           "weird (источник не классифицирован — не подтверждён как живой)",
		"":                "нет данных",
	}
	for in, want := range cases {
		if got := dataSourceLabel(in); got != want {
			t.Errorf("dataSourceLabel(%q) = %q; want %q", in, got, want)
		}
	}
}

// TestGateLabel — нулевой/отсутствующий порог → «гейт не задан» (не «≥ 0%» как факт) (P5).
func TestGateLabel(t *testing.T) {
	if got := gateLabel(0); got != "гейт не задан" {
		t.Errorf("gateLabel(0) = %q; want «гейт не задан»", got)
	}
	if got := gateLabel(0.7); got != "гейт ≥ 70%" {
		t.Errorf("gateLabel(0.7) = %q; want «гейт ≥ 70%%»", got)
	}
}
