package repository

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
	"github.com/tsmc/report-api/internal/model"
)

// ReportRepository handles queries for aggregated reports using ClickHouse.
type ReportRepository struct {
	chConn clickhouse.Conn
}

// NewReportRepository opens a ClickHouse native TCP connection.
func NewReportRepository(chAddr, chUser, chPass string) (*ReportRepository, error) {
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
	return &ReportRepository{chConn: chConn}, nil
}

// GetAggregated reads pre-aggregated reports from ClickHouse dynamically.
func (r *ReportRepository) GetAggregated(ctx context.Context, orgUnitIDs []string, startDate, endDate string) ([]model.AggregatedRow, error) {
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
			toString(org_unit_id) AS org_unit_id,
			toString(date_val) AS report_date,
			toUInt64(sum(num_ins)) AS total_entries,
			toUInt64(sum(num_outs)) AS total_exits,
			toUInt64(uniq(employee_id)) AS unique_employees,
			COALESCE(avg(daily_hours), 0.0) AS avg_hours
		FROM (
			SELECT
				employee_id,
				org_unit_id,
				toDate(event_time) AS date_val,
				countIf(direction = 'IN') AS num_ins,
				countIf(direction = 'OUT') AS num_outs,
				multiIf(
					minIf(event_time, direction = 'IN') > toDateTime64(0, 3, 'UTC') AND maxIf(event_time, direction = 'OUT') > toDateTime64(0, 3, 'UTC'),
					dateDiff('second', minIf(event_time, direction = 'IN'), maxIf(event_time, direction = 'OUT')) / 3600.0,
					NULL
				) AS daily_hours
			FROM inout_events
			WHERE org_unit_id IN (?)
			  AND event_time >= toDateTime64(?, 3, 'UTC')
			  AND event_time < addDays(toDateTime64(?, 3, 'UTC'), 1)
			  AND status = 'ALLOW'
			GROUP BY employee_id, org_unit_id, date_val
		)
		GROUP BY org_unit_id, date_val
		ORDER BY date_val ASC`

	rows, err := r.chConn.Query(ctx, query, orgUUIDs, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.AggregatedRow
	for rows.Next() {
		var row model.AggregatedRow
		var totalEntries, totalExits, uniqueEmployees uint64
		if err := rows.Scan(&row.OrgUnitID, &row.ReportDate, &totalEntries,
			&totalExits, &uniqueEmployees, &row.AvgHours); err != nil {
			return nil, err
		}
		row.TotalEntries = int(totalEntries)
		row.TotalExits = int(totalExits)
		row.UniqueEmployees = int(uniqueEmployees)
		results = append(results, row)
	}
	return results, rows.Err()
}

// GetSummary returns a single aggregated summary across all org units and the entire date range from ClickHouse.
func (r *ReportRepository) GetSummary(ctx context.Context, orgUnitIDs []string, startDate, endDate string) (model.DepartmentSummary, error) {
	var summary model.DepartmentSummary
	if len(orgUnitIDs) == 0 {
		return summary, nil
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
			toUInt64(sum(total_entries)),
			toUInt64(sum(total_exits)),
			toUInt64(uniq(employee_id)),
			COALESCE(avg(daily_hours), 0.0) AS avg_hours
		FROM (
			SELECT
				employee_id,
				toDate(event_time) AS date_val,
				countIf(direction = 'IN') AS total_entries,
				countIf(direction = 'OUT') AS total_exits,
				multiIf(
					minIf(event_time, direction = 'IN') > toDateTime64(0, 3, 'UTC') AND maxIf(event_time, direction = 'OUT') > toDateTime64(0, 3, 'UTC'),
					dateDiff('second', minIf(event_time, direction = 'IN'), maxIf(event_time, direction = 'OUT')) / 3600.0,
					NULL
				) AS daily_hours
			FROM inout_events
			WHERE org_unit_id IN (?)
			  AND event_time >= toDateTime64(?, 3, 'UTC')
			  AND event_time < addDays(toDateTime64(?, 3, 'UTC'), 1)
			  AND status = 'ALLOW'
			GROUP BY employee_id, date_val
		)`

	var totalEntries, totalExits, uniqueEmployees uint64
	var avgHours float64

	err := r.chConn.QueryRow(ctx, query, orgUUIDs, startDate, endDate).Scan(
		&totalEntries, &totalExits, &uniqueEmployees, &avgHours)
	if err != nil {
		return summary, err
	}

	summary.TotalEntries = int(totalEntries)
	summary.TotalExits = int(totalExits)
	summary.UniqueEmployees = int(uniqueEmployees)
	summary.AvgHoursPerDay = avgHours

	return summary, nil
}

// Close closes the database connection.
func (r *ReportRepository) Close() error {
	return r.chConn.Close()
}
