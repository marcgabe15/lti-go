// Package dynreg implements the LTI Dynamic Registration flow: a
// platform admin is redirected to this tool with a pointer to the
// platform's OpenID configuration, the tool registers itself against the
// platform's registration endpoint, and the resulting Platform is
// persisted (inactive, pending review) via a lti.Store.
//
// See https://www.imsglobal.org/spec/lti-dr/v1p0.
package dynreg

// PlatformMessage describes one LTI message type a platform supports, as
// advertised in its OpenID configuration.
type PlatformMessage struct {
	Type       string   `json:"type"`
	Placements []string `json:"placements,omitempty"`
}

// PlatformConfiguration is the LTI-specific portion of a platform's
// OpenID configuration document.
type PlatformConfiguration struct {
	ProductFamilyCode  string            `json:"product_family_code,omitempty"`
	Version            string            `json:"version,omitempty"`
	MessagesSupported  []PlatformMessage `json:"messages_supported,omitempty"`
	VariablesSupported []string          `json:"variables_supported,omitempty"`
}

// OpenIDConfiguration is a platform's OpenID configuration document, as
// fetched from the openid_configuration URL the platform redirects the
// registration request to.
type OpenIDConfiguration struct {
	Issuer                string                 `json:"issuer"`
	AuthorizationEndpoint string                 `json:"authorization_endpoint"`
	TokenEndpoint         string                 `json:"token_endpoint"`
	RegistrationEndpoint  string                 `json:"registration_endpoint"`
	JWKSURI               string                 `json:"jwks_uri"`
	ScopesSupported       []string               `json:"scopes_supported,omitempty"`
	PlatformConfiguration *PlatformConfiguration `json:"https://purl.imsglobal.org/spec/lti-platform-configuration,omitempty"`
}

// ToolMessage declares one LTI message type this tool supports when
// registering.
type ToolMessage struct {
	Type             string            `json:"type"`
	TargetLinkURI    string            `json:"target_link_uri,omitempty"`
	Label            string            `json:"label,omitempty"`
	IconURI          string            `json:"icon_uri,omitempty"`
	CustomParameters map[string]string `json:"custom_parameters,omitempty"`
	Placements       []string          `json:"placements,omitempty"`
}

// ToolConfiguration is the LTI-specific portion of the registration
// request this tool sends to the platform.
type ToolConfiguration struct {
	Domain           string            `json:"domain,omitempty"`
	Description      string            `json:"description,omitempty"`
	TargetLinkURI    string            `json:"target_link_uri"`
	CustomParameters map[string]string `json:"custom_parameters,omitempty"`
	Claims           []string          `json:"claims,omitempty"`
	Messages         []ToolMessage     `json:"messages,omitempty"`
}

// registrationRequest is the OpenID Connect Dynamic Client Registration
// request body, extended with the LTI tool configuration claim.
type registrationRequest struct {
	ClientName              string            `json:"client_name"`
	InitiateLoginURI        string            `json:"initiate_login_uri"`
	RedirectURIs            []string          `json:"redirect_uris"`
	JWKSURI                 string            `json:"jwks_uri"`
	TokenEndpointAuthMethod string            `json:"token_endpoint_auth_method"`
	GrantTypes              []string          `json:"grant_types"`
	ResponseTypes           []string          `json:"response_types"`
	ApplicationType         string            `json:"application_type"`
	Scope                   string            `json:"scope,omitempty"`
	ToolConfiguration       ToolConfiguration `json:"https://purl.imsglobal.org/spec/lti-tool-configuration"`
}

// responseToolConfiguration is the LTI-specific portion of a successful
// registration response: the platform's assigned deployment_id, plus an
// echo of some request fields.
type responseToolConfiguration struct {
	DeploymentID string `json:"deployment_id,omitempty"`
}

// registrationResponse is the platform's response to a successful
// registration request.
type registrationResponse struct {
	ClientID          string                     `json:"client_id"`
	ToolConfiguration *responseToolConfiguration `json:"https://purl.imsglobal.org/spec/lti-tool-configuration,omitempty"`
}
