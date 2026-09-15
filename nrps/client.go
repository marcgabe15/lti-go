package nrps

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/marcgabe15/lti-go"
	"github.com/marcgabe15/lti-go/internal/linkheader"
	"github.com/marcgabe15/lti-go/token"
)

// Client calls the Names and Role Provisioning Service endpoint granted
// to one completed launch. Construct one with NewClientForLaunch.
type Client struct {
	membershipsURL string
	platform       *lti.Platform
	tokens         token.Source
	httpClient     *http.Client
}

// Option configures optional Client behavior.
type Option func(*Client)

// WithHTTPClient overrides the *http.Client used to call the platform.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// NewClientForLaunch returns a Client for the NRPS endpoint granted to
// claims. It returns lti.ErrNRPSNotAvailable if the launch did not grant
// NRPS access.
func NewClientForLaunch(claims *lti.Claims, tokens token.Source, opts ...Option) (*Client, error) {
	if claims.NRPSEndpoint == nil {
		return nil, lti.ErrNRPSNotAvailable
	}
	c := &Client{
		membershipsURL: claims.NRPSEndpoint.ContextMembershipsURL,
		platform:       claims.Platform(),
		tokens:         tokens,
		httpClient:     http.DefaultClient,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

func (c *Client) do(ctx context.Context, req *http.Request) (*http.Response, error) {
	tok, err := c.tokens.Token(ctx, c.platform, []string{ScopeContextMembershipReadonly})
	if err != nil {
		return nil, fmt.Errorf("nrps: obtain access token: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nrps: request: %w", err)
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("nrps: platform returned %d: %s", resp.StatusCode, body)
	}
	return resp, nil
}

// GetMembersParams filters Client.GetMembers.
type GetMembersParams struct {
	Role           string
	ResourceLinkID string
	Limit          int
	// MaxPages caps how many pages GetMembers follows via the response's
	// Link header (rel="next"). 0 means unbounded.
	MaxPages int
}

// GetMembers returns the course roster, following Link-header
// pagination (bounded by params.MaxPages, if set).
func (c *Client) GetMembers(ctx context.Context, params *GetMembersParams) ([]*Member, error) {
	u, err := url.Parse(c.membershipsURL)
	if err != nil {
		return nil, fmt.Errorf("nrps: invalid memberships url: %w", err)
	}
	if params != nil {
		q := u.Query()
		if params.Role != "" {
			q.Set("role", params.Role)
		}
		if params.ResourceLinkID != "" {
			q.Set("rlid", params.ResourceLinkID)
		}
		if params.Limit > 0 {
			q.Set("limit", strconv.Itoa(params.Limit))
		}
		u.RawQuery = q.Encode()
	}

	var members []*Member
	next := u.String()
	for page := 0; next != ""; page++ {
		if params != nil && params.MaxPages > 0 && page >= params.MaxPages {
			break
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, next, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", mediaTypeMembershipContainer)

		resp, err := c.do(ctx, req)
		if err != nil {
			return nil, err
		}
		var container membershipContainer
		err = json.NewDecoder(resp.Body).Decode(&container)
		linkHeader := resp.Header.Get("Link")
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("nrps: decode response: %w", err)
		}
		members = append(members, container.Members...)
		next = linkheader.Next(linkHeader)
	}
	return members, nil
}
