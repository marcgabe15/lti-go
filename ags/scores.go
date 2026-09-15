package ags

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// SubmitScore submits a score for one user against the line item
// identified by lineItemID (its own URL). If score.Timestamp is zero, it
// is set to the current time.
func (c *Client) SubmitScore(ctx context.Context, lineItemID string, score *Score) error {
	if score.Timestamp.IsZero() {
		score.Timestamp = time.Now()
	}
	body, err := json.Marshal(score)
	if err != nil {
		return fmt.Errorf("ags: marshal score: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSuffix(lineItemID, "/")+"/scores", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", mediaTypeScore)

	resp, err := c.do(ctx, req, []string{ScopeScore})
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
