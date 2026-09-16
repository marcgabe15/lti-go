package deeplink

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/marcgabe15/lti-go"
	"github.com/marcgabe15/lti-go/internal/idgen"
	internaljose "github.com/marcgabe15/lti-go/internal/jose"
)

const responseTTL = 5 * 60 // seconds; kept short since it's presented immediately after the launch that requested it

// Response builds a Deep Linking response for one completed
// LtiDeepLinkingRequest launch. Construct one with NewResponseForLaunch,
// add content items with AddItem, then Sign (or WriteAutoSubmitForm) it.
type Response struct {
	claims     *lti.Claims
	keyManager lti.KeyManager
	items      []ContentItem

	// Message is shown to the user on success (the "msg" claim).
	Message string
	// ErrorMessage is shown to the user on failure (the "errormsg"
	// claim). Set this instead of Message to report a failure; a
	// response can carry either, not both.
	ErrorMessage string
	// Log and ErrorLog are recorded by the platform for diagnostics but
	// not shown to the user.
	Log      string
	ErrorLog string
}

// NewResponseForLaunch returns a Response for the deep linking settings
// granted by claims. It returns lti.ErrDeepLinkingNotAvailable if claims
// is not a deep linking launch.
func NewResponseForLaunch(claims *lti.Claims, keyManager lti.KeyManager) (*Response, error) {
	if claims.DeepLinking == nil {
		return nil, lti.ErrDeepLinkingNotAvailable
	}
	return &Response{claims: claims, keyManager: keyManager}, nil
}

// Items returns the content items added so far.
func (r *Response) Items() []ContentItem {
	return slices.Clone(r.items)
}

// AddItem adds a content item to the response, after checking it against
// the launch's declared accept_types and accept_multiple constraints. It
// returns an error without adding the item if either constraint would be
// violated.
func (r *Response) AddItem(item ContentItem) error {
	dl := r.claims.DeepLinking
	if len(dl.AcceptTypes) > 0 && !slices.Contains(dl.AcceptTypes, item.contentItemType()) {
		return fmt.Errorf("deeplink: content item type %q is not accepted by this launch (accepted: %v)", item.contentItemType(), dl.AcceptTypes)
	}
	if !dl.AcceptMultiple && len(r.items) >= 1 {
		return fmt.Errorf("deeplink: this launch does not accept multiple content items")
	}
	r.items = append(r.items, item)
	return nil
}

func marshalContentItem(item ContentItem) (map[string]any, error) {
	b, err := json.Marshal(item)
	if err != nil {
		return nil, fmt.Errorf("deeplink: marshal content item: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("deeplink: marshal content item: %w", err)
	}
	m["type"] = item.contentItemType()
	return m, nil
}

// Sign builds and RS256-signs the Deep Linking response JWT, using the
// signing key this tool generated for the launch's platform.
func (r *Response) Sign(ctx context.Context) (string, error) {
	platform := r.claims.Platform()
	kp, err := r.keyManager.KeyPair(ctx, platform.ID)
	if err != nil {
		return "", fmt.Errorf("deeplink: resolve signing key: %w", err)
	}
	priv, err := internaljose.ParseRSAPrivateKeyPEM(kp.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("deeplink: parse signing key: %w", err)
	}

	items := make([]map[string]any, 0, len(r.items))
	for _, item := range r.items {
		m, err := marshalContentItem(item)
		if err != nil {
			return "", err
		}
		items = append(items, m)
	}

	id, err := idgen.New(16)
	if err != nil {
		return "", fmt.Errorf("deeplink: generate jti: %w", err)
	}
	nonce, err := idgen.New(16)
	if err != nil {
		return "", fmt.Errorf("deeplink: generate nonce: %w", err)
	}

	now := time.Now().Unix()
	payload := map[string]any{
		"iss":             platform.ClientID,
		"aud":             []string{platform.Issuer},
		"exp":             now + responseTTL,
		"iat":             now,
		"nonce":           nonce,
		"jti":             id,
		claimDeploymentID: r.claims.DeploymentID,
		claimMessageType:  messageTypeDeepLinkingResponse,
		claimVersion:      ltiVersion1p3,
		claimContentItems: items,
	}
	if r.claims.DeepLinking.Data != "" {
		payload[claimData] = r.claims.DeepLinking.Data
	}
	if r.Message != "" {
		payload[claimMsg] = r.Message
	}
	if r.ErrorMessage != "" {
		payload[claimErrorMsg] = r.ErrorMessage
	}
	if r.Log != "" {
		payload[claimLog] = r.Log
	}
	if r.ErrorLog != "" {
		payload[claimErrorLog] = r.ErrorLog
	}

	token, err := internaljose.SignClaims(priv, kp.KeyID, payload)
	if err != nil {
		return "", fmt.Errorf("deeplink: sign response: %w", err)
	}
	return token, nil
}
