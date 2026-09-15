package lti

import (
	"time"

	"github.com/go-jose/go-jose/v3/jwt"
)

// MessageType is the LTI message type of a launch.
type MessageType string

const (
	MessageTypeResourceLinkRequest     MessageType = messageTypeResourceLink
	MessageTypeDeepLinkingRequest      MessageType = messageTypeDeepLinking
	MessageTypeSubmissionReviewRequest MessageType = messageTypeSubmissionReview
)

// ContextClaim is the LTI context claim: the course, section, or other
// container the launch occurred in.
type ContextClaim struct {
	ID    string
	Label string
	Title string
	Type  []string
}

// ResourceLinkClaim is the LTI resource_link claim.
type ResourceLinkClaim struct {
	ID          string
	Title       string
	Description string
}

// LaunchPresentationClaim describes how the platform is presenting the
// tool (e.g. iframe vs new window, dimensions, locale).
type LaunchPresentationClaim struct {
	DocumentTarget string
	Height         int
	Width          int
	ReturnURL      string
	Locale         string
}

// AGSEndpointClaim describes the Assignment and Grade Services endpoints
// and scopes granted to this launch. Present only when the platform
// granted AGS access; nil otherwise. Consumed by the ags package
// (added in a later phase).
type AGSEndpointClaim struct {
	Scope     []string
	LineItems string
	LineItem  string
}

// NRPSEndpointClaim describes the Names and Role Provisioning Service
// endpoint granted to this launch. Present only when the platform granted
// NRPS access; nil otherwise. Consumed by the nrps package (added in a
// later phase).
type NRPSEndpointClaim struct {
	ContextMembershipsURL string
	ServiceVersions       []string
}

// DeepLinkingSettingsClaim describes a deep linking request's
// constraints. Present only when MessageType ==
// MessageTypeDeepLinkingRequest. Consumed by the deeplink package (added
// in a later phase).
type DeepLinkingSettingsClaim struct {
	DeepLinkReturnURL string
	AcceptTypes       []string
	AcceptMediaTypes  string
	AcceptMultiple    bool
	AutoCreate        bool
	Title             string
	Text              string
	Data              string
}

// Claims is the verified, parsed representation of an LTI launch (the
// platform's id_token). It is available from a request's context after
// Tool.LaunchHandler or Tool.VerifyLaunch succeeds.
type Claims struct {
	MessageType   MessageType
	Issuer        string
	Subject       string
	Audience      []string
	ClientID      string
	DeploymentID  string
	TargetLinkURI string
	IssuedAt      time.Time
	ExpiresAt     time.Time
	Nonce         string
	Roles         []string

	Context            *ContextClaim
	ResourceLink       *ResourceLinkClaim
	LaunchPresentation *LaunchPresentationClaim
	AGSEndpoint        *AGSEndpointClaim
	NRPSEndpoint       *NRPSEndpointClaim
	DeepLinking        *DeepLinkingSettingsClaim

	Custom map[string]string

	// LTIK is the opaque session-resumption token issued for this launch
	// by Tool.LaunchHandler. Pass it to Tool.VerifyLTIK to resume the
	// launch from a later, unrelated request.
	LTIK string

	platform *Platform
}

// Platform returns the resolved Platform this launch came from.
func (c *Claims) Platform() *Platform { return c.platform }

// rawLaunchClaims mirrors the wire representation of an LTI 1.3 id_token:
// standard JWT claims plus every LTI/AGS/NRPS/Deep-Linking claim this
// package understands, keyed by their (URI) claim names.
type rawLaunchClaims struct {
	jwt.Claims
	Nonce string `json:"nonce"`
	Azp   string `json:"azp,omitempty"`

	MessageType         string                       `json:"https://purl.imsglobal.org/spec/lti/claim/message_type"`
	Version             string                       `json:"https://purl.imsglobal.org/spec/lti/claim/version"`
	DeploymentID        string                       `json:"https://purl.imsglobal.org/spec/lti/claim/deployment_id"`
	TargetLinkURI       string                       `json:"https://purl.imsglobal.org/spec/lti/claim/target_link_uri"`
	Roles               []string                     `json:"https://purl.imsglobal.org/spec/lti/claim/roles"`
	Context             *rawContextClaim             `json:"https://purl.imsglobal.org/spec/lti/claim/context,omitempty"`
	ResourceLink        *rawResourceLinkClaim        `json:"https://purl.imsglobal.org/spec/lti/claim/resource_link,omitempty"`
	LaunchPresentation  *rawLaunchPresentationClaim  `json:"https://purl.imsglobal.org/spec/lti/claim/launch_presentation,omitempty"`
	Custom              map[string]string            `json:"https://purl.imsglobal.org/spec/lti/claim/custom,omitempty"`
	AGSEndpoint         *rawAGSEndpointClaim         `json:"https://purl.imsglobal.org/spec/lti-ags/claim/endpoint,omitempty"`
	NRPSEndpoint        *rawNRPSEndpointClaim        `json:"https://purl.imsglobal.org/spec/lti-nrps/claim/namesroleservice,omitempty"`
	DeepLinkingSettings *rawDeepLinkingSettingsClaim `json:"https://purl.imsglobal.org/spec/lti-dl/claim/deep_linking_settings,omitempty"`
}

