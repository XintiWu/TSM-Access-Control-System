package repository

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/tsmc/aggregation-worker/internal/model"
)

type InOutRepository struct {
	db *sql.DB
}

func NewInOutRepository(dsn string) (*InOutRepository, error) {
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
	return &InOutRepository{db: db}, nil
}

func (r *InOutRepository) Insert(ctx context.Context, e model.InOutEvent) error {
	var reason sql.NullString
	if e.Reason != nil {
		reason = sql.NullString{String: *e.Reason, Valid: true}
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT IGNORE INTO inout_events
		(id, employee_id, door_id, direction, event_time, status, reason, source_ip, card_uid)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.EventID, e.EmployeeID, e.DoorID, e.Direction, e.EventTime,
		e.Status, reason, e.SourceIP, e.CardUID,
	)
	return err
}

func (r *InOutRepository) Close() error {
	return r.db.Close()
}
