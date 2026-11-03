package word

import (
	"container/list"
	"errors"
	"sync"

	"m-v-k/backend/internal/word/mongol"
	"m-v-k/backend/internal/word/store"
)

type Store interface {
	GetCandidates(word string) ([]store.ScoredWord, bool)
	GetNextWords(word string) ([]store.ScoredWord, bool)
}

type UserWordStore interface {
	Store
	AddUserWord(keys []string, word string, nextWords []string) error
}

type UserOverlayGenerationStore interface {
	UserOverlayGeneration() (uint64, bool)
}

type Service struct {
	store Store
	cache *resultCache

	mu               sync.Mutex
	cacheEpoch       uint64
	observedStoreGen uint64
	storeGenReady    bool
}

func NewService(store Store, cacheCapacity int) *Service {
	if cacheCapacity <= 0 {
		cacheCapacity = 1024
	}
	return &Service{
		store: store,
		cache: newResultCache(cacheCapacity),
	}
}

func (s *Service) GetCandidates(word string) ([]store.ScoredWord, error) {
	s.syncUserOverlayCache()
	return s.lookup("c:"+word, word, s.store.GetCandidates), nil
}

func (s *Service) GetNextWords(word string) ([]store.ScoredWord, error) {
	s.syncUserOverlayCache()
	return s.lookup("n:"+word, word, s.store.GetNextWords), nil
}

func (s *Service) syncUserOverlayCache() {
	genStore, ok := s.store.(UserOverlayGenerationStore)
	if !ok {
		return
	}
	gen, ok := genStore.UserOverlayGeneration()
	if !ok {
		return
	}

	s.mu.Lock()
	if !s.storeGenReady {
		s.storeGenReady = true
		s.observedStoreGen = gen
		s.mu.Unlock()
		return
	}
	changed := gen != s.observedStoreGen
	if changed {
		s.observedStoreGen = gen
		s.cacheEpoch++
	}
	s.mu.Unlock()
	if changed {
		s.cache.clear()
	}
}

func (s *Service) invalidateCache() {
	if genStore, ok := s.store.(UserOverlayGenerationStore); ok {
		if gen, ok := genStore.UserOverlayGeneration(); ok {
			s.mu.Lock()
			s.storeGenReady = true
			s.observedStoreGen = gen
			s.cacheEpoch++
			s.mu.Unlock()
			s.cache.clear()
			return
		}
	}
	s.mu.Lock()
	s.cacheEpoch++
	s.mu.Unlock()
	s.cache.clear()
}

func (s *Service) epoch() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cacheEpoch
}

func (s *Service) AddUserWord(word string, nextWords []string) error {
	userStore, ok := s.store.(UserWordStore)
	if !ok {
		return errors.New("user dictionary is not configured")
	}

	keys := mongol.CandidateKeys(word)
	if len(keys) == 0 {
		return errors.New("word is required")
	}

	if err := userStore.AddUserWord(keys, word, nextWords); err != nil {
		return err
	}

	// Drop every cached query, including spellings that do not match the
	// stored lookup keys. The next request reads the updated overlay.
	s.invalidateCache()
	return nil
}

func (s *Service) lookup(cacheKey, word string, load func(string) ([]store.ScoredWord, bool)) []store.ScoredWord {
	epoch := s.epoch()
	if cached, ok := s.cache.get(cacheKey); ok {
		return cached
	}

	words, ok := load(word)
	if !ok {
		words = []store.ScoredWord{}
	}
	// Skip the put when feedback landed while this load was in flight.
	// Otherwise a slow read of the old list would be cached again.
	if ok && len(words) > 0 && s.epoch() == epoch {
		s.cache.put(cacheKey, words)
	}
	return cloneScored(words)
}

func cloneScored(values []store.ScoredWord) []store.ScoredWord {
	if len(values) == 0 {
		return []store.ScoredWord{}
	}
	out := make([]store.ScoredWord, len(values))
	copy(out, values)
	return out
}

type cacheEntry struct {
	key   string
	value []store.ScoredWord
}

type resultCache struct {
	mu       sync.Mutex
	ll       *list.List
	items    map[string]*list.Element
	capacity int
}

func newResultCache(capacity int) *resultCache {
	if capacity <= 0 {
		capacity = 1
	}
	return &resultCache{
		ll:       list.New(),
		items:    make(map[string]*list.Element, capacity),
		capacity: capacity,
	}
}

func (c *resultCache) get(key string) ([]store.ScoredWord, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	el, ok := c.items[key]
	if !ok {
		return nil, false
	}
	c.ll.MoveToFront(el)
	return cloneScored(el.Value.(*cacheEntry).value), true
}

func (c *resultCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ll.Init()
	c.items = make(map[string]*list.Element, c.capacity)
}

func (c *resultCache) put(key string, value []store.ScoredWord) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cloned := cloneScored(value)
	if el, ok := c.items[key]; ok {
		el.Value.(*cacheEntry).value = cloned
		c.ll.MoveToFront(el)
		return
	}

	el := c.ll.PushFront(&cacheEntry{key: key, value: cloned})
	c.items[key] = el
	if c.ll.Len() <= c.capacity {
		return
	}

	oldest := c.ll.Back()
	if oldest == nil {
		return
	}
	c.ll.Remove(oldest)
	delete(c.items, oldest.Value.(*cacheEntry).key)
}
