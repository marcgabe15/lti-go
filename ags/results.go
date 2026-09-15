package ags

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/marcgabe15/lti-go/internal/linkheader"
)

// GetResultsParams filters Client.GetResults.
type GetResultsParams struct {
	UserID string
	Limit  int
}

// GetResults returns the recorded results for the line item identified
// by lineItemID (its own URL), following Link-header pagination.
func (c *Client) GetResults(ctx context.Context, lineItemID string, params *GetResultsParams) ([]*Result, error) {
	u, err := url.Parse(strings.TrimSuffix(lineItemID, "/") + "/results")
	if err != nil {
		return nil, fmt.Errorf("ags: invalid line item url: %w", err)
	}
	if params != nil {
		q := u.Query()
		if params.UserID != "" {
			q.Set("user_id", params.UserID)
		}
		if params.Limit > 0 {
			q.Set("limit", strconv.Itoa(params.Limit))
		}
		u.RawQuery = q.Encode()
	}

	var results []*Result
	next := u.String()
	for next != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, next, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", mediaTypeResultContainer)

		resp, err := c.do(ctx, req, []string{ScopeResultReadonly})
		if err != nil {
			return nil, err
		}
		var page []*Result
		err = json.NewDecoder(resp.Body).Decode(&page)
		linkHeader := resp.Header.Get("Link")
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("ags: decode results: %w", err)
		}
		results = append(results, page...)
		next = linkheader.Next(linkHeader)
	}
	return results, nil
}
