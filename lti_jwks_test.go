package lti_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	josepkg "github.com/go-jose/go-jose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marcgabe15/lti-go"
	"github.com/marcgabe15/lti-go/ltitest"
	"github.com/marcgabe15/lti-go/memstore"
)

func TestJWKSHandler_ServesOneKeyPerPlatform(t *testing.T) {
	store := memstore.New()
	tool, err := lti.New(lti.Config{Issuer: "https://tool.example.com", Store: store})
	require.NoError(t, err)

	ctx := context.Background()
	_, err = tool.Platforms().Register(ctx, &lti.Platform{Issuer: "https://a.example.com", ClientID: "a", AuthenticationEndpoint: "https://a.example.com/auth"})
	require.NoError(t, err)
	_, err = tool.Platforms().Register(ctx, &lti.Platform{Issuer: "https://b.example.com", ClientID: "b", AuthenticationEndpoint: "https://b.example.com/auth"})
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/lti/keys", nil)
	rec := httptest.NewRecorder()
	tool.JWKSHandler().ServeHTTP(rec, req)

	require.Equal(t, 200, rec.Code)
	var set josepkg.JSONWebKeySet
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &set))
	assert.Len(t, set.Keys, 2)
	for _, k := range set.Keys {
		assert.Equal(t, "RS256", k.Algorithm)
		assert.Equal(t, "sig", k.Use)
		assert.NotEmpty(t, k.KeyID)
	}
}

func TestPlatformManager_RotateKeysChangesJWKS(t *testing.T) {
	store := memstore.New()
	tool, err := lti.New(lti.Config{Issuer: "https://tool.example.com", Store: store})
	require.NoError(t, err)

	ctx := context.Background()
	platform, err := tool.Platforms().Register(ctx, &lti.Platform{Issuer: "https://a.example.com", ClientID: "a", AuthenticationEndpoint: "https://a.example.com/auth"})
	require.NoError(t, err)

	before, err := store.GetKeyPair(ctx, platform.ID)
	require.NoError(t, err)

	_, err = tool.Platforms().RotateKeys(ctx, platform.ID)
	require.NoError(t, err)

	after, err := store.GetKeyPair(ctx, platform.ID)
	require.NoError(t, err)
	assert.NotEqual(t, before.KeyID, after.KeyID)
}

func TestLTITestClock_Advance(t *testing.T) {
	// Sanity check for the ltitest.Clock helper used by consumers to test
	// their own expiry-sensitive logic against this package.
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := ltitest.NewClock(start)
	assert.True(t, clock.Now().Equal(start))

	clock.Advance(25 * time.Hour)
	assert.True(t, clock.Now().Equal(start.Add(25*time.Hour)))
}
