package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// EmployeeRepository provides DB-level queries for the access-api fallback path.
// This is only used when Redis is unavailable.
type EmployeeRepository struct {
	db *sql.DB
}

func NewEmployeeRepository(dsn string) (*EmployeeRepository, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &EmployeeRepository{db: db}, nil
}

// IsActive checks whether the employee is active (not banned).
// Returns (true, nil) if active, (false, nil) if banned, error otherwise.
func (r *EmployeeRepository) IsActive(ctx context.Context, userID string) (bool, error) {
	var active bool
	err := r.db.QueryRowContext(ctx,
		`SELECT is_active FROM employee WHERE id = ? LIMIT 1`, userID).Scan(&active)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil // unknown user — allow (fail-open for resilience)
	}
	if err != nil {
		return false, err
	}
	return active, nil
}

// LookupCardUID returns the employee UUID for a given card_uid.
// Returns ("", nil) if the card is not found.
func (r *EmployeeRepository) LookupCardUID(ctx context.Context, cardUID string) (string, error) {
	var userID string
	err := r.db.QueryRowContext(ctx,
		`SELECT id FROM employee WHERE card_uid = ? LIMIT 1`, cardUID).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return userID, nil
}

func (r *EmployeeRepository) Close() error {
	return r.db.Close()
}
