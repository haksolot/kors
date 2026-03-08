package middleware

import (
    "context"
    "crypto/rsa"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "math/big"
    "net/http"
    "sync"
    "time"
)

type JWKSCache struct {
    mu        sync.RWMutex
    keys      map[string]*rsa.PublicKey
    fetchedAt time.Time
    ttl       time.Duration
    endpoint  string
}

func NewJWKSCache(endpoint string, ttl time.Duration) *JWKSCache {
    return &JWKSCache{endpoint: endpoint, ttl: ttl, keys: make(map[string]*rsa.PublicKey)}
}

// GetKey retourne la cle publique RSA pour un kid donne.
// Rafraichit le cache si expire.
func (c *JWKSCache) GetKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
    c.mu.RLock()
    if time.Since(c.fetchedAt) < c.ttl {
        if key, ok := c.keys[kid]; ok {
            c.mu.RUnlock()
            return key, nil
        }
    }
    c.mu.RUnlock()
    return c.refresh(ctx, kid)
}

func (c *JWKSCache) refresh(ctx context.Context, kid string) (*rsa.PublicKey, error) {
    c.mu.Lock()
    defer c.mu.Unlock()

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint, nil)
    if err != nil {
        return nil, err
    }
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
    }
    defer resp.Body.Close()

    var jwks struct {
        Keys []struct {
            Kid string `json:"kid"`
            N   string `json:"n"`
            E   string `json:"e"`
        } `json:"keys"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
        return nil, fmt.Errorf("failed to decode JWKS: %w", err)
    }

    c.keys = make(map[string]*rsa.PublicKey)
    c.fetchedAt = time.Now()

    for _, k := range jwks.Keys {
        nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
        if err != nil {
            continue
        }
        eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
        if err != nil {
            continue
        }
        e := int(new(big.Int).SetBytes(eBytes).Int64())
        pub := &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}
        c.keys[k.Kid] = pub
    }

    if key, ok := c.keys[kid]; ok {
        return key, nil
    }
    return nil, fmt.Errorf("key %q not found in JWKS", kid)
}
