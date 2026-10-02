# PostgreSQL Store

`pgstore` is a PostgreSQL implementation of `lti.Store`. Pass it your
own `*sql.DB` and it creates its own tables on first use -- there's no
separate migration tool or step to run.

```go
import (
	"database/sql"

	_ "github.com/jackc/pgx/v5/stdlib" // or _ "github.com/lib/pq"

	"github.com/marcgabe15/lti-go/pgstore"
)

db, err := sql.Open("pgx", "postgres://user:pass@localhost/mydb")
if err != nil {
	log.Fatal(err)
}

store, err := pgstore.New(ctx, db) // creates tables if they don't exist
if err != nil {
	log.Fatal(err)
}

tool, err := lti.New(lti.Config{Issuer: "https://tool.example.com", Store: store})
```

Any `database/sql` driver works -- `pgstore` only uses the standard
`*sql.DB` interface, not a specific driver's native API. `pgx`'s stdlib
adapter (`github.com/jackc/pgx/v5/stdlib`) and `github.com/lib/pq` are
both confirmed to support the single multi-statement `Exec` call
`pgstore.New` uses to create its schema.

## Table and column names

`pgstore`'s schema deliberately mirrors ltijs's MongoDB schema (the
`platforms`, `nonces`, `idTokens`, and `accesstokens` collections)
wherever the data model overlaps, snake_cased for SQL -- so if you're
coming from ltijs, or inspecting the database directly, the names should
look familiar:

| Table | ltijs collection | Notes |
| --- | --- | --- |
| `platforms` | `platforms` | `url` (this SDK's `Platform.Issuer`), `client_id`, `authentication_endpoint`, `access_token_endpoint`, `active` all match ltijs's field names 1:1 |
| `nonces` | `nonces` | ltijs uses a Mongo TTL index on `createdAt`; Postgres has no equivalent, so `expires_at` is stored directly |
| `id_tokens` | `idTokens` | ltijs flattens raw claims onto the document; here the parsed `Claims` is one `JSONB` column instead |
| `access_tokens` | `accesstokens` | keyed by `(platform_id, scopes)` rather than ltijs's `(platformUrl, clientId, scopes)`, since this SDK has a surrogate platform id |
| `deployments` | *(none)* | ltijs doesn't track `deployment_id` per platform at all -- this SDK deliberately closes that gap, see `docs/DESIGN.md` |
| `platform_keys` | *(none, in the current schema)* | ltijs's current schema embeds `keys.public`/`keys.private` directly on the platform document; closer to ltijs's *legacy* schema, which used separate `publickeys`/`privatekeys` collections keyed by `kid` |

See `pgstore/schema.sql` for the full DDL with a line-by-line mapping in
its header comment.

## A caveat: `Claims.Platform()` after a round trip

`lti.Claims` carries an unexported back-reference to its resolved
`*lti.Platform`, set by `Tool.VerifyLaunch`. `encoding/json` silently
drops unexported fields, so a `Claims` you read back via
`Store.GetLaunch` (used internally by `Tool.VerifyLTIK`) will have
`Claims.Platform()` return `nil`, even though every other field survived
the round trip intact.

This is why `LaunchRecord` carries `PlatformID` as its own column,
separate from the JSONB `claims` blob -- if you need the `Platform` back
after resuming a launch via `ltik`, re-resolve it yourself:

```go
record, err := tool.VerifyLTIK(ctx, ltik)
platform, err := tool.Platforms().Get(ctx, record.PlatformID)
```

## Testing against a real database

`pgstore`'s own test suite (`pgstore_test.go`) runs the full
`storetest` conformance suite against a real PostgreSQL database when
`PGSTORE_TEST_DSN` is set, and is skipped otherwise:

```sh
createdb lti_go_pgstore_test
PGSTORE_TEST_DSN='postgres://localhost/lti_go_pgstore_test?sslmode=disable' go test ./pgstore/...
```

If you're extending `pgstore` yourself, this is the test to run --
`storetest` includes a concurrent `ConsumeNonce` check that only a real
database (not `memstore`'s in-process mutex) genuinely exercises.
