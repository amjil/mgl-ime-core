package word

import (
	"container/list"
	"errors"
	"sync"

	"m-v-k/backend/internal/word/mongol"
)

type Store interface {
	GetCandidates(word string) ([]string, bool)
	GetNextWords(word string) ([]string, bool)
}

type UserWordStore interface {
	Store
	AddUserWord(keys []string, word string, nextWords []string) error
}

type UserOverlayGenerationStore interface {
	UserOverlayGeneration() (uint64, bool)
}

type Service struct {
	store           Store
	cache           *resultCache
	userOverlayGen  uint64
	userOverlaySeen bool
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

func (s *Service) GetCandidates(word string) ([]string, error) {
	s.syncUserOverlayCache()
	return s.lookup("c:"+word, word, s.store.GetCandidates), nil
}

func (s *Service) GetNextWords(word string) ([]string, error) {
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
	if !s.userOverlaySeen {
		s.userOverlaySeen = true
		s.userOverlayGen = gen
		return
	}
	if gen != s.userOverlayGen {
		s.cache.clear()
		s.userOverlayGen = gen
	}
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

	for _, key := range keys {
		s.cache.remove("c:" + key)
		for _, variant := range mongol.LookupKeyVariants(key) {
			s.cache.remove("c:" + variant)
		}
	}
	for _, key := range mongol.LookupKeyVariants(word) {
		s.cache.remove("n:" + key)
	}
	s.syncUserOverlayCache()
	return nil
}

func (s *Service) lookup(cacheKey, word string, load func(string) ([]string, bool)) []string {
	if cached, ok := s.cache.get(cacheKey); ok {
		return cached
	}

	words, ok := load(word)
	if !ok {
		words = []string{}
	}
	if ok && len(words) > 0 {
		s.cache.put(cacheKey, words)
	}
	return cloneSlice(words)
}

func cloneSlice(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}

type cacheEntry struct {
	key   string
	value []string
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

func (c *resultCache) get(key string) ([]string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	el, ok := c.items[key]
	if !ok {
		return nil, false
	}
	c.ll.MoveToFront(el)
	return cloneSlice(el.Value.(*cacheEntry).value), true
}

func (c *resultCache) remove(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	el, ok := c.items[key]
	if !ok {
		return
	}
	c.ll.Remove(el)
	delete(c.items, key)
}

func (c *resultCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ll.Init()
	c.items = make(map[string]*list.Element, c.capacity)
}

func (c *resultCache) put(key string, value []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cloned := cloneSlice(value)
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
