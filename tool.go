package lti

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/marcgabe15/lti-go/jwkset"
)

// Tool is the composition root for an LTI 1.3 tool provider: it verifies
// inbound launches from platforms and exposes ready-to-mount HTTP
// handlers for the OIDC login and launch endpoints and this tool's JWKS.
// Construct one with New.
type Tool struct {
	issuer     string
	store      Store
	keyManager KeyManager

	clock        Clock
	httpClient   *http.Client
	nonceTTL     time.Duration
	stateTTL     time.Duration
	ltikTTL      time.Duration
	launchTTL    time.Duration
	iatMaxAge    time.Duration
	errorHandler ErrorHandler

	onUnknownDeployment UnknownDeploymentHandler
	jwksFetcher         jwkset.Fetcher
}

// New builds a Tool from cfg and opts.
func New(cfg Config, opts ...Option) (*Tool, error) {
	if cfg.Issuer == "" {
		return nil, wrap(ErrInvalidConfig, errors.New("Config.Issuer is required"))
	}
	if cfg.Store == nil {
		return nil, wrap(ErrInvalidConfig, errors.New("Config.Store is required"))
	}

	o := toolOptions{
		clock:        realClock{},
		httpClient:   http.DefaultClient,
		nonceTTL:     defaultNonceTTL,
		stateTTL:     defaultStateTTL,
		ltikTTL:      defaultLTIKTTL,
		launchTTL:    defaultLaunchTTL,
		iatMaxAge:    defaultIATMaxAge,
		jwksCacheTTL: defaultJWKSCacheTTL,
		errorHandler: defaultErrorHandler,
	}
	for _, opt := range opts {
		opt(&o)
	}

	keyManager := cfg.KeyManager
	if keyManager == nil {
		keyManager = NewRSAKeyManager(cfg.Store)
	}

	return &Tool{
		issuer:              cfg.Issuer,
		store:               cfg.Store,
		keyManager:          keyManager,
		clock:               o.clock,
		httpClient:          o.httpClient,
		nonceTTL:            o.nonceTTL,
		stateTTL:            o.stateTTL,
		ltikTTL:             o.ltikTTL,
		launchTTL:           o.launchTTL,
		iatMaxAge:           o.iatMaxAge,
		errorHandler:        o.errorHandler,
		onUnknownDeployment: o.onUnknownDeployment,
		jwksFetcher:         jwkset.NewCachingFetcher(o.httpClient, o.jwksCacheTTL),
	}, nil
}

func (t *Tool) now() time.Time { return t.clock.Now() }

func (t *Tool) writeError(w http.ResponseWriter, r *http.Request, err error) {
	t.errorHandler(w, r, err)
}

func defaultErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusBadRequest
	if code, ok := CodeOf(err); ok {
		switch code {
		case CodeUnregisteredPlatform, CodeInactivePlatform, CodeUnknownDeployment:
			status = http.StatusForbidden
		case CodeInvalidLTIK, CodeLaunchNotFound:
			status = http.StatusUnauthorized
		}
	}
	http.Error(w, err.Error(), status)
}

// Platforms returns a facade for registering and managing platforms and
// their signing keys and deployments.
func (t *Tool) Platforms() *PlatformManager {
	return &PlatformManager{store: t.store, keyManager: t.keyManager}
}

// PlatformManager provides CRUD operations for registering and managing
// platforms, their signing keys, and their deployments.
type PlatformManager struct {
	store      Store
	keyManager KeyManager
}

// Register adds a new Platform, generating its dedicated signing key
// pair. New platforms are active according to p.Active; platforms
// registered via Dynamic Registration (a later phase) default to
// inactive pending manual review.
func (m *PlatformManager) Register(ctx context.Context, p *Platform) (*Platform, error) {
	if p.KeyConfig.Method == "" {
		p.KeyConfig.Method = KeyConfigMethodJWKSet
	}
	created, err := m.store.CreatePlatform(ctx, p)
	if err != nil {
		return nil, err
	}
	if _, err := m.keyManager.KeyPair(ctx, created.ID); err != nil {
		return nil, err
	}
	return created, nil
}

// Get returns the platform with the given id.
func (m *PlatformManager) Get(ctx context.Context, id string) (*Platform, error) {
	return m.store.GetPlatform(ctx, id)
}

// FindByIssuerAndClientID returns the platform registered for the given
// issuer and client_id.
func (m *PlatformManager) FindByIssuerAndClientID(ctx context.Context, issuer, clientID string) (*Platform, error) {
	return m.store.FindPlatform(ctx, issuer, clientID)
}

// List returns platforms matching params (or all platforms if params is
// nil).
func (m *PlatformManager) List(ctx context.Context, params *ListPlatformsParams) ([]*Platform, error) {
	return m.store.ListPlatforms(ctx, params)
}

// Update persists changes to an existing platform.
func (m *PlatformManager) Update(ctx context.Context, p *Platform) (*Platform, error) {
	return m.store.UpdatePlatform(ctx, p)
}

// Activate marks a platform active, allowing launches from it to be
// accepted.
func (m *PlatformManager) Activate(ctx context.Context, id string) (*Platform, error) {
	return m.setActive(ctx, id, true)
}

// Deactivate marks a platform inactive, rejecting future launches from it
// with ErrInactivePlatform.
func (m *PlatformManager) Deactivate(ctx context.Context, id string) (*Platform, error) {
	return m.setActive(ctx, id, false)
}

func (m *PlatformManager) setActive(ctx context.Context, id string, active bool) (*Platform, error) {
	p, err := m.store.GetPlatform(ctx, id)
	if err != nil {
		return nil, err
	}
	p.Active = active
	return m.store.UpdatePlatform(ctx, p)
}

// Delete removes a platform.
func (m *PlatformManager) Delete(ctx context.Context, id string) error {
	return m.store.DeletePlatform(ctx, id)
}

// RotateKeys generates a fresh signing key pair for platform id,
// immediately invalidating the previous one (it is dropped from
// Tool.JWKSHandler's response and can no longer verify tokens this tool
// previously signed).
func (m *PlatformManager) RotateKeys(ctx context.Context, id string) (*KeyPair, error) {
	return m.keyManager.Rotate(ctx, id)
}

// Deployments returns a manager for platformID's registered deployments.
func (m *PlatformManager) Deployments(platformID string) *DeploymentManager {
	return &DeploymentManager{store: m.store, platformID: platformID}
}

// DeploymentManager provides CRUD operations for the deployments of one
// platform.
type DeploymentManager struct {
	store      Store
	platformID string
}

// Register adds a deployment_id this tool should accept launches from.
func (m *DeploymentManager) Register(ctx context.Context, deploymentID, name string) (*Deployment, error) {
	return m.store.CreateDeployment(ctx, &Deployment{PlatformID: m.platformID, DeploymentID: deploymentID, Name: name})
}

// List returns all registered deployments for this platform.
func (m *DeploymentManager) List(ctx context.Context) ([]*Deployment, error) {
	return m.store.ListDeployments(ctx, m.platformID)
}

// Delete removes a registered deployment.
func (m *DeploymentManager) Delete(ctx context.Context, deploymentID string) error {
	return m.store.DeleteDeployment(ctx, m.platformID, deploymentID)
}
