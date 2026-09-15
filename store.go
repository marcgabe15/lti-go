package lti

import (
	"context"
	"time"
)

// ListPlatformsParams filters Store.ListPlatforms. A nil *ListPlatformsParams
// (or a zero value) lists all platforms.
type ListPlatformsParams struct {
	// Active, if non-nil, restricts the list to platforms with a matching
	// Active value.
	Active *bool
}

// PlatformStore persists Platform records. Get/Find implementations
// return ErrNotFound when no matching platform exists.
type PlatformStore interface {
	CreatePlatform(ctx context.Context, p *Platform) (*Platform, error)
	GetPlatform(ctx context.Context, id string) (*Platform, error)
	FindPlatform(ctx context.Context, issuer, clientID string) (*Platform, error)
	ListPlatforms(ctx context.Context, params *ListPlatformsParams) ([]*Platform, error)
	UpdatePlatform(ctx context.Context, p *Platform) (*Platform, error)
	DeletePlatform(ctx context.Context, id string) error
}

// DeploymentStore persists Deployment records, closing the deployment_id
// tracking gap left open by some LTI 1.3 tool implementations. Find
// returns ErrNotFound when the deployment isn't registered.
type DeploymentStore interface {
	CreateDeployment(ctx context.Context, d *Deployment) (*Deployment, error)
	FindDeployment(ctx context.Context, platformID, deploymentID string) (*Deployment, error)
	ListDeployments(ctx context.Context, platformID string) ([]*Deployment, error)
	DeleteDeployment(ctx context.Context, platformID, deploymentID string) error
}

// NonceStore protects against OIDC replay attacks.
type NonceStore interface {
	// SaveNonce records a freshly issued nonce with an expiry.
	SaveNonce(ctx context.Context, nonce string, expiresAt time.Time) error
	// ConsumeNonce atomically checks that nonce exists and has not
	// expired, and removes it so it cannot be consumed again. It returns
	// ErrNonceReused if the nonce was already consumed, was never issued,
	// or has expired. Implementations must make this check-and-delete
	// atomic so concurrent requests replaying the same nonce cannot both
	// succeed.
	ConsumeNonce(ctx context.Context, nonce string) error
}

// LaunchRecord is what an ltik token refers back to: the verified claims
// of a completed launch, so a tool can resume it in a later request (for
// example from a background job).
type LaunchRecord struct {
	ID         string
	PlatformID string
	Claims     *Claims
	CreatedAt  time.Time
	ExpiresAt  time.Time
}

// LaunchStore persists completed launches so they can be resumed via
// ltik. Get returns ErrNotFound once a record has expired or been
// deleted.
type LaunchStore interface {
	SaveLaunch(ctx context.Context, l *LaunchRecord) (id string, err error)
	GetLaunch(ctx context.Context, id string) (*LaunchRecord, error)
	DeleteLaunch(ctx context.Context, id string) error
}

// TokenCacheKey identifies a cached OAuth2 access token obtained from a
// platform for a given set of scopes.
type TokenCacheKey struct {
	PlatformID string
	// Scopes is the canonical (space-joined, sorted) scope list the token
	// was requested for.
	Scopes string
}

// CachedToken is a cached OAuth2 access token.
type CachedToken struct {
	AccessToken string
	ExpiresAt   time.Time
}

// TokenCacheStore caches OAuth2 access tokens obtained from platforms
// (used by the ags and nrps packages in later phases).
type TokenCacheStore interface {
	GetCachedToken(ctx context.Context, key TokenCacheKey) (*CachedToken, error)
	SaveCachedToken(ctx context.Context, key TokenCacheKey, tok *CachedToken) error
}

// KeyPair is the RSA key pair this tool uses to sign JWTs toward a
// specific platform (state, ltik, and in later phases client assertions
// and deep linking responses), and to serve via Tool.JWKSHandler.
type KeyPair struct {
	PlatformID string
	KeyID      string
	// PrivateKey is PEM-encoded (PKCS#1 or PKCS#8).
	PrivateKey []byte
	// PublicKey is PEM-encoded (PKIX).
	PublicKey []byte
	CreatedAt time.Time
}

// KeyStore persists per-platform signing key pairs. Get returns
// ErrNotFound when no key pair has been generated yet for a platform.
type KeyStore interface {
	SaveKeyPair(ctx context.Context, kp *KeyPair) error
	GetKeyPair(ctx context.Context, platformID string) (*KeyPair, error)
	DeleteKeyPair(ctx context.Context, platformID string) error
}

// Store is the persistence contract lti-go depends on. Implement it
// against your own database, or use memstore.Store for tests and
// examples. Any implementation should pass the storetest conformance
// suite.
type Store interface {
	PlatformStore
	DeploymentStore
	NonceStore
	LaunchStore
	TokenCacheStore
	KeyStore
}
