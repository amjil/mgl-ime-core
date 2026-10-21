package store

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRealUserNextBaraganMerge(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Join(filepath.Dir(file), "..", "..", "..")

	userNext := filepath.Join(dir, "user_next.json")
	if _, err := os.Stat(userNext); err != nil {
		t.Skip("user_next.json not present")
	}

	fullPath, err := ResolveDataPath(dir, "full.json")
	if err != nil {
		t.Fatal(err)
	}
	nextPath, err := ResolveDataPath(dir, "next.json")
	if err != nil {
		t.Fatal(err)
	}
	userFull := filepath.Join(dir, "user_full.json")

	st, err := NewJSONStoreWithUser(fullPath, nextPath, userFull, userNext)
	if err != nil {
		t.Fatal(err)
	}

	const key = "ᠪᠠᠷᠠᠭᠠᠨ"
	words, ok := st.GetNextWords(key)
	if !ok {
		t.Fatal("expected hit for baragan")
	}
	if len(words) < 2 || words[0] != "ᠡᠭᠦᠯᠡ" || words[1] != "ᠰᠡᠭᠦᠳᠡᠷ" {
		t.Fatalf("user_next not merged first: %#v", words)
	}
}
