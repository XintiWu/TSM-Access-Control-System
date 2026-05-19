package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var ErrNotFound = errors.New("employee not found")

type EmployeeRepository struct {
	db *sql.DB
}

func NewEmployeeRepository(dsn string) (*EmployeeRepository, error) {
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
	return &EmployeeRepository{db: db}, nil
}

func (r *EmployeeRepository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

func (r *EmployeeRepository) Exists(ctx context.Context, userID string) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT 1 FROM employee WHERE id = ? LIMIT 1`, userID).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *EmployeeRepository) SetActive(ctx context.Context, userID string, active bool) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE employee SET is_active = ? WHERE id = ?`, active, userID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *EmployeeRepository) Close() error {
	return r.db.Close()
}
