package token_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marcgabe15/lti-go"
	"github.com/marcgabe15/lti-go/memstore"
	"github.com/marcgabe15/lti-go/token"
)

func TestCachingSource_FetchesAndCaches(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "client_credentials", r.Form.Get("grant_type"))
		assert.Equal(t, "urn:ietf:params:oauth:client-assertion-type:jwt-bearer", r.Form.Get("client_assertion_type"))
		assert.NotEmpty(t, r.Form.Get("client_assertion"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": fmt.Sprintf("token-%d", requestCount),
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer srv.Close()

	ctx := context.Background()
	store := memstore.New()
	km := lti.NewRSAKeyManager(store)

	platform, err := store.CreatePlatform(ctx, &lti.Platform{
		Issuer: "https://platform.example.com", ClientID: "client-1", AccessTokenEndpoint: srv.URL,
	})
	require.NoError(t, err)
	_, err = km.KeyPair(ctx, platform.ID)
	require.NoError(t, err)

	src := token.NewCachingSource(store, km)

	tok1, err := src.Token(ctx, platform, []string{"scope-a", "scope-b"})
	require.NoError(t, err)
	assert.Equal(t, "token-1", tok1)
	assert.Equal(t, 1, requestCount)

	// Same scopes, different order: should be served from cache.
	tok2, err := src.Token(ctx, platform, []string{"scope-b", "scope-a"})
	require.NoError(t, err)
	assert.Equal(t, "token-1", tok2)
	assert.Equal(t, 1, requestCount)

	// Different scopes: should trigger a new fetch.
	tok3, err := src.Token(ctx, platform, []string{"scope-c"})
	require.NoError(t, err)
	assert.Equal(t, "token-2", tok3)
	assert.Equal(t, 2, requestCount)
}

func TestCachingSource_PlatformErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"invalid_client"}`))
	}))
	defer srv.Close()

	ctx := context.Background()
	store := memstore.New()
	km := lti.NewRSAKeyManager(store)
	platform, err := store.CreatePlatform(ctx, &lti.Platform{
		Issuer: "https://platform.example.com", ClientID: "client-1", AccessTokenEndpoint: srv.URL,
	})
	require.NoError(t, err)

	src := token.NewCachingSource(store, km)
	_, err = src.Token(ctx, platform, []string{"scope-a"})
	assert.Error(t, err)
}
