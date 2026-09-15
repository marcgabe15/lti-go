package lti

import (
	"context"
	"net/http"
	"time"
)

const (
	defaultNonceTTL     = 5 * time.Minute
	defaultStateTTL     = 10 * time.Minute
	defaultLTIKTTL      = 24 * time.Hour
	defaultLaunchTTL    = 24 * time.Hour
	defaultIATMaxAge    = 10 * time.Second
	defaultJWKSCacheTTL = 5 * time.Minute
)

// UnknownDeploymentHandler decides whether to accept a launch whose
// deployment_id has not been registered via the Store. Returning true
// registers the deployment (via Store.CreateDeployment) so future
// launches with the same deployment_id are accepted without invoking the
// handler again. If nil, launches with an unrecognized deployment_id are
// rejected with ErrUnknownDeployment.
type UnknownDeploymentHandler func(ctx context.Context, platform *Platform, deploymentID string) (allow bool, err error)

// ErrorHandler writes an HTTP response for an error returned while
// handling a login or launch request. Use CodeOf(err) to distinguish
// error kinds. The default handler writes err.Error() as plain text with
// a status code chosen from the error's Code.
type ErrorHandler func(w http.ResponseWriter, r *http.Request, err error)

// Config holds the required configuration for a Tool.
type Config struct {
	// Issuer is this tool's own issuer / base URL. It is used as the
	// issuer of state and ltik tokens, and (in later phases) as the iss
	// of deep linking responses and the tool's JWKS uri.
	Issuer string

	// Store is the persistence backend for platforms, deployments,
	// nonces, launches, cached tokens, and signing keys. Required.
	Store Store

	// KeyManager resolves and rotates per-platform signing keys. If nil,
	// a default RSAKeyManager backed by Store is used.
	KeyManager KeyManager
}

type toolOptions struct {
	clock               Clock
	httpClient          *http.Client
	nonceTTL            time.Duration
	stateTTL            time.Duration
	ltikTTL             time.Duration
	launchTTL           time.Duration
	iatMaxAge           time.Duration
	jwksCacheTTL        time.Duration
	errorHandler        ErrorHandler
	onUnknownDeployment UnknownDeploymentHandler
}

// Option configures optional Tool behavior.
type Option func(*toolOptions)

// WithClock overrides the Clock used for expiry checks. Intended for
// tests; see ltitest.Clock.
func WithClock(c Clock) Option { return func(o *toolOptions) { o.clock = c } }

// WithHTTPClient overrides the *http.Client used to fetch platform JWKS
// (and, in later phases, to call platform token/AGS/NRPS endpoints).
func WithHTTPClient(hc *http.Client) Option { return func(o *toolOptions) { o.httpClient = hc } }

// WithNonceTTL overrides how long an issued nonce remains valid before it
// must be consumed. Default 5 minutes.
func WithNonceTTL(d time.Duration) Option { return func(o *toolOptions) { o.nonceTTL = d } }

// WithStateTTL overrides how long the signed OIDC state token remains
// valid. Default 10 minutes.
func WithStateTTL(d time.Duration) Option { return func(o *toolOptions) { o.stateTTL = d } }

// WithLTIKTTL overrides how long an issued ltik session-resumption token
// remains valid. Default 24 hours.
func WithLTIKTTL(d time.Duration) Option { return func(o *toolOptions) { o.ltikTTL = d } }

// WithIATMaxAge overrides the maximum allowed age of an id_token's iat
// claim at verification time, guarding against replay of stale tokens.
// Default 10 seconds; pass 0 to disable the check.
func WithIATMaxAge(d time.Duration) Option { return func(o *toolOptions) { o.iatMaxAge = d } }

// WithJWKSCacheTTL overrides how long a fetched platform JWK Set is
// cached before being refetched. Default 5 minutes.
func WithJWKSCacheTTL(d time.Duration) Option { return func(o *toolOptions) { o.jwksCacheTTL = d } }

// WithErrorHandler overrides how LoginHandler and LaunchHandler respond
// to errors.
func WithErrorHandler(h ErrorHandler) Option { return func(o *toolOptions) { o.errorHandler = h } }

// WithUnknownDeploymentHandler configures how VerifyLaunch treats a
// deployment_id that isn't registered in the Store. Without this option,
// such launches are rejected with ErrUnknownDeployment.
func WithUnknownDeploymentHandler(h UnknownDeploymentHandler) Option {
	return func(o *toolOptions) { o.onUnknownDeployment = h }
}
