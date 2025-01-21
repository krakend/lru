lru
=========

A simple LRU cache implementation that also evicts elements when an specific size threshold is met

### Usage

```
go get github.com/krakend/lru
```

```go
maxSize := (1024 * 1024 * 100) // 100 MB
maxItems := 100000

cache, err := lru.NewLruCache(maxSize, maxItems)

cache.Set("key1", "value")

value, exists := cache.Get("key1")

cache.Delete("key1")
```