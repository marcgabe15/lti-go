package dynreg

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/marcgabe15/lti-go"
)

const defaultToolName = "LTI Tool"

// Config holds the required and optional configuration for a Handler.
type Config struct {
	// Store is where newly registered platforms and deployments are
	// persisted. Required.
	Store lti.Store

	// ToolName is the client_name sent to the platform. Defaults to
	// "LTI Tool".
	ToolName string
	// InitiateLoginURI is this tool's OIDC login-initiation endpoint
	// (the URL Tool.LoginHandler is mounted at). Required.
	InitiateLoginURI string
	// RedirectURIs are this tool's launch endpoint URL(s) (the URL(s)
	// Tool.LaunchHandler is mounted at). Required, at least one.
	RedirectURIs []string
	// JWKSURI is this tool's own JWKS endpoint URL (the URL
	// Tool.JWKSHandler is mounted at). Required.
	JWKSURI string
	// TargetLinkURI is the default target_link_uri advertised for
	// resource-link launches.
	TargetLinkURI string

	// SupportDeepLinking advertises an LtiDeepLinkingRequest message in
	// addition to the resource-link message.
	SupportDeepLinking bool
	// Scopes are the OAuth2 scopes this tool requests (for example AGS's
	// ags.ScopeLineItem, NRPS's nrps.ScopeContextMembershipReadonly).
	Scopes []string
	// CustomParameters are default custom launch parameters requested
	// for every message type.
	CustomParameters map[string]string

	// HTTPClient is used to fetch the platform's OpenID configuration
	// and to call its registration endpoint. Defaults to
	// http.DefaultClient.
	HTTPClient *http.Client

	// OnRegistered is called after the new Platform (and any deployment)
	// has been persisted, so the caller can render a branded completion
	// page. If nil, a minimal default page is written that posts
	// {subject: "org.imsglobal.lti.close"} to close the registration
	// popup, per the Dynamic Registration specification's completion
	// flow.
	OnRegistered func(w http.ResponseWriter, r *http.Request, platform *lti.Platform)
}

// Handler serves the LTI Dynamic Registration initiation endpoint.
// Construct one with NewHandler and mount it at the URL you advertise to
// platforms for registration (commonly something like
// "/lti/register").
type Handler struct {
	cfg Config
}

// NewHandler validates cfg and returns a Handler.
func NewHandler(cfg Config) (*Handler, error) {
	if cfg.Store == nil {
		return nil, errors.New("dynreg: Config.Store is required")
	}
	if cfg.InitiateLoginURI == "" {
		return nil, errors.New("dynreg: Config.InitiateLoginURI is required")
	}
	if len(cfg.RedirectURIs) == 0 {
		return nil, errors.New("dynreg: Config.RedirectURIs must have at least one entry")
	}
	if cfg.JWKSURI == "" {
		return nil, errors.New("dynreg: Config.JWKSURI is required")
	}
	if cfg.ToolName == "" {
		cfg.ToolName = defaultToolName
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	return &Handler{cfg: cfg}, nil
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	openIDConfigURL := r.URL.Query().Get("openid_configuration")
	if openIDConfigURL == "" {
		http.Error(w, "dynreg: missing openid_configuration query parameter", http.StatusBadRequest)
		return
	}
	registrationToken := r.URL.Query().Get("registration_token")

	platformConfig, err := h.fetchOpenIDConfiguration(ctx, openIDConfigURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	regResp, err := h.register(ctx, platformConfig.RegistrationEndpoint, registrationToken, h.buildRegistrationRequest())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	platform, err := h.persistPlatform(ctx, platformConfig, regResp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if h.cfg.OnRegistered != nil {
		h.cfg.OnRegistered(w, r, platform)
		return
	}
	writeDefaultCompletionPage(w)
}

func (h *Handler) fetchOpenIDConfiguration(ctx context.Context, configURL string) (*OpenIDConfiguration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, configURL, nil)
	if err != nil {
		return nil, fmt.Errorf("dynreg: build request: %w", err)
	}
	resp, err := h.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dynreg: fetch platform openid configuration: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("dynreg: platform returned %d fetching openid configuration: %s", resp.StatusCode, body)
	}
	var config OpenIDConfiguration
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, fmt.Errorf("dynreg: decode platform openid configuration: %w", err)
	}
	if config.Issuer == "" || config.RegistrationEndpoint == "" || config.AuthorizationEndpoint == "" || config.JWKSURI == "" {
		return nil, errors.New("dynreg: platform openid configuration is missing required fields")
	}
	return &config, nil
}

