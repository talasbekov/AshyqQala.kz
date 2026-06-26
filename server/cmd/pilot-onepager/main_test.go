package main

import (
	"context"
	"testing"
)

// TestReadVerdict_RealArtifact — парсинг ЗАКОММИЧЕННОГО вердикта Stage-0 (токен-независимо, без БД): one-pager
// реально опирается на артефакт Epic 0 (AC-3). Пинит текущее состояние артефакта.
func TestReadVerdict_RealArtifact(t *testing.T) {
	vs, err := readVerdict("../../../docs/ops/stage0-verdict-20260620.json")
	if err != nil {
		t.Fatalf("readVerdict: %v", err)
	}
	if vs.Verdict != "no_go" {
		t.Errorf("verdict = %q; want no_go (текущий артефакт)", vs.Verdict)
	}
	if vs.DataSource != "file:./data" {
		t.Errorf("data_source = %q; want file:./data (синтетика)", vs.DataSource)
	}
	if vs.GeoCoverage != nil {
		t.Errorf("geo coverage должен быть nil (insufficient), got %v", *vs.GeoCoverage)
	}
	if vs.ContractsInWindow != 300 {
		t.Errorf("contracts_in_window = %d; want 300", vs.ContractsInWindow)
	}
	if vs.GeoGateThreshold != 0.7 {
		t.Errorf("geo_gate_threshold = %v; want 0.7", vs.GeoGateThreshold)
	}
}

type stubQuerier struct{ marked, active, recomp int64 }

func (s stubQuerier) CountMarkedContracts(context.Context) (int64, error)   { return s.marked, nil }
func (s stubQuerier) CountActiveFlags(context.Context) (int64, error)       { return s.active, nil }
func (s stubQuerier) CountRecomputableFlags(context.Context) (int64, error) { return s.recomp, nil }

// TestReadMetrics — агрегатор N/M/X из querier (без БД, мок).
func TestReadMetrics(t *testing.T) {
	m, err := readMetrics(context.Background(), stubQuerier{marked: 300, active: 12, recomp: 10})
	if err != nil {
		t.Fatal(err)
	}
	if m.Marked != 300 || m.ActiveFlags != 12 || m.Recomputable != 10 {
		t.Errorf("metrics = %+v; want {300,12,10}", m)
	}
}
