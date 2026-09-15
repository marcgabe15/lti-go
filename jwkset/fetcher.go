// Package jwkset fetches and caches a platform's published JSON Web Key
// Set, used to resolve the key that signed an incoming id_token by its
// "kid" header.
package jwkset

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	josepkg "github.com/go-jose/go-jose/v3"
)

// Fetcher resolves a signing key by kid from a platform's published JWK
// Set at jwksURI.
type Fetcher interface {
	Key(ctx context.Context, jwksURI, kid string) (any, error)
}

// CachingFetcher fetches and caches JWK Sets over HTTP, keyed by jwksURI.
// On a cache hit that doesn't contain the requested kid, it forces exactly
// one refetch before giving up -- this supports platforms rotating their
// signing key without any coordination.
type CachingFetcher struct {
	hc  *http.Client
	ttl time.Duration

	mu    sync.Mutex
	cache map[string]cacheEntry
}

type cacheEntry struct {
	set       josepkg.JSONWebKeySet
	expiresAt time.Time
}

// NewCachingFetcher returns a CachingFetcher that uses hc (or
// http.DefaultClient if nil) to fetch JWK Sets, caching each for ttl (or 5
// minutes if ttl <= 0).
func NewCachingFetcher(hc *http.Client, ttl time.Duration) *CachingFetcher {
	if hc == nil {
		hc = http.DefaultClient
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &CachingFetcher{hc: hc, ttl: ttl, cache: make(map[string]cacheEntry)}
}

// Key implements Fetcher.
func (f *CachingFetcher) Key(ctx context.Context, jwksURI, kid string) (any, error) {
	set, err := f.get(ctx, jwksURI, false)
	if err != nil {
		return nil, err
	}
	if key, ok := lookup(set, kid); ok {
		return key, nil
	}
	// kid miss: force exactly one refetch in case the platform rotated its
	// signing key since we last cached the set.
	set, err = f.get(ctx, jwksURI, true)
	if err != nil {
		return nil, err
	}
	if key, ok := lookup(set, kid); ok {
		return key, nil
	}
	return nil, fmt.Errorf("jwkset: no key with kid %q found at %s", kid, jwksURI)
}

func (f *CachingFetcher) get(ctx context.Context, jwksURI string, force bool) (josepkg.JSONWebKeySet, error) {
	f.mu.Lock()
	entry, ok := f.cache[jwksURI]
	f.mu.Unlock()
	if ok && !force && time.Now().Before(entry.expiresAt) {
		return entry.set, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURI, nil)
	if err != nil {
		return josepkg.JSONWebKeySet{}, fmt.Errorf("jwkset: build request for %s: %w", jwksURI, err)
	}
	resp, err := f.hc.Do(req)
	if err != nil {
		return josepkg.JSONWebKeySet{}, fmt.Errorf("jwkset: fetch %s: %w", jwksURI, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return josepkg.JSONWebKeySet{}, fmt.Errorf("jwkset: fetch %s: unexpected status %d", jwksURI, resp.StatusCode)
	}
	var set josepkg.JSONWebKeySet
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return josepkg.JSONWebKeySet{}, fmt.Errorf("jwkset: decode %s: %w", jwksURI, err)
	}

	f.mu.Lock()
	f.cache[jwksURI] = cacheEntry{set: set, expiresAt: time.Now().Add(f.ttl)}
	f.mu.Unlock()
	return set, nil
}

func lookup(set josepkg.JSONWebKeySet, kid string) (any, bool) {
	for _, k := range set.Keys {
		if k.KeyID == kid {
			return k.Key, true
		}
	}
	// Some platforms publish a single key and omit "kid" on their tokens.
	if kid == "" && len(set.Keys) == 1 {
		return set.Keys[0].Key, true
	}
	return nil, false
}
