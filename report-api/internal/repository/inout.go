package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	"github.com/tsmc/report-api/internal/model"
)

// InOutRepository handles raw inout_events queries using ClickHouse.
type InOutRepository struct {
	chConn clickhouse.Conn
}

// NewInOutRepository opens a ClickHouse native TCP connection.
func NewInOutRepository(chAddr, chUser, chPass string) (*InOutRepository, error) {
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
	return &InOutRepository{chConn: chConn}, nil
}

// GetPersonalEvents returns raw events for a single employee within a date range from ClickHouse.
func (r *InOutRepository) GetPersonalEvents(ctx context.Context, employeeID, startDate, endDate string) ([]model.InOutEvent, error) {
	empUUID, err := uuid.Parse(employeeID)
	if err != nil {
		return nil, fmt.Errorf("invalid employee id: %w", err)
	}

	rows, err := r.chConn.Query(ctx, `
		SELECT id, employee_id, door_id, direction, event_time, status,
		       reason, COALESCE(card_uid,''), COALESCE(source_ip,'')
		FROM inout_events
		WHERE employee_id = ?
		  AND event_time >= toDateTime64(?, 3, 'UTC')
		  AND event_time < addDays(toDateTime64(?, 3, 'UTC'), 1)
		ORDER BY event_time ASC`,
		empUUID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
}

// AuditFilter holds optional filters for the audit log query.
type AuditFilter struct {
	StartDate  string
	EndDate    string
	EmployeeID string   // optional
	DoorID     string   // optional
	Status     string   // optional: ALLOW / DENY
	OrgUnitIDs []string // restrict to employees in these org units
	Page       int
	PageSize   int
}

// GetAuditEvents returns paginated raw events with optional filters from ClickHouse.
func (r *InOutRepository) GetAuditEvents(ctx context.Context, f AuditFilter) ([]model.InOutEvent, int, error) {
	var whereClauses []string
	var args []interface{}

	whereClauses = append(whereClauses, "event_time >= toDateTime64(?, 3, 'UTC') AND event_time < addDays(toDateTime64(?, 3, 'UTC'), 1)")
	args = append(args, f.StartDate, f.EndDate)

	if f.EmployeeID != "" {
		empUUID, err := uuid.Parse(f.EmployeeID)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid employee id: %w", err)
		}
		whereClauses = append(whereClauses, "employee_id = ?")
		args = append(args, empUUID)
	}
	if f.DoorID != "" {
		doorUUID, err := uuid.Parse(f.DoorID)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid door id: %w", err)
		}
		whereClauses = append(whereClauses, "door_id = ?")
		args = append(args, doorUUID)
	}
	if f.Status != "" {
		whereClauses = append(whereClauses, "status = ?")
		args = append(args, f.Status)
	}

	if len(f.OrgUnitIDs) > 0 {
		var orgUUIDs []uuid.UUID
		for _, id := range f.OrgUnitIDs {
			u, err := uuid.Parse(id)
			if err == nil {
				orgUUIDs = append(orgUUIDs, u)
			}
		}
		whereClauses = append(whereClauses, "org_unit_id IN (?)")
		args = append(args, orgUUIDs)
	}

	where := strings.Join(whereClauses, " AND ")

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM inout_events WHERE %s", where)
	var totalCount uint64
	if err := r.chConn.QueryRow(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		return nil, 0, err
	}

	// Paginated results
	offset := (f.Page - 1) * f.PageSize
	selectQuery := fmt.Sprintf(`
		SELECT id, employee_id, door_id, direction, event_time,
		       status, reason, COALESCE(card_uid,''), COALESCE(source_ip,'')
		FROM inout_events
		WHERE %s
		ORDER BY event_time DESC
		LIMIT ? OFFSET ?`, where)

	allArgs := make([]interface{}, len(args))
	copy(allArgs, args)
	allArgs = append(allArgs, f.PageSize, offset)

	rows, err := r.chConn.Query(ctx, selectQuery, allArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	events, err := scanEvents(rows)
	if err != nil {
		return nil, 0, err
	}
	return events, int(totalCount), nil
}

// GetEventsForExport returns all events for an org-unit subtree in a date range (no pagination) from ClickHouse.
func (r *InOutRepository) GetEventsForExport(ctx context.Context, orgUnitIDs []string, startDate, endDate string) ([]model.InOutEvent, error) {
	if len(orgUnitIDs) == 0 {
		return nil, nil
	}
	var orgUUIDs []uuid.UUID
	for _, id := range orgUnitIDs {
		u, err := uuid.Parse(id)
		if err == nil {
			orgUUIDs = append(orgUUIDs, u)
		}
	}

	rows, err := r.chConn.Query(ctx, `
		SELECT id, employee_id, door_id, direction, event_time,
		       status, reason, COALESCE(card_uid,''), COALESCE(source_ip,'')
		FROM inout_events
		WHERE event_time >= toDateTime64(?, 3, 'UTC')
		  AND event_time < addDays(toDateTime64(?, 3, 'UTC'), 1)
		  AND org_unit_id IN (?)
		ORDER BY event_time ASC`,
		startDate, endDate, orgUUIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
}

func scanEvents(rows driver.Rows) ([]model.InOutEvent, error) {
	var events []model.InOutEvent
	for rows.Next() {
		var e model.InOutEvent
		var eventID, employeeID, doorID uuid.UUID
		var direction, status string
		var reason *string
		var eventTime time.Time
		var cardUID, sourceIP string

		if err := rows.Scan(&eventID, &employeeID, &doorID, &direction,
			&eventTime, &status, &reason, &cardUID, &sourceIP); err != nil {
			return nil, err
		}

		e.EventID = eventID.String()
		e.EmployeeID = employeeID.String()
		e.DoorID = doorID.String()
		e.Direction = direction
		e.EventTime = eventTime
		e.Status = status
		e.Reason = reason
		e.CardUID = cardUID
		e.SourceIP = sourceIP

		events = append(events, e)
	}
	return events, rows.Err()
}

// Close closes the database connection.
func (r *InOutRepository) Close() error {
	return r.chConn.Close()
}
