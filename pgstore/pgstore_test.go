package pgstore_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	// The pgstore package itself is driver-agnostic (it only needs a
	// *sql.DB); this test needs an actual driver to connect with, and
	// lib/pq is one of the two drivers pgstore documents as supporting
	// its multi-statement schema Exec.
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marcgabe15/lti-go"
	"github.com/marcgabe15/lti-go/pgstore"
	"github.com/marcgabe15/lti-go/storetest"
)

// TestStore runs the full lti.Store conformance suite against a real
// PostgreSQL database. It requires PGSTORE_TEST_DSN to be set to a
// connection string for a database it's safe to create/drop tables in;
// otherwise it's skipped.
//
//	createdb lti_go_pgstore_test
//	PGSTORE_TEST_DSN='postgres://localhost/lti_go_pgstore_test?sslmode=disable' go test ./pgstore/...
func TestStore(t *testing.T) {
	dsn := os.Getenv("PGSTORE_TEST_DSN")
	if dsn == "" {
		t.Skip("PGSTORE_TEST_DSN not set; skipping pgstore integration test")
	}

	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.Ping())

	tableCount := 0
	storetest.Run(t, func() lti.Store {
		tableCount++
		// storetest.Run calls this once per subtest and expects a fresh,
		// empty store each time; truncate rather than reopening a
		// connection.
		if tableCount > 1 {
			_, err := db.Exec(`TRUNCATE platforms, deployments, nonces, id_tokens, access_tokens, platform_keys`)
			require.NoError(t, err)
		}
		store, err := pgstore.New(context.Background(), db)
		require.NoError(t, err)
		return store
	})
}

func TestNew_CreatesSchemaIdempotently(t *testing.T) {
	dsn := os.Getenv("PGSTORE_TEST_DSN")
	if dsn == "" {
		t.Skip("PGSTORE_TEST_DSN not set; skipping pgstore integration test")
	}

	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	_, err = pgstore.New(ctx, db)
	require.NoError(t, err, "first New should create the schema")
	_, err = pgstore.New(ctx, db)
	require.NoError(t, err, "second New against an existing schema should not error")

	rows, err := db.QueryContext(ctx, `SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'`)
	require.NoError(t, err)
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		tables = append(tables, name)
	}
	for _, want := range []string{"platforms", "deployments", "nonces", "id_tokens", "access_tokens", "platform_keys"} {
		assert.Contains(t, tables, want)
	}
}
