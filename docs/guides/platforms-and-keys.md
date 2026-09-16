# Platforms and Keys

## Registering a platform

Every LMS your tool talks to is a `Platform`. Register one via
`Tool.Platforms()`:

```go
platform, err := tool.Platforms().Register(ctx, &lti.Platform{
	Issuer:                 "https://platform.example.com",
	ClientID:               "client-id-issued-by-platform",
	Name:                   "Example University",
	AuthenticationEndpoint: "https://platform.example.com/auth",
	AccessTokenEndpoint:    "https://platform.example.com/token",
	Active:                 true,
	KeyConfig: lti.KeyConfig{
		Method:  lti.KeyConfigMethodJWKSet,
		JWKSURI: "https://platform.example.com/jwks",
	},
})
```

`Register` also generates this tool's dedicated RSA signing key pair for
that platform (see "Signing keys" below) -- you don't need a separate
step for that.

### `Issuer` and `ClientID`

These identify the platform/tool pair and come from whatever the
platform's admin UI shows you when you register your tool there (or from
[Dynamic Registration](./dynamic-registration.md), which fills them in
automatically). `Issuer` is the platform's own identity (often its base
URL); `ClientID` is the identifier *that platform* assigned to *your
tool* -- it is not global, and the same tool will have a different
`ClientID` at every platform it's registered with.

### `KeyConfig`

Controls how this tool resolves the platform's public key(s) to verify
the id_tokens it sends:

| Method | When to use |
| --- | --- |
| `KeyConfigMethodJWKSet` + `JWKSURI` | The common case: the platform publishes a JWKS endpoint. Keys are fetched, cached, and looked up by `kid`; a `kid` miss triggers exactly one refetch, so platform-side key rotation is handled automatically. |
| `KeyConfigMethodJWK` + `JWK` | The platform gave you a single JWK document up front instead of a URL. |
| `KeyConfigMethodRSAKey` + `RSAKey` | The platform gave you a raw PEM public key instead. |

### Activation

`Active: false` rejects launches from that platform with
`lti.ErrInactivePlatform` (checked at both login and launch). Platforms
registered via Dynamic Registration default to inactive, on the
assumption a human should confirm the relationship first:

```go
_, err := tool.Platforms().Activate(ctx, platform.ID)
_, err := tool.Platforms().Deactivate(ctx, platform.ID)
```

## Deployments

The LTI 1.3 spec expects a tool to recognize specific `deployment_id`
values per platform, not just accept any value the platform sends.
`lti-go` enforces this: `VerifyLaunch` rejects an unrecognized
`deployment_id` with `lti.ErrUnknownDeployment` unless you register it
first or configure a hook.

```go
_, err := tool.Platforms().Deployments(platform.ID).Register(ctx, "deployment-id-1", "Main Deployment")
```

If you'd rather trust deployments on first use instead of pre-registering
them (common when a platform can spin up new deployments without
notifying you, e.g. a multi-tenant LMS), configure a hook at `New`:

```go
tool, err := lti.New(cfg,
	lti.WithUnknownDeploymentHandler(func(ctx context.Context, platform *lti.Platform, deploymentID string) (bool, error) {
		return true, nil // trust-on-first-use; return false to reject instead
	}),
)
```

Returning `true` registers the deployment so it's recognized without the
hook running again.

## Signing keys

Each `Platform` gets its own RSA-2048 key pair, generated on first use
(lazily, via the default `RSAKeyManager`) or eagerly during
`Platforms().Register`. This key signs everything this tool sends toward
that platform: the OIDC `state` parameter, `ltik` session tokens, and (if
you use them) Deep Linking responses and AGS/NRPS client assertions.

Keys are per-platform, not one tool-wide key, so rotating one platform's
key doesn't affect any other platform relationship:

```go
_, err := tool.Platforms().RotateKeys(ctx, platform.ID)
```

Rotation replaces the key immediately -- the old key stops being served
from `Tool.JWKSHandler()` right away, with no overlap window. If a
platform has already cached your old public key, expect a brief window
of failed verifications on its side until it refetches your JWKS.

### Bringing your own key management

The default `RSAKeyManager` stores keys via your `Store`'s `KeyStore`
methods. To use a KMS/HSM instead, implement the `lti.KeyManager`
interface yourself and pass it in `Config.KeyManager`:

```go
type KeyManager interface {
	KeyPair(ctx context.Context, platformID string) (*KeyPair, error)
	Rotate(ctx context.Context, platformID string) (*KeyPair, error)
}
```