func (h *Handler) buildRegistrationRequest() *registrationRequest {
	messages := []ToolMessage{{
		Type:             "LtiResourceLinkRequest",
		TargetLinkURI:    h.cfg.TargetLinkURI,
		CustomParameters: h.cfg.CustomParameters,
	}}
	if h.cfg.SupportDeepLinking {
		messages = append(messages, ToolMessage{
			Type:          "LtiDeepLinkingRequest",
			TargetLinkURI: h.cfg.TargetLinkURI,
		})
	}

	return &registrationRequest{
		ClientName:              h.cfg.ToolName,
		InitiateLoginURI:        h.cfg.InitiateLoginURI,
		RedirectURIs:            h.cfg.RedirectURIs,
		JWKSURI:                 h.cfg.JWKSURI,
		TokenEndpointAuthMethod: "private_key_jwt",
		GrantTypes:              []string{"client_credentials", "implicit"},
		ResponseTypes:           []string{"id_token"},
		ApplicationType:         "web",
		Scope:                   strings.Join(h.cfg.Scopes, " "),
		ToolConfiguration: ToolConfiguration{
			Domain:           domainOf(h.cfg.TargetLinkURI),
			TargetLinkURI:    h.cfg.TargetLinkURI,
			CustomParameters: h.cfg.CustomParameters,
			Messages:         messages,
		},
	}
}

func (h *Handler) register(ctx context.Context, registrationEndpoint, registrationToken string, reqBody *registrationRequest) (*registrationResponse, error) {
	if registrationEndpoint == "" {
		return nil, errors.New("dynreg: platform openid configuration did not include a registration_endpoint")
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("dynreg: marshal registration request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, registrationEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("dynreg: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if registrationToken != "" {
		req.Header.Set("Authorization", "Bearer "+registrationToken)
	}

	resp, err := h.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dynreg: register with platform: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("dynreg: platform returned %d registering tool: %s", resp.StatusCode, respBody)
	}

	var out registrationResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("dynreg: decode registration response: %w", err)
	}
	if out.ClientID == "" {
		return nil, errors.New("dynreg: platform registration response is missing client_id")
	}
	return &out, nil
}

func (h *Handler) persistPlatform(ctx context.Context, oidc *OpenIDConfiguration, reg *registrationResponse) (*lti.Platform, error) {
	platform, err := h.cfg.Store.CreatePlatform(ctx, &lti.Platform{
		Issuer:                 oidc.Issuer,
		ClientID:               reg.ClientID,
		Name:                   h.cfg.ToolName,
		AuthenticationEndpoint: oidc.AuthorizationEndpoint,
		AccessTokenEndpoint:    oidc.TokenEndpoint,
		// New registrations are inactive pending manual review, matching
		// common LTI 1.3 tool practice: dynamic registration is
		// initiated by any platform admin, so a human should confirm
		// the relationship before launches are accepted.
		Active: false,
		KeyConfig: lti.KeyConfig{
			Method: lti.KeyConfigMethodJWKSet,
			// The platform's own JWKS (for verifying id_tokens it
			// signs) comes from its openid_configuration, not from the
			// registration response -- the jwks_uri in that response is
			// an echo of the tool's own jwks_uri we just sent it.
			JWKSURI: oidc.JWKSURI,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("dynreg: register platform: %w", err)
	}

	if reg.ToolConfiguration != nil && reg.ToolConfiguration.DeploymentID != "" {
		if _, err := h.cfg.Store.CreateDeployment(ctx, &lti.Deployment{
			PlatformID:   platform.ID,
			DeploymentID: reg.ToolConfiguration.DeploymentID,
		}); err != nil {
			return nil, fmt.Errorf("dynreg: register deployment: %w", err)
		}
	}
	return platform, nil
}

func domainOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

const defaultCompletionHTML = `<!DOCTYPE html>
<html>
<head><title>Registration Complete</title></head>
<body>
<p>Registration complete. This window will close automatically.</p>
<script>
(window.opener || window.parent).postMessage({subject: 'org.imsglobal.lti.close'}, '*');
</script>
</body>
</html>
`

func writeDefaultCompletionPage(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, defaultCompletionHTML)
}
