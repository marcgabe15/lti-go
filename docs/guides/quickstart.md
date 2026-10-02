# Quickstart

This walks through standing up a minimal LTI 1.3 tool from an empty Go
module.

## 1. Install

```sh
go get github.com/marcgabe15/lti-go
```

## 2. Pick a Store

Everything `lti-go` persists (platforms, deployments, nonces, launch
records, cached tokens, signing keys) goes through the `lti.Store`
interface. For local development and tests, use the in-memory
`memstore`:

```go
import "github.com/marcgabe15/lti-go/memstore"

store := memstore.New()
```

For production, use `pgstore` if you're on PostgreSQL -- see
[PostgreSQL Store](./postgres-store.md) -- or implement `lti.Store`
against your own database; [Testing Your Tool](./testing.md) covers how
to verify a custom implementation against the same conformance suite
`memstore` and `pgstore` both pass.

## 3. Construct a Tool

```go
import "github.com/marcgabe15/lti-go"

tool, err := lti.New(lti.Config{
	Issuer: "https://tool.example.com", // this tool's own base URL
	Store:  store,
})
if err != nil {
	log.Fatal(err)
}
```

`Issuer` is used as the issuer of the internal tokens this tool signs
(state, `ltik`) and, in dynamic registration, as the basis for this
tool's advertised identity. It is not a platform's issuer -- see
[Platforms and Keys](./platforms-and-keys.md) for that.

## 4. Register a platform

A "platform" is the LMS you're integrating with (Canvas, Moodle, etc).
You need its issuer, the client_id *it* assigned to your tool, and how
to fetch its public keys:

```go
ctx := context.Background()
_, err = tool.Platforms().Register(ctx, &lti.Platform{
	Issuer:                 "https://platform.example.com",
	ClientID:               "client-id-issued-by-platform",
	AuthenticationEndpoint: "https://platform.example.com/auth",
	AccessTokenEndpoint:    "https://platform.example.com/token",
	Active:                 true,
	KeyConfig: lti.KeyConfig{
		Method:  lti.KeyConfigMethodJWKSet,
		JWKSURI: "https://platform.example.com/jwks",
	},
})
```

If you don't have these details yet, see
[Dynamic Registration](./dynamic-registration.md) for a self-service
alternative.

## 5. Mount the routes

```go
mux := http.NewServeMux()
mux.Handle("/lti/login", tool.LoginHandler())
mux.Handle("/lti/launch", tool.LaunchHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	claims, _ := lti.ClaimsFromContext(r.Context())
	fmt.Fprintf(w, "Hello, %s! Roles: %v", claims.Subject, claims.Roles)
})))
mux.Handle("/lti/jwks", tool.JWKSHandler())

http.ListenAndServe(":8080", mux)
```

Register these three URLs (`/lti/login`, `/lti/launch`, `/lti/jwks`) with
the platform when you set up the integration -- they're commonly called
the "OIDC login URL", "target link URI" / "redirect URI", and "JWKS URL"
respectively in an LMS's admin UI.

## 6. Confirm it works

The full runnable version of the above lives at
`examples/basic-tool/main.go`:

```sh
go run ./examples/basic-tool
curl http://localhost:8080/lti/jwks   # should print a JSON Web Key Set
```

To exercise the actual login+launch flow without a live LMS, see
[Testing Your Tool](./testing.md) -- `ltitest.FakePlatform` can drive a
real launch against your `Tool` in a unit test.

## Next steps

- [Handling a Launch](./handling-a-launch.md) for what's in `Claims`
- [Grades and Roster](./grades-and-roster.md) to post scores or read the
  class list
- [Deep Linking](./deep-linking.md) to let instructors pick content from
  your tool
