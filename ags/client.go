package ags

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/marcgabe15/lti-go"
	"github.com/marcgabe15/lti-go/internal/linkheader"
	"github.com/marcgabe15/lti-go/token"
)

// Client calls the Assignment and Grade Services endpoints granted to
// one completed launch. Construct one with NewClientForLaunch.
type Client struct {
	lineItemsURL string
	platform     *lti.Platform
	tokens       token.Source
	httpClient   *http.Client
}

// Option configures optional Client behavior.
type Option func(*Client)

// WithHTTPClient overrides the *http.Client used to call the platform.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// NewClientForLaunch returns a Client for the AGS endpoints granted to
// claims. It returns lti.ErrAGSNotAvailable if the launch did not grant
// AGS access.
func NewClientForLaunch(claims *lti.Claims, tokens token.Source, opts ...Option) (*Client, error) {
	if claims.AGSEndpoint == nil {
		return nil, lti.ErrAGSNotAvailable
	}
	c := &Client{
		lineItemsURL: claims.AGSEndpoint.LineItems,
		platform:     claims.Platform(),
		tokens:       tokens,
		httpClient:   http.DefaultClient,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

func (c *Client) do(ctx context.Context, req *http.Request, scopes []string) (*http.Response, error) {
	tok, err := c.tokens.Token(ctx, c.platform, scopes)
	if err != nil {
		return nil, fmt.Errorf("ags: obtain access token: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ags: request: %w", err)
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ags: platform returned %d: %s", resp.StatusCode, body)
	}
	return resp, nil
}

// GetLineItemsParams filters Client.GetLineItems.
type GetLineItemsParams struct {
	ResourceLinkID string
	ResourceID     string
	Tag            string
	Limit          int
}

// GetLineItems returns the line items matching params (or all line items
// this launch's platform grants access to, if params is nil), following
// Link-header pagination.
func (c *Client) GetLineItems(ctx context.Context, params *GetLineItemsParams) ([]*LineItem, error) {
	u, err := url.Parse(c.lineItemsURL)
	if err != nil {
		return nil, fmt.Errorf("ags: invalid line items url: %w", err)
	}
	if params != nil {
		q := u.Query()
		if params.ResourceLinkID != "" {
			q.Set("resource_link_id", params.ResourceLinkID)
		}
		if params.ResourceID != "" {
			q.Set("resource_id", params.ResourceID)
		}
		if params.Tag != "" {
			q.Set("tag", params.Tag)
		}
		if params.Limit > 0 {
			q.Set("limit", strconv.Itoa(params.Limit))
		}
		u.RawQuery = q.Encode()
	}

	var items []*LineItem
	next := u.String()
	for next != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, next, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", mediaTypeLineItemContainer)

		resp, err := c.do(ctx, req, []string{ScopeLineItemReadonly, ScopeLineItem})
		if err != nil {
			return nil, err
		}
		var page []*LineItem
		err = json.NewDecoder(resp.Body).Decode(&page)
		linkHeader := resp.Header.Get("Link")
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("ags: decode line items: %w", err)
		}
		items = append(items, page...)
		next = linkheader.Next(linkHeader)
	}
	return items, nil
}

// CreateLineItem creates a new line item.
func (c *Client) CreateLineItem(ctx context.Context, li *LineItem) (*LineItem, error) {
	body, err := json.Marshal(li)
	if err != nil {
		return nil, fmt.Errorf("ags: marshal line item: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.lineItemsURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mediaTypeLineItem)
	req.Header.Set("Accept", mediaTypeLineItem)

	resp, err := c.do(ctx, req, []string{ScopeLineItem})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out LineItem
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("ags: decode line item: %w", err)
	}
	return &out, nil
}

// GetLineItem fetches a single line item by its id (the line item's own
// URL, as returned by GetLineItems or CreateLineItem).
func (c *Client) GetLineItem(ctx context.Context, id string) (*LineItem, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, id, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", mediaTypeLineItem)

	resp, err := c.do(ctx, req, []string{ScopeLineItemReadonly, ScopeLineItem})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out LineItem
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("ags: decode line item: %w", err)
	}
	return &out, nil
}

// UpdateLineItem replaces an existing line item. li.ID (its URL) must be
// set.
func (c *Client) UpdateLineItem(ctx context.Context, li *LineItem) (*LineItem, error) {
	if li.ID == "" {
		return nil, errors.New("ags: line item ID (its URL) is required to update it")
	}
	body, err := json.Marshal(li)
	if err != nil {
		return nil, fmt.Errorf("ags: marshal line item: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, li.ID, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mediaTypeLineItem)
	req.Header.Set("Accept", mediaTypeLineItem)

	resp, err := c.do(ctx, req, []string{ScopeLineItem})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out LineItem
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("ags: decode line item: %w", err)
	}
	return &out, nil
}

// DeleteLineItem deletes a line item by its id (its URL).
func (c *Client) DeleteLineItem(ctx context.Context, id string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, id, nil)
	if err != nil {
		return err
	}
	resp, err := c.do(ctx, req, []string{ScopeLineItem})
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
