package lti

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"

	josepkg "github.com/go-jose/go-jose/v3"

	internaljose "github.com/marcgabe15/lti-go/internal/jose"
)

var errMissingLaunchParams = errors.New("missing required parameter: id_token or state")
var errMissingIssuer = errors.New("id_token is missing the iss claim")

// LaunchHandler returns an http.Handler for the LTI launch endpoint. A
// platform POSTs its id_token and state here (response_mode=form_post).
// On success, the verified Claims (including the newly issued LTIK) are
// stored in the request context (retrievable with ClaimsFromContext) and
// next is invoked; on failure, the configured ErrorHandler writes the
// response and next is not called.
func (t *Tool) LaunchHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := t.VerifyLaunch(r)
		if err != nil {
			t.writeError(w, r, err)
			return
		}

		ctx := r.Context()
		record := &LaunchRecord{
			PlatformID: claims.Platform().ID,
			Claims:     claims,
			CreatedAt:  t.now(),
			ExpiresAt:  t.now().Add(t.launchTTL),
		}
		launchID, err := t.store.SaveLaunch(ctx, record)
		if err != nil {
			t.writeError(w, r, fmt.Errorf("lti: save launch: %w", err))
			return
		}

		ltik, err := t.issueLTIK(ctx, claims.Platform(), launchID)
		if err != nil {
			t.writeError(w, r, err)
			return
		}
		claims.LTIK = ltik

		next.ServeHTTP(w, r.WithContext(ContextWithClaims(ctx, claims)))
	})
}

// VerifyLaunch verifies the id_token and state carried by r (a
// form_post from a platform) and returns the resulting Claims. It is the
// primitive underneath LaunchHandler, for callers who want to wire their
// own middleware instead of using the provided handler (for example to
// integrate with an existing session/cookie layer).
//
// VerifyLaunch does not issue an ltik or persist a LaunchRecord; use
// LaunchHandler if you want that behavior.
func (t *Tool) VerifyLaunch(r *http.Request) (*Claims, error) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		return nil, wrap(ErrInvalidIDToken, err)
	}
	rawIDToken := r.Form.Get("id_token")
	rawState := r.Form.Get("state")
	if rawIDToken == "" || rawState == "" {
		return nil, wrap(ErrInvalidIDToken, errMissingLaunchParams)
	}

	parsedIDToken, err := internaljose.ParseSigned(rawIDToken)
	if err != nil {
		return nil, wrap(ErrInvalidIDToken, err)
	}
	if err := requireRS256(parsedIDToken); err != nil {
		return nil, err
	}

	var unverified rawLaunchClaims
	if err := parsedIDToken.UnsafeClaimsWithoutVerification(&unverified); err != nil {
		return nil, wrap(ErrInvalidIDToken, err)
	}
	if unverified.Issuer == "" {
		return nil, wrap(ErrInvalidIDToken, errMissingIssuer)
	}

	platform, err := t.resolvePlatformForLaunch(ctx, &unverified)
	if err != nil {
		return nil, err
	}

	if _, err := t.verifyState(ctx, platform, rawState); err != nil {
		return nil, err
	}

	key, err := t.resolveVerificationKey(ctx, platform, internaljose.KeyID(parsedIDToken))
	if err != nil {
		return nil, wrap(ErrUnknownSigningKey, err)
	}

	var verified rawLaunchClaims
	if err := internaljose.VerifyWithKey(parsedIDToken, key, &verified); err != nil {
		return nil, wrap(ErrInvalidIDToken, err)
	}

	if err := t.validateStandardClaims(&verified, platform); err != nil {
		return nil, err
	}
	if err := t.store.ConsumeNonce(ctx, verified.Nonce); err != nil {
		return nil, wrap(ErrNonceReused, err)
	}
	if err := t.checkDeployment(ctx, platform, verified.DeploymentID); err != nil {
		return nil, err
	}

	return buildClaims(&verified, platform), nil
}

