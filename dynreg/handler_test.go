package dynreg_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marcgabe15/lti-go"
	"github.com/marcgabe15/lti-go/dynreg"
	"github.com/marcgabe15/lti-go/memstore"
)

func newFakePlatformServer(t *testing.T, registrationReceived *map[string]any) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var srv *httptest.Server

	mux.HandleFunc("/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 srv.URL,
			"authorization_endpoint": srv.URL + "/auth",
			"token_endpoint":         srv.URL + "/token",
			"registration_endpoint":  srv.URL + "/register",
			"jwks_uri":               srv.URL + "/jwks",
		})
	})
	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer reg-token-123", r.Header.Get("Authorization"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(registrationReceived))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"client_id": "new-client-id",
			"https://purl.imsglobal.org/spec/lti-tool-configuration": map[string]any{
				"deployment_id": "deployment-xyz",
			},
		})
	})

	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestHandler_FullRegistrationFlow(t *testing.T) {
	var registrationReceived map[string]any
	srv := newFakePlatformServer(t, &registrationReceived)

	store := memstore.New()
	handler, err := dynreg.NewHandler(dynreg.Config{
		Store:              store,
		ToolName:           "My Tool",
		InitiateLoginURI:   "https://tool.example.com/lti/login",
		RedirectURIs:       []string{"https://tool.example.com/lti/launch"},
		JWKSURI:            "https://tool.example.com/lti/jwks",
		TargetLinkURI:      "https://tool.example.com/lti/launch",
		SupportDeepLinking: true,
		Scopes:             []string{"https://purl.imsglobal.org/spec/lti-ags/scope/lineitem"},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/lti/register?"+url.Values{
		"openid_configuration": {srv.URL + "/openid-configuration"},
		"registration_token":   {"reg-token-123"},
	}.Encode(), nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "org.imsglobal.lti.close")

	ctx := context.Background()
	platforms, err := store.ListPlatforms(ctx, nil)
	require.NoError(t, err)
	require.Len(t, platforms, 1)
	assert.Equal(t, srv.URL, platforms[0].Issuer)
	assert.Equal(t, "new-client-id", platforms[0].ClientID)
	assert.False(t, platforms[0].Active, "dynamically registered platforms should start inactive pending review")
	assert.Equal(t, srv.URL+"/jwks", platforms[0].KeyConfig.JWKSURI)
	assert.Equal(t, srv.URL+"/auth", platforms[0].AuthenticationEndpoint)
	assert.Equal(t, srv.URL+"/token", platforms[0].AccessTokenEndpoint)

	deployments, err := store.ListDeployments(ctx, platforms[0].ID)
	require.NoError(t, err)
	require.Len(t, deployments, 1)
	assert.Equal(t, "deployment-xyz", deployments[0].DeploymentID)

	assert.Equal(t, "https://tool.example.com/lti/login", registrationReceived["initiate_login_uri"])
	assert.Equal(t, "private_key_jwt", registrationReceived["token_endpoint_auth_method"])
	toolConfig, ok := registrationReceived["https://purl.imsglobal.org/spec/lti-tool-configuration"].(map[string]any)
	require.True(t, ok)
	messages, ok := toolConfig["messages"].([]any)
	require.True(t, ok)
	assert.Len(t, messages, 2, "resource link + deep linking messages should both be advertised")
}

func TestHandler_MissingOpenIDConfiguration(t *testing.T) {
	store := memstore.New()
	handler, err := dynreg.NewHandler(dynreg.Config{
		Store:            store,
		InitiateLoginURI: "https://tool.example.com/lti/login",
		RedirectURIs:     []string{"https://tool.example.com/lti/launch"},
		JWKSURI:          "https://tool.example.com/lti/jwks",
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/lti/register", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandler_RegistrationEndpointFailure(t *testing.T) {
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 srv.URL,
			"authorization_endpoint": srv.URL + "/auth",
			"token_endpoint":         srv.URL + "/token",
			"registration_endpoint":  srv.URL + "/register",
			"jwks_uri":               srv.URL + "/jwks",
		})
	})
	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	srv = httptest.NewServer(mux)
	defer srv.Close()

	store := memstore.New()
	handler, err := dynreg.NewHandler(dynreg.Config{
		Store:            store,
		InitiateLoginURI: "https://tool.example.com/lti/login",
		RedirectURIs:     []string{"https://tool.example.com/lti/launch"},
		JWKSURI:          "https://tool.example.com/lti/jwks",
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/lti/register?"+url.Values{
		"openid_configuration": {srv.URL + "/openid-configuration"},
	}.Encode(), nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadGateway, rec.Code)

	platforms, err := store.ListPlatforms(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, platforms)
}

func TestHandler_OnRegisteredHookOverridesDefaultPage(t *testing.T) {
	var registrationReceived map[string]any
	srv := newFakePlatformServer(t, &registrationReceived)

	store := memstore.New()
	var hookCalled bool
	var hookPlatform *lti.Platform
	handler, err := dynreg.NewHandler(dynreg.Config{
		Store:            store,
		InitiateLoginURI: "https://tool.example.com/lti/login",
		RedirectURIs:     []string{"https://tool.example.com/lti/launch"},
		JWKSURI:          "https://tool.example.com/lti/jwks",
		OnRegistered: func(w http.ResponseWriter, r *http.Request, platform *lti.Platform) {
			hookCalled = true
			hookPlatform = platform
			w.WriteHeader(http.StatusCreated)
		},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/lti/register?"+url.Values{
		"openid_configuration": {srv.URL + "/openid-configuration"},
		"registration_token":   {"reg-token-123"},
	}.Encode(), nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.True(t, hookCalled)
	require.NotNil(t, hookPlatform)
	assert.Equal(t, "new-client-id", hookPlatform.ClientID)
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.NotContains(t, rec.Body.String(), "org.imsglobal.lti.close")
}

func TestNewHandler_ValidatesConfig(t *testing.T) {
	_, err := dynreg.NewHandler(dynreg.Config{})
	assert.Error(t, err)

	_, err = dynreg.NewHandler(dynreg.Config{Store: memstore.New()})
	assert.Error(t, err, "missing InitiateLoginURI/RedirectURIs/JWKSURI should be rejected")
}
