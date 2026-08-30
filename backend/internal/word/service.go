package word

import (
	"container/list"
	"sync"
)

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
	return s.lookup("c:"+word, word, s.store.GetCandidates), nil
}

func (s *Service) GetNextWords(word string) ([]string, error) {
	return s.lookup("n:"+word, word, s.store.GetNextWords), nil
}

func (s *Service) lookup(cacheKey, word string, load func(string) ([]string, bool)) []string {
	if cached, ok := s.cache.get(cacheKey); ok {
		return cached
	}

	words, ok := load(word)
	if !ok {
		words = []string{}
	}
	s.cache.put(cacheKey, words)
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
