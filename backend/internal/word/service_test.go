package word

import "testing"

type countingStore struct {
	candidates map[string][]string
	nextWords  map[string][]string
	calls      map[string]int
}

func (s *countingStore) GetCandidates(word string) ([]string, bool) {
	s.calls["c:"+word]++
	values, ok := s.candidates[word]
	return values, ok
}

func (s *countingStore) GetNextWords(word string) ([]string, bool) {
	s.calls["n:"+word]++
	values, ok := s.nextWords[word]
	return values, ok
}

func newCountingStore() *countingStore {
	return &countingStore{
		candidates: map[string][]string{
			"a": {"a1"},
			"b": {"b1"},
			"c": {"c1"},
		},
		nextWords: map[string][]string{
			"a": {"an"},
		},
		calls: map[string]int{},
	}
}

func TestServiceCachesHits(t *testing.T) {
	st := newCountingStore()
	svc := NewService(st, 8)

	got, err := svc.GetCandidates("a")
	if err != nil || len(got) != 1 || got[0] != "a1" {
		t.Fatalf("first lookup: %#v err=%v", got, err)
	}
	got, err = svc.GetCandidates("a")
	if err != nil || len(got) != 1 || got[0] != "a1" {
		t.Fatalf("cached lookup: %#v err=%v", got, err)
	}
	if st.calls["c:a"] != 1 {
		t.Fatalf("expected 1 store call, got %d", st.calls["c:a"])
	}
}

func TestServiceNamespacesCaches(t *testing.T) {
	st := newCountingStore()
	svc := NewService(st, 8)

	candidates, _ := svc.GetCandidates("a")
	nextWords, _ := svc.GetNextWords("a")
	if candidates[0] != "a1" || nextWords[0] != "an" {
		t.Fatalf("candidates=%#v next=%#v", candidates, nextWords)
	}
	if st.calls["c:a"] != 1 || st.calls["n:a"] != 1 {
		t.Fatalf("unexpected calls: %#v", st.calls)
	}
}

func TestServiceMissReturnsEmptySlice(t *testing.T) {
	st := newCountingStore()
	svc := NewService(st, 8)

	got, err := svc.GetCandidates("missing")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("expected empty slice, got %#v", got)
	}
	got, err = svc.GetCandidates("missing")
	if err != nil || len(got) != 0 {
		t.Fatalf("cached miss: %#v err=%v", got, err)
	}
	if st.calls["c:missing"] != 1 {
		t.Fatalf("expected miss to be cached, calls=%d", st.calls["c:missing"])
	}
}

func TestServiceReturnedSliceIsIsolated(t *testing.T) {
	st := newCountingStore()
	svc := NewService(st, 8)

	got, _ := svc.GetCandidates("a")
	got[0] = "mutated"
	got, _ = svc.GetCandidates("a")
	if got[0] != "a1" {
		t.Fatalf("cache leaked mutation: %#v", got)
	}
	if st.candidates["a"][0] != "a1" {
		t.Fatalf("store leaked mutation: %#v", st.candidates["a"])
	}
}

func TestServiceLRUEvictsColdKey(t *testing.T) {
	st := newCountingStore()
	svc := NewService(st, 2)

	_, _ = svc.GetCandidates("a")
	_, _ = svc.GetCandidates("b")
	_, _ = svc.GetCandidates("c") // evicts a
	_, _ = svc.GetCandidates("a")

	if st.calls["c:a"] != 2 {
		t.Fatalf("expected a to be evicted, calls=%d", st.calls["c:a"])
	}
}

func TestServiceLRUTouchesOnGet(t *testing.T) {
	st := newCountingStore()
	svc := NewService(st, 2)

	_, _ = svc.GetCandidates("a")
	_, _ = svc.GetCandidates("b")
	_, _ = svc.GetCandidates("a") // touch a, b becomes cold
	_, _ = svc.GetCandidates("c") // evicts b

	_, _ = svc.GetCandidates("a")
	if st.calls["c:a"] != 1 {
		t.Fatalf("hot key a should stay cached, calls=%d", st.calls["c:a"])
	}
	_, _ = svc.GetCandidates("b")
	if st.calls["c:b"] != 2 {
		t.Fatalf("cold key b should be evicted, calls=%d", st.calls["c:b"])
	}
}
