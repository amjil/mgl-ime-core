package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"m-v-k/backend/internal/word/mongol"
)

func TestJSONStoreUserOverlayMergeAndPersist(t *testing.T) {
	dir := t.TempDir()
	full := filepath.Join(dir, "full.json")
	next := filepath.Join(dir, "next.json")
	userFull := filepath.Join(dir, "user_full.json")
	userNext := filepath.Join(dir, "user_next.json")

	word := "ᠰᠡᠪᠡᠷᠡᠭᠡᠳᠡᠭ"
	abbrev := mongol.AbbrevKey(word)

	if err := os.WriteFile(full, []byte(`{"`+abbrev+`":["base"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(next, []byte(`{"`+word+`":["old"]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	st, err := NewJSONStoreWithUser(full, next, userFull, userNext)
	if err != nil {
		t.Fatal(err)
	}

	keys := mongol.CandidateKeys(word)
	if err := st.AddUserWord(keys, word, []string{"ᠬᠡ", "ᠨᠡᠭᠡ"}); err != nil {
		t.Fatal(err)
	}

	got, ok := st.GetCandidates(abbrev)
	if !ok || got[0] != word || got[1] != "base" {
		t.Fatalf("merged candidates: %#v ok=%v", got, ok)
	}

	nextWords, ok := st.GetNextWords(word)
	if !ok || nextWords[0] != "ᠬᠡ" || nextWords[1] != "ᠨᠡᠭᠡ" || nextWords[2] != "old" {
		t.Fatalf("merged next words: %#v ok=%v", nextWords, ok)
	}

	raw, err := os.ReadFile(userFull)
	if err != nil {
		t.Fatal(err)
	}
	var userFullMap map[string][]string
	if err := json.Unmarshal(raw, &userFullMap); err != nil {
		t.Fatal(err)
	}
	if userFullMap[word][0] != word || userFullMap[abbrev][0] != word {
		t.Fatalf("unexpected user_full.json: %#v", userFullMap)
	}

	raw, err = os.ReadFile(userNext)
	if err != nil {
		t.Fatal(err)
	}
	var userNextMap map[string][]string
	if err := json.Unmarshal(raw, &userNextMap); err != nil {
		t.Fatal(err)
	}
	if userNextMap[word][0] != "ᠬᠡ" || userNextMap[word][1] != "ᠨᠡᠭᠡ" {
		t.Fatalf("unexpected user_next.json: %#v", userNextMap)
	}
}
