package repository

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
	"github.com/tsmc/report-api/internal/model"
)

// OrgRepository handles queries against org_unit and employee in ClickHouse.
type OrgRepository struct {
	chConn clickhouse.Conn
}

// NewOrgRepository opens a ClickHouse connection for org/employee queries.
func NewOrgRepository(chAddr, chUser, chPass string) (*OrgRepository, error) {
	chConn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{chAddr},
		Auth: clickhouse.Auth{
			Database: "access_control",
			Username: chUser,
			Password: chPass,
		},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	if err := chConn.Ping(context.Background()); err != nil {
		_ = chConn.Close()
		return nil, err
	}
	return &OrgRepository{chConn: chConn}, nil
}

func (r *OrgRepository) Ping(ctx context.Context) error {
	return r.chConn.Ping(ctx)
}

// GetOrgUnit returns a single org_unit by ID. Returns nil if not found.
func (r *OrgRepository) GetOrgUnit(ctx context.Context, id string) (*model.OrgUnit, error) {
	orgID, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}
	var count uint64
	if err := r.chConn.QueryRow(ctx, `SELECT count() FROM org_unit WHERE id = ?`, orgID).Scan(&count); err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, nil
	}

	var u model.OrgUnit
	var idOut uuid.UUID
	var parentID *uuid.UUID
	var depth uint8
	err = r.chConn.QueryRow(ctx, `
		SELECT id, name, parent_id, depth, materialized_path
		FROM org_unit WHERE id = ?`, orgID).Scan(
		&idOut, &u.Name, &parentID, &depth, &u.MaterializedPath,
	)
	u.ID = idOut.String()
	u.Depth = int(depth)
	if err != nil {
		return nil, err
	}
	if parentID != nil {
		u.ParentID = parentID.String()
	}
	return &u, nil
}

// GetSubtreeIDs returns all org_unit IDs under the given unit (inclusive).
func (r *OrgRepository) GetSubtreeIDs(ctx context.Context, orgUnitID string) ([]string, error) {
	orgID, err := uuid.Parse(orgUnitID)
	if err != nil {
		return nil, err
	}
	var path string
	err = r.chConn.QueryRow(ctx,
		`SELECT materialized_path FROM org_unit WHERE id = ?`, orgID).Scan(&path)
	if err != nil {
		return nil, err
	}

	rows, err := r.chConn.Query(ctx,
		`SELECT toString(id) FROM org_unit WHERE materialized_path LIKE concat(?, '%')`, path)
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
func (r *OrgRepository) GetEmployeeOrgUnitID(ctx context.Context, employeeID string) (string, error) {
	id, err := uuid.Parse(employeeID)
	if err != nil {
		return "", err
	}
	var count uint64
	if err := r.chConn.QueryRow(ctx, `SELECT count() FROM employee WHERE id = ?`, id).Scan(&count); err != nil {
		return "", err
	}
	if count == 0 {
		return "", nil
	}
	var orgUnitID *uuid.UUID
	err = r.chConn.QueryRow(ctx, `
		SELECT argMax(org_unit_id, updated_at) FROM employee WHERE id = ? GROUP BY id`, id).Scan(&orgUnitID)
	if err != nil {
		return "", err
	}
	if orgUnitID == nil {
		return "", nil
	}
	return orgUnitID.String(), nil
}

// IsInSubtree checks whether targetOrgUnitID is a descendant (or equal) of requesterOrgUnitID.
func (r *OrgRepository) IsInSubtree(ctx context.Context, requesterOrgUnitID, targetOrgUnitID string) (bool, error) {
	reqID, err := uuid.Parse(requesterOrgUnitID)
	if err != nil {
		return false, err
	}
	tgtID, err := uuid.Parse(targetOrgUnitID)
	if err != nil {
		return false, err
	}
	var requesterPath, targetPath string
	err = r.chConn.QueryRow(ctx,
		`SELECT materialized_path FROM org_unit WHERE id = ?`, reqID).Scan(&requesterPath)
	if err != nil {
		return false, err
	}
	err = r.chConn.QueryRow(ctx,
		`SELECT materialized_path FROM org_unit WHERE id = ?`, tgtID).Scan(&targetPath)
	if err != nil {
		return false, err
	}
	return len(targetPath) >= len(requesterPath) && targetPath[:len(requesterPath)] == requesterPath, nil
}

// GetChildUnits returns the direct children of an org_unit.
func (r *OrgRepository) GetChildUnits(ctx context.Context, parentID string) ([]model.OrgUnit, error) {
	pid, err := uuid.Parse(parentID)
	if err != nil {
		return nil, err
	}
	rows, err := r.chConn.Query(ctx, `
		SELECT id, name, parent_id, depth, materialized_path
		FROM org_unit WHERE parent_id = ?`, pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var units []model.OrgUnit
	for rows.Next() {
		var u model.OrgUnit
		var idOut uuid.UUID
		var parent *uuid.UUID
		var depth uint8
		if err := rows.Scan(&idOut, &u.Name, &parent, &depth, &u.MaterializedPath); err != nil {
			return nil, err
		}
		u.ID = idOut.String()
		u.Depth = int(depth)
		if parent != nil {
			u.ParentID = parent.String()
		}
		units = append(units, u)
	}
	return units, rows.Err()
}

func (r *OrgRepository) Close() error {
	return r.chConn.Close()
}
