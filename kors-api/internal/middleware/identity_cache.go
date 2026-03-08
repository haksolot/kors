package middleware

import (
    "sync"
    "time"
    "github.com/google/uuid"
)

type cachedIdentity struct {
    id        uuid.UUID
    expiresAt time.Time
}

type IdentityCache struct {
    mu    sync.RWMutex
    cache map[string]cachedIdentity
    ttl   time.Duration
}

func NewIdentityCache(ttl time.Duration) *IdentityCache {
    c := &IdentityCache{cache: make(map[string]cachedIdentity), ttl: ttl}
    go c.evict()
    return c
}

func (c *IdentityCache) Get(externalID string) (uuid.UUID, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    entry, ok := c.cache[externalID]
    if !ok || time.Now().After(entry.expiresAt) {
        return uuid.Nil, false
    }
    return entry.id, true
}

func (c *IdentityCache) Set(externalID string, id uuid.UUID) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.cache[externalID] = cachedIdentity{id: id, expiresAt: time.Now().Add(c.ttl)}
}

func (c *IdentityCache) evict() {
    ticker := time.NewTicker(c.ttl)
    for range ticker.C {
        c.mu.Lock()
        for k, v := range c.cache {
            if time.Now().After(v.expiresAt) {
                delete(c.cache, k)
            }
        }
        c.mu.Unlock()
    }
}