type rawContextClaim struct {
	ID    string   `json:"id"`
	Label string   `json:"label,omitempty"`
	Title string   `json:"title,omitempty"`
	Type  []string `json:"type,omitempty"`
}

type rawResourceLinkClaim struct {
	ID          string `json:"id"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

type rawLaunchPresentationClaim struct {
	DocumentTarget string `json:"document_target,omitempty"`
	Height         int    `json:"height,omitempty"`
	Width          int    `json:"width,omitempty"`
	ReturnURL      string `json:"return_url,omitempty"`
	Locale         string `json:"locale,omitempty"`
}

type rawAGSEndpointClaim struct {
	Scope     []string `json:"scope,omitempty"`
	LineItems string   `json:"lineitems,omitempty"`
	LineItem  string   `json:"lineitem,omitempty"`
}

type rawNRPSEndpointClaim struct {
	ContextMembershipsURL string   `json:"context_memberships_url,omitempty"`
	ServiceVersions       []string `json:"service_versions,omitempty"`
}

type rawDeepLinkingSettingsClaim struct {
	DeepLinkReturnURL string   `json:"deep_link_return_url,omitempty"`
	AcceptTypes       []string `json:"accept_types,omitempty"`
	AcceptMediaTypes  string   `json:"accept_media_types,omitempty"`
	AcceptMultiple    bool     `json:"accept_multiple,omitempty"`
	AutoCreate        bool     `json:"auto_create,omitempty"`
	Title             string   `json:"title,omitempty"`
	Text              string   `json:"text,omitempty"`
	Data              string   `json:"data,omitempty"`
}

func buildClaims(c *rawLaunchClaims, p *Platform) *Claims {
	claims := &Claims{
		MessageType:   MessageType(c.MessageType),
		Issuer:        c.Issuer,
		Subject:       c.Subject,
		Audience:      []string(c.Audience),
		ClientID:      p.ClientID,
		DeploymentID:  c.DeploymentID,
		TargetLinkURI: c.TargetLinkURI,
		Nonce:         c.Nonce,
		Roles:         c.Roles,
		Custom:        c.Custom,
		platform:      p,
	}
	if c.IssuedAt != nil {
		claims.IssuedAt = c.IssuedAt.Time()
	}
	if c.Expiry != nil {
		claims.ExpiresAt = c.Expiry.Time()
	}
	if c.Context != nil {
		claims.Context = &ContextClaim{ID: c.Context.ID, Label: c.Context.Label, Title: c.Context.Title, Type: c.Context.Type}
	}
	if c.ResourceLink != nil {
		claims.ResourceLink = &ResourceLinkClaim{ID: c.ResourceLink.ID, Title: c.ResourceLink.Title, Description: c.ResourceLink.Description}
	}
	if c.LaunchPresentation != nil {
		claims.LaunchPresentation = &LaunchPresentationClaim{
			DocumentTarget: c.LaunchPresentation.DocumentTarget,
			Height:         c.LaunchPresentation.Height,
			Width:          c.LaunchPresentation.Width,
			ReturnURL:      c.LaunchPresentation.ReturnURL,
			Locale:         c.LaunchPresentation.Locale,
		}
	}
	if c.AGSEndpoint != nil {
		claims.AGSEndpoint = &AGSEndpointClaim{Scope: c.AGSEndpoint.Scope, LineItems: c.AGSEndpoint.LineItems, LineItem: c.AGSEndpoint.LineItem}
	}
	if c.NRPSEndpoint != nil {
		claims.NRPSEndpoint = &NRPSEndpointClaim{ContextMembershipsURL: c.NRPSEndpoint.ContextMembershipsURL, ServiceVersions: c.NRPSEndpoint.ServiceVersions}
	}
	if c.DeepLinkingSettings != nil {
		claims.DeepLinking = &DeepLinkingSettingsClaim{
			DeepLinkReturnURL: c.DeepLinkingSettings.DeepLinkReturnURL,
			AcceptTypes:       c.DeepLinkingSettings.AcceptTypes,
			AcceptMediaTypes:  c.DeepLinkingSettings.AcceptMediaTypes,
			AcceptMultiple:    c.DeepLinkingSettings.AcceptMultiple,
			AutoCreate:        c.DeepLinkingSettings.AutoCreate,
			Title:             c.DeepLinkingSettings.Title,
			Text:              c.DeepLinkingSettings.Text,
			Data:              c.DeepLinkingSettings.Data,
		}
	}
	return claims
}
