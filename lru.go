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

	if ent, ok := c.items[key]; ok {
		// Existing item: swap the value and keep inUseSize in step with it,
		// so a later release subtracts exactly what was added.
		c.evictList.MoveToFront(ent)
		e := ent.Value.(*entry)
		c.inUseSize = c.inUseSize - uint64(len(e.value)) + uint64(len(resp))
		e.value = resp
	} else {
		c.items[key] = c.evictList.PushFront(&entry{key, resp})
		c.inUseSize += uint64(len(resp))
	}

	// Verify size not exceeded and evict if necessary. Stop once the list
	// is empty, so a counter that drifted can never spin with the lock held.
	for c.inUseSize > c.maxSize || uint64(len(c.items)) > c.maxItems {
		e := c.evictList.Back()
		if e == nil {
			c.inUseSize = 0
			break
		}
		c.release(e)
	}
	c.mu.Unlock()
}

func (c *LruCache) Delete(key string) {
	c.mu.Lock()

	if ent, ok := c.items[key]; ok {
		c.release(ent)
	}

	c.mu.Unlock()
}

func (c *LruCache) release(element *list.Element) {
	if element == nil {
		return
	}

	delete(c.items, element.Value.(*entry).key)
	c.inUseSize -= uint64(len(element.Value.(*entry).value))
	c.evictList.Remove(element)
	element.Value = nil
}
