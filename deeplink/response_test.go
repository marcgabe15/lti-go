package deeplink_test

import (
	"context"
	"strings"
	"testing"

	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marcgabe15/lti-go"
	"github.com/marcgabe15/lti-go/deeplink"
	internaljose "github.com/marcgabe15/lti-go/internal/jose"
	"github.com/marcgabe15/lti-go/ltitest"
	"github.com/marcgabe15/lti-go/memstore"
)

func TestNewResponseForLaunch_NotAvailable(t *testing.T) {
	claims := &lti.Claims{} // no DeepLinking set
	store := memstore.New()
	_, err := deeplink.NewResponseForLaunch(claims, lti.NewRSAKeyManager(store))
	assert.ErrorIs(t, err, lti.ErrDeepLinkingNotAvailable)
}

func TestAddItem_RejectsUnacceptedType(t *testing.T) {
	claims := &lti.Claims{DeepLinking: &lti.DeepLinkingSettingsClaim{AcceptTypes: []string{"link"}}}
	store := memstore.New()
	resp, err := deeplink.NewResponseForLaunch(claims, lti.NewRSAKeyManager(store))
	require.NoError(t, err)

	err = resp.AddItem(deeplink.LTIResourceLink{Title: "not accepted"})
	assert.Error(t, err)

	err = resp.AddItem(deeplink.Link{Title: "accepted", URL: "https://example.com"})
	assert.NoError(t, err)
}

func TestAddItem_RejectsMultipleWhenNotAllowed(t *testing.T) {
	claims := &lti.Claims{DeepLinking: &lti.DeepLinkingSettingsClaim{AcceptMultiple: false}}
	store := memstore.New()
	resp, err := deeplink.NewResponseForLaunch(claims, lti.NewRSAKeyManager(store))
	require.NoError(t, err)

	require.NoError(t, resp.AddItem(deeplink.LTIResourceLink{Title: "first"}))
	err = resp.AddItem(deeplink.LTIResourceLink{Title: "second"})
	assert.Error(t, err)
	assert.Len(t, resp.Items(), 1)
}

// TestDeepLinkingResponse_EndToEnd drives a real DeepLinkingRequest
// launch, builds a response with a real content item, signs it, and
// verifies the resulting JWT's signature against the tool's own key
// (proving Sign used the correct per-platform key and claim shape) and
// that content_items round-trips with its type discriminator.
func TestDeepLinkingResponse_EndToEnd(t *testing.T) {
	fp, err := ltitest.NewFakePlatform("https://platform.example.com", "tool-client-id")
	require.NoError(t, err)
	defer fp.Close()

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

	result, err := fp.Launch(tool, "https://tool.example.com/lti/launch", ltitest.IDTokenClaims{
		Subject:       "instructor-1",
		DeploymentID:  "deployment-1",
		MessageType:   "LtiDeepLinkingRequest",
		TargetLinkURI: "https://tool.example.com/lti/launch",
		Extra: map[string]any{
			"https://purl.imsglobal.org/spec/lti-dl/claim/deep_linking_settings": map[string]any{
				"deep_link_return_url": "https://platform.example.com/deep_link_return",
				"accept_types":         []string{"ltiResourceLink"},
				"accept_multiple":      false,
				"data":                 "opaque-platform-state",
			},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, result.Claims)

	resp, err := deeplink.NewResponseForLaunch(result.Claims, tool.KeyManager())
	require.NoError(t, err)
	require.NoError(t, resp.AddItem(deeplink.LTIResourceLink{
		Title: "New Assignment",
		URL:   "https://tool.example.com/assignments/1",
		LineItem: &deeplink.LineItem{
			ScoreMaximum: 100,
			Label:        "New Assignment",
		},
	}))

	rawToken, err := resp.Sign(ctx)
	require.NoError(t, err)

	// Verify the signature against the tool's own key for this
	// platform -- the same key the platform would resolve via the
	// tool's JWKS endpoint in a real deployment.
	kp, err := tool.KeyManager().KeyPair(ctx, platform.ID)
	require.NoError(t, err)
	pub, err := internaljose.ParseRSAPublicKeyPEM(kp.PublicKey)
	require.NoError(t, err)

	parsed, err := jwt.ParseSigned(rawToken)
	require.NoError(t, err)
	var claims map[string]any
	require.NoError(t, parsed.Claims(pub, &claims))

	assert.Equal(t, "LtiDeepLinkingResponse", claims["https://purl.imsglobal.org/spec/lti/claim/message_type"])
	assert.Equal(t, "deployment-1", claims["https://purl.imsglobal.org/spec/lti/claim/deployment_id"])
	assert.Equal(t, "opaque-platform-state", claims["https://purl.imsglobal.org/spec/lti-dl/claim/data"])
	assert.Equal(t, fp.ClientID, claims["iss"])

	items, ok := claims["https://purl.imsglobal.org/spec/lti-dl/claim/content_items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1)
	item := items[0].(map[string]any)
	assert.Equal(t, "ltiResourceLink", item["type"])
	assert.Equal(t, "New Assignment", item["title"])

	// And RenderAutoSubmitForm/WriteAutoSubmitForm produce a form
	// targeting the launch's declared return URL.
	var buf strings.Builder
	require.NoError(t, deeplink.RenderAutoSubmitForm(&buf, rawToken, "https://platform.example.com/deep_link_return"))
	assert.Contains(t, buf.String(), `action="https://platform.example.com/deep_link_return"`)
	assert.Contains(t, buf.String(), rawToken)
}
