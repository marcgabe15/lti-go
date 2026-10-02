// Package pgstore is a PostgreSQL implementation of lti.Store. Pass it
// your own *sql.DB (opened with whichever driver you prefer, e.g.
// github.com/jackc/pgx/v5/stdlib or github.com/lib/pq) and it creates
// its tables on first use -- there is no separate migration step to run.
//
// Table and column names follow ltijs's MongoDB schema where the data
// model overlaps; see schema.sql for the exact mapping and where (and
// why) this SDK's schema diverges from it.
package pgstore

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"strings"

	"github.com/marcgabe15/lti-go"
)

//go:embed schema.sql
var schemaSQL string

// Store is a PostgreSQL-backed lti.Store.
type Store struct {
	db *sql.DB
}

var _ lti.Store = (*Store)(nil)

// New wraps db as a Store, creating its tables if they don't already
// exist. It is safe to call from multiple processes concurrently at
// startup (the DDL is idempotent), but is not intended to be called on
// every request -- call it once and reuse the returned *Store.
//
// db must be a PostgreSQL connection; any database/sql driver works
// (github.com/jackc/pgx/v5/stdlib and github.com/lib/pq are both known
// to support the single multi-statement Exec this uses to apply the
// schema).
func New(ctx context.Context, db *sql.DB) (*Store, error) {
	if _, err := db.ExecContext(ctx, schemaSQL); err != nil {
		return nil, fmt.Errorf("pgstore: create schema: %w", err)
	}
	return &Store{db: db}, nil
}

// isUniqueViolation reports whether err is a PostgreSQL unique_violation
// (SQLSTATE 23505). This avoids a hard dependency on a specific driver's
// error type (*pgconn.PgError for pgx, *pq.Error for lib/pq, ...): pgx
// includes "SQLSTATE 23505" in its Error() string, and lib/pq includes
// the human-readable "duplicate key value violates unique constraint"
// phrase in its own, so matching on either covers both.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "duplicate key value violates unique constraint")
}
