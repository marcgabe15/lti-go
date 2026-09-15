package lti

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/marcgabe15/lti-go/internal/idgen"
)

// KeyManager resolves and rotates the RSA key pair this tool uses to sign
// JWTs (state, ltik, and in later phases client assertions and deep
// linking responses) toward a specific platform, and to serve via
// Tool.JWKSHandler.
//
// The default implementation, RSAKeyManager, generates one key pair per
// platform on first use, persisted via a KeyStore. Implement KeyManager
// yourself to use a KMS/HSM instead.
type KeyManager interface {
	// KeyPair returns the current key pair for platformID, generating one
	// on first use.
	KeyPair(ctx context.Context, platformID string) (*KeyPair, error)
	// Rotate generates and stores a fresh key pair for platformID,
	// immediately replacing the previous one.
	Rotate(ctx context.Context, platformID string) (*KeyPair, error)
}

// RSAKeyManager is the default KeyManager: it generates one RSA-2048 key
// pair per platform on first use and persists it via a KeyStore.
type RSAKeyManager struct {
	store KeyStore
	bits  int
}

// NewRSAKeyManager returns an RSAKeyManager backed by store.
func NewRSAKeyManager(store KeyStore) *RSAKeyManager {
	return &RSAKeyManager{store: store, bits: 2048}
}

// KeyPair implements KeyManager.
func (m *RSAKeyManager) KeyPair(ctx context.Context, platformID string) (*KeyPair, error) {
	kp, err := m.store.GetKeyPair(ctx, platformID)
	if err == nil {
		return kp, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	return m.generate(ctx, platformID)
}

// Rotate implements KeyManager.
func (m *RSAKeyManager) Rotate(ctx context.Context, platformID string) (*KeyPair, error) {
	return m.generate(ctx, platformID)
}

func (m *RSAKeyManager) generate(ctx context.Context, platformID string) (*KeyPair, error) {
	priv, err := rsa.GenerateKey(rand.Reader, m.bits)
	if err != nil {
		return nil, fmt.Errorf("lti: generate RSA key: %w", err)
	}
	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("lti: marshal private key: %w", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("lti: marshal public key: %w", err)
	}
	kid, err := idgen.New(8)
	if err != nil {
		return nil, fmt.Errorf("lti: generate key id: %w", err)
	}
	kp := &KeyPair{
		PlatformID: platformID,
		KeyID:      kid,
		PrivateKey: pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER}),
		PublicKey:  pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}),
		CreatedAt:  time.Now(),
	}
	if err := m.store.SaveKeyPair(ctx, kp); err != nil {
		return nil, fmt.Errorf("lti: save key pair: %w", err)
	}
	return kp, nil
}
