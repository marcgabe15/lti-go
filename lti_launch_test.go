package lti_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marcgabe15/lti-go"
	"github.com/marcgabe15/lti-go/ltitest"
	"github.com/marcgabe15/lti-go/memstore"
)

func newTestTool(t *testing.T, fp *ltitest.FakePlatform) *lti.Tool {
	t.Helper()
	store := memstore.New()
	tool, err := lti.New(lti.Config{Issuer: "https://tool.example.com", Store: store})
	require.NoError(t, err)

	ctx := context.Background()
	platform, err := tool.Platforms().Register(ctx, &lti.Platform{
		Issuer:                 fp.Issuer,
		ClientID:               fp.ClientID,
		AuthenticationEndpoint: fp.AuthenticationEndpoint(),
		Active:                 true,
		KeyConfig: lti.KeyConfig{
			Method:  lti.KeyConfigMethodJWKSet,
			JWKSURI: fp.JWKSURI(),
		},
	})
	require.NoError(t, err)

	_, err = tool.Platforms().Deployments(platform.ID).Register(ctx, "deployment-1", "Test Deployment")
	require.NoError(t, err)

	return tool
}

func TestLaunchRoundTrip_ResourceLink(t *testing.T) {
	fp, err := ltitest.NewFakePlatform("https://platform.example.com", "tool-client-id")
	require.NoError(t, err)
	defer fp.Close()

	tool := newTestTool(t, fp)

	result, err := fp.Launch(tool, "https://tool.example.com/lti/launch", ltitest.IDTokenClaims{
		Subject:       "user-42",
		DeploymentID:  "deployment-1",
		MessageType:   "LtiResourceLinkRequest",
		TargetLinkURI: "https://tool.example.com/lti/launch",
		Roles:         []string{"http://purl.imsglobal.org/vocab/lis/v2/membership#Learner"},
	})
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, result.LaunchRecorder.Code, "launch response body: %s", result.LaunchRecorder.Body.String())
	require.NotNil(t, result.Claims)
	assert.Equal(t, lti.MessageTypeResourceLinkRequest, result.Claims.MessageType)
	assert.Equal(t, "user-42", result.Claims.Subject)
	assert.Equal(t, "deployment-1", result.Claims.DeploymentID)
	assert.NotEmpty(t, result.Claims.LTIK)
	assert.Equal(t, fp.Issuer, result.Claims.Platform().Issuer)

	// A verified launch's ltik should resolve back to the same claims via
	// VerifyLTIK, proving the session-resumption path works end to end.
	record, err := tool.VerifyLTIK(context.Background(), result.Claims.LTIK)
	require.NoError(t, err)
	assert.Equal(t, "user-42", record.Claims.Subject)
}

func TestLaunchRoundTrip_DeepLinking(t *testing.T) {
	fp, err := ltitest.NewFakePlatform("https://platform.example.com", "tool-client-id")
	require.NoError(t, err)
	defer fp.Close()

	tool := newTestTool(t, fp)

	result, err := fp.Launch(tool, "https://tool.example.com/lti/launch", ltitest.IDTokenClaims{
		Subject:       "instructor-1",
		DeploymentID:  "deployment-1",
		MessageType:   "LtiDeepLinkingRequest",
		TargetLinkURI: "https://tool.example.com/lti/launch",
		Extra: map[string]any{
			"https://purl.imsglobal.org/spec/lti-dl/claim/deep_linking_settings": map[string]any{
				"deep_link_return_url": "https://platform.example.com/deep_link_return",
				"accept_types":         []string{"ltiResourceLink"},
				"accept_multiple":      true,
			},
		},
	})
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, result.LaunchRecorder.Code, "launch response body: %s", result.LaunchRecorder.Body.String())
	require.NotNil(t, result.Claims)
	assert.Equal(t, lti.MessageTypeDeepLinkingRequest, result.Claims.MessageType)
	require.NotNil(t, result.Claims.DeepLinking)
	assert.Equal(t, "https://platform.example.com/deep_link_return", result.Claims.DeepLinking.DeepLinkReturnURL)
	assert.True(t, result.Claims.DeepLinking.AcceptMultiple)
}

func TestLaunchRoundTrip_UnknownDeploymentRejected(t *testing.T) {
	fp, err := ltitest.NewFakePlatform("https://platform.example.com", "tool-client-id")
	require.NoError(t, err)
	defer fp.Close()

	tool := newTestTool(t, fp)

	result, err := fp.Launch(tool, "https://tool.example.com/lti/launch", ltitest.IDTokenClaims{
		Subject:       "user-1",
		DeploymentID:  "never-registered",
		TargetLinkURI: "https://tool.example.com/lti/launch",
	})
	require.NoError(t, err)

	assert.Equal(t, http.StatusForbidden, result.LaunchRecorder.Code)
	assert.Nil(t, result.Claims)
}

func TestLaunchRoundTrip_UnknownDeploymentAllowedViaHook(t *testing.T) {
	fp, err := ltitest.NewFakePlatform("https://platform.example.com", "tool-client-id")
	require.NoError(t, err)
	defer fp.Close()

	store := memstore.New()
	tool, err := lti.New(lti.Config{Issuer: "https://tool.example.com", Store: store},
		lti.WithUnknownDeploymentHandler(func(ctx context.Context, platform *lti.Platform, deploymentID string) (bool, error) {
			return true, nil
		}),
	)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = tool.Platforms().Register(ctx, &lti.Platform{
		Issuer:                 fp.Issuer,
		ClientID:               fp.ClientID,
		AuthenticationEndpoint: fp.AuthenticationEndpoint(),
		Active:                 true,
		KeyConfig:              lti.KeyConfig{Method: lti.KeyConfigMethodJWKSet, JWKSURI: fp.JWKSURI()},
	})
	require.NoError(t, err)

	result, err := fp.Launch(tool, "https://tool.example.com/lti/launch", ltitest.IDTokenClaims{
		Subject:       "user-1",
		DeploymentID:  "first-use-deployment",
		TargetLinkURI: "https://tool.example.com/lti/launch",
	})
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, result.LaunchRecorder.Code, "launch response body: %s", result.LaunchRecorder.Body.String())
	require.NotNil(t, result.Claims)
	assert.Equal(t, "first-use-deployment", result.Claims.DeploymentID)
}
