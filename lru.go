package lru

import (
	"container/list"
	"errors"
	"sync"
)

type LruCache struct {
	maxItems  uint64
	maxSize   uint64
	inUseSize uint64

	mu sync.RWMutex

	evictList *list.List
	items     map[string]*list.Element
}

type entry struct {
	key   string
	value []byte
}

var (
	ErrInvalidMaxSize  = errors.New("invalid max size, must be greater than 0")
	ErrInvalidMaxItems = errors.New("invalid max items, must be greater than 0")
)

func NewLruCache(maxBytes, maxItems uint64) (*LruCache, error) {
	if maxBytes <= 0 {
		return nil, ErrInvalidMaxSize
	}
	if maxItems <= 0 {
		return nil, ErrInvalidMaxItems
	}

	return &LruCache{
		maxItems:  maxItems,
		maxSize:   maxBytes,
		items:     make(map[string]*list.Element),
		evictList: list.New(),
	}, nil
}

func (c *LruCache) Get(key string) (res []byte, status bool) {
	c.mu.Lock()

	var ent *list.Element
	if ent, status = c.items[key]; status {
		c.evictList.MoveToFront(ent)
		res = ent.Value.(*entry).value
	}

	c.mu.Unlock()
	return
}

func (c *LruCache) Set(key string, resp []byte) {
	if len(resp) >= int(c.maxSize) {
		return
	}

	c.mu.Lock()

	// Check for existing item
	if ent, ok := c.items[key]; ok {
		c.evictList.MoveToFront(ent)
		ent.Value.(*entry).value = resp
		c.mu.Unlock()
		return
	}

	// Add new item
	c.items[key] = c.evictList.PushFront(&entry{key, resp})

	c.inUseSize += uint64(len(resp))
	// Verify size not exceeded and evict if necessary
	for c.inUseSize > c.maxSize || uint64(len(c.items)) > c.maxItems {
		if e := c.evictList.Back(); e != nil {
			delete(c.items, e.Value.(*entry).key)
			c.inUseSize -= uint64(len(e.Value.(*entry).value))
			c.evictList.Remove(e)
		}
	}
	c.mu.Unlock()
}

func (c *LruCache) Delete(key string) {
	c.mu.Lock()

	if ent, ok := c.items[key]; ok {
		delete(c.items, ent.Value.(*entry).key)
		c.inUseSize -= uint64(len(ent.Value.(*entry).value))
		c.evictList.Remove(ent)
	}

	c.mu.Unlock()
}
