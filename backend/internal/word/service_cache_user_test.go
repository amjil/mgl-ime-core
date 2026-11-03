package word

import (
	"testing"

	"m-v-k/backend/internal/word/mongol"
	"m-v-k/backend/internal/word/store"
)

type overlayStore struct {
	candidates map[string][]store.ScoredWord
	nextWords  map[string][]store.ScoredWord
}

func (s *overlayStore) GetCandidates(word string) ([]store.ScoredWord, bool) {
	v, ok := s.candidates[word]
	return v, ok
}

func (s *overlayStore) GetNextWords(word string) ([]store.ScoredWord, bool) {
	v, ok := s.nextWords[word]
	return v, ok
}

func (s *overlayStore) AddUserWord(keys []string, word string, nextWords []string) error {
	added := store.ScoredWord{Word: word, Order: store.UserFeedbackBaseOrder}
	for _, key := range keys {
		s.candidates[key] = append([]store.ScoredWord{added}, s.candidates[key]...)
	}
	if len(nextWords) > 0 {
		ranked := make([]store.ScoredWord, 0, len(nextWords))
		for i, w := range nextWords {
			ranked = append(ranked, store.ScoredWord{
				Word:  w,
				Order: store.UserFeedbackBaseOrder - float64(i),
			})
		}
		s.nextWords[word] = append(ranked, s.nextWords[word]...)
	}
	return nil
}

func TestAddUserWordInvalidatesCandidateQueryCache(t *testing.T) {
	const key = "ᠣᠬᠯᠬ"
	const added = "ᠣᠬᠢᠯᠠᠬᠤ"

	st := &overlayStore{
		candidates: map[string][]store.ScoredWord{
			key: {{Word: "base-only", Order: 1}},
		},
		nextWords: map[string][]store.ScoredWord{},
	}
	svc := NewService(st, 8)

	before, _ := svc.GetCandidates(key)
	if len(before) != 1 || before[0].Word != "base-only" {
		t.Fatalf("before add: %#v", before)
	}

	if err := svc.AddUserWord(added, nil); err != nil {
		t.Fatal(err)
	}

	after, _ := svc.GetCandidates(key)
	if len(after) == 0 || after[0].Word != added {
		t.Fatalf("after add: %#v, want first %q (cache may be stale)", after, added)
	}
}

func TestAddUserWordInvalidatesNextWordQueryCache(t *testing.T) {
	const word = "ᠣᠬᠢᠯᠠᠬᠤ"
	const next = "ᠳᠠᠭᠤᠯᠠᠬᠤ"

	st := &overlayStore{
		candidates: map[string][]store.ScoredWord{},
		nextWords: map[string][]store.ScoredWord{
			word: {{Word: "old-next", Order: 1}},
		},
	}
	svc := NewService(st, 8)

	before, _ := svc.GetNextWords(word)
	if len(before) != 1 || before[0].Word != "old-next" {
		t.Fatalf("before add: %#v", before)
	}

	if err := svc.AddUserWord(word, []string{next}); err != nil {
		t.Fatal(err)
	}

	after, _ := svc.GetNextWords(word)
	if len(after) == 0 || after[0].Word != next {
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
