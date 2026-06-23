package goszakup_test

import (
	"testing"

	"ashyqqala/server/internal/goszakup"
)

func TestFileSource_ReadsLots(t *testing.T) {
	src := goszakup.NewFileSource("testdata")
	var got []map[string]any
	err := src.Fetch("lots", nil, 0, func(items []map[string]any) error {
		got = append(got, items...)
		return nil
	})
	if err != nil {
		t.Fatalf("Fetch lots: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ожидалось 2 лота, получено %d", len(got))
	}
	if got[0]["lot_id"] != "LOT-0001" {
		t.Errorf("lot_id[0] = %v, ожидалось LOT-0001", got[0]["lot_id"])
	}
}

func TestFileSource_Max(t *testing.T) {
	src := goszakup.NewFileSource("testdata")
	var n int
	if err := src.Fetch("lots", nil, 1, func(items []map[string]any) error {
		n = len(items)
		return nil
	}); err != nil {
		t.Fatalf("Fetch lots (max=1): %v", err)
	}
	if n != 1 {
		t.Fatalf("max=1: ожидался 1 элемент, получено %d", n)
	}
}

func TestFileSource_MissingFile_HonestError(t *testing.T) {
	src := goszakup.NewFileSource("testdata")
	err := src.Fetch("nonexistent", nil, 0, func([]map[string]any) error { return nil })
	if err == nil {
		t.Fatal("ожидалась честная ошибка на отсутствующем файле, получено nil")
	}
}

func TestFileSource_Name(t *testing.T) {
	if goszakup.NewFileSource("/x/y").Name() != "file:/x/y" {
		t.Fatalf("Name() = %q", goszakup.NewFileSource("/x/y").Name())
	}
}
