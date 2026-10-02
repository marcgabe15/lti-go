# Changelog

All notable changes to this project are documented in this file.
This project adheres to [Semantic Versioning](https://semver.org/); until
v1.0.0, minor versions may include breaking changes.

## Unreleased

### Added

- Initial v0.1 release: OIDC third-party login initiation, LTI 1.3 launch
  verification (state, nonce, id_token, resource link / deep linking /
  submission review message types), per-platform RSA signing keys,
  `ltik` session-resumption tokens, this tool's JWKS endpoint
  (`Tool.JWKSHandler`), platform and deployment management
  (`Tool.Platforms()`), a pluggable `Store` interface with an in-memory
  reference implementation (`memstore`) and conformance suite
  (`storetest`), and a fake-platform test harness (`ltitest`) for
  end-to-end testing without a live LMS.
- Assignment and Grade Services (`ags`): line item CRUD, score
  submission, and result retrieval, with `Link`-header pagination.
- Names and Role Provisioning Service (`nrps`): paginated roster
  retrieval (`GetMembers`), with a `MaxPages` guard against
  misbehaving/looping `Link` headers.
- `token`: OAuth2 access-token acquisition from a platform via
  client_credentials + private_key_jwt client assertion, cached per
  platform+scopes via `Store`. Shared by `ags` and `nrps`.
- `ltitest.FakePlatform` now also serves a fake OAuth2 token endpoint
  (`TokenEndpoint`), enabling end-to-end tests of the full
  launch -> token -> AGS/NRPS call path.
- Deep Linking (`deeplink`): response builder (`NewResponseForLaunch`,
  `AddItem`, `Sign`, `WriteAutoSubmitForm`), content item types
  (`LTIResourceLink`, `Link`, `HTML`, `Image`, `File`), and an
  auto-submitting HTML form renderer. `AddItem` enforces the launch's
  declared `accept_types`/`accept_multiple` constraints.
- Dynamic Registration (`dynreg`): a mountable `Handler` that fetches a
  platform's OpenID configuration, registers this tool against its
  registration endpoint, and persists the resulting `Platform`
  (inactive, pending review) and any assigned `deployment_id`.
- `docs/guides/`: task-oriented how-to documentation -- quickstart,
  platform/key management, handling a launch, Deep Linking,
  grades/roster, Dynamic Registration, testing, and the error model.
- `pgstore`: a PostgreSQL implementation of `lti.Store`. Wraps a
  consumer-supplied `*sql.DB` (any `database/sql` driver) and creates
  its tables idempotently on first use -- no separate migration tool or
  step. Table/column names mirror ltijs's MongoDB schema where the data
  model overlaps (`platforms`, `nonces`, `id_tokens`, `access_tokens`);
  `deployments` and `platform_keys` have no ltijs equivalent, since they
  close gaps this SDK deliberately closes (see docs/DESIGN.md).
  Verified against a real PostgreSQL database via `storetest`.
- `storetest`'s conformance suite now creates a real `Platform` before
  testing deployments, cached tokens, and keys, instead of referencing a
  synthetic platform ID that was never created -- `memstore`'s
  unconstrained maps didn't care, but this is required for a `Store`
  that enforces referential integrity (like `pgstore`) to be tested
  correctly.
