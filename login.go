package lti

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/marcgabe15/lti-go/internal/idgen"
)

const (
	nonceIDLength = 16
	stateIDLength = 16
)

var errMissingLoginParams = errors.New("missing required parameter: iss, login_hint, or target_link_uri")
var errAmbiguousPlatform = errors.New("multiple platforms registered for issuer; client_id is required")

// LoginRequest is a parsed OIDC third-party login initiation request sent
// by a platform to start an LTI launch.
type LoginRequest struct {
	Issuer        string
	LoginHint     string
	TargetLinkURI string
	ClientID      string
	DeploymentID  string
	MessageHint   string
}

func parseLoginRequest(r *http.Request) (*LoginRequest, error) {
	if err := r.ParseForm(); err != nil {
		return nil, wrap(ErrInvalidLoginRequest, err)
	}
	v := r.Form
	lr := &LoginRequest{
		Issuer:        v.Get("iss"),
		LoginHint:     v.Get("login_hint"),
		TargetLinkURI: v.Get("target_link_uri"),
		ClientID:      v.Get("client_id"),
		DeploymentID:  v.Get("lti_deployment_id"),
		MessageHint:   v.Get("lti_message_hint"),
	}
	if lr.Issuer == "" || lr.LoginHint == "" || lr.TargetLinkURI == "" {
		return nil, wrap(ErrInvalidLoginRequest, errMissingLoginParams)
	}
	return lr, nil
}

// LoginHandler returns an http.Handler for the OIDC third-party login
// initiation endpoint. A platform sends the browser here first; it
// resolves the registered Platform, issues a nonce and signed state, and
// redirects the browser to the platform's authentication endpoint.
func (t *Tool) LoginHandler() http.Handler {
	return http.HandlerFunc(t.handleLogin)
}

func (t *Tool) handleLogin(w http.ResponseWriter, r *http.Request) {
	lr, err := parseLoginRequest(r)
	if err != nil {
		t.writeError(w, r, err)
		return
	}

	ctx := r.Context()
	platform, err := t.resolvePlatformForLogin(ctx, lr)
	if err != nil {
		t.writeError(w, r, err)
		return
	}

	nonce, err := idgen.New(nonceIDLength)
	if err != nil {
		t.writeError(w, r, fmt.Errorf("lti: generate nonce: %w", err))
		return
	}
	if err := t.store.SaveNonce(ctx, nonce, t.now().Add(t.nonceTTL)); err != nil {
		t.writeError(w, r, fmt.Errorf("lti: save nonce: %w", err))
		return
	}

	stateID, err := idgen.New(stateIDLength)
	if err != nil {
		t.writeError(w, r, fmt.Errorf("lti: generate state id: %w", err))
		return
	}
	state, err := t.signState(ctx, platform, stateID, lr.TargetLinkURI)
	if err != nil {
		t.writeError(w, r, err)
		return
	}

	redirectURL, err := buildAuthRequestURL(platform, lr, state, nonce)
	if err != nil {
		t.writeError(w, r, err)
		return
	}
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (t *Tool) resolvePlatformForLogin(ctx context.Context, lr *LoginRequest) (*Platform, error) {
	if lr.ClientID != "" {
		p, err := t.store.FindPlatform(ctx, lr.Issuer, lr.ClientID)
		if err != nil {
			return nil, wrap(ErrUnregisteredPlatform, err)
		}
		if !p.Active {
			return nil, ErrInactivePlatform
		}
		return p, nil
	}

	platforms, err := t.store.ListPlatforms(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("lti: list platforms: %w", err)
	}
	var match *Platform
	for _, p := range platforms {
		if p.Issuer != lr.Issuer {
			continue
		}
		if match != nil {
			return nil, wrap(ErrUnregisteredPlatform, errAmbiguousPlatform)
		}
		match = p
	}
	if match == nil {
		return nil, ErrUnregisteredPlatform
	}
	if !match.Active {
		return nil, ErrInactivePlatform
	}
	return match, nil
}

func buildAuthRequestURL(p *Platform, lr *LoginRequest, state, nonce string) (string, error) {
	u, err := url.Parse(p.AuthenticationEndpoint)
	if err != nil {
		return "", fmt.Errorf("lti: invalid authentication endpoint: %w", err)
	}
	q := u.Query()
	q.Set("scope", "openid")
	q.Set("response_type", "id_token")
	q.Set("response_mode", "form_post")
	q.Set("prompt", "none")
	q.Set("client_id", p.ClientID)
	q.Set("redirect_uri", lr.TargetLinkURI)
	q.Set("login_hint", lr.LoginHint)
	q.Set("state", state)
	q.Set("nonce", nonce)
	q.Set("id_token_signed_response_alg", "RS256")
	if lr.MessageHint != "" {
		q.Set("lti_message_hint", lr.MessageHint)
	}
	if lr.DeploymentID != "" {
		q.Set("lti_deployment_id", lr.DeploymentID)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}
