# lti-go

A Go SDK for building [LTI 1.3](https://www.imsglobal.org/spec/lti/v1p3)
tool providers -- the side of the protocol that receives launches from a
platform (an LMS like Canvas, Moodle, or Blackboard).

## Status

Core launch verification (OIDC login, id_token/state/nonce validation,
`ltik` session resumption, JWKS, platform/deployment management),
Assignment and Grade Services (`ags`), and Names and Role Provisioning
Service (`nrps`) are implemented. Deep Linking response building and
Dynamic Registration land in later releases -- see
[CHANGELOG.md](./CHANGELOG.md) and [docs/DESIGN.md](./docs/DESIGN.md)
for the roadmap.

## Install

```sh
go get github.com/marcgabe15/lti-go
```

## Quickstart

```go
store := memstore.New() // or your own lti.Store implementation

tool, err := lti.New(lti.Config{
	Issuer: "https://tool.example.com",
	Store:  store,
})
if err != nil {
	log.Fatal(err)
}

// Register the platforms (LMSs) your tool trusts.
tool.Platforms().Register(ctx, &lti.Platform{
	Issuer:                 "https://platform.example.com",
	ClientID:               "client-id-issued-by-platform",
	AuthenticationEndpoint: "https://platform.example.com/auth",
	Active:                 true,
	KeyConfig: lti.KeyConfig{
		Method:  lti.KeyConfigMethodJWKSet,
		JWKSURI: "https://platform.example.com/jwks",
	},
})

mux := http.NewServeMux()
mux.Handle("/lti/login", tool.LoginHandler())
mux.Handle("/lti/launch", tool.LaunchHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	claims, _ := lti.ClaimsFromContext(r.Context())
	fmt.Fprintf(w, "Hello, %s! Roles: %v", claims.Subject, claims.Roles)
})))
mux.Handle("/lti/jwks", tool.JWKSHandler())

http.ListenAndServe(":8080", mux)
```

Run the fuller runnable example:

```sh
go run ./examples/basic-tool
```

## Two ways to consume a launch

`LaunchHandler` is the ready-to-mount option: it verifies the launch,
issues an `ltik` session token, and calls your `next` handler with
`*lti.Claims` in the request context.

If you're wiring your own middleware (integrating with an existing
session/cookie layer, for example), use the primitive underneath directly:

```go
claims, err := tool.VerifyLaunch(r)
```

## Persistence

All state (platforms, deployments, nonces, launch records, cached
tokens, and per-platform signing keys) goes through the `lti.Store`
interface. `memstore.New()` provides an in-memory implementation for
tests and local development; implement `lti.Store` against your own
database for production, and verify it with the conformance suite:

```go
func TestStore(t *testing.T) {
	storetest.Run(t, func() lti.Store { return NewMyStore() })
}
```

## Testing your tool

`ltitest.FakePlatform` simulates a platform end-to-end -- it generates
its own signing key, serves a JWKS endpoint, and can drive your `Tool`
through a full login+launch round trip without any network calls to a
real LMS:

```go
fp, _ := ltitest.NewFakePlatform("https://platform.example.com", "client-id")
defer fp.Close()

result, err := fp.Launch(tool, "https://tool.example.com/lti/launch", ltitest.IDTokenClaims{
	Subject:       "user-1",
	DeploymentID:  "deployment-1",
	TargetLinkURI: "https://tool.example.com/lti/launch",
})
// result.Claims holds what your launch handler received.
```

## Assignment and Grade Services / Names and Role Provisioning Service

AGS and NRPS clients are built from a completed launch's `Claims`, not
constructed directly with a token -- only a verified launch carries the
platform and endpoint claims they need, and each is granted (or not)
per-launch. `ags.NewClientForLaunch`/`nrps.NewClientForLaunch` return
`lti.ErrAGSNotAvailable`/`lti.ErrNRPSNotAvailable` if the platform didn't
grant that service to this launch.

Both share a `token.Source`, obtained once from your `Tool` and reused
across launches (it caches access tokens per platform+scopes via your
`Store`):

```go
tokens := token.NewCachingSource(tool.Store(), tool.KeyManager())

mux.Handle("/lti/launch", tool.LaunchHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	claims, _ := lti.ClaimsFromContext(r.Context())

	if agsClient, err := ags.NewClientForLaunch(claims, tokens); err == nil {
		agsClient.SubmitScore(r.Context(), lineItemURL, &ags.Score{
			UserID:           claims.Subject,
			ScoreGiven:       ptr(9.0),
			ScoreMaximum:     ptr(10.0),
			ActivityProgress: ags.ActivityProgressCompleted,
			GradingProgress:  ags.GradingProgressFullyGraded,
		})
	}

	if nrpsClient, err := nrps.NewClientForLaunch(claims, tokens); err == nil {
		members, _ := nrpsClient.GetMembers(r.Context(), nil)
		_ = members // roster: user_id, roles, name, email, ...
	}
})))
```

## Errors

Errors are a single wrapped type (`*lti.Error`) discriminated by
`lti.ErrorCode`, supporting both idiomatic forms:

```go
if errors.Is(err, lti.ErrUnregisteredPlatform) { ... }

if code, ok := lti.CodeOf(err); ok {
	switch code {
	case lti.CodeInactivePlatform:
		// ...
	}
}
```

## A note on scope vs. other LTI 1.3 libraries

This SDK deliberately closes a gap some LTI 1.3 tool implementations
leave open: `deployment_id` is checked against a registered list per
platform (see `Tool.Platforms().Deployments(...)`), not just validated as
present. It also does not implement the "Platform Storage"
(postMessage/localStorage) third-party-cookie mitigation some tool
libraries add on top of the core spec -- a signed state JWT plus atomic
nonce consumption already satisfies LTI 1.3's CSRF/replay requirements
without a server-side session. See `docs/DESIGN.md` for the full
rationale.

## License

MIT, see [LICENSE](./LICENSE).
