package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"mgl-ime-core/backend/internal/word/mongol"
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
	if !ok || got[0].Word != word || got[0].Order < UserFeedbackBaseOrder || got[1].Word != "base" {
		t.Fatalf("merged candidates: %#v ok=%v", got, ok)
	}

	nextWords, ok := st.GetNextWords(word)
	if !ok || nextWords[0].Word != "ᠬᠡ" || nextWords[1].Word != "ᠨᠡᠭᠡ" || nextWords[2].Word != "old" {
		t.Fatalf("merged next words: %#v ok=%v", nextWords, ok)
	}
	if nextWords[0].Order < UserFeedbackBaseOrder || nextWords[0].Order <= nextWords[1].Order {
		t.Fatalf("feedback next words not ranked first: %#v", nextWords)
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

func TestAddUserScoredWordWeightedNextWords(t *testing.T) {
	dir := t.TempDir()
	full := filepath.Join(dir, "full.json")
	next := filepath.Join(dir, "next.json")
	userFull := filepath.Join(dir, "user_full.json")
	userNext := filepath.Join(dir, "user_next.json")

	word := "ᠮᠤᠩᠭᠤᠯ"
	if err := os.WriteFile(full, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(next, []byte(`{"`+word+`":["base"]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	st, err := NewJSONStoreWithUser(full, next, userFull, userNext)
	if err != nil {
		t.Fatal(err)
	}

	keys := mongol.CandidateKeys(word)
	err = st.AddUserScoredWord(keys, ScoredWord{Word: word}, []ScoredWord{
		{Word: "ᠤᠯᠤᠰ", Order: 10},
		{Word: "ᠬᠡᠯᠡ", Order: 50},
	})
	if err != nil {
		t.Fatal(err)
	}

	got, ok := st.GetNextWords(word)
	if !ok || len(got) < 3 {
		t.Fatalf("GetNextWords = %#v ok=%v", got, ok)
	}
	if got[0].Word != "ᠬᠡᠯᠡ" || got[1].Word != "ᠤᠯᠤᠰ" || got[2].Word != "base" {
		t.Fatalf("weighted order = %#v", got)
	}
	if got[0].Order <= got[1].Order || got[1].Order < UserFeedbackBaseOrder {
		t.Fatalf("feedback scores = %#v", got[:2])
	}
}

func TestAddUserWordMRUBoostsLatestFeedback(t *testing.T) {
	dir := t.TempDir()
	full := filepath.Join(dir, "full.json")
	next := filepath.Join(dir, "next.json")
	userFull := filepath.Join(dir, "user_full.json")
	userNext := filepath.Join(dir, "user_next.json")

	word := "ᠮᠤᠩᠭᠤᠯ"
	if err := os.WriteFile(full, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(next, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	st, err := NewJSONStoreWithUser(full, next, userFull, userNext)
	if err != nil {
		t.Fatal(err)
	}

	keys := mongol.CandidateKeys(word)
	if err := st.AddUserWord(keys, word, []string{"old-a", "old-b"}); err != nil {
		t.Fatal(err)
	}
	if err := st.AddUserWord(keys, word, []string{"new-a"}); err != nil {
		t.Fatal(err)
	}

	got, ok := st.GetNextWords(word)
	if !ok || len(got) < 3 || got[0].Word != "new-a" {
		t.Fatalf("MRU next words = %#v ok=%v", got, ok)
	}
}

func TestNextWordsLookupHitsNormalizedKey(t *testing.T) {
	dir := t.TempDir()
	full := filepath.Join(dir, "full.json")
	next := filepath.Join(dir, "next.json")
	userFull := filepath.Join(dir, "user_full.json")
	userNext := filepath.Join(dir, "user_next.json")

	raw := "ᠪᠠᠷᠠᠭᠠᠨ\u180e"
	norm := mongol.NormalizeLookupKey(raw)
	if err := os.WriteFile(full, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(next, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	st, err := NewJSONStoreWithUser(full, next, userFull, userNext)
	if err != nil {
		t.Fatal(err)
	}

	keys := mongol.CandidateKeys(raw)
	if err := st.AddUserWord(keys, raw, []string{"feedback"}); err != nil {
		t.Fatal(err)
	}

	got, ok := st.GetNextWords(norm)
	if !ok || len(got) == 0 || got[0].Word != "feedback" {
		t.Fatalf("GetNextWords(%q) = %#v ok=%v", norm, got, ok)
	}
}
