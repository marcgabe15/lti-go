// Package ltitest provides test helpers for exercising a Tool without a
// live platform: a FakePlatform that generates its own signing key and
// serves JWKS, a RoundTripper for mocking outbound HTTP, and a
// manually-advanceable Clock.
package ltitest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	josepkg "github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"

	"github.com/marcgabe15/lti-go"
)

// FakePlatform simulates an LTI platform (LMS) for testing tool
// implementations end-to-end without a live LMS. It generates its own RSA
// key pair and serves authorization/JWKS endpoints via httptest.Server.
type FakePlatform struct {
	Issuer   string
	ClientID string

	// LastLoginRequest captures the form values of the most recent
	// request to the fake authorization endpoint.
	LastLoginRequest url.Values

	server *httptest.Server
	priv   *rsa.PrivateKey
	kid    string
}

// NewFakePlatform starts a fake platform with its own RSA key pair. Call
// Close when done. issuer and clientID identify the platform/tool pair as
// they would appear in a real registration.
func NewFakePlatform(issuer, clientID string) (*FakePlatform, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("ltitest: generate key: %w", err)
	}
	fp := &FakePlatform{Issuer: issuer, ClientID: clientID, priv: priv, kid: "fake-platform-key"}

	mux := http.NewServeMux()
	mux.HandleFunc("/auth", fp.handleAuth)
	mux.HandleFunc("/jwks", fp.handleJWKS)
	fp.server = httptest.NewServer(mux)
	return fp, nil
}

// Close shuts down the fake platform's HTTP server.
func (fp *FakePlatform) Close() { fp.server.Close() }

// AuthenticationEndpoint returns the fake platform's OIDC authorization
// endpoint URL, suitable for Platform.AuthenticationEndpoint.
func (fp *FakePlatform) AuthenticationEndpoint() string { return fp.server.URL + "/auth" }

// JWKSURI returns the fake platform's JWKS endpoint URL, suitable for
// Platform.KeyConfig.JWKSURI.
func (fp *FakePlatform) JWKSURI() string { return fp.server.URL + "/jwks" }

// PublicKeyPEM returns the fake platform's public key, PEM-encoded --
// useful for registering a Platform with KeyConfigMethodRSAKey instead of
// JWKSURI.
func (fp *FakePlatform) PublicKeyPEM() ([]byte, error) {
	der, err := x509.MarshalPKIXPublicKey(&fp.priv.PublicKey)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), nil
}

func (fp *FakePlatform) handleAuth(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	fp.LastLoginRequest = r.Form
	w.WriteHeader(http.StatusOK)
}

func (fp *FakePlatform) handleJWKS(w http.ResponseWriter, r *http.Request) {
	set := josepkg.JSONWebKeySet{Keys: []josepkg.JSONWebKey{{
		Key:       &fp.priv.PublicKey,
		KeyID:     fp.kid,
		Algorithm: string(josepkg.RS256),
		Use:       "sig",
	}}}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(set)
}

// IDTokenClaims are the claims to embed in a signed id_token built by
// SignIDToken or driven automatically by Launch.
type IDTokenClaims struct {
	Subject       string
	DeploymentID  string
	MessageType   string
	TargetLinkURI string
	Nonce         string
	Roles         []string
	Extra         map[string]any
	IssuedAt      time.Time
	ExpiresAt     time.Time
}

// SignIDToken builds and RS256-signs an id_token as this fake platform,
// for a launch to fp.ClientID.
func (fp *FakePlatform) SignIDToken(c IDTokenClaims) (string, error) {
	now := time.Now()
	if c.IssuedAt.IsZero() {
		c.IssuedAt = now
	}
	if c.ExpiresAt.IsZero() {
		c.ExpiresAt = now.Add(5 * time.Minute)
	}
	if c.MessageType == "" {
		c.MessageType = "LtiResourceLinkRequest"
	}

	claims := map[string]any{
		"iss":   fp.Issuer,
		"sub":   c.Subject,
		"aud":   []string{fp.ClientID},
		"azp":   fp.ClientID,
		"exp":   c.ExpiresAt.Unix(),
		"iat":   c.IssuedAt.Unix(),
		"nonce": c.Nonce,
		"https://purl.imsglobal.org/spec/lti/claim/message_type":    c.MessageType,
		"https://purl.imsglobal.org/spec/lti/claim/version":         "1.3.0",
		"https://purl.imsglobal.org/spec/lti/claim/deployment_id":   c.DeploymentID,
		"https://purl.imsglobal.org/spec/lti/claim/target_link_uri": c.TargetLinkURI,
		"https://purl.imsglobal.org/spec/lti/claim/roles":           c.Roles,
	}
	maps.Copy(claims, c.Extra)

	signer, err := josepkg.NewSigner(josepkg.SigningKey{Algorithm: josepkg.RS256, Key: fp.priv},
		(&josepkg.SignerOptions{}).WithHeader("kid", fp.kid).WithType("JWT"))
	if err != nil {
		return "", fmt.Errorf("ltitest: new signer: %w", err)
	}
	token, err := jwt.Signed(signer).Claims(claims).CompactSerialize()
	if err != nil {
		return "", fmt.Errorf("ltitest: sign id_token: %w", err)
	}
	return token, nil
}

// LaunchResult holds what happened when Launch drove a tool through a
// full login+launch round trip.
type LaunchResult struct {
	LoginRecorder  *httptest.ResponseRecorder
	LaunchRecorder *httptest.ResponseRecorder
	// Claims is the launch context observed by the handler passed to
	// LaunchHandler, or nil if the launch did not reach it.
	Claims *lti.Claims
}

// Launch drives tool's LoginHandler and LaunchHandler exactly as a real
// platform round trip would: it issues a login-initiation request,
// follows the resulting redirect's state/nonce, signs a matching
// id_token as this fake platform, and posts it to the launch handler.
// claims.Nonce is overwritten with the nonce the tool issued during
// login; any other fields on claims are used as given.
func (fp *FakePlatform) Launch(tool *lti.Tool, targetLinkURI string, claims IDTokenClaims) (*LaunchResult, error) {
	loginURL := "/lti/login?" + url.Values{
		"iss":             {fp.Issuer},
		"login_hint":      {"fake-user"},
		"target_link_uri": {targetLinkURI},
		"client_id":       {fp.ClientID},
	}.Encode()
	loginReq := httptest.NewRequest(http.MethodGet, loginURL, nil)
	loginRec := httptest.NewRecorder()
	tool.LoginHandler().ServeHTTP(loginRec, loginReq)

	result := &LaunchResult{LoginRecorder: loginRec}
	if loginRec.Code != http.StatusFound {
		return result, fmt.Errorf("ltitest: login handler returned %d, want %d (body: %s)", loginRec.Code, http.StatusFound, loginRec.Body.String())
	}

	redirect, err := url.Parse(loginRec.Header().Get("Location"))
	if err != nil {
		return result, fmt.Errorf("ltitest: parse redirect location: %w", err)
	}
	q := redirect.Query()
	claims.Nonce = q.Get("nonce")
	state := q.Get("state")

	idToken, err := fp.SignIDToken(claims)
	if err != nil {
		return result, err
	}

	var captured *lti.Claims
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured, _ = lti.ClaimsFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	form := url.Values{"id_token": {idToken}, "state": {state}}
	launchReq := httptest.NewRequest(http.MethodPost, "/lti/launch", strings.NewReader(form.Encode()))
	launchReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	launchRec := httptest.NewRecorder()
	tool.LaunchHandler(next).ServeHTTP(launchRec, launchReq)

	result.LaunchRecorder = launchRec
	result.Claims = captured
	return result, nil
}
