package pgstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/marcgabe15/lti-go"
)

func (s *Store) GetCachedToken(ctx context.Context, key lti.TokenCacheKey) (*lti.CachedToken, error) {
	var tok lti.CachedToken
	err := s.db.QueryRowContext(ctx, `
		SELECT access_token, expires_at FROM access_tokens
		WHERE platform_id = $1 AND scopes = $2 AND expires_at > now()`,
		key.PlatformID, key.Scopes,
	).Scan(&tok.AccessToken, &tok.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, lti.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("pgstore: get cached token: %w", err)
	}
	return &tok, nil
}

func (s *Store) SaveCachedToken(ctx context.Context, key lti.TokenCacheKey, tok *lti.CachedToken) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO access_tokens (platform_id, scopes, access_token, expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (platform_id, scopes) DO UPDATE SET
			access_token = EXCLUDED.access_token, expires_at = EXCLUDED.expires_at`,
		key.PlatformID, key.Scopes, tok.AccessToken, tok.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("pgstore: save cached token: %w", err)
	}
	return nil
}
