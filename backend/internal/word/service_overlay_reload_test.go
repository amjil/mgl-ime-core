package word_test

import (
	"os"
	"path/filepath"
	"testing"

	"m-v-k/backend/internal/word"
	"m-v-k/backend/internal/word/store"
)

func TestServiceSeesUserOverlayAfterExternalFileUpdate(t *testing.T) {
	dir := t.TempDir()

	full := filepath.Join(dir, "full.json")
	next := filepath.Join(dir, "next.json")
	userFull := filepath.Join(dir, "user_full.json")
	userNext := filepath.Join(dir, "user_next.json")

	const key = "ᠣᠬᠯᠬ"
	const added = "ᠣᠬᠢᠯᠠᠬᠤ"

	if err := os.WriteFile(full, []byte(`{"`+key+`":["base-only"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(next, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userFull, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userNext, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	st, err := store.NewJSONStoreWithUser(full, next, userFull, userNext)
	if err != nil {
		t.Fatal(err)
	}
	svc := word.NewService(st, 8)

	before, err := svc.GetCandidates(key)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 1 || before[0] != "base-only" {
		t.Fatalf("before file update: %#v", before)
	}

	// Simulate feedback written directly to disk (not via POST /api/user_words).
	payload := `{"` + key + `":["` + added + `"]}`
	if err := os.WriteFile(userFull, []byte(payload), 0o644); err != nil {
		t.Fatal(err)
	}

	after, err := svc.GetCandidates(key)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) == 0 || after[0].Word != added {
		t.Fatalf("after file update: %#v, want first %q (service cache must refresh)", after, added)
	}
}
