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
