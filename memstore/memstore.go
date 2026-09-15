// Package memstore provides an in-memory implementation of lti.Store for
// tests and examples. It is not safe for production use: all state is
// lost on process restart and nothing is shared across processes.
package memstore

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/marcgabe15/lti-go"
	"github.com/marcgabe15/lti-go/internal/idgen"
)

// Store is an in-memory lti.Store implementation.
type Store struct {
	mu sync.Mutex

	platforms   map[string]*lti.Platform
	deployments map[string]*lti.Deployment
	nonces      map[string]time.Time
	launches    map[string]*lti.LaunchRecord
	tokens      map[lti.TokenCacheKey]*lti.CachedToken
	keys        map[string]*lti.KeyPair
}

var _ lti.Store = (*Store)(nil)

// New returns an empty Store.
func New() *Store {
	return &Store{
		platforms:   make(map[string]*lti.Platform),
		deployments: make(map[string]*lti.Deployment),
		nonces:      make(map[string]time.Time),
		launches:    make(map[string]*lti.LaunchRecord),
		tokens:      make(map[lti.TokenCacheKey]*lti.CachedToken),
		keys:        make(map[string]*lti.KeyPair),
	}
}

// --- PlatformStore ---

func (s *Store) CreatePlatform(_ context.Context, p *lti.Platform) (*lti.Platform, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, existing := range s.platforms {
		if existing.Issuer == p.Issuer && existing.ClientID == p.ClientID {
			return nil, lti.ErrPlatformAlreadyExists
		}
	}
	id, err := idgen.New(12)
	if err != nil {
		return nil, err
	}
	cp := *p
	cp.ID = id
	now := time.Now()
	cp.CreatedAt = now
	cp.UpdatedAt = now
	s.platforms[id] = &cp

	out := cp
	return &out, nil
}

func (s *Store) GetPlatform(_ context.Context, id string) (*lti.Platform, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.platforms[id]
	if !ok {
		return nil, lti.ErrNotFound
	}
	out := *p
	return &out, nil
}

func (s *Store) FindPlatform(_ context.Context, issuer, clientID string) (*lti.Platform, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, p := range s.platforms {
		if p.Issuer == issuer && p.ClientID == clientID {
			out := *p
			return &out, nil
		}
	}
	return nil, lti.ErrNotFound
}

func (s *Store) ListPlatforms(_ context.Context, params *lti.ListPlatformsParams) ([]*lti.Platform, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []*lti.Platform
	for _, p := range s.platforms {
		if params != nil && params.Active != nil && p.Active != *params.Active {
			continue
		}
		cp := *p
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *Store) UpdatePlatform(_ context.Context, p *lti.Platform) (*lti.Platform, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.platforms[p.ID]; !ok {
		return nil, lti.ErrNotFound
	}
	cp := *p
	cp.UpdatedAt = time.Now()
	s.platforms[p.ID] = &cp

	out := cp
	return &out, nil
}

func (s *Store) DeletePlatform(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.platforms[id]; !ok {
		return lti.ErrNotFound
	}
	delete(s.platforms, id)
	return nil
}

// --- DeploymentStore ---

func deploymentKey(platformID, deploymentID string) string {
	return platformID + "\x00" + deploymentID
}

func (s *Store) CreateDeployment(_ context.Context, d *lti.Deployment) (*lti.Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := deploymentKey(d.PlatformID, d.DeploymentID)
	if _, ok := s.deployments[key]; ok {
		return nil, lti.ErrDeploymentAlreadyExists
	}
	id, err := idgen.New(12)
	if err != nil {
		return nil, err
	}
	cp := *d
	cp.ID = id
	cp.CreatedAt = time.Now()
	s.deployments[key] = &cp

	out := cp
	return &out, nil
}

func (s *Store) FindDeployment(_ context.Context, platformID, deploymentID string) (*lti.Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.deployments[deploymentKey(platformID, deploymentID)]
	if !ok {
		return nil, lti.ErrNotFound
	}
	out := *d
	return &out, nil
}

func (s *Store) ListDeployments(_ context.Context, platformID string) ([]*lti.Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []*lti.Deployment
	for _, d := range s.deployments {
		if d.PlatformID == platformID {
			cp := *d
			out = append(out, &cp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *Store) DeleteDeployment(_ context.Context, platformID, deploymentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := deploymentKey(platformID, deploymentID)
	if _, ok := s.deployments[key]; !ok {
		return lti.ErrNotFound
	}
	delete(s.deployments, key)
	return nil
}

// --- NonceStore ---

func (s *Store) SaveNonce(_ context.Context, nonce string, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nonces[nonce] = expiresAt
	return nil
}

func (s *Store) ConsumeNonce(_ context.Context, nonce string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	expiresAt, ok := s.nonces[nonce]
	if !ok {
		return lti.ErrNonceReused
	}
	delete(s.nonces, nonce)
	if time.Now().After(expiresAt) {
		return lti.ErrNonceReused
	}
	return nil
}

// --- LaunchStore ---

func (s *Store) SaveLaunch(_ context.Context, l *lti.LaunchRecord) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := l.ID
	if id == "" {
		var err error
		id, err = idgen.New(16)
		if err != nil {
			return "", err
		}
	}
	cp := *l
	cp.ID = id
	s.launches[id] = &cp
	return id, nil
}

func (s *Store) GetLaunch(_ context.Context, id string) (*lti.LaunchRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	l, ok := s.launches[id]
	if !ok {
		return nil, lti.ErrNotFound
	}
	if time.Now().After(l.ExpiresAt) {
		delete(s.launches, id)
		return nil, lti.ErrNotFound
	}
	out := *l
	return &out, nil
}

func (s *Store) DeleteLaunch(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.launches[id]; !ok {
		return lti.ErrNotFound
	}
	delete(s.launches, id)
	return nil
}

// --- TokenCacheStore ---

func (s *Store) GetCachedToken(_ context.Context, key lti.TokenCacheKey) (*lti.CachedToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tok, ok := s.tokens[key]
	if !ok {
		return nil, lti.ErrNotFound
	}
	if time.Now().After(tok.ExpiresAt) {
		delete(s.tokens, key)
		return nil, lti.ErrNotFound
	}
	out := *tok
	return &out, nil
}

func (s *Store) SaveCachedToken(_ context.Context, key lti.TokenCacheKey, tok *lti.CachedToken) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cp := *tok
	s.tokens[key] = &cp
	return nil
}

// --- KeyStore ---

func (s *Store) SaveKeyPair(_ context.Context, kp *lti.KeyPair) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cp := *kp
	s.keys[kp.PlatformID] = &cp
	return nil
}

func (s *Store) GetKeyPair(_ context.Context, platformID string) (*lti.KeyPair, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	kp, ok := s.keys[platformID]
	if !ok {
		return nil, lti.ErrNotFound
	}
	out := *kp
	return &out, nil
}

func (s *Store) DeleteKeyPair(_ context.Context, platformID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.keys[platformID]; !ok {
		return lti.ErrNotFound
	}
	delete(s.keys, platformID)
	return nil
}
