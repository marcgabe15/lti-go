-- Schema for pgstore.Store. Applied automatically (idempotently, via
-- CREATE TABLE IF NOT EXISTS) by pgstore.New -- you do not need to run
-- this yourself.
--
-- Table and column names follow ltijs's MongoDB schema
-- (src/services/database-manager/mongo/database-schemas.ts) where the
-- data model overlaps, snake_cased for SQL:
--   platforms    -> ltijs "platforms" collection (url, clientId, name,
--                   authenticationEndpoint, accessTokenEndpoint,
--                   idTokenValidation.method/.key, active, keys.public/.private)
--   nonces       -> ltijs "nonces" collection (nonce)
--   id_tokens    -> ltijs "idTokens" collection (the stored launch record;
--                   ltijs flattens raw claims into the document, we store
--                   them as one JSONB column instead)
--   access_tokens -> ltijs "accesstokens" collection (scopes, and the
--                   access_token field nested in its "value" blob)
--
-- Two tables have no ltijs equivalent, because they close gaps this SDK
-- deliberately closes (see docs/DESIGN.md):
--   deployments    -- ltijs does not track deployment_id per platform at all
--   platform_keys  -- ltijs's current schema embeds keys.public/private
--                     directly on the platform document; this SDK generates
--                     and stores them separately (closer to ltijs's *legacy*
--                     schema, which used separate publickeys/privatekeys
--                     collections keyed by kid -- we use one table since our
--                     KeyPair type holds both keys together)
--
-- ltijs stores an authorization_server field on platforms; this SDK's
-- lti.Platform type does not currently expose that field, so there is no
-- matching column here.

-- IDs are generated in Go (internal/idgen), not by a Postgres function,
-- so this schema has no dependency on pgcrypto or a minimum Postgres
-- version for UUID generation.

CREATE TABLE IF NOT EXISTS platforms (
    id                       TEXT PRIMARY KEY,
    url                      TEXT NOT NULL, -- ltijs: platforms.url (this SDK's lti.Platform.Issuer)
    client_id                TEXT NOT NULL, -- ltijs: platforms.clientId
    name                     TEXT NOT NULL DEFAULT '',
    authentication_endpoint  TEXT NOT NULL DEFAULT '',
    access_token_endpoint    TEXT NOT NULL DEFAULT '',
    key_method               TEXT NOT NULL DEFAULT '', -- ltijs: platforms.idTokenValidation.method
    key_jwks_uri             TEXT NOT NULL DEFAULT '', -- ltijs: platforms.idTokenValidation.key, when method is a JWK set URI
    key_jwk                  TEXT NOT NULL DEFAULT '', -- ltijs: platforms.idTokenValidation.key, when method is a single JWK
    key_rsa                  TEXT NOT NULL DEFAULT '', -- ltijs: platforms.idTokenValidation.key, when method is a raw RSA key
    active                   BOOLEAN NOT NULL DEFAULT true, -- ltijs: platforms.active
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (url, client_id) -- ltijs: unique index on {url, clientId}
);

CREATE TABLE IF NOT EXISTS deployments (
    id            TEXT PRIMARY KEY,
    platform_id   TEXT NOT NULL REFERENCES platforms(id) ON DELETE CASCADE,
    deployment_id TEXT NOT NULL,
    name          TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (platform_id, deployment_id)
);

CREATE TABLE IF NOT EXISTS nonces ( -- ltijs: "nonces" collection
    nonce      TEXT PRIMARY KEY, -- ltijs: nonces.nonce
    expires_at TIMESTAMPTZ NOT NULL -- ltijs instead uses a Mongo TTL index on createdAt; we store the resolved expiry directly since Postgres has no equivalent to a TTL index and ConsumeNonce needs it in one atomic statement
);

CREATE TABLE IF NOT EXISTS id_tokens ( -- ltijs: "idTokens" collection
    id          TEXT PRIMARY KEY,
    platform_id TEXT NOT NULL, -- ltijs: idTokens["https://.../claim/platform_id"] (internal bookkeeping field, same purpose)
    claims      JSONB NOT NULL, -- ltijs flattens the raw claims onto the document itself; we store the parsed lti.Claims as one JSONB value instead
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(), -- ltijs: idTokens.createdAt
    expires_at  TIMESTAMPTZ NOT NULL -- ltijs instead applies a 24h TTL index on createdAt
);

CREATE TABLE IF NOT EXISTS access_tokens ( -- ltijs: "accesstokens" collection
    platform_id  TEXT NOT NULL REFERENCES platforms(id) ON DELETE CASCADE,
    scopes       TEXT NOT NULL, -- ltijs: accesstokens.scopes
    access_token TEXT NOT NULL, -- ltijs: accesstokens.value.access_token
    expires_at   TIMESTAMPTZ NOT NULL, -- ltijs instead applies a 1h TTL index on createdAt
    PRIMARY KEY (platform_id, scopes) -- ltijs: unique index on {platformUrl, clientId, scopes}
);

CREATE TABLE IF NOT EXISTS platform_keys ( -- see note above re: ltijs's keys.public/.private vs. legacy publickeys/privatekeys
    platform_id TEXT PRIMARY KEY REFERENCES platforms(id) ON DELETE CASCADE,
    kid         TEXT NOT NULL, -- ltijs (legacy schema): publickeys.kid / privatekeys.kid
    private_key BYTEA NOT NULL, -- ltijs: platforms.keys.private (or legacy privatekeys.data, encrypted; we store PEM bytes directly)
    public_key  BYTEA NOT NULL, -- ltijs: platforms.keys.public (or legacy publickeys.data)
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
