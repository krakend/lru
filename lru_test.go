package lru

import (
	"strconv"
	"testing"
	"time"
)

// setWithTimeout fails the test if Set does not return, instead of hanging
// the whole test binary.
func setWithTimeout(t *testing.T, c *LruCache, key string, value []byte) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		c.Set(key, value)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("Set(%q) did not return: eviction loop is spinning with the lock held", key)
	}
}

func TestSet_updateAdjustsSize(t *testing.T) {
	c, _ := NewLruCache(10000, 10)
	c.Set("a", make([]byte, 10))
	c.Set("b", make([]byte, 20))

	c.Set("a", make([]byte, 100))
	if c.inUseSize != 120 {
		t.Errorf("after growing a: expecting inUseSize 120, got %d", c.inUseSize)
	}

	c.Set("a", make([]byte, 5))
	if c.inUseSize != 25 {
		t.Errorf("after shrinking a: expecting inUseSize 25, got %d", c.inUseSize)
	}
}

// An entry that grows in place and is then deleted used to wrap the uint64
// counter around, and the next Set of a new key spun forever.
func TestDelete_afterGrowingUpdate(t *testing.T) {
	c, _ := NewLruCache(32<<20, 500)
	c.Set("k", make([]byte, 50000))
	c.Set("k", make([]byte, 51000))
	c.Delete("k")

	if c.inUseSize != 0 {
		t.Errorf("expecting inUseSize 0 after deleting the only key, got %d", c.inUseSize)
	}

	setWithTimeout(t, c, "k", make([]byte, 300))
	if _, ok := c.Get("k"); !ok {
		t.Errorf("expecting key k to exist")
	}
}

// Same wrap-around through the other order: a small value stored first
// (an error response), overwritten in place by a larger one, then deleted.
func TestDelete_afterSmallThenLargeUpdate(t *testing.T) {
	c, _ := NewLruCache(32<<20, 500)
	c.Set("other", make([]byte, 10))
	c.Set("k", make([]byte, 300))
	c.Set("k", make([]byte, 50000))
	c.Delete("k")

	if c.inUseSize != 10 {
		t.Errorf("expecting inUseSize 10, got %d", c.inUseSize)
	}

	setWithTimeout(t, c, "k", make([]byte, 300))
}

// Growing an entry in place must count against max size and evict the
// least recently used entries.
func TestSet_updateEvictsDueToSize(t *testing.T) {
	c, _ := NewLruCache(100, 10)
	c.Set("a", make([]byte, 30))
	c.Set("b", make([]byte, 30))
	c.Set("c", make([]byte, 30))

	setWithTimeout(t, c, "c", make([]byte, 60))

	if _, ok := c.Get("a"); ok {
		t.Errorf("expecting key a to be evicted")
	}
	if _, ok := c.Get("c"); !ok {
		t.Errorf("expecting key c to exist")
	}
	if c.inUseSize > c.maxSize {
		t.Errorf("expecting inUseSize <= %d, got %d", c.maxSize, c.inUseSize)
	}
}

func TestSet_driftedCounterDoesNotSpin(t *testing.T) {
	c, _ := NewLruCache(100, 10)
	c.Set("a", make([]byte, 10))
	c.inUseSize += 1000 // a counter that drifted

	setWithTimeout(t, c, "b", make([]byte, 10))
	if c.inUseSize != 0 {
		t.Errorf("expecting inUseSize 0 after emptying the list, got %d", c.inUseSize)
	}
}

func TestNewLruCache(t *testing.T) {
	_, err := NewLruCache(0, 0)
	if err != ErrInvalidMaxSize {
		t.Errorf("expecting ErrInvalidMaxSize, got %v", err)
	}

	_, err = NewLruCache(1, 0)
	if err != ErrInvalidMaxItems {
		t.Errorf("expecting ErrInvalidMaxItems, got %v", err)
	}
}

func TestDelete(t *testing.T) {
	c, _ := NewLruCache(10000, 3)
	c.Set("a", []byte("A"))
	c.Set("b", []byte("B"))
	c.Set("c", []byte("C"))

	c.Delete("a")
	if _, ok := c.Get("a"); ok {
		t.Errorf("expecting key a to be deleted")
	}
}

func TestGet(t *testing.T) {
	c, _ := NewLruCache(10000, 3)

	for i := 1; i <= 3; i++ {
		v := strconv.Itoa(i)
		c.Set(v, []byte(v))
	}

	for i := 1; i <= 3; i++ {
		v := strconv.Itoa(i)
		if _, ok := c.Get(v); !ok {
			t.Errorf("expecting key %s to exist", v)
		}
	}

	if _, ok := c.Get("4"); ok {
		t.Errorf("expecting key 4 to not exist")
	}
}

func TestSet_invalidSize(t *testing.T) {
	c, _ := NewLruCache(10, 10)
	c.Set("too big", []byte("too big to fit the cache size"))

	if _, ok := c.Get("too big"); ok {
		t.Errorf("expecting key a to not exist")
	}
}

func TestSet_evictDueToItems(t *testing.T) {
	c, _ := NewLruCache(10000, 10)

	for i := 1; i <= 10; i++ {
		v := strconv.Itoa(i)
		c.Set(v, []byte(v))
	}

	c.Set("11", []byte("11"))

	if _, ok := c.Get("1"); ok {
		t.Errorf("expecting key 1 to be evicted")
	}

	c.Get("2")

	c.Set("12", []byte("12"))
	if _, ok := c.Get("2"); !ok {
		t.Errorf("expecting key 2 to exist")
	}
}

func TestSet_evictDueToSize(t *testing.T) {
	c, _ := NewLruCache(10, 100)

	for i := 1; i <= 10; i++ {
		v := strconv.Itoa(i)
		c.Set(v, []byte("v"))
	}

	c.Set("11", []byte("v"))

	if _, ok := c.Get("1"); ok {
		t.Errorf("expecting key 1 to be evicted")
	}

	c.Set("12", []byte("vvv"))
	for i := 2; i <= 4; i++ {
		v := strconv.Itoa(i)
		if _, ok := c.Get(v); ok {
			t.Errorf("expecting key %s to be evicted", v)
		}
	}
}

func TestSet_moveToFront(t *testing.T) {
	c, _ := NewLruCache(10000, 10)

	for i := 1; i <= 10; i++ {
		v := strconv.Itoa(i)
		c.Set(v, []byte(v))
	}

	c.Set("1", []byte("1"))
	c.Set("11", []byte("11"))

	if _, ok := c.Get("1"); !ok {
		t.Errorf("expecting key 1 to exist")
	}
	if _, ok := c.Get("2"); ok {
		t.Errorf("expecting key 2 to be evicted")
	}
}
