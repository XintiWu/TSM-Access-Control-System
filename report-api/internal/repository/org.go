package repository

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/tsmc/report-api/internal/model"
)

// OrgRepository handles queries against the org_unit table.
type OrgRepository struct {
	db *sql.DB
}

// NewOrgRepository opens a MariaDB connection for org_unit queries.
func NewOrgRepository(dsn string) (*OrgRepository, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &OrgRepository{db: db}, nil
}

// GetOrgUnit returns a single org_unit by ID. Returns nil if not found.
func (r *OrgRepository) GetOrgUnit(ctx context.Context, id string) (*model.OrgUnit, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, COALESCE(parent_id,''), depth, materialized_path
		 FROM org_unit WHERE id = ?`, id)
	var u model.OrgUnit
	if err := row.Scan(&u.ID, &u.Name, &u.ParentID, &u.Depth, &u.MaterializedPath); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

// GetSubtreeIDs returns all org_unit IDs under the given unit (inclusive),
// using the materialized_path LIKE pattern for fast subtree lookups.
func (r *OrgRepository) GetSubtreeIDs(ctx context.Context, orgUnitID string) ([]string, error) {
	// First get the materialized_path of the given org unit.
	var path string
	err := r.db.QueryRowContext(ctx,
		`SELECT materialized_path FROM org_unit WHERE id = ?`, orgUnitID).Scan(&path)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT id FROM org_unit WHERE materialized_path LIKE CONCAT(?, '%')`, path)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetEmployeeOrgUnitID returns the org_unit_id for the given employee.
// Returns ("", nil) if the employee is not found or has no org_unit.
func (r *OrgRepository) GetEmployeeOrgUnitID(ctx context.Context, employeeID string) (string, error) {
	var orgUnitID sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT org_unit_id FROM employee WHERE id = ?`, employeeID).Scan(&orgUnitID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	if !orgUnitID.Valid {
		return "", nil
	}
	return orgUnitID.String, nil
}

// IsInSubtree checks whether targetOrgUnitID is a descendant (or equal) of requesterOrgUnitID.
func (r *OrgRepository) IsInSubtree(ctx context.Context, requesterOrgUnitID, targetOrgUnitID string) (bool, error) {
	// Get target's materialized_path and check if it starts with requester's path.
	var requesterPath, targetPath string
	err := r.db.QueryRowContext(ctx,
		`SELECT materialized_path FROM org_unit WHERE id = ?`, requesterOrgUnitID).Scan(&requesterPath)
	if err != nil {
		return false, err
	}
	err = r.db.QueryRowContext(ctx,
		`SELECT materialized_path FROM org_unit WHERE id = ?`, targetOrgUnitID).Scan(&targetPath)
	if err != nil {
		return false, err
	}
	return len(targetPath) >= len(requesterPath) && targetPath[:len(requesterPath)] == requesterPath, nil
}

// GetChildUnits returns the direct children of an org_unit.
func (r *OrgRepository) GetChildUnits(ctx context.Context, parentID string) ([]model.OrgUnit, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, COALESCE(parent_id,''), depth, materialized_path
		 FROM org_unit WHERE parent_id = ?`, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var units []model.OrgUnit
	for rows.Next() {
		var u model.OrgUnit
		if err := rows.Scan(&u.ID, &u.Name, &u.ParentID, &u.Depth, &u.MaterializedPath); err != nil {
			return nil, err
		}
		units = append(units, u)
	}
	return units, rows.Err()
}

// Close closes the database connection.
func (r *OrgRepository) Close() error {
	return r.db.Close()
}
