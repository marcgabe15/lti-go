package lti_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marcgabe15/lti-go"
	"github.com/marcgabe15/lti-go/ltitest"
	"github.com/marcgabe15/lti-go/memstore"
)

func TestLogin_UnregisteredPlatform(t *testing.T) {
	store := memstore.New()
	tool, err := lti.New(lti.Config{Issuer: "https://tool.example.com", Store: store})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/lti/login?"+url.Values{
		"iss":             {"https://unknown-platform.example.com"},
		"login_hint":      {"user"},
		"target_link_uri": {"https://tool.example.com/lti/launch"},
	}.Encode(), nil)
	rec := httptest.NewRecorder()
	tool.LoginHandler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestLogin_InactivePlatform(t *testing.T) {
	fp, err := ltitest.NewFakePlatform("https://platform.example.com", "client-1")
	require.NoError(t, err)
	defer fp.Close()

	store := memstore.New()
	tool, err := lti.New(lti.Config{Issuer: "https://tool.example.com", Store: store})
	require.NoError(t, err)

	ctx := context.Background()
	_, err = tool.Platforms().Register(ctx, &lti.Platform{
		Issuer:                 fp.Issuer,
		ClientID:               fp.ClientID,
		AuthenticationEndpoint: fp.AuthenticationEndpoint(),
		Active:                 false,
		KeyConfig:              lti.KeyConfig{Method: lti.KeyConfigMethodJWKSet, JWKSURI: fp.JWKSURI()},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/lti/login?"+url.Values{
		"iss":             {fp.Issuer},
		"login_hint":      {"user"},
		"target_link_uri": {"https://tool.example.com/lti/launch"},
		"client_id":       {fp.ClientID},
	}.Encode(), nil)
	rec := httptest.NewRecorder()
	tool.LoginHandler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func setUpToolAndPlatform(t *testing.T) (*lti.Tool, *ltitest.FakePlatform) {
	t.Helper()
	fp, err := ltitest.NewFakePlatform("https://platform.example.com", "client-1")
	require.NoError(t, err)
	t.Cleanup(fp.Close)

	store := memstore.New()
	tool, err := lti.New(lti.Config{Issuer: "https://tool.example.com", Store: store})
	require.NoError(t, err)

	ctx := context.Background()
	platform, err := tool.Platforms().Register(ctx, &lti.Platform{
		Issuer:                 fp.Issuer,
		ClientID:               fp.ClientID,
		AuthenticationEndpoint: fp.AuthenticationEndpoint(),
		Active:                 true,
		KeyConfig:              lti.KeyConfig{Method: lti.KeyConfigMethodJWKSet, JWKSURI: fp.JWKSURI()},
	})
	require.NoError(t, err)
	_, err = tool.Platforms().Deployments(platform.ID).Register(ctx, "deployment-1", "")
	require.NoError(t, err)

	return tool, fp
}

func TestLaunch_ReusedNonceRejected(t *testing.T) {
	tool, fp := setUpToolAndPlatform(t)

	// Perform login to obtain a valid state+nonce pair.
	loginReq := httptest.NewRequest(http.MethodGet, "/lti/login?"+url.Values{
		"iss":             {fp.Issuer},
		"login_hint":      {"user"},
		"target_link_uri": {"https://tool.example.com/lti/launch"},
		"client_id":       {fp.ClientID},
	}.Encode(), nil)
	loginRec := httptest.NewRecorder()
	tool.LoginHandler().ServeHTTP(loginRec, loginReq)
	require.Equal(t, http.StatusFound, loginRec.Code)

	redirect, err := url.Parse(loginRec.Header().Get("Location"))
	require.NoError(t, err)
	nonce := redirect.Query().Get("nonce")
	state := redirect.Query().Get("state")

	idToken, err := fp.SignIDToken(ltitest.IDTokenClaims{
		Subject: "user-1", DeploymentID: "deployment-1",
		TargetLinkURI: "https://tool.example.com/lti/launch", Nonce: nonce,
	})
	require.NoError(t, err)

	launch := func() *httptest.ResponseRecorder {
		form := url.Values{"id_token": {idToken}, "state": {state}}
		req := httptest.NewRequest(http.MethodPost, "/lti/launch", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		tool.LaunchHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })).ServeHTTP(rec, req)
		return rec
	}

	first := launch()
	assert.Equal(t, http.StatusOK, first.Code, "first launch body: %s", first.Body.String())

	second := launch()
	assert.Equal(t, http.StatusBadRequest, second.Code, "replayed nonce should be rejected")
}

func TestLaunch_AzpMismatchRejected(t *testing.T) {
	tool, fp := setUpToolAndPlatform(t)

	// A multi-value aud where azp doesn't match the registered client_id
	// must be rejected per the LTI/OIDC spec, even though the aud list
	// does contain the registered client_id.
	result, err := fp.Launch(tool, "https://tool.example.com/lti/launch", ltitest.IDTokenClaims{
		Subject:       "user-1",
		DeploymentID:  "deployment-1",
		TargetLinkURI: "https://tool.example.com/lti/launch",
		Extra: map[string]any{
			"aud": []string{fp.ClientID, "some-other-audience"},
			"azp": "some-other-audience",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, result.LaunchRecorder.Code, "launch response body: %s", result.LaunchRecorder.Body.String())
	assert.Nil(t, result.Claims)
}

func TestLaunch_ExpiredIDTokenRejected(t *testing.T) {
	tool, fp := setUpToolAndPlatform(t)

	result, err := fp.Launch(tool, "https://tool.example.com/lti/launch", ltitest.IDTokenClaims{
		Subject:       "user-1",
		DeploymentID:  "deployment-1",
		TargetLinkURI: "https://tool.example.com/lti/launch",
		IssuedAt:      time.Now().Add(-time.Hour),
		ExpiresAt:     time.Now().Add(-time.Minute),
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, result.LaunchRecorder.Code)
	assert.Nil(t, result.Claims)
}

func TestLaunch_StaleIssuedAtRejected(t *testing.T) {
	tool, fp := setUpToolAndPlatform(t)

	result, err := fp.Launch(tool, "https://tool.example.com/lti/launch", ltitest.IDTokenClaims{
		Subject:       "user-1",
		DeploymentID:  "deployment-1",
		TargetLinkURI: "https://tool.example.com/lti/launch",
		IssuedAt:      time.Now().Add(-time.Minute), // older than the default 10s iat max age
		ExpiresAt:     time.Now().Add(5 * time.Minute),
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, result.LaunchRecorder.Code)
	assert.Nil(t, result.Claims)
}

func TestLaunch_MissingDeploymentIDRejected(t *testing.T) {
	tool, fp := setUpToolAndPlatform(t)

	result, err := fp.Launch(tool, "https://tool.example.com/lti/launch", ltitest.IDTokenClaims{
		Subject:       "user-1",
		TargetLinkURI: "https://tool.example.com/lti/launch",
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, result.LaunchRecorder.Code)
	assert.Nil(t, result.Claims)
}
