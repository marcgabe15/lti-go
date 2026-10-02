package pgstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/marcgabe15/lti-go"
	"github.com/marcgabe15/lti-go/internal/idgen"
)

const platformIDLength = 16

func (s *Store) CreatePlatform(ctx context.Context, p *lti.Platform) (*lti.Platform, error) {
	id, err := idgen.New(platformIDLength)
	if err != nil {
		return nil, fmt.Errorf("pgstore: generate platform id: %w", err)
	}

	row := s.db.QueryRowContext(ctx, `
		INSERT INTO platforms (
			id, url, client_id, name, authentication_endpoint, access_token_endpoint,
			key_method, key_jwks_uri, key_jwk, key_rsa, active
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING created_at, updated_at`,
		id, p.Issuer, p.ClientID, p.Name, p.AuthenticationEndpoint, p.AccessTokenEndpoint,
		p.KeyConfig.Method, p.KeyConfig.JWKSURI, p.KeyConfig.JWK, p.KeyConfig.RSAKey, p.Active,
	)

	out := *p
	out.ID = id
	if err := row.Scan(&out.CreatedAt, &out.UpdatedAt); err != nil {
		if isUniqueViolation(err) {
			return nil, lti.ErrPlatformAlreadyExists
		}
		return nil, fmt.Errorf("pgstore: create platform: %w", err)
	}
	return &out, nil
}

func scanPlatform(row interface{ Scan(...any) error }) (*lti.Platform, error) {
	var p lti.Platform
	err := row.Scan(
		&p.ID, &p.Issuer, &p.ClientID, &p.Name, &p.AuthenticationEndpoint, &p.AccessTokenEndpoint,
		&p.KeyConfig.Method, &p.KeyConfig.JWKSURI, &p.KeyConfig.JWK, &p.KeyConfig.RSAKey,
		&p.Active, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, lti.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("pgstore: scan platform: %w", err)
	}
	return &p, nil
}

const selectPlatformColumns = `
	id, url, client_id, name, authentication_endpoint, access_token_endpoint,
	key_method, key_jwks_uri, key_jwk, key_rsa, active, created_at, updated_at`

func (s *Store) GetPlatform(ctx context.Context, id string) (*lti.Platform, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+selectPlatformColumns+` FROM platforms WHERE id = $1`, id)
	return scanPlatform(row)
}

func (s *Store) FindPlatform(ctx context.Context, issuer, clientID string) (*lti.Platform, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+selectPlatformColumns+` FROM platforms WHERE url = $1 AND client_id = $2`, issuer, clientID)
	return scanPlatform(row)
}

func (s *Store) ListPlatforms(ctx context.Context, params *lti.ListPlatformsParams) ([]*lti.Platform, error) {
	query := `SELECT ` + selectPlatformColumns + ` FROM platforms`
	var args []any
	if params != nil && params.Active != nil {
		query += ` WHERE active = $1`
		args = append(args, *params.Active)
	}
	query += ` ORDER BY id`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("pgstore: list platforms: %w", err)
	}
	defer rows.Close()

	var out []*lti.Platform
	for rows.Next() {
		p, err := scanPlatform(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pgstore: list platforms: %w", err)
	}
	return out, nil
}

func (s *Store) UpdatePlatform(ctx context.Context, p *lti.Platform) (*lti.Platform, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE platforms SET
			url = $2, client_id = $3, name = $4, authentication_endpoint = $5, access_token_endpoint = $6,
			key_method = $7, key_jwks_uri = $8, key_jwk = $9, key_rsa = $10, active = $11, updated_at = now()
		WHERE id = $1
		RETURNING `+selectPlatformColumns,
		p.ID, p.Issuer, p.ClientID, p.Name, p.AuthenticationEndpoint, p.AccessTokenEndpoint,
		p.KeyConfig.Method, p.KeyConfig.JWKSURI, p.KeyConfig.JWK, p.KeyConfig.RSAKey, p.Active,
	)
	return scanPlatform(row)
}

func (s *Store) DeletePlatform(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM platforms WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("pgstore: delete platform: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("pgstore: delete platform: %w", err)
	}
	if n == 0 {
		return lti.ErrNotFound
	}
	return nil
}
