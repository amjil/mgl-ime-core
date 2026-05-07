package word

import "sync"

type Store interface {
	GetCandidates(word string) ([]string, bool)
	GetNextWords(word string) ([]string, bool)
}

type Service struct {
	store Store
	cache *resultCache
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
	if cached, ok := s.cache.get("c:" + word); ok {
		return cached, nil
	}

	words, ok := s.store.GetCandidates(word)
	if !ok {
		words = []string{}
	}
	s.cache.put("c:"+word, words)
	return cloneSlice(words), nil
}

func (s *Service) GetNextWords(word string) ([]string, error) {
	if cached, ok := s.cache.get("n:" + word); ok {
		return cached, nil
	}

	words, ok := s.store.GetNextWords(word)
	if !ok {
		words = []string{}
	}
	s.cache.put("n:"+word, words)
	return cloneSlice(words), nil
}

func cloneSlice(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}

type resultCache struct {
	mu       sync.RWMutex
	order    []string
	data     map[string][]string
	capacity int
}

func newResultCache(capacity int) *resultCache {
	return &resultCache{
		order:    make([]string, 0, capacity),
		data:     make(map[string][]string, capacity),
		capacity: capacity,
	}
}

func (c *resultCache) get(key string) ([]string, bool) {
	c.mu.RLock()
	value, ok := c.data[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	return cloneSlice(value), true
}

func (c *resultCache) put(key string, value []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.data[key]; !exists {
		c.order = append(c.order, key)
	}
	c.data[key] = cloneSlice(value)

	if len(c.order) <= c.capacity {
		return
	}

	oldest := c.order[0]
	c.order = c.order[1:]
	delete(c.data, oldest)
}
