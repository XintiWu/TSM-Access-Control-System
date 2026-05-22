package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/tsmc/aggregation-worker/internal/model"
)

type InOutRepository struct {
	db     *sql.DB
	chConn clickhouse.Conn
}

func NewInOutRepository(mariaDSN string, chAddr, chUser, chPass string) (*InOutRepository, error) {
	// Connect to MariaDB
	db, err := sql.Open("mysql", mariaDSN)
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

	// Connect to ClickHouse
	chConn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{chAddr},
		Auth: clickhouse.Auth{
			Database: "access_control",
			Username: chUser,
			Password: chPass,
		},
		Settings: clickhouse.Settings{
			"async_insert":          1,
			"wait_for_async_insert": 1,
		},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := chConn.Ping(context.Background()); err != nil {
		_ = db.Close()
		_ = chConn.Close()
		return nil, err
	}

	return &InOutRepository{db: db, chConn: chConn}, nil
}

func (r *InOutRepository) Insert(ctx context.Context, e model.InOutEvent, orgUnitID string) error {
	var reason sql.NullString
	if e.Reason != nil {
		reason = sql.NullString{String: *e.Reason, Valid: true}
	}

	eventID, err := uuid.Parse(e.EventID)
	if err != nil {
		return err
	}
	employeeID, err := uuid.Parse(e.EmployeeID)
	if err != nil {
		return err
	}
	doorID, err := uuid.Parse(e.DoorID)
	if err != nil {
		return err
	}

	var orgID uuid.UUID
	if orgUnitID != "" {
		parsedOrg, err := uuid.Parse(orgUnitID)
		if err == nil {
			orgID = parsedOrg
		}
	}

	err = r.chConn.Exec(ctx, `
		INSERT INTO inout_events
		(id, employee_id, door_id, direction, event_time, status, reason, source_ip, card_uid, org_unit_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		eventID, employeeID, doorID, e.Direction, e.EventTime,
		e.Status, reason, e.SourceIP, e.CardUID, orgID,
	)
	return err
}

func (r *InOutRepository) GetEmployeeOrgUnitID(ctx context.Context, employeeID string) (string, error) {
	var orgUnitID sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT org_unit_id FROM employee WHERE id = ? LIMIT 1`, employeeID).Scan(&orgUnitID)
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

func (r *InOutRepository) Close() error {
	_ = r.db.Close()
	return r.chConn.Close()
}
