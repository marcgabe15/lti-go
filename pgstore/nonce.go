package pgstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/marcgabe15/lti-go"
)

func (s *Store) SaveNonce(ctx context.Context, nonce string, expiresAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO nonces (nonce, expires_at) VALUES ($1, $2)`, nonce, expiresAt)
	if err != nil {
		return fmt.Errorf("pgstore: save nonce: %w", err)
	}
	return nil
}

// ConsumeNonce deletes nonce and checks its expiry in a single
// DELETE ... RETURNING statement, so concurrent callers racing to
// consume the same nonce cannot both succeed: PostgreSQL's row-level
// locking serializes the two DELETEs, and only the one that actually
// removed a row gets a value back to check.
//
// A nonce row is always removed once looked up here, expired or not
// (matching memstore's behavior) -- but a nonce that's issued and then
// never looked up again (an abandoned login flow) is never cleaned up by
// this method. If that matters for your traffic volume, run a periodic
// `DELETE FROM nonces WHERE expires_at < now()` yourself.
func (s *Store) ConsumeNonce(ctx context.Context, nonce string) error {
	var expiresAt time.Time
	err := s.db.QueryRowContext(ctx,
		`DELETE FROM nonces WHERE nonce = $1 RETURNING expires_at`, nonce,
	).Scan(&expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return lti.ErrNonceReused
	}
	if err != nil {
		return fmt.Errorf("pgstore: consume nonce: %w", err)
	}
	if time.Now().After(expiresAt) {
		return lti.ErrNonceReused
	}
	return nil
}
