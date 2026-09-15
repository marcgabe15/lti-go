package lti

import "time"

// KeyConfigMethod selects how a Platform's public key(s) for id_token
// verification are resolved.
type KeyConfigMethod string

const (
	// KeyConfigMethodJWKSet fetches the platform's key set from JWKSURI,
	// caches it, and looks up the signing key by kid. This is the common
	// case and what Dynamic Registration always produces.
	KeyConfigMethodJWKSet KeyConfigMethod = "jwks_uri"
	// KeyConfigMethodJWK uses a single JWK given up front, in JWK.
	KeyConfigMethodJWK KeyConfigMethod = "jwk"
	// KeyConfigMethodRSAKey uses a raw RSA public key PEM given up front,
	// in RSAKey.
	KeyConfigMethodRSAKey KeyConfigMethod = "rsa_key"
)

// KeyConfig describes how to resolve the platform's public key(s) used to
// verify id_tokens and other JWTs it signs.
type KeyConfig struct {
	Method KeyConfigMethod

	// JWKSURI is used when Method == KeyConfigMethodJWKSet.
	JWKSURI string
	// JWK is a single JWK JSON document, used when Method ==
	// KeyConfigMethodJWK.
	JWK string
	// RSAKey is a PEM-encoded RSA public key, used when Method ==
	// KeyConfigMethodRSAKey.
	RSAKey string
}

// Platform represents a registered LTI platform (an LMS) that this tool
// has a trust relationship with. Each Platform gets its own dedicated RSA
// signing key pair (see KeyManager), used for every JWT this tool signs
// toward that platform.
type Platform struct {
	ID       string
	Issuer   string
	ClientID string
	Name     string

	// AuthenticationEndpoint is the platform's OIDC authorization
	// endpoint, used to redirect the browser during login.
	AuthenticationEndpoint string
	// AccessTokenEndpoint is the platform's OAuth2 token endpoint, used
	// in later phases to obtain access tokens for AGS/NRPS calls.
	AccessTokenEndpoint string

	KeyConfig KeyConfig

	// Active controls whether launches from this platform are accepted.
	// Platforms registered via Dynamic Registration default to inactive
	// pending manual review.
	Active bool

	CreatedAt time.Time
	UpdatedAt time.Time
}

// Deployment represents one LTI deployment of this tool within a
// Platform. The LTI 1.3 spec expects a tool to recognize specific
// deployment_ids per platform registration; Tool.VerifyLaunch enforces
// this against the Store unless an UnknownDeploymentHandler is
// configured to accept new deployments on first use.
type Deployment struct {
	ID           string
	PlatformID   string
	DeploymentID string
	Name         string

	CreatedAt time.Time
}
