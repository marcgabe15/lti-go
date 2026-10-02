package pgstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/marcgabe15/lti-go"
	"github.com/marcgabe15/lti-go/internal/idgen"
)

const deploymentIDLength = 16

func (s *Store) CreateDeployment(ctx context.Context, d *lti.Deployment) (*lti.Deployment, error) {
	id, err := idgen.New(deploymentIDLength)
	if err != nil {
		return nil, fmt.Errorf("pgstore: generate deployment id: %w", err)
	}

	row := s.db.QueryRowContext(ctx, `
		INSERT INTO deployments (id, platform_id, deployment_id, name)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at`,
		id, d.PlatformID, d.DeploymentID, d.Name,
	)

	out := *d
	out.ID = id
	if err := row.Scan(&out.CreatedAt); err != nil {
		if isUniqueViolation(err) {
			return nil, lti.ErrDeploymentAlreadyExists
		}
		return nil, fmt.Errorf("pgstore: create deployment: %w", err)
	}
	return &out, nil
}

func scanDeployment(row interface{ Scan(...any) error }) (*lti.Deployment, error) {
	var d lti.Deployment
	err := row.Scan(&d.ID, &d.PlatformID, &d.DeploymentID, &d.Name, &d.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, lti.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("pgstore: scan deployment: %w", err)
	}
	return &d, nil
}

const selectDeploymentColumns = `id, platform_id, deployment_id, name, created_at`

func (s *Store) FindDeployment(ctx context.Context, platformID, deploymentID string) (*lti.Deployment, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+selectDeploymentColumns+` FROM deployments WHERE platform_id = $1 AND deployment_id = $2`,
		platformID, deploymentID)
	return scanDeployment(row)
}

func (s *Store) ListDeployments(ctx context.Context, platformID string) ([]*lti.Deployment, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+selectDeploymentColumns+` FROM deployments WHERE platform_id = $1 ORDER BY id`, platformID)
	if err != nil {
		return nil, fmt.Errorf("pgstore: list deployments: %w", err)
	}
	defer rows.Close()

	var out []*lti.Deployment
	for rows.Next() {
		d, err := scanDeployment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pgstore: list deployments: %w", err)
	}
	return out, nil
}

func (s *Store) DeleteDeployment(ctx context.Context, platformID, deploymentID string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM deployments WHERE platform_id = $1 AND deployment_id = $2`, platformID, deploymentID)
	if err != nil {
		return fmt.Errorf("pgstore: delete deployment: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("pgstore: delete deployment: %w", err)
	}
	if n == 0 {
		return lti.ErrNotFound
	}
	return nil
}
