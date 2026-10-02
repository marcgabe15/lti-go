package pgstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/marcgabe15/lti-go"
	"github.com/marcgabe15/lti-go/internal/idgen"
)

const launchIDLength = 16

// SaveLaunch stores l.Claims as JSONB. Note that lti.Claims carries an
// unexported back-reference to its resolved *lti.Platform (set by
// Tool.VerifyLaunch), which encoding/json silently drops -- a Claims
// read back via GetLaunch will have Claims.Platform() == nil. Use the
// returned LaunchRecord's PlatformID (a plain column, not part of the
// JSONB blob) to re-resolve it via Store.GetPlatform if you need it.
func (s *Store) SaveLaunch(ctx context.Context, l *lti.LaunchRecord) (string, error) {
	claimsJSON, err := json.Marshal(l.Claims)
	if err != nil {
		return "", fmt.Errorf("pgstore: marshal claims: %w", err)
	}

	id := l.ID
	if id == "" {
		id, err = idgen.New(launchIDLength)
		if err != nil {
			return "", fmt.Errorf("pgstore: generate launch id: %w", err)
		}
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO id_tokens (id, platform_id, claims, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5)`,
		id, l.PlatformID, claimsJSON, l.CreatedAt, l.ExpiresAt,
	)
	if err != nil {
		return "", fmt.Errorf("pgstore: save launch: %w", err)
	}
	return id, nil
}

func (s *Store) GetLaunch(ctx context.Context, id string) (*lti.LaunchRecord, error) {
	var l lti.LaunchRecord
	var claimsJSON []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT platform_id, claims, created_at, expires_at
		FROM id_tokens WHERE id = $1 AND expires_at > now()`, id,
	).Scan(&l.PlatformID, &claimsJSON, &l.CreatedAt, &l.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, lti.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("pgstore: get launch: %w", err)
	}

	var claims lti.Claims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, fmt.Errorf("pgstore: unmarshal claims: %w", err)
	}
	l.ID = id
	l.Claims = &claims
	return &l, nil
}

func (s *Store) DeleteLaunch(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM id_tokens WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("pgstore: delete launch: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("pgstore: delete launch: %w", err)
	}
	if n == 0 {
		return lti.ErrNotFound
	}
	return nil
}
