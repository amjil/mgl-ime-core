package word

import (
	"testing"

	"m-v-k/backend/internal/word/mongol"
)

type overlayStore struct {
	candidates map[string][]string
	nextWords  map[string][]string
}

func (s *overlayStore) GetCandidates(word string) ([]string, bool) {
	v, ok := s.candidates[word]
	return v, ok
}

func (s *overlayStore) GetNextWords(word string) ([]string, bool) {
	v, ok := s.nextWords[word]
	return v, ok
}

func (s *overlayStore) AddUserWord(keys []string, word string, nextWords []string) error {
	for _, key := range keys {
		s.candidates[key] = append([]string{word}, s.candidates[key]...)
	}
	if len(nextWords) > 0 {
		s.nextWords[word] = append(nextWords, s.nextWords[word]...)
	}
	return nil
}

func TestAddUserWordInvalidatesCandidateQueryCache(t *testing.T) {
	const key = "ᠣᠬᠯᠬ"
	const added = "ᠣᠬᠢᠯᠠᠬᠤ"

	st := &overlayStore{
		candidates: map[string][]string{key: {"base-only"}},
		nextWords:  map[string][]string{},
	}
	svc := NewService(st, 8)

	before, _ := svc.GetCandidates(key)
	if len(before) != 1 || before[0] != "base-only" {
		t.Fatalf("before add: %#v", before)
	}

	if err := svc.AddUserWord(added, nil); err != nil {
		t.Fatal(err)
	}

	after, _ := svc.GetCandidates(key)
	if len(after) == 0 || after[0] != added {
		t.Fatalf("after add: %#v, want first %q (cache may be stale)", after, added)
	}
}

func TestAddUserWordInvalidatesNextWordQueryCache(t *testing.T) {
	const word = "ᠣᠬᠢᠯᠠᠬᠤ"
	const next = "ᠳᠠᠭᠤᠯᠠᠬᠤ"

	st := &overlayStore{
		candidates: map[string][]string{},
		nextWords:  map[string][]string{word: {"old-next"}},
	}
	svc := NewService(st, 8)

	before, _ := svc.GetNextWords(word)
	if len(before) != 1 || before[0] != "old-next" {
		t.Fatalf("before add: %#v", before)
	}

	if err := svc.AddUserWord(word, []string{next}); err != nil {
		t.Fatal(err)
	}

	after, _ := svc.GetNextWords(word)
	if len(after) == 0 || after[0] != next {
		t.Fatalf("after add: %#v, want first %q", after, next)
	}
}

func TestCandidateKeysForOkhilahu(t *testing.T) {
	word := "ᠣᠬᠢᠯᠠᠬᠤ"
	keys := mongol.CandidateKeys(word)
	if len(keys) != 2 || keys[1] != "ᠣᠬᠯᠬ" {
		t.Fatalf("CandidateKeys = %#v", keys)
	}
}
