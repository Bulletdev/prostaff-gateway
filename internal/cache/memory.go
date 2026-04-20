package cache

import (
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

type entry struct {
	data      []byte
	expiresAt time.Time
}

type Memory struct {
	cache *lru.Cache[string, entry]
}

func NewMemory(gcInterval time.Duration, maxSize int) *Memory {
	c, _ := lru.New[string, entry](maxSize)
	m := &Memory{cache: c}
	go m.gc(gcInterval)
	return m
}

func (m *Memory) Get(key string) ([]byte, bool) {
	v, ok := m.cache.Get(key)
	if !ok {
		return nil, false
	}
	if time.Now().After(v.expiresAt) {
		m.cache.Remove(key)
		return nil, false
	}
	return v.data, true
}

func (m *Memory) Set(key string, data []byte, ttl time.Duration) {
	m.cache.Add(key, entry{
		data:      data,
		expiresAt: time.Now().Add(ttl),
	})
}

func (m *Memory) SetNegative(key string, ttl time.Duration) {
	m.cache.Add("404:"+key, entry{
		data:      nil,
		expiresAt: time.Now().Add(ttl),
	})
}

func (m *Memory) IsNegative(key string) bool {
	v, ok := m.cache.Get("404:" + key)
	if !ok {
		return false
	}
	if time.Now().After(v.expiresAt) {
		m.cache.Remove("404:" + key)
		return false
	}
	return true
}

func (m *Memory) gc(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		for _, key := range m.cache.Keys() {
			if v, ok := m.cache.Peek(key); ok && now.After(v.expiresAt) {
				m.cache.Remove(key)
			}
		}
	}
}
