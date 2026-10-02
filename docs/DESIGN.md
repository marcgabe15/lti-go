# lti-go design

## Context

This SDK implements the tool-provider side of
[LTI 1.3](https://www.imsglobal.org/spec/lti/v1p3), following Go SDK
conventions. It was designed by studying two references: `ltijs` (a
mature JS/TS implementation of the full spec) and `clerk-sdk-go` (an
example of idiomatic Go SDK structure), and reconciling them where the
two domains genuinely differ.

## The core architectural tension

`clerk-sdk-go` wraps one outbound REST API (Clerk's). An LTI tool SDK is
fundamentally different: it's primarily an **inbound HTTP verifier**
(validating launches pushed by a platform/LMS), which then unlocks
**per-launch outbound clients** (AGS, NRPS, in later phases) scoped to
whichever platform initiated that specific launch. There's no single "the
API" to wrap, and no single account/API key the way Clerk's SDK assumes --
a tool talks to many platforms, each with its own key and endpoints.

This ruled out carrying over Clerk's dominant pattern (many resource
subpackages, each a thin CRUD wrapper around one fixed authenticated REST
API, with a global-singleton `SetKey`/`GetBackend` convenience layer and
generated `api.go` package-level functions). That pattern assumes one API
and one account used repeatedly; it doesn't fit a per-launch,
per-platform auth model. What was carried over: package hygiene (root
holds domain types + interfaces, subpackages hold capabilities),
context-first methods, pointer/option-based config, a `Clock` interface
for deterministic tests, `RoundTripper`-based test mocking, and
CHANGELOG/SemVer discipline.

Resolution:

- A single composition root, `lti.Tool`, built via
  `lti.New(cfg lti.Config, opts ...Option)` -- `Config` carries required
  dependencies (`Store`, the tool's own issuer identity); functional
  options cover optional knobs (`WithClock`, `WithHTTPClient`,
  `WithNonceTTL`, `WithStateTTL`, `WithIATMaxAge`, ...).
- Inbound handling is exposed two ways: ready-to-mount `http.Handler`s
  (`LoginHandler()`, `LaunchHandler(next http.Handler)`, `JWKSHandler()`)
  for the common case, and a bare primitive
  (`VerifyLaunch(r) (*Claims, error)`) plus `ContextWithClaims`/
  `ClaimsFromContext` for anyone wiring their own middleware.
- Outbound clients are never constructed with a bare token by the
  caller -- they come off a completed launch's `Claims`
  (`ags.NewClientForLaunch(claims, tokens)` /
  `nrps.NewClientForLaunch(claims, tokens)`), because only a verified
  launch carries the platform and endpoint claims needed to build them,
  and each service is granted (or not) per-launch. This was originally
  sketched as `claims.AGS(ctx)`/`claims.NRPS(ctx)` sugar methods on
  `Claims` itself, but that would require the root `lti` package to
  import `ags`/`nrps`/`token` for their concrete return types -- and
  those packages must import `lti` for `Platform`, `TokenCacheStore`,
  and `KeyManager`, which creates the same import-cycle shape the
  `keymanager` subpackage hit (see below). `Tool.Store()` and
  `Tool.KeyManager()` accessors let a caller build one shared
  `token.Source` (`token.NewCachingSource(tool.Store(), tool.KeyManager())`)
  and reuse it across every launch.

## Package layout (v0.1)

```
lti-go/
  lti.go, doc.go                Package doc, Version const
  config.go, tool.go            Composition root: Config, Option, New(), Tool + handler methods
  claims.go                     Claims type (parsed+validated launch), raw wire-format structs
  claimuris.go                  LTI message-type/version constants
  context.go                    ContextWithClaims / ClaimsFromContext
  login.go                      OIDC login-initiation handler
  oidc.go                       Signed state token build/verify, RS256 enforcement, expiry checks
  launch.go                     VerifyLaunch: platform resolution, id_token verification, claim validation
  ltik.go                       Session-resumption token issuance/verification
  keyset.go                     JWKSHandler -- serves this tool's JWKS
  keymanager.go                 KeyManager interface + default RSAKeyManager (Store-backed)
  platform.go                   Platform, Deployment, KeyConfig domain types
  store.go                      Store interface (the persistence seam)
  errors.go                     Error/ErrorCode hierarchy, sentinels
  clock.go                      Clock interface + real implementation

  internal/idgen/                Random ID generation (nonces, state ids, key ids, launch ids)
  internal/jose/                 Small go-jose wrappers: PEM parsing, sign/verify/parse helpers
  internal/linkheader/           RFC 8288 Link-header rel="next" parsing, shared by ags and nrps

  jwkset/                        Fetch + cache a platform's published JWKS, kid-miss forces one refetch
  token/                         Access-token acquisition: client_credentials + JWT client-assertion, cached
  ags/                           Assignment & Grade Services client (line items, scores, results)
  nrps/                          Names & Role Provisioning Service client (paginated roster)
  memstore/                      In-memory reference Store implementation
  storetest/                     Store conformance test suite -- any Store implementation should pass it
  ltitest/                       Fake-platform test harness (incl. a fake token endpoint), RoundTripper mock, adjustable Clock
  examples/basic-tool/           Minimal runnable tool
```

`keymanager.KeyManager` was originally planned as a separate subpackage,
but its default implementation needs `Store`/`KeyPair` types from the
root package while the root package needs to reference `KeyManager` in
`Config`/`Tool` -- a subpackage split would have created an import cycle
(`lti` &rarr; `keymanager` &rarr; `lti`). Since the default implementation is
pure local crypto with no external calls, it was folded into the root
package instead; `jwkset` stayed a separate subpackage because it makes
outbound HTTP calls and has no dependency back on root types.

## Key interfaces

**Store** (mirrors ltijs's `DatabaseManager`, the one pluggable
persistence seam):

```go
type Store interface {
    PlatformStore    // Create/Get/Find(issuer,clientID)/List/Update/Delete
    DeploymentStore  // Create/Find(platformID,deploymentID)/List/Delete
    NonceStore       // SaveNonce, ConsumeNonce (atomic delete-and-check)
    LaunchStore      // Save/Get/Delete launch records (what ltik points at)
    TokenCacheStore  // Get/SaveCachedToken keyed by {PlatformID, Scopes}
    KeyStore         // Save/Get/DeleteKeyPair per platform
}
```

**Claims** (the launch-context type):

```go
type Claims struct {
    MessageType, Issuer, Subject string
    DeploymentID, TargetLinkURI  string
    Roles []string
    Context *ContextClaim
    ResourceLink *ResourceLinkClaim
    AGSEndpoint *AGSEndpointClaim   // nil if AGS not granted for this launch
    NRPSEndpoint *NRPSEndpointClaim // nil if NRPS not granted
    DeepLinking *DeepLinkingSettingsClaim
    Custom map[string]string
    LTIK string
    // unexported: resolved *Platform
}
func (c *Claims) Platform() *Platform
```

`AGSEndpoint`/`NRPSEndpoint` are populated straight off the id_token by
`VerifyLaunch` regardless of whether those services end up being used;
`DeepLinking` likewise. See "Package layout" above for why the client
constructors live in `ags`/`nrps` as `NewClientForLaunch(claims, tokens)`
rather than as methods on `Claims`.

**Errors**: one wrapped struct (`Error{Code ErrorCode, Msg string, Err
error}`) with `Unwrap`/`Is` plus sentinel vars (`ErrUnregisteredPlatform`,
`ErrInactivePlatform`, `ErrNonceReused`, `ErrInvalidLTIK`,
`ErrUnknownDeployment`, ...) covering the error conditions other LTI 1.3
libraries model as distinct exception subclasses -- supports both
`errors.Is` (sentinel/code match) and `errors.As` (structured access to
the wrapped cause), which is more idiomatic Go than a large type
hierarchy.

## Design decisions

1. **JOSE library: `github.com/go-jose/go-jose/v3`.** Its
   `JSONWebKey`/`JSONWebKeySet` types cover both directions LTI needs
   (verify platform JWTs via fetched JWKS, serve the tool's own JWKS)
   without a second dependency for JWK conversion.
2. **Deferred the "Platform Storage" postMessage/localStorage
   third-party-cookie dance** that some tool libraries add. The signed
   state JWT (self-verifying, platform-specific key) plus atomic nonce
   consumption already satisfies LTI 1.3's core CSRF/replay requirements
   without any server-side session. The browser-storage dance exists
   purely to survive third-party-cookie blocking inside a platform's
   launch iframe -- a browser-privacy mitigation, not a spec requirement.
   Cutting it from v1 avoids a large HTML/JS-template surface area; it's
   a candidate for a future add-on package if real deployments hit
   iframe/cookie issues.
3. **Closed the deployment_id tracking gap** some LTI 1.3 tool
   implementations leave open (validating the claim is present, but never
   checking it against a registered list per platform, though the spec
   expects this). `Deployment` is a first-class `Store` entity, checked
   during `VerifyLaunch`, with an optional
   `WithUnknownDeploymentHandler` hook so integrators can choose strict
   allow-listing or trust-on-first-use.
4. **One RSA keypair per registered Platform**, not one tool-wide key --
   state/ltik/(later) client-assertion JWTs are inherently
   platform-scoped, and this lets `RotateKeys` be scoped per
   platform-relationship without affecting others.
5. **`Config` holds no tool-wide `client_id`** -- LTI client_ids are
   per-platform (issued by each platform at registration), so `Config`
   only carries the tool's own issuer/identity plus the required `Store`.

## Phased delivery plan

- **v0.1 (shipped)** -- Core launch verification: `Platform`/
  `Deployment` types, `Store` + `memstore` + `storetest`, `keymanager`,
  OIDC login-init, full `VerifyLaunch` (state, nonce, id_token, all 3
  message-type claim schemas, deployment enforcement), `ltik`,
  `JWKSHandler`, error hierarchy, `ltitest` fake-platform harness,
  `examples/basic-tool`. This is the entire spec-mandated trust boundary
  and is fully testable without a live LMS.
- **v0.3 (shipped, ahead of v0.2)** -- Outbound services: `token`
  (client_credentials + JWT client-assertion, `TokenCacheStore`-backed
  caching), `ags` (line item CRUD, score submission, results,
  `Link`-header pagination), `nrps` (paginated roster with a `MaxPages`
  guard). Constructed via `ags.NewClientForLaunch(claims, tokens)` /
  `nrps.NewClientForLaunch(claims, tokens)` rather than `Claims` sugar
  methods (see "Package layout"). `ltitest.FakePlatform` gained a fake
  token endpoint so the full launch -> token -> AGS/NRPS path is
  covered by an in-process integration test with no live LMS.
- **v0.2 (shipped)** -- Deep Linking (`deeplink`): response builder,
  content-item types (`LTIResourceLink`, `Link`, `HTML`, `Image`,
  `File`), accept-type/accept-multiple enforcement in `AddItem`,
  auto-submit form renderer. Constructed via
  `deeplink.NewResponseForLaunch(claims, keyManager)`, mirroring the
  `ags`/`nrps` `NewClientForLaunch` pattern -- same import-cycle
  reasoning applies (`deeplink` needs `lti.Platform`/`lti.KeyManager`,
  so `Claims` can't return a concrete `*deeplink.Response`).
- **v0.4 (shipped)** -- Dynamic Registration (`dynreg`): a mountable
  `Handler` that fetches a platform's OpenID configuration, POSTs a
  registration request extended with the LTI Tool Configuration claim,
  and persists the resulting `Platform` (inactive by default) and
  `deployment_id` via `Store` directly (no `Tool`/`KeyManager`
  dependency needed -- key generation stays lazy, on first login/launch
  toward the new platform). `OnRegistered` lets callers replace the
  default "close this popup" completion page.
- **v1.0** -- Stabilization: API freeze review, lint-clean, race-tested,
  CHANGELOG/UPGRADING conventions locked, godoc pass.

All planned phases through v0.4 have shipped; `PlatformManager`
ergonomics polish and any spec gaps found in real-world use are the main
candidates for a v0.5 before the v1.0 stabilization pass.

- **pgstore (shipped)** -- a PostgreSQL `lti.Store` implementation.
  Deliberately driver-agnostic: it only depends on `*sql.DB`, not a
  specific driver's native API, so it doesn't force a dependency choice
  on consumers (the same reasoning that keeps the root module's own
  dependency footprint to `go-jose` + `testify`). Table and column names
  mirror ltijs's MongoDB schema (`platforms`, `nonces`, `idTokens`,
  `accesstokens` collections) wherever the data model overlaps -- see
  `pgstore/schema.sql`'s header comment for the full mapping, including
  where this SDK's schema necessarily diverges (`deployments` has no
  ltijs equivalent; `id_tokens.claims` is one `JSONB` column instead of
  flattened top-level fields; `platform_keys` is a separate table,
  closer to ltijs's *legacy* key-storage schema than its current one).
  Schema creation is idempotent `CREATE TABLE IF NOT EXISTS` run once at
  `New`, not a goose/migration-tool integration -- there's exactly one
  version of the schema to apply, so a migration framework's versioning
  machinery would be pure overhead here.

## Verification (v0.1)

- `go build ./...`, `go vet ./...`, `gofmt -s -l .` clean.
- `storetest.Run(t, func() lti.Store { return memstore.New() })` --
  including a concurrent `ConsumeNonce` test proving atomicity (20
  goroutines racing to consume one nonce; exactly one succeeds).
- `ltitest.FakePlatform` end-to-end round trip: a fake platform (own
  keypair, fake auth/JWKS endpoints via `httptest.Server`) drives the
  tool's real `LoginHandler` &rarr; captures state/nonce &rarr; signs a
  matching id_token &rarr; POSTs to the real `LaunchHandler` &rarr; asserts the
  downstream handler received correct `*Claims` and a valid `ltik`.
  Covered for resource-link and deep-linking message types.
- Negative-path tests asserting rejection (HTTP 400/403) for: reused
  nonce, azp/aud mismatch with multiple audiences, expired id_token,
  stale `iat`, missing `deployment_id`, unregistered platform, inactive
  platform, unknown deployment (both rejected and accepted-via-hook
  paths).
- `go run ./examples/basic-tool` as a manual smoke test -- confirmed the
  JWKS endpoint serves a valid RSA public key for a registered platform.

## Verification (v0.3: ags, nrps, token)

- `token`: a fake token-endpoint `httptest.Server` verifies the request
  is a well-formed client_credentials + client_assertion grant; a
  caching test proves scope-set canonicalization (order-independent) and
  that distinct scope sets trigger independent fetches/cache entries; an
  error-propagation test for a rejected grant.
- `ags`/`nrps`: unit tests against fake HTTP servers with a stub
  `token.Source`, covering line item CRUD, score submission, result
  retrieval, roster retrieval, `Link`-header pagination (including a
  `MaxPages` test that would loop forever without the guard), and the
  `ErrAGSNotAvailable`/`ErrNRPSNotAvailable` not-granted path.
- `TestAGSAndNRPS_EndToEnd` (root package): the real path end to end --
  a real `ltitest.FakePlatform` login+launch grants AGS/NRPS via id_token
  claims, `token.NewCachingSource(tool.Store(), tool.KeyManager())`
  fetches real access tokens from the fake platform's token endpoint
  (proving the client-assertion flow actually round-trips), and
  `ags`/`nrps` clients built from the resulting `Claims` call fake
  AGS/NRPS servers with those tokens.

## Verification (v0.2: deeplink)

- Unit tests for `AddItem`'s accept-type and accept-multiple enforcement.
- `TestDeepLinkingResponse_EndToEnd`: a real `ltitest.FakePlatform`
  DeepLinkingRequest launch, a signed response built from the resulting
  `Claims`, verified by parsing the JWT against the tool's own key for
  that platform (the same key a real platform would resolve via this
  tool's JWKS) and asserting the `content_items` claim round-trips with
  its `type` discriminator intact.

## Verification (v0.4: dynreg)

- `TestHandler_FullRegistrationFlow`: a fake platform serves both an
  OpenID configuration and a registration endpoint; asserts the
  persisted `Platform` has the platform's (not the tool's) JWKS URI,
  starts inactive, and that the returned `deployment_id` was registered.
- Negative-path tests: missing `openid_configuration` query parameter
  (400), a rejecting registration endpoint (502, and no `Platform`
  persisted), and config validation (`NewHandler` rejects a `Config`
  missing `Store`/`InitiateLoginURI`/`RedirectURIs`/`JWKSURI`).
- `TestHandler_OnRegisteredHookOverridesDefaultPage` confirms the default
  completion page is skipped when a custom hook is provided.
