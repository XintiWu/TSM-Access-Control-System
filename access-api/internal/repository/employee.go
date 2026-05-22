package repository

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
)

// EmployeeRepository provides DB-level queries for the access-api fallback path.
// This is only used when Redis is unavailable.
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

// IsActive checks whether the employee is active (not banned).
// Returns (true, nil) if active, (false, nil) if banned, error otherwise.
func (r *EmployeeRepository) IsActive(ctx context.Context, userID string) (bool, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return false, err
	}
	var count uint64
	if err := r.chConn.QueryRow(ctx, `SELECT count() FROM employee WHERE id = ?`, id).Scan(&count); err != nil {
		return false, err
	}
	if count == 0 {
		return true, nil // unknown user — allow (fail-open for resilience)
	}
	var active uint8
	err = r.chConn.QueryRow(ctx, `
		SELECT argMax(is_active, updated_at) FROM employee WHERE id = ? GROUP BY id`, id).Scan(&active)
	if err != nil {
		return false, err
	}
	return active == 1, nil
}

// LookupCardUID returns the employee UUID for a given card_uid.
// Returns ("", nil) if the card is not found.
// GetLastPassbackState returns the last ALLOW swipe direction for anti-passback when Redis is down.
func (r *EmployeeRepository) GetLastPassbackState(ctx context.Context, userID string) (string, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return "", err
	}
	var direction string
	err = r.chConn.QueryRow(ctx, `
		SELECT direction FROM inout_events
		WHERE employee_id = ? AND status = 'ALLOW'
		ORDER BY event_time DESC
		LIMIT 1`, id).Scan(&direction)
	if err != nil {
		return "", nil
	}
	return direction, nil
}

func (r *EmployeeRepository) LookupCardUID(ctx context.Context, cardUID string) (string, error) {
	var userID uuid.UUID
	err := r.chConn.QueryRow(ctx, `
		SELECT id FROM employee
		WHERE card_uid = ?
		ORDER BY updated_at DESC
		LIMIT 1`, cardUID).Scan(&userID)
	if err != nil {
		return "", nil
	}
	return userID.String(), nil
}

func (r *EmployeeRepository) Close() error {
	return r.chConn.Close()
}
