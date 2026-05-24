package repository

import (
	"context"
	"crypto/tls"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/tsmc/report-api/internal/model"
)

// ReportRepository handles queries for aggregated reports using ClickHouse.
type ReportRepository struct {
	chConn clickhouse.Conn
}

// NewReportRepository opens a ClickHouse native TCP connection.
func NewReportRepository(chAddr, chUser, chPass string) (*ReportRepository, error) {
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
	return &ReportRepository{chConn: chConn}, nil
}

// GetAggregated reads pre_aggregated_reports (MV-backed) and enriches with live avg hours per day.
func (r *ReportRepository) GetAggregated(ctx context.Context, orgUnitIDs []string, startDate, endDate string) ([]model.AggregatedRow, error) {
	if len(orgUnitIDs) == 0 {
		return nil, nil
	}

	query := `
		SELECT
			'' AS org_unit_id,
			toString(report_date) AS report_date,
			toUInt64(sum(total_entries)) AS total_entries,
			toUInt64(sum(total_exits)) AS total_exits,
			toUInt64(uniqMerge(unique_employees)) AS unique_employees
		FROM pre_aggregated_reports
		WHERE toString(org_unit_id) IN (?)
		  AND report_date >= toDate(?)
		  AND report_date <= toDate(?)
		GROUP BY report_date
		ORDER BY report_date ASC`

	rows, err := r.chConn.Query(ctx, query, orgUnitIDs, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dayTotals := make(map[string]*model.AggregatedRow)
	var order []string
	for rows.Next() {
		var row model.AggregatedRow
		var totalEntries, totalExits, uniqueEmployees uint64
		if err := rows.Scan(&row.OrgUnitID, &row.ReportDate, &totalEntries,
			&totalExits, &uniqueEmployees); err != nil {
			return nil, err
		}
		row.TotalEntries = int(totalEntries)
		row.TotalExits = int(totalExits)
		row.UniqueEmployees = int(uniqueEmployees)

		if existing, ok := dayTotals[row.ReportDate]; ok {
			existing.TotalEntries += row.TotalEntries
			existing.TotalExits += row.TotalExits
			existing.UniqueEmployees += row.UniqueEmployees
		} else {
			copy := row
			dayTotals[row.ReportDate] = &copy
			order = append(order, row.ReportDate)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(order) == 0 {
		return r.getAggregatedLive(ctx, orgUnitIDs, startDate, endDate)
	}

	trends, err := r.GetAttendanceTrends(ctx, orgUnitIDs, startDate, endDate)
	if err != nil {
		return nil, err
	}
	trendMap := make(map[string]float64, len(trends))
	for _, t := range trends {
		trendMap[t.PeriodStart] = t.AvgHours
	}

	var results []model.AggregatedRow
	for _, d := range order {
		row := dayTotals[d]
		if h, ok := trendMap[d]; ok {
			row.AvgHours = h
		}
		results = append(results, *row)
	}
	return results, nil
}

// GetSummary returns aggregated summary using pre_aggregated_reports plus live avg hours / late rate.
func (r *ReportRepository) GetSummary(ctx context.Context, orgUnitIDs []string, startDate, endDate string) (model.DepartmentSummary, error) {
	var summary model.DepartmentSummary
	if len(orgUnitIDs) == 0 {
		return summary, nil
	}

	query := `
		SELECT
			toUInt64(sum(total_entries)),
			toUInt64(sum(total_exits)),
			toUInt64(uniqMerge(unique_employees))
		FROM pre_aggregated_reports
		WHERE toString(org_unit_id) IN (?)
		  AND report_date >= toDate(?)
		  AND report_date <= toDate(?)`

	var totalEntries, totalExits, uniqueEmployees uint64
	err := r.chConn.QueryRow(ctx, query, orgUnitIDs, startDate, endDate).Scan(
		&totalEntries, &totalExits, &uniqueEmployees)
	if err != nil {
		return summary, err
	}

	if totalEntries == 0 && totalExits == 0 {
		liveRows, lerr := r.getAggregatedLive(ctx, orgUnitIDs, startDate, endDate)
		if lerr == nil && len(liveRows) > 0 {
			for _, row := range liveRows {
				summary.TotalEntries += row.TotalEntries
				summary.TotalExits += row.TotalExits
				summary.UniqueEmployees += row.UniqueEmployees
			}
		}
	} else {
		summary.TotalEntries = int(totalEntries)
		summary.TotalExits = int(totalExits)
		summary.UniqueEmployees = int(uniqueEmployees)
	}

	trends, err := r.GetAttendanceTrends(ctx, orgUnitIDs, startDate, endDate)
	if err != nil {
		return summary, err
	}
	var sumHours float64
	var sumLate, sumHead int
	for _, t := range trends {
		sumHours += t.AvgHours
		sumLate += t.LateCount
		sumHead += t.Headcount
	}
	if len(trends) > 0 {
		summary.AvgHoursPerDay = sumHours / float64(len(trends))
	}
	if sumHead > 0 {
		summary.LateRate = float64(sumLate) / float64(sumHead)
	}

	return summary, nil
}

// GetOnSiteCount returns employees whose latest ALLOW swipe in the subtree is IN.
func (r *ReportRepository) GetOnSiteCount(ctx context.Context, orgUnitIDs []string) (int, error) {
	if len(orgUnitIDs) == 0 {
		return 0, nil
	}
	var count uint64
	err := r.chConn.QueryRow(ctx, `
		SELECT count()
		FROM (
			SELECT employee_id, argMax(direction, event_time) AS last_dir
			FROM inout_events
			WHERE toString(org_unit_id) IN (?) AND status = 'ALLOW'
			GROUP BY employee_id
		)
		WHERE last_dir = 'IN'`, orgUnitIDs).Scan(&count)
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// getAggregatedLive scans inout_events when pre_aggregated_reports has no rows yet.
func (r *ReportRepository) getAggregatedLive(ctx context.Context, orgUnitIDs []string, startDate, endDate string) ([]model.AggregatedRow, error) {
	query := `
		SELECT
			'' AS org_unit_id,
			toString(date_val) AS report_date,
			toUInt64(sum(num_ins)) AS total_entries,
			toUInt64(sum(num_outs)) AS total_exits,
			toUInt64(uniq(employee_id)) AS unique_employees,
			COALESCE(avg(daily_hours), 0.0) AS avg_hours
		FROM (
			SELECT
				employee_id,
				toDate(event_time) AS date_val,
				countIf(direction = 'IN') AS num_ins,
				countIf(direction = 'OUT') AS num_outs,
				multiIf(
					minIf(event_time, direction = 'IN') > toDateTime64(0, 3, 'UTC') AND maxIf(event_time, direction = 'OUT') > toDateTime64(0, 3, 'UTC'),
					dateDiff('second', minIf(event_time, direction = 'IN'), maxIf(event_time, direction = 'OUT')) / 3600.0,
					NULL
				) AS daily_hours
			FROM inout_events
			WHERE toString(org_unit_id) IN (?)
			  AND event_time >= toDateTime64(?, 3, 'UTC')
			  AND event_time < addDays(toDateTime64(?, 3, 'UTC'), 1)
			  AND status = 'ALLOW'
			GROUP BY employee_id, date_val
		)
		GROUP BY date_val
		ORDER BY date_val ASC`

	rows, err := r.chConn.Query(ctx, query, orgUnitIDs, startDate, endDate)
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

// Close closes the database connection.
func (r *ReportRepository) Close() error {
	return r.chConn.Close()
}