func (t *Tool) resolvePlatformForLaunch(ctx context.Context, claims *rawLaunchClaims) (*Platform, error) {
	for _, candidate := range audienceCandidates(claims) {
		p, err := t.store.FindPlatform(ctx, claims.Issuer, candidate)
		if err == nil {
			if !p.Active {
				return nil, ErrInactivePlatform
			}
			return p, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("lti: find platform: %w", err)
		}
	}
	return nil, ErrUnregisteredPlatform
}

func audienceCandidates(c *rawLaunchClaims) []string {
	var out []string
	if c.Azp != "" {
		out = append(out, c.Azp)
	}
	out = append(out, []string(c.Audience)...)
	return out
}

func audienceContains(audience []string, clientID string) bool {
	return slices.Contains(audience, clientID)
}

func (t *Tool) resolveVerificationKey(ctx context.Context, p *Platform, kid string) (any, error) {
	switch p.KeyConfig.Method {
	case KeyConfigMethodRSAKey:
		return internaljose.ParseRSAPublicKeyPEM([]byte(p.KeyConfig.RSAKey))
	case KeyConfigMethodJWK:
		var jwk josepkg.JSONWebKey
		if err := json.Unmarshal([]byte(p.KeyConfig.JWK), &jwk); err != nil {
			return nil, fmt.Errorf("lti: parse configured JWK: %w", err)
		}
		return jwk.Key, nil
	case KeyConfigMethodJWKSet:
		return t.jwksFetcher.Key(ctx, p.KeyConfig.JWKSURI, kid)
	default:
		return nil, fmt.Errorf("lti: platform %s has an unknown key config method %q", p.ID, p.KeyConfig.Method)
	}
}

func (t *Tool) validateStandardClaims(c *rawLaunchClaims, p *Platform) error {
	if c.Version != ltiVersion1p3 {
		return errorf(CodeInvalidIDToken, "unsupported lti version %q", c.Version)
	}
	switch c.MessageType {
	case messageTypeResourceLink, messageTypeDeepLinking, messageTypeSubmissionReview:
	default:
		return errorf(CodeInvalidMessageType, "message_type %q is not supported", c.MessageType)
	}
	if c.DeploymentID == "" {
		return wrap(ErrMissingClaim, errors.New("missing deployment_id claim"))
	}
	if c.Nonce == "" {
		return wrap(ErrInvalidNonce, errors.New("missing nonce claim"))
	}
	if c.Issuer != p.Issuer {
		return errorf(CodeInvalidIDToken, "iss %q does not match registered platform issuer %q", c.Issuer, p.Issuer)
	}

	if err := checkExpiry(c.Claims, t.now()); err != nil {
		return err
	}

	if !audienceContains(c.Audience, p.ClientID) {
		return errorf(CodeInvalidAudience, "aud does not include client_id %q", p.ClientID)
	}
	if len(c.Audience) > 1 && c.Azp != p.ClientID {
		return wrap(ErrAzpMismatch, nil)
	}

	if t.iatMaxAge > 0 && c.IssuedAt != nil {
		if t.now().Sub(c.IssuedAt.Time()) > t.iatMaxAge {
			return wrap(ErrTokenTooOld, nil)
		}
	}
	return nil
}

func (t *Tool) checkDeployment(ctx context.Context, p *Platform, deploymentID string) error {
	_, err := t.store.FindDeployment(ctx, p.ID, deploymentID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("lti: find deployment: %w", err)
	}

	if t.onUnknownDeployment == nil {
		return ErrUnknownDeployment
	}
	allow, err := t.onUnknownDeployment(ctx, p, deploymentID)
	if err != nil {
		return err
	}
	if !allow {
		return ErrUnknownDeployment
	}
	if _, err := t.store.CreateDeployment(ctx, &Deployment{PlatformID: p.ID, DeploymentID: deploymentID}); err != nil {
		return fmt.Errorf("lti: register deployment: %w", err)
	}
	return nil
}
