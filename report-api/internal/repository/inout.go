package repository

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	"github.com/tsmc/report-api/internal/model"
)

// InOutRepository handles raw inout_events queries.
type InOutRepository interface {
	GetPersonalEvents(ctx context.Context, employeeID, startDate, endDate string) ([]model.InOutEvent, error)
	GetAuditEvents(ctx context.Context, f AuditFilter) ([]model.InOutEvent, int, error)
	GetEventsForExport(ctx context.Context, orgUnitIDs []string, startDate, endDate string) ([]model.InOutEvent, error)
	GetSecurityDenySummary(ctx context.Context, orgUnitIDs []string, startDate, endDate string) (model.SecurityDenySummary, error)
	GetEmployeeReportRows(ctx context.Context, orgUnitIDs []string, startDate, endDate string) ([]model.EmployeeReportRow, error)
	Close() error
}

type chInOutRepository struct {
	chConn clickhouse.Conn
}

// NewInOutRepository opens a ClickHouse native TCP connection.
func NewInOutRepository(chAddr, chUser, chPass string) (InOutRepository, error) {
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
	return &chInOutRepository{chConn: chConn}, nil
}

// GetPersonalEvents returns raw events for a single employee within a date range from ClickHouse.
func (r *chInOutRepository) GetPersonalEvents(ctx context.Context, employeeID, startDate, endDate string) ([]model.InOutEvent, error) {
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
func (r *chInOutRepository) GetAuditEvents(ctx context.Context, f AuditFilter) ([]model.InOutEvent, int, error) {
	where, args, err := buildAuditWhereClauses(f)
	if err != nil {
		return nil, 0, err
	}

	countQuery := auditCountQueryPrefix + where
	var totalCount uint64
	if err := r.chConn.QueryRow(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		return nil, 0, err
	}

	offset := (f.Page - 1) * f.PageSize
	selectQuery := auditSelectQueryPrefix + where + auditSelectQuerySuffix

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

const (
	auditDateRangeClause   = "event_time >= toDateTime64(?, 3, 'UTC') AND event_time < addDays(toDateTime64(?, 3, 'UTC'), 1)"
	auditEmployeeClause    = "employee_id = ?"
	auditDoorClause        = "door_id = ?"
	auditStatusClause      = "status = ?"
	auditOrgUnitInClause   = "org_unit_id IN (?)"
	auditCountQueryPrefix  = "SELECT COUNT(*) FROM inout_events WHERE "
	auditSelectQueryPrefix = `
		SELECT id, employee_id, door_id, direction, event_time,
		       status, reason, COALESCE(card_uid,''), COALESCE(source_ip,'')
		FROM inout_events
		WHERE `
	auditSelectQuerySuffix = `
		ORDER BY event_time DESC
		LIMIT ? OFFSET ?`
)

func buildAuditWhereClauses(f AuditFilter) (string, []interface{}, error) {
	clauses := []string{auditDateRangeClause}
	args := []interface{}{f.StartDate, f.EndDate}

	if f.EmployeeID != "" {
		empUUID, err := uuid.Parse(f.EmployeeID)
		if err != nil {
			return "", nil, fmt.Errorf("invalid employee id: %w", err)
		}
		clauses = append(clauses, auditEmployeeClause)
		args = append(args, empUUID)
	}
	if f.DoorID != "" {
		doorUUID, err := uuid.Parse(f.DoorID)
		if err != nil {
			return "", nil, fmt.Errorf("invalid door id: %w", err)
		}
		clauses = append(clauses, auditDoorClause)
		args = append(args, doorUUID)
	}
	if f.Status != "" {
		clauses = append(clauses, auditStatusClause)
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
		clauses = append(clauses, auditOrgUnitInClause)
		args = append(args, orgUUIDs)
	}

	return strings.Join(clauses, " AND "), args, nil
}

// GetEventsForExport returns all events for an org-unit subtree in a date range (no pagination) from ClickHouse.
func (r *chInOutRepository) GetEventsForExport(ctx context.Context, orgUnitIDs []string, startDate, endDate string) ([]model.InOutEvent, error) {
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

// GetSecurityDenySummary counts anti-passback and permission-denied events in range.
func (r *chInOutRepository) GetSecurityDenySummary(ctx context.Context, orgUnitIDs []string, startDate, endDate string) (model.SecurityDenySummary, error) {
	var out model.SecurityDenySummary
	if len(orgUnitIDs) == 0 {
		return out, nil
	}
	var orgUUIDs []uuid.UUID
	for _, id := range orgUnitIDs {
		u, err := uuid.Parse(id)
		if err == nil {
			orgUUIDs = append(orgUUIDs, u)
		}
	}
	query := `
		SELECT
			toUInt64(countIf(status = 'DENY' AND reason = 'ANTI_PASSBACK')),
			toUInt64(countIf(status = 'DENY' AND reason = 'PERMISSION_DENIED'))
		FROM inout_events
		WHERE event_time >= toDateTime64(?, 3, 'UTC')
		  AND event_time < addDays(toDateTime64(?, 3, 'UTC'), 1)
		  AND org_unit_id IN (?)`
	var ap, pd uint64
	err := r.chConn.QueryRow(ctx, query, startDate, endDate, orgUUIDs).Scan(&ap, &pd)
	out.AntiPassbackDenies = int(ap)
	out.PermissionDenied = int(pd)
	return out, err
}

// GetEmployeeReportRows aggregates per-employee swipe, hours, and deny counts.
func (r *chInOutRepository) GetEmployeeReportRows(ctx context.Context, orgUnitIDs []string, startDate, endDate string) ([]model.EmployeeReportRow, error) {
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
	query := `
		SELECT
			toString(employee_id) AS employee_id,
			toUInt64(sum(day_swipes)) AS total_swipes,
			toFloat64(sum(daily_hours)) AS total_hours,
			toUInt64(sum(passback_denies)) AS passback_denies,
			toUInt64(sum(permission_denies)) AS permission_denies,
			toUInt64(sum(missing_punch)) AS missing_punch_days
		FROM (
			SELECT
				employee_id,
				toDate(event_time) AS day,
				count() AS day_swipes,
				countIf(status = 'DENY' AND reason = 'ANTI_PASSBACK') AS passback_denies,
				countIf(status = 'DENY' AND reason = 'PERMISSION_DENIED') AS permission_denies,
				multiIf(
					countIf(status = 'ALLOW' AND direction = 'IN') > 0
						AND countIf(status = 'ALLOW' AND direction = 'OUT') = 0, 1, 0
				) AS missing_punch,
				multiIf(
					minIf(event_time, status = 'ALLOW' AND direction = 'IN') > toDateTime64(0, 3, 'UTC')
						AND maxIf(event_time, status = 'ALLOW' AND direction = 'OUT') > toDateTime64(0, 3, 'UTC'),
					dateDiff('second',
						minIf(event_time, status = 'ALLOW' AND direction = 'IN'),
						maxIf(event_time, status = 'ALLOW' AND direction = 'OUT')) / 3600.0,
					0.0
				) AS daily_hours
			FROM inout_events
			WHERE event_time >= toDateTime64(?, 3, 'UTC')
			  AND event_time < addDays(toDateTime64(?, 3, 'UTC'), 1)
			  AND org_unit_id IN (?)
			GROUP BY employee_id, day
		)
		GROUP BY employee_id
		ORDER BY total_swipes DESC`
	rows, err := r.chConn.Query(ctx, query, startDate, endDate, orgUUIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []model.EmployeeReportRow
	for rows.Next() {
		var row model.EmployeeReportRow
		var swipes, passback, perm, missing uint64
		if err := rows.Scan(&row.EmployeeID, &swipes, &row.TotalHours, &passback, &perm, &missing); err != nil {
			return nil, err
		}
		row.TotalSwipes = int(swipes)
		row.AntiPassbackDenies = int(passback)
		row.PermissionDenied = int(perm)
		row.MissingPunchDays = int(missing)
		results = append(results, row)
	}
	return results, rows.Err()
}

// Close closes the database connection.
func (r *chInOutRepository) Close() error {
	return r.chConn.Close()
}
