package util

import (
	"time"
)

const (
	defaultTTL = time.Hour * 24 * 365 * 10 // 10 years
)

type CacheValue[T any] struct {
	Value   T
	Expires time.Time
}

type Cache[T any] struct {
	Data            map[string]CacheValue[T]
	doneChan        chan interface{}
	cleanupInterval time.Duration
}

func NewCache[T any](cleanupInterval time.Duration) *Cache[T] {
	done := make(chan interface{})
	cache := Cache[T]{
		Data:            make(map[string]CacheValue[T]),
		doneChan:        done,
		cleanupInterval: cleanupInterval,
	}

	go func() {
		timer := time.NewTicker(cleanupInterval)
		defer timer.Stop()

		for {
			select {
			case <-timer.C:
				cache.DeleteExpired()
			case <-done:
				return
			}
		}
	}()

	return &cache
}

func (c *Cache[T]) Get(key string) (*T, bool) {
	v, ok := c.Data[key]
	if !ok {
		return nil, false
	}
	return &v.Value, ok
}

func (c *Cache[T]) Set(key string, value T, ttl time.Duration) {
	if ttl <= 0 {
		ttl = defaultTTL
	}

	c.Data[key] = CacheValue[T]{Value: value, Expires: time.Now().Add(ttl)}
}

func (c *Cache[T]) Delete(key string) {
	delete(c.Data, key)
}

func (c *Cache[T]) Clear() {
	c.Data = make(map[string]CacheValue[T])
}

func (c *Cache[T]) DeleteExpired() {
	now := time.Now()
	for k, v := range c.Data {
		if v.Expires.Before(now) {
			delete(c.Data, k)
		}
	}
}

func (c *Cache[T]) Close() {
	close(c.doneChan)
}
