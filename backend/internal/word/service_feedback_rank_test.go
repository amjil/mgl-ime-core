package word_test

import (
	"os"
	"path/filepath"
	"testing"

	"m-v-k/backend/internal/word"
	"m-v-k/backend/internal/word/store"
)

func TestFeedbackAppearsOnNextQueryAndRanksFirst(t *testing.T) {
	dir := t.TempDir()
	full := filepath.Join(dir, "full.json")
	next := filepath.Join(dir, "next.json")
	userFull := filepath.Join(dir, "user_full.json")
	userNext := filepath.Join(dir, "user_next.json")

	const abbrev = "ᠬᠨ"
	const feedbackWord = "ᠬᠠᠨᠢ"
	const feedbackNext = "ᠨᠥᠬᠥᠷ"

	if err := os.WriteFile(full, []byte(`{"`+abbrev+`":[["ᠬᠡᠨ",36741720],["ᠬᠢᠨ",10]]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(next, []byte(`{"`+feedbackWord+`":[["ᠬᠡ","57223"]]}`), 0o644); err != nil {
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

	// A trailing MVS makes the cache key differ from the stored abbrev key.
	query := abbrev + "\u180e"
	before, err := svc.GetCandidates(query)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) == 0 || before[0].Word != "ᠬᠡᠨ" {
		t.Fatalf("before feedback: %#v", before)
	}
	if _, err := svc.GetNextWords(feedbackWord); err != nil {
		t.Fatal(err)
	}

	if err := svc.AddUserWord(feedbackWord, []string{feedbackNext}); err != nil {
		t.Fatal(err)
	}

	after, err := svc.GetCandidates(query)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) < 2 || after[0].Word != feedbackWord || after[0].Order < store.UserFeedbackBaseOrder {
		t.Fatalf("candidates after feedback: %#v", after)
	}
	if after[1].Word != "ᠬᠡᠨ" {
		t.Fatalf("dictionary order after feedback word: %#v", after)
	}

	again, err := svc.GetCandidates(query)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) == 0 || again[0].Word != feedbackWord {
		t.Fatalf("cached candidates after feedback: %#v", again)
	}

	nextWords, err := svc.GetNextWords(feedbackWord)
	if err != nil {
		t.Fatal(err)
	}
	if len(nextWords) < 2 || nextWords[0].Word != feedbackNext || nextWords[0].Order < store.UserFeedbackBaseOrder {
		t.Fatalf("next words after feedback: %#v", nextWords)
	}
	if nextWords[1].Word != "ᠬᠡ" || nextWords[1].Order != 57223 {
		t.Fatalf("dictionary next word: %#v", nextWords)
	}

	byAbbrev, err := svc.GetNextWords(abbrev)
	if err != nil {
		t.Fatal(err)
	}
	if len(byAbbrev) == 0 || byAbbrev[0].Word != feedbackNext || byAbbrev[0].Order < store.UserFeedbackBaseOrder {
		t.Fatalf("abbrev next words after feedback: %#v", byAbbrev)
	}
}
