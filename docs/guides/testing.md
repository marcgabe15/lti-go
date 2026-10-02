# Testing Your Tool

## `memstore` for unit tests

Use `memstore.New()` wherever a test needs an `lti.Store` -- it's a
plain in-memory implementation, safe to construct fresh per test:

```go
store := memstore.New()
tool, _ := lti.New(lti.Config{Issuer: "https://tool.example.com", Store: store})
```

## Verifying your own `Store` implementation

If you're implementing `lti.Store` against a real database (or using
`pgstore`'s own test as a template -- see
[PostgreSQL Store](./postgres-store.md)), run it through the same
conformance suite `memstore` passes:

```go
func TestMyStore(t *testing.T) {
	storetest.Run(t, func() lti.Store { return NewMyStore(testDB) })
}
```

This checks platform/deployment CRUD, atomic nonce consumption (under
concurrency), launch record expiry, token caching, and key storage --
the full `lti.Store` contract, not just the happy path. Tests for
deployments, cached tokens, and keys create a real platform first and
reference its actual ID, so a `Store` that enforces referential
integrity (a foreign key on `platform_id`, for example) is exercised
correctly rather than failing on a synthetic ID that was never created.

## Driving a real launch with `ltitest.FakePlatform`

`ltitest.FakePlatform` simulates a platform end to end: it generates its
own RSA key pair and serves fake authorization, JWKS, and OAuth2 token
endpoints via `httptest.Server`, so you can drive your `Tool` through a
real login+launch round trip with no network calls to an actual LMS.

```go
fp, err := ltitest.NewFakePlatform("https://platform.example.com", "test-client-id")
require.NoError(t, err)
defer fp.Close()

store := memstore.New()
tool, _ := lti.New(lti.Config{Issuer: "https://tool.example.com", Store: store})

platform, _ := tool.Platforms().Register(ctx, &lti.Platform{
	Issuer:                 fp.Issuer,
	ClientID:               fp.ClientID,
	AuthenticationEndpoint: fp.AuthenticationEndpoint(),
	AccessTokenEndpoint:    fp.TokenEndpoint(),
	Active:                 true,
	KeyConfig:              lti.KeyConfig{Method: lti.KeyConfigMethodJWKSet, JWKSURI: fp.JWKSURI()},
})
tool.Platforms().Deployments(platform.ID).Register(ctx, "deployment-1", "")

result, err := fp.Launch(tool, "https://tool.example.com/lti/launch", ltitest.IDTokenClaims{
	Subject:       "user-1",
	DeploymentID:  "deployment-1",
	TargetLinkURI: "https://tool.example.com/lti/launch",
	Roles:         []string{"http://purl.imsglobal.org/vocab/lis/v2/membership#Learner"},
})
require.NoError(t, err)
require.Equal(t, http.StatusOK, result.LaunchRecorder.Code)
// result.Claims is what your launch handler received.
```

`fp.Launch` does the full dance for you: POSTs a login-initiation
request to your real `LoginHandler`, captures the `state`/`nonce` from
the redirect, signs a matching id_token, and POSTs it to your real
`LaunchHandler`. If you need finer control (testing a specific failure
mode), drive the steps yourself -- see `lti_errors_test.go` in the module
root for examples of tampering with individual fields (expired tokens,
reused nonces, mismatched audiences, etc).

### Testing AGS/NRPS/Deep Linking code

Grant a service by including its claim in `Extra`:

```go
result, _ := fp.Launch(tool, targetURL, ltitest.IDTokenClaims{
	Subject: "user-1", DeploymentID: "deployment-1", TargetLinkURI: targetURL,
	Extra: map[string]any{
		"https://purl.imsglobal.org/spec/lti-ags/claim/endpoint": map[string]any{
			"scope":     []string{ags.ScopeLineItemReadonly},
			"lineitems": agsTestServer.URL,
		},
	},
})
```

Point the endpoint URL at your own `httptest.Server` standing in for the
platform's AGS/NRPS API, and use a stub `token.Source` (a one-method
interface) instead of a real `token.CachingSource` if you don't need to
exercise the token-acquisition path itself:

```go
type stubTokenSource struct{ token string }

func (s stubTokenSource) Token(context.Context, *lti.Platform, []string) (string, error) {
	return s.token, nil
}
```

For a full launch -> token -> AGS/NRPS integration test, see
`lti_ags_nrps_integration_test.go` in the module root, which drives the
whole path against `fp.TokenEndpoint()`.

## `RoundTripper` for mocking arbitrary HTTP

`ltitest.RoundTripper` is a minimal `http.RoundTripper` for stubbing an
outbound call (a platform JWKS fetch, for instance) without spinning up
an `httptest.Server`:

```go
hc := &http.Client{Transport: &ltitest.RoundTripper{
	Handler: func(r *http.Request) (*http.Response, error) {
		return ltitest.JSONResponse(200, `{"keys":[...]}`), nil
	},
}}
```

## `ltitest.Clock` for expiry logic

Pass a manually-advanceable clock to test TTL-sensitive behavior (state
expiry, `ltik` expiry, cached-token expiry) deterministically instead of
sleeping in tests:

```go
clock := ltitest.NewClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
tool, _ := lti.New(cfg, lti.WithClock(clock), lti.WithStateTTL(time.Minute))

clock.Advance(2 * time.Minute) // now past the state TTL
// a launch attempted now should fail with lti.ErrStateExpired
```
