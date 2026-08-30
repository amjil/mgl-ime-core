package store

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestParseWordListSupportsTupleAndString(t *testing.T) {
	got, err := parseWordList(json.RawMessage(`[
		["ᠨᠡᠯᠬᠡᠭᠡᠷ", 2.0],
		"ᠨᠡᠯᠪᠡᠭᠡᠷ",
		["ᠨᠡᠯᠳᠡᠭᠡᠷ", 3.0e-6]
	]`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 words, got %d", len(got))
	}
}

func TestParseWordListSkipsInvalidTuple(t *testing.T) {
	got, err := parseWordList(json.RawMessage(`[
		[123, 0.2],
		["ok", 0.1],
		null
	]`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != "ok" {
		t.Fatalf("unexpected output: %#v", got)
	}
}

func TestParseWordListRejectsNonList(t *testing.T) {
	if _, err := parseWordList(json.RawMessage(`{"a":1}`)); err == nil {
		t.Fatal("expected error for non-list value")
	}
}

func TestResolveDataPathPrefersExactThenGzip(t *testing.T) {
	dir := t.TempDir()
	plain := filepath.Join(dir, "full.json")
	if err := os.WriteFile(plain, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveDataPath(dir, "full.json")
	if err != nil {
		t.Fatal(err)
	}
	if got != plain {
		t.Fatalf("expected %s, got %s", plain, got)
	}

	gzDir := t.TempDir()
	gzPath := filepath.Join(gzDir, "full.json.gz")
	if err := os.WriteFile(gzPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = ResolveDataPath(gzDir, "full.json")
	if err != nil {
		t.Fatal(err)
	}
	if got != gzPath {
		t.Fatalf("expected %s, got %s", gzPath, got)
	}
}

func TestResolveDataPathMissing(t *testing.T) {
	if _, err := ResolveDataPath(t.TempDir(), "full.json"); err == nil {
		t.Fatal("expected missing file error")
	}
}

func TestLoadWordMapPlainAndGzip(t *testing.T) {
	payload := `{"a":[["an", 1], "ab"],"empty":[]}`
	dir := t.TempDir()

	plain := filepath.Join(dir, "data.json")
	if err := os.WriteFile(plain, []byte(payload), 0o644); err != nil {
		t.Fatal(err)
	}

	gzPath := filepath.Join(dir, "data.json.gz")
	file, err := os.Create(gzPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := gzip.NewWriter(file)
	if _, err := zw.Write([]byte(payload)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{plain, gzPath} {
		got, err := loadWordMap(path)
		if err != nil {
			t.Fatalf("load %s: %v", path, err)
		}
		if len(got["a"]) != 2 || got["a"][0] != "an" || got["a"][1] != "ab" {
			t.Fatalf("unexpected map for %s: %#v", path, got)
		}
		if got["empty"] == nil || len(got["empty"]) != 0 {
			t.Fatalf("expected empty list for %s: %#v", path, got["empty"])
		}
	}
}

func TestNewJSONStoreLoadsBothFiles(t *testing.T) {
	dir := t.TempDir()
	full := filepath.Join(dir, "full.json")
	next := filepath.Join(dir, "next.json")
	if err := os.WriteFile(full, []byte(`{"a":["aa"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(next, []byte(`{"a":[["ab","12"]]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	st, err := NewJSONStore(full, next)
	if err != nil {
		t.Fatal(err)
	}
	words, ok := st.GetCandidates("a")
	if !ok || len(words) != 1 || words[0] != "aa" {
		t.Fatalf("candidates: %#v ok=%v", words, ok)
	}
	nextWords, ok := st.GetNextWords("a")
	if !ok || len(nextWords) != 1 || nextWords[0] != "ab" {
		t.Fatalf("next words: %#v ok=%v", nextWords, ok)
	}
	if _, ok := st.GetCandidates("missing"); ok {
		t.Fatal("expected miss")
	}
}

func TestResolveRealDataFiles(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Join(filepath.Dir(file), "..", "..", "..")

	full, err := ResolveDataPath(dir, "full.json")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(full) != "full.json" && filepath.Base(full) != "full.json.gz" {
		t.Fatalf("unexpected full path: %s", full)
	}

	next, err := ResolveDataPath(dir, "next.json")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(next) != "next.json" && filepath.Base(next) != "next.json.gz" {
		t.Fatalf("unexpected next path: %s", next)
	}
}

func TestLoadRealNextJSON(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "..", "next.json")
	if _, err := os.Stat(path); err != nil {
		t.Skip("next.json not present")
	}

	got, err := loadWordMap(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 1000 {
		t.Fatalf("expected a large next-word map, got %d keys", len(got))
	}
}
