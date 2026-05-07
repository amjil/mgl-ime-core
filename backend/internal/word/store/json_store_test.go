package store

import "testing"

func TestNormalizeWordListSupportsTupleAndString(t *testing.T) {
	got, err := normalizeWordList([]any{
		[]any{"ᠨᠡᠯᠬᠡᠭᠡᠷ", 2.0},
		"ᠨᠡᠯᠪᠡᠭᠡᠷ",
		[]any{"ᠨᠡᠯᠳᠡᠭᠡᠷ", 3.0e-6},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("expected 3 words, got %d", len(got))
	}
}

func TestNormalizeWordListRejectsInvalidTuple(t *testing.T) {
	got, err := normalizeWordList([]any{
		[]any{123, 0.2},
		[]any{"ok", 0.1},
		nil,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != "ok" {
		t.Fatalf("unexpected output: %#v", got)
	}
}
