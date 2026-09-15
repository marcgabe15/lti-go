package lti

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-jose/go-jose/v3/jwt"

	"github.com/marcgabe15/lti-go/internal/idgen"
	internaljose "github.com/marcgabe15/lti-go/internal/jose"
)

// ltikClaims is the payload of the ltik session-resumption token: an
// opaque, self-verifying pointer to a LaunchRecord.
type ltikClaims struct {
	jwt.Claims
	LaunchID string `json:"launchId"`
}

func (t *Tool) issueLTIK(ctx context.Context, platform *Platform, launchID string) (string, error) {
	kp, err := t.keyManager.KeyPair(ctx, platform.ID)
	if err != nil {
		return "", fmt.Errorf("lti: resolve signing key: %w", err)
	}
	priv, err := internaljose.ParseRSAPrivateKeyPEM(kp.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("lti: parse signing key: %w", err)
	}
	id, err := idgen.New(16)
	if err != nil {
		return "", fmt.Errorf("lti: generate ltik id: %w", err)
	}
	now := t.now()
	claims := ltikClaims{
		Claims: jwt.Claims{
			Issuer:   t.issuer,
			Subject:  platform.ID,
			IssuedAt: jwt.NewNumericDate(now),
			Expiry:   jwt.NewNumericDate(now.Add(t.ltikTTL)),
			ID:       id,
		},
		LaunchID: launchID,
	}
	token, err := internaljose.SignClaims(priv, kp.KeyID, claims)
	if err != nil {
		return "", fmt.Errorf("lti: sign ltik: %w", err)
	}
	return token, nil
}

// VerifyLTIK verifies an ltik token (as issued by LaunchHandler) and
// returns the LaunchRecord it refers to, allowing a launch to be resumed
// outside of the original request -- for example from a background job.
func (t *Tool) VerifyLTIK(ctx context.Context, ltik string) (*LaunchRecord, error) {
	parsed, err := internaljose.ParseSigned(ltik)
	if err != nil {
		return nil, wrap(ErrInvalidLTIK, err)
	}
	if err := requireRS256(parsed); err != nil {
		return nil, err
	}

	var unverified ltikClaims
	if err := parsed.UnsafeClaimsWithoutVerification(&unverified); err != nil {
		return nil, wrap(ErrInvalidLTIK, err)
	}

	platform, err := t.store.GetPlatform(ctx, unverified.Subject)
	if err != nil {
		return nil, wrap(ErrInvalidLTIK, err)
	}
	kp, err := t.keyManager.KeyPair(ctx, platform.ID)
	if err != nil {
		return nil, wrap(ErrInvalidLTIK, err)
	}
	pub, err := internaljose.ParseRSAPublicKeyPEM(kp.PublicKey)
	if err != nil {
		return nil, wrap(ErrInvalidLTIK, err)
	}

	var claims ltikClaims
	if err := internaljose.VerifyWithKey(parsed, pub, &claims); err != nil {
		return nil, wrap(ErrInvalidLTIK, err)
	}
	if claims.Issuer != t.issuer {
		return nil, ErrInvalidLTIK
	}
	if err := checkExpiry(claims.Claims, t.now()); err != nil {
		return nil, wrap(ErrInvalidLTIK, err)
	}

	record, err := t.store.GetLaunch(ctx, claims.LaunchID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrLaunchNotFound
		}
		return nil, fmt.Errorf("lti: get launch: %w", err)
	}
	return record, nil
}
