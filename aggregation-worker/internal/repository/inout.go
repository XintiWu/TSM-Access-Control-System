package repository

import (
	"context"
	"crypto/tls"
	"database/sql"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
	"github.com/tsmc/aggregation-worker/internal/model"
)

type InOutRepository struct {
	chConn clickhouse.Conn
}

func NewInOutRepository(chAddr, chUser, chPass string) (*InOutRepository, error) {
	var tlsConfig *tls.Config
	if strings.Contains(chAddr, ":9440") {
		tlsConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

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
		TLS:         tlsConfig,
	})
	if err != nil {
		return nil, err
	}
	if err := chConn.Ping(context.Background()); err != nil {
		_ = chConn.Close()
		return nil, err
	}
	return &InOutRepository{chConn: chConn}, nil
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
	id, err := uuid.Parse(employeeID)
	if err != nil {
		return "", err
	}
	var orgUnitID *uuid.UUID
	var count uint64
	if err := r.chConn.QueryRow(ctx, `SELECT count() FROM employee WHERE id = ?`, id).Scan(&count); err != nil {
		return "", err
	}
	if count == 0 {
		return "", nil
	}
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

func (r *InOutRepository) Close() error {
	return r.chConn.Close()
}
