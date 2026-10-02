// Package storetest provides a conformance test suite that any lti.Store
// implementation should pass.
package storetest

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marcgabe15/lti-go"
)

var testPlatformCounter int64

// createTestPlatform inserts a real Platform via s and returns it. Tests
// for entities that reference a platform (deployments, cached tokens,
// keys) must use an ID that actually exists rather than a bare literal
// like "p1" -- a relational Store is expected to enforce that foreign
// key, even though memstore's plain maps don't care.
func createTestPlatform(t *testing.T, ctx context.Context, s lti.Store) *lti.Platform {
	t.Helper()
	n := atomic.AddInt64(&testPlatformCounter, 1)
	p, err := s.CreatePlatform(ctx, &lti.Platform{
		Issuer:   fmt.Sprintf("https://storetest.example.com/%d", n),
		ClientID: fmt.Sprintf("client-%d", n),
	})
	require.NoError(t, err)
	return p
}

// Run executes the Store conformance suite against a fresh store returned
// by newStore for each subtest.
func Run(t *testing.T, newStore func() lti.Store) {
	t.Helper()
	t.Run("Platforms", func(t *testing.T) { testPlatforms(t, newStore()) })
	t.Run("Deployments", func(t *testing.T) { testDeployments(t, newStore()) })
	t.Run("Nonces", func(t *testing.T) { testNonces(t, newStore()) })
	t.Run("NonceConcurrency", func(t *testing.T) { testNonceConcurrency(t, newStore()) })
	t.Run("Launches", func(t *testing.T) { testLaunches(t, newStore()) })
	t.Run("TokenCache", func(t *testing.T) { testTokenCache(t, newStore()) })
	t.Run("Keys", func(t *testing.T) { testKeys(t, newStore()) })
}

func testPlatforms(t *testing.T, s lti.Store) {
	ctx := context.Background()

	p, err := s.CreatePlatform(ctx, &lti.Platform{Issuer: "https://platform.example", ClientID: "client-1", Name: "Example"})
	require.NoError(t, err)
	require.NotEmpty(t, p.ID)

	_, err = s.CreatePlatform(ctx, &lti.Platform{Issuer: "https://platform.example", ClientID: "client-1"})
	require.ErrorIs(t, err, lti.ErrPlatformAlreadyExists)

	got, err := s.GetPlatform(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, p.Issuer, got.Issuer)

	found, err := s.FindPlatform(ctx, "https://platform.example", "client-1")
	require.NoError(t, err)
	assert.Equal(t, p.ID, found.ID)

	_, err = s.FindPlatform(ctx, "https://platform.example", "nope")
	require.ErrorIs(t, err, lti.ErrNotFound)

	list, err := s.ListPlatforms(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	got.Name = "Renamed"
	updated, err := s.UpdatePlatform(ctx, got)
	require.NoError(t, err)
	assert.Equal(t, "Renamed", updated.Name)

	require.NoError(t, s.DeletePlatform(ctx, p.ID))
	_, err = s.GetPlatform(ctx, p.ID)
	require.ErrorIs(t, err, lti.ErrNotFound)
}

func testDeployments(t *testing.T, s lti.Store) {
	ctx := context.Background()
	platformID := createTestPlatform(t, ctx, s).ID

	d, err := s.CreateDeployment(ctx, &lti.Deployment{PlatformID: platformID, DeploymentID: "d1"})
	require.NoError(t, err)
	require.NotEmpty(t, d.ID)

	_, err = s.CreateDeployment(ctx, &lti.Deployment{PlatformID: platformID, DeploymentID: "d1"})
	require.ErrorIs(t, err, lti.ErrDeploymentAlreadyExists)

	found, err := s.FindDeployment(ctx, platformID, "d1")
	require.NoError(t, err)
	assert.Equal(t, d.ID, found.ID)

	_, err = s.FindDeployment(ctx, platformID, "missing")
	require.ErrorIs(t, err, lti.ErrNotFound)

	list, err := s.ListDeployments(ctx, platformID)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	require.NoError(t, s.DeleteDeployment(ctx, platformID, "d1"))
	_, err = s.FindDeployment(ctx, platformID, "d1")
	require.ErrorIs(t, err, lti.ErrNotFound)
}

func testNonces(t *testing.T, s lti.Store) {
	ctx := context.Background()

	require.NoError(t, s.SaveNonce(ctx, "n1", time.Now().Add(time.Minute)))
	require.NoError(t, s.ConsumeNonce(ctx, "n1"))
	require.ErrorIs(t, s.ConsumeNonce(ctx, "n1"), lti.ErrNonceReused)

	require.NoError(t, s.SaveNonce(ctx, "n2", time.Now().Add(-time.Minute)))
	require.ErrorIs(t, s.ConsumeNonce(ctx, "n2"), lti.ErrNonceReused)

	require.ErrorIs(t, s.ConsumeNonce(ctx, "never-issued"), lti.ErrNonceReused)
}

func testNonceConcurrency(t *testing.T, s lti.Store) {
	ctx := context.Background()
	require.NoError(t, s.SaveNonce(ctx, "race", time.Now().Add(time.Minute)))

	const attempts = 20
	results := make(chan error, attempts)
	var wg sync.WaitGroup
	wg.Add(attempts)
	for range attempts {
		go func() {
			defer wg.Done()
			results <- s.ConsumeNonce(ctx, "race")
		}()
	}
	wg.Wait()
	close(results)

	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	assert.Equal(t, 1, successes, "exactly one concurrent ConsumeNonce call should succeed")
}

func testLaunches(t *testing.T, s lti.Store) {
	ctx := context.Background()

	record := &lti.LaunchRecord{PlatformID: "p1", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}
	id, err := s.SaveLaunch(ctx, record)
	require.NoError(t, err)
	require.NotEmpty(t, id)

	got, err := s.GetLaunch(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "p1", got.PlatformID)

	require.NoError(t, s.DeleteLaunch(ctx, id))
	_, err = s.GetLaunch(ctx, id)
	require.ErrorIs(t, err, lti.ErrNotFound)
}

func testTokenCache(t *testing.T, s lti.Store) {
	ctx := context.Background()
	platformID := createTestPlatform(t, ctx, s).ID
	key := lti.TokenCacheKey{PlatformID: platformID, Scopes: "a b"}

	_, err := s.GetCachedToken(ctx, key)
	require.ErrorIs(t, err, lti.ErrNotFound)

	require.NoError(t, s.SaveCachedToken(ctx, key, &lti.CachedToken{AccessToken: "tok", ExpiresAt: time.Now().Add(time.Hour)}))
	got, err := s.GetCachedToken(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, "tok", got.AccessToken)
}

func testKeys(t *testing.T, s lti.Store) {
	ctx := context.Background()
	platformID := createTestPlatform(t, ctx, s).ID

	_, err := s.GetKeyPair(ctx, platformID)
	require.ErrorIs(t, err, lti.ErrNotFound)

	kp := &lti.KeyPair{PlatformID: platformID, KeyID: "k1", PrivateKey: []byte("priv"), PublicKey: []byte("pub")}
	require.NoError(t, s.SaveKeyPair(ctx, kp))

	got, err := s.GetKeyPair(ctx, platformID)
	require.NoError(t, err)
	assert.Equal(t, "k1", got.KeyID)

	require.NoError(t, s.DeleteKeyPair(ctx, platformID))
	_, err = s.GetKeyPair(ctx, platformID)
	require.ErrorIs(t, err, lti.ErrNotFound)
}
