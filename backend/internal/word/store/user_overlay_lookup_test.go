package store

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCandidatesOverlayMatchesAbbrevWhenOnlyFullWordKeyStored(t *testing.T) {
	dir := t.TempDir()
	full := filepath.Join(dir, "full.json")
	next := filepath.Join(dir, "next.json")
	userFull := filepath.Join(dir, "user_full.json")
	userNext := filepath.Join(dir, "user_next.json")

	const abbrev = "ᠬᠨ"
	const word = "ᠬᠠᠨᠢ"

	if err := os.WriteFile(full, []byte(`{"`+abbrev+`":["ᠬᠡᠨ","ᠬᠢᠨ"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(next, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// Only full-word lookup key — no explicit abbrev index entry.
	if err := os.WriteFile(userFull, []byte(`{"`+word+`":["`+word+`"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userNext, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	st, err := NewJSONStoreWithUser(full, next, userFull, userNext)
	if err != nil {
		t.Fatal(err)
	}

	got, ok := st.GetCandidates(abbrev)
	if !ok {
		t.Fatal("expected hit")
	}
	if len(got) == 0 || got[0].Word != word || got[0].Order < UserFeedbackBaseOrder {
		t.Fatalf("GetCandidates(%q) = %#v, want %q first", abbrev, got, word)
	}
}

func TestRealKhaniUserOverlayAbbrev(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Join(filepath.Dir(file), "..", "..", "..")

	userFull := filepath.Join(dir, "user_full.json")
	if _, err := os.Stat(userFull); err != nil {
		t.Skip("user_full.json not present")
	}

	fullPath, err := ResolveDataPath(dir, "full.json")
	if err != nil {
		t.Fatal(err)
	}
	nextPath, err := ResolveDataPath(dir, "next.json")
	if err != nil {
		t.Fatal(err)
	}
	userNext := filepath.Join(dir, "user_next.json")

	st, err := NewJSONStoreWithUser(fullPath, nextPath, userFull, userNext)
	if err != nil {
		t.Fatal(err)
	}

	const abbrev = "ᠬᠨ"
	const word = "ᠬᠠᠨᠢ"

	got, ok := st.GetCandidates(abbrev)
	if !ok || len(got) == 0 || got[0].Word != word || got[0].Order < UserFeedbackBaseOrder {
		t.Fatalf("GetCandidates(%q) = %#v ok=%v, want first %q", abbrev, got, ok, word)
	}
}
