package pgstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/marcgabe15/lti-go"
)

func (s *Store) SaveKeyPair(ctx context.Context, kp *lti.KeyPair) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO platform_keys (platform_id, kid, private_key, public_key)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (platform_id) DO UPDATE SET
			kid = EXCLUDED.kid, private_key = EXCLUDED.private_key, public_key = EXCLUDED.public_key`,
		kp.PlatformID, kp.KeyID, kp.PrivateKey, kp.PublicKey,
	)
	if err != nil {
		return fmt.Errorf("pgstore: save key pair: %w", err)
	}
	return nil
}

func (s *Store) GetKeyPair(ctx context.Context, platformID string) (*lti.KeyPair, error) {
	var kp lti.KeyPair
	kp.PlatformID = platformID
	err := s.db.QueryRowContext(ctx, `
		SELECT kid, private_key, public_key, created_at FROM platform_keys WHERE platform_id = $1`,
		platformID,
	).Scan(&kp.KeyID, &kp.PrivateKey, &kp.PublicKey, &kp.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, lti.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("pgstore: get key pair: %w", err)
	}
	return &kp, nil
}

func (s *Store) DeleteKeyPair(ctx context.Context, platformID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM platform_keys WHERE platform_id = $1`, platformID)
	if err != nil {
		return fmt.Errorf("pgstore: delete key pair: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("pgstore: delete key pair: %w", err)
	}
	if n == 0 {
		return lti.ErrNotFound
	}
	return nil
}
