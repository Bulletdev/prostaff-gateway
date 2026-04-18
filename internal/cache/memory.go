package cache

import (
	"sync"
	"time"
)

type entry struct {
	data      []byte
	expiresAt time.Time
}

// Memory is a thread-safe in-process cache with TTL (L1).
type Memory struct {
	store sync.Map
}

func NewMemory(gcInterval time.Duration) *Memory {
	m := &Memory{}
	go m.gc(gcInterval)
	return m
}

func (m *Memory) Get(key string) ([]byte, bool) {
	v, ok := m.store.Load(key)
	if !ok {
		return nil, false
	}
	e := v.(entry)
	if time.Now().After(e.expiresAt) {
		m.store.Delete(key)
		return nil, false
	}
	return e.data, true
}

func (m *Memory) Set(key string, data []byte, ttl time.Duration) {
	m.store.Store(key, entry{
		data:      data,
		expiresAt: time.Now().Add(ttl),
	})
}

func (m *Memory) gc(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		m.store.Range(func(k, v interface{}) bool {
			if now.After(v.(entry).expiresAt) {
				m.store.Delete(k)
			}
			return true
		})
	}
}
