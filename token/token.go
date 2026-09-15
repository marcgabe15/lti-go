// Package token obtains OAuth2 access tokens from LTI platforms using the
// client_credentials grant with a private_key_jwt client assertion (RFC
// 7523 / IMS Security Framework), caching them via a
// lti.TokenCacheStore. It is the shared plumbing the ags and nrps
// packages use to authenticate their calls.
package token

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/go-jose/go-jose/v3/jwt"

	"github.com/marcgabe15/lti-go"
	"github.com/marcgabe15/lti-go/internal/idgen"
	internaljose "github.com/marcgabe15/lti-go/internal/jose"
)

const (
	grantTypeClientCredentials    = "client_credentials"
	clientAssertionTypeJWTBearer  = "urn:ietf:params:oauth:client-assertion-type:jwt-bearer"
	assertionTTL                  = time.Minute
	tokenExpirySafetyMargin       = 60 * time.Second
	defaultAssumedExpiryIfMissing = time.Hour
)

// Source obtains an OAuth2 access token for platform, valid for scopes.
// Implementations should cache tokens and only fetch a new one once the
// cached one is at or near expiry.
type Source interface {
	Token(ctx context.Context, platform *lti.Platform, scopes []string) (string, error)
}

// CachingSource is the default Source: it requests tokens via the
// client_credentials grant, signing a client assertion with the
// platform's dedicated key pair (via a lti.KeyManager), and caches the
// result via a lti.TokenCacheStore.
type CachingSource struct {
	store      lti.TokenCacheStore
	keyManager lti.KeyManager
	httpClient *http.Client
	clock      lti.Clock
}

// Option configures optional CachingSource behavior.
type Option func(*CachingSource)

// WithHTTPClient overrides the *http.Client used to call platform token
// endpoints.
func WithHTTPClient(hc *http.Client) Option {
	return func(s *CachingSource) { s.httpClient = hc }
}

// WithClock overrides the Clock used to compute token expiry. Intended
// for tests.
func WithClock(c lti.Clock) Option {
	return func(s *CachingSource) { s.clock = c }
}

// NewCachingSource returns a CachingSource backed by store (for caching)
// and keyManager (for signing client assertions).
func NewCachingSource(store lti.TokenCacheStore, keyManager lti.KeyManager, opts ...Option) *CachingSource {
	s := &CachingSource{
		store:      store,
		keyManager: keyManager,
		httpClient: http.DefaultClient,
		clock:      systemClock{},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// Token implements Source.
func (s *CachingSource) Token(ctx context.Context, platform *lti.Platform, scopes []string) (string, error) {
	canonical := canonicalScopes(scopes)
	key := lti.TokenCacheKey{PlatformID: platform.ID, Scopes: canonical}

	if cached, err := s.store.GetCachedToken(ctx, key); err == nil {
		return cached.AccessToken, nil
	} else if !errors.Is(err, lti.ErrNotFound) {
		return "", fmt.Errorf("token: get cached token: %w", err)
	}

	accessToken, expiresIn, err := s.fetch(ctx, platform, canonical)
	if err != nil {
		return "", err
	}

	if expiresIn <= 0 {
		expiresIn = defaultAssumedExpiryIfMissing
	}
	expiresAt := s.clock.Now().Add(expiresIn - tokenExpirySafetyMargin)
	cached := &lti.CachedToken{AccessToken: accessToken, ExpiresAt: expiresAt}
	if err := s.store.SaveCachedToken(ctx, key, cached); err != nil {
		return "", fmt.Errorf("token: cache access token: %w", err)
	}
	return accessToken, nil
}

func (s *CachingSource) fetch(ctx context.Context, platform *lti.Platform, scopes string) (accessToken string, expiresIn time.Duration, err error) {
	kp, err := s.keyManager.KeyPair(ctx, platform.ID)
	if err != nil {
		return "", 0, fmt.Errorf("token: resolve signing key: %w", err)
	}
	priv, err := internaljose.ParseRSAPrivateKeyPEM(kp.PrivateKey)
	if err != nil {
		return "", 0, fmt.Errorf("token: parse signing key: %w", err)
	}

	assertion, err := buildClientAssertion(priv, kp.KeyID, platform.ClientID, platform.AccessTokenEndpoint, s.clock.Now())
	if err != nil {
		return "", 0, err
	}

	form := url.Values{
		"grant_type":            {grantTypeClientCredentials},
		"client_assertion_type": {clientAssertionTypeJWTBearer},
		"client_assertion":      {assertion},
		"scope":                 {scopes},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, platform.AccessTokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, fmt.Errorf("token: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("token: request access token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("token: platform returned %d: %s", resp.StatusCode, body)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", 0, fmt.Errorf("token: decode response: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return "", 0, errors.New("token: platform response is missing access_token")
	}
	return tokenResp.AccessToken, time.Duration(tokenResp.ExpiresIn) * time.Second, nil
}

func buildClientAssertion(priv *rsa.PrivateKey, kid, clientID, audience string, now time.Time) (string, error) {
	id, err := idgen.New(16)
	if err != nil {
		return "", fmt.Errorf("token: generate assertion id: %w", err)
	}
	claims := jwt.Claims{
		Issuer:   clientID,
		Subject:  clientID,
		Audience: jwt.Audience{audience},
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(now.Add(assertionTTL)),
		ID:       id,
	}
	token, err := internaljose.SignClaims(priv, kid, claims)
	if err != nil {
		return "", fmt.Errorf("token: sign client assertion: %w", err)
	}
	return token, nil
}

// canonicalScopes returns scopes sorted and space-joined, so that
// requesting the same set of scopes in a different order still hits the
// token cache.
func canonicalScopes(scopes []string) string {
	sorted := append([]string(nil), scopes...)
	sort.Strings(sorted)
	return strings.Join(sorted, " ")
}
