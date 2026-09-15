package lti_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marcgabe15/lti-go"
	"github.com/marcgabe15/lti-go/ags"
	"github.com/marcgabe15/lti-go/ltitest"
	"github.com/marcgabe15/lti-go/memstore"
	"github.com/marcgabe15/lti-go/nrps"
	"github.com/marcgabe15/lti-go/token"
)

// TestAGSAndNRPS_EndToEnd exercises the full path: a real launch (real
// state/nonce/id_token verification) grants AGS and NRPS access, and the
// resulting Claims are used to obtain a real access token from the fake
// platform's token endpoint (via the token package's client-assertion
// flow) and call fake AGS/NRPS servers with it.
func TestAGSAndNRPS_EndToEnd(t *testing.T) {
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
		AccessTokenEndpoint:    fp.TokenEndpoint(),
		Active:                 true,
		KeyConfig:              lti.KeyConfig{Method: lti.KeyConfigMethodJWKSet, JWKSURI: fp.JWKSURI()},
	})
	require.NoError(t, err)
	_, err = tool.Platforms().Deployments(platform.ID).Register(ctx, "deployment-1", "")
	require.NoError(t, err)

	// Fake AGS/NRPS servers standing in for the platform's grading and
	// roster endpoints.
	agsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer fake-access-token-1", r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode([]*ags.LineItem{{ID: "li-1", Label: "Assignment 1", ScoreMaximum: 100}})
	}))
	defer agsSrv.Close()

	nrpsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer fake-access-token-2", r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode(map[string]any{"members": []map[string]any{{"user_id": "user-1", "roles": []string{"Learner"}}}})
	}))
	defer nrpsSrv.Close()

	result, err := fp.Launch(tool, "https://tool.example.com/lti/launch", ltitest.IDTokenClaims{
		Subject:       "instructor-1",
		DeploymentID:  "deployment-1",
		TargetLinkURI: "https://tool.example.com/lti/launch",
		Extra: map[string]any{
			"https://purl.imsglobal.org/spec/lti-ags/claim/endpoint": map[string]any{
				"scope":     []string{ags.ScopeLineItemReadonly},
				"lineitems": agsSrv.URL,
			},
			"https://purl.imsglobal.org/spec/lti-nrps/claim/namesroleservice": map[string]any{
				"context_memberships_url": nrpsSrv.URL,
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, result.LaunchRecorder.Code, "launch response body: %s", result.LaunchRecorder.Body.String())
	require.NotNil(t, result.Claims)
	require.NotNil(t, result.Claims.AGSEndpoint)
	require.NotNil(t, result.Claims.NRPSEndpoint)

	tokens := token.NewCachingSource(tool.Store(), tool.KeyManager())

	agsClient, err := ags.NewClientForLaunch(result.Claims, tokens)
	require.NoError(t, err)
	items, err := agsClient.GetLineItems(ctx, nil)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "Assignment 1", items[0].Label)

	nrpsClient, err := nrps.NewClientForLaunch(result.Claims, tokens)
	require.NoError(t, err)
	members, err := nrpsClient.GetMembers(ctx, nil)
	require.NoError(t, err)
	require.Len(t, members, 1)
	assert.Equal(t, "user-1", members[0].UserID)

	// AGS and NRPS requested different scopes, so each got its own token
	// from the platform (cache keyed by scope set) -- two real requests
	// to the fake platform's token endpoint.
	assert.Equal(t, 2, fp.TokenRequestCount())
}
