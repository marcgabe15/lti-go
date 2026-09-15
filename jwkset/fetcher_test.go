package jwkset_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	josepkg "github.com/go-jose/go-jose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marcgabe15/lti-go/jwkset"
)

func serveJWKS(t *testing.T, keys ...josepkg.JSONWebKey) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		set := josepkg.JSONWebKeySet{Keys: keys}
		_ = json.NewEncoder(w).Encode(set)
	}))
}

func TestCachingFetcher_ResolvesByKID(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	srv := serveJWKS(t, josepkg.JSONWebKey{Key: &priv.PublicKey, KeyID: "kid-1", Algorithm: "RS256", Use: "sig"})
	defer srv.Close()

	f := jwkset.NewCachingFetcher(nil, 0)
	key, err := f.Key(context.Background(), srv.URL, "kid-1")
	require.NoError(t, err)
	assert.Equal(t, &priv.PublicKey, key)
}

func TestCachingFetcher_KIDMissForcesRefetch(t *testing.T) {
	priv1, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	priv2, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	requestCount := 0
	current := josepkg.JSONWebKey{Key: &priv1.PublicKey, KeyID: "kid-1", Algorithm: "RS256", Use: "sig"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount > 1 {
			current = josepkg.JSONWebKey{Key: &priv2.PublicKey, KeyID: "kid-2", Algorithm: "RS256", Use: "sig"}
		}
		_ = json.NewEncoder(w).Encode(josepkg.JSONWebKeySet{Keys: []josepkg.JSONWebKey{current}})
	}))
	defer srv.Close()

	f := jwkset.NewCachingFetcher(nil, 0)
	// Prime the cache with kid-1.
	_, err = f.Key(context.Background(), srv.URL, "kid-1")
	require.NoError(t, err)
	assert.Equal(t, 1, requestCount)

	// Asking for kid-2 (not in the cached set) should force exactly one
	// refetch, which now returns kid-2 -- simulating the platform having
	// rotated its signing key.
	key, err := f.Key(context.Background(), srv.URL, "kid-2")
	require.NoError(t, err)
	assert.Equal(t, &priv2.PublicKey, key)
	assert.Equal(t, 2, requestCount)
}

func TestCachingFetcher_UnknownKIDErrors(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	srv := serveJWKS(t, josepkg.JSONWebKey{Key: &priv.PublicKey, KeyID: "kid-1", Algorithm: "RS256", Use: "sig"})
	defer srv.Close()

	f := jwkset.NewCachingFetcher(nil, 0)
	_, err = f.Key(context.Background(), srv.URL, "does-not-exist")
	assert.Error(t, err)
}
