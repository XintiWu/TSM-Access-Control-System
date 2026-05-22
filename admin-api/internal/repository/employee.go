package repository

import (
	"context"
	"errors"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
)

var ErrNotFound = errors.New("employee not found")

type EmployeeRepository struct {
	chConn clickhouse.Conn
}

func NewEmployeeRepository(chAddr, chUser, chPass string) (*EmployeeRepository, error) {
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
	return &EmployeeRepository{chConn: chConn}, nil
}

func (r *EmployeeRepository) Ping(ctx context.Context) error {
	return r.chConn.Ping(ctx)
}

func (r *EmployeeRepository) Exists(ctx context.Context, userID string) (bool, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return false, err
	}
	var count uint64
	err = r.chConn.QueryRow(ctx, `SELECT count() FROM employee WHERE id = ?`, id).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *EmployeeRepository) SetActive(ctx context.Context, userID string, active bool) error {
	row, err := r.getLatest(ctx, userID)
	if err != nil {
		return err
	}
	if row == nil {
		return ErrNotFound
	}

	activeVal := uint8(0)
	if active {
		activeVal = 1
	}

	return r.chConn.Exec(ctx, `
		INSERT INTO employee (id, name, card_uid, is_active, org_unit_id, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		row.id, row.name, row.cardUID, activeVal, row.orgUnitID, time.Now().UTC(),
	)
}

type employeeSnapshot struct {
	id        uuid.UUID
	name      string
	cardUID   *string
	isActive  uint8
	orgUnitID *uuid.UUID
}

func (r *EmployeeRepository) getLatest(ctx context.Context, userID string) (*employeeSnapshot, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return nil, err
	}
	var count uint64
	if err := r.chConn.QueryRow(ctx, `SELECT count() FROM employee WHERE id = ?`, id).Scan(&count); err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, nil
	}

	var (
		name      string
		cardUID   *string
		isActive  uint8
		orgUnitID *uuid.UUID
	)
	err = r.chConn.QueryRow(ctx, `
		SELECT
			argMax(name, updated_at),
			argMax(card_uid, updated_at),
			argMax(is_active, updated_at),
			argMax(org_unit_id, updated_at)
		FROM employee
		WHERE id = ?
		GROUP BY id`, id).Scan(&name, &cardUID, &isActive, &orgUnitID)
	if err != nil {
		return nil, err
	}
	return &employeeSnapshot{id: id, name: name, cardUID: cardUID, isActive: isActive, orgUnitID: orgUnitID}, nil
}

func (r *EmployeeRepository) Close() error {
	return r.chConn.Close()
}
