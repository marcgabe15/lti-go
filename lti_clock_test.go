package lti_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marcgabe15/lti-go"
	"github.com/marcgabe15/lti-go/ltitest"
	"github.com/marcgabe15/lti-go/memstore"
)

func TestLaunch_ExpiredStateRejectedViaClock(t *testing.T) {
	fp, err := ltitest.NewFakePlatform("https://platform.example.com", "client-1")
	require.NoError(t, err)
	defer fp.Close()

	clock := ltitest.NewClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	store := memstore.New()
	tool, err := lti.New(
		lti.Config{Issuer: "https://tool.example.com", Store: store},
		lti.WithClock(clock),
		lti.WithStateTTL(time.Minute),
	)
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

	// Advance the clock past the state's TTL between login and launch --
	// something that's impractical to test deterministically without an
	// injectable Clock.
	clock.Advance(2 * time.Minute)

	result, err := fp.Launch(tool, "https://tool.example.com/lti/launch", ltitest.IDTokenClaims{
		Subject:       "user-1",
		DeploymentID:  "deployment-1",
		TargetLinkURI: "https://tool.example.com/lti/launch",
		IssuedAt:      clock.Now(),
		ExpiresAt:     clock.Now().Add(5 * time.Minute),
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, result.LaunchRecorder.Code, "expired state should be rejected: %s", result.LaunchRecorder.Body.String())
	assert.Nil(t, result.Claims)
}
