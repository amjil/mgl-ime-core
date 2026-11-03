package word_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"m-v-k/backend/internal/word"
	"m-v-k/backend/internal/word/store"
)

func TestOkhilahuUserOverlayCandidates(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Join(filepath.Dir(file), "..", "..")

	full, err := store.ResolveDataPath(dir, "full.json")
	if err != nil {
		t.Fatal(err)
	}
	next, err := store.ResolveDataPath(dir, "next.json")
	if err != nil {
		t.Fatal(err)
	}
	userFull := filepath.Join(dir, "user_full.json")
	userNext := filepath.Join(dir, "user_next.json")

	st, err := store.NewJSONStoreWithUser(full, next, userFull, userNext)
	if err != nil {
		t.Fatal(err)
	}

	const key = "ᠣᠬᠯᠬ"
	const want = "ᠣᠬᠢᠯᠠᠬᠤ"

	got, ok := st.GetCandidates(key)
	if !ok || len(got) == 0 || got[0].Word != want {
		t.Fatalf("JSONStore GetCandidates(%q) = %#v ok=%v, want first %q", key, got, ok, want)
	}

	svc := word.NewService(st, 2048)
	first, err := svc.GetCandidates(key)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) == 0 || first[0] != want {
		t.Fatalf("service first query = %#v, want first %q", first, want)
	}
	second, err := svc.GetCandidates(key)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) == 0 || second[0] != want {
		t.Fatalf("service cached query = %#v, want first %q", second, want)
	}
}
