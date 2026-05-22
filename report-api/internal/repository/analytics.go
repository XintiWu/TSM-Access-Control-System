package repository

import (
	"context"
	"time"

	"github.com/tsmc/report-api/internal/model"
)

// DoorHeatmapRow is swipe volume per door for a time window.
type DoorHeatmapRow struct {
	DoorID     string
	DoorName   string
	Site       string
	SwipeCount uint64
}

// GetDoorHeatmap returns per-door swipe counts in the last `minutes` (live + pre-aggregated minute buckets).
func (r *ReportRepository) GetDoorHeatmap(ctx context.Context, minutes int) ([]DoorHeatmapRow, error) {
	if minutes < 1 {
		minutes = 60
	}
	since := time.Now().UTC().Add(-time.Duration(minutes) * time.Minute)

	query := `
		SELECT
			toString(e.door_id) AS door_id,
			any(d.name) AS door_name,
			any(d.site) AS site,
			count() AS swipe_count
		FROM inout_events AS e
		LEFT JOIN access_control.door AS d ON e.door_id = d.id
		WHERE e.event_time >= ?
		GROUP BY e.door_id
		ORDER BY swipe_count DESC`

	rows, err := r.chConn.Query(ctx, query, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DoorHeatmapRow
	for rows.Next() {
		var row DoorHeatmapRow
		if err := rows.Scan(&row.DoorID, &row.DoorName, &row.Site, &row.SwipeCount); err != nil {
			return nil, err
		}
		if row.DoorName == "" {
			row.DoorName = row.DoorID
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// PeriodAttendanceMetrics holds avg hours and late rate for one period.
type PeriodAttendanceMetrics struct {
	PeriodStart string
	PeriodEnd   string
	AvgHours    float64
	LateRate    float64
	LateCount   int
	Headcount   int
}

// GetAttendanceTrends computes avg hours and late rate (first ALLOW IN after 09:00 UTC) per calendar day.
func (r *ReportRepository) GetAttendanceTrends(ctx context.Context, orgUnitIDs []string, startDate, endDate string) ([]PeriodAttendanceMetrics, error) {
	if len(orgUnitIDs) == 0 {
		return nil, nil
	}
	query := `
		SELECT
			toString(day) AS period_start,
			toString(day) AS period_end,
			COALESCE(avg(daily_hours), 0) AS avg_hours,
			if(count() > 0, sum(is_late) / count(), 0) AS late_rate,
			toInt32(sum(is_late)) AS late_count,
			toInt32(count()) AS headcount
		FROM (
			SELECT
				employee_id,
				toDate(event_time) AS day,
				multiIf(
					minIf(event_time, direction = 'IN') > toDateTime64(0, 3, 'UTC')
						AND maxIf(event_time, direction = 'OUT') > toDateTime64(0, 3, 'UTC'),
					dateDiff('second',
						minIf(event_time, direction = 'IN'),
						maxIf(event_time, direction = 'OUT')) / 3600.0,
					NULL
				) AS daily_hours,
				if(
					minIf(event_time, direction = 'IN' AND status = 'ALLOW')
						> toDateTime64(concat(toString(toDate(event_time)), ' 09:00:00'), 3, 'UTC'),
					1, 0
				) AS is_late
			FROM inout_events
			WHERE toString(org_unit_id) IN (?)
			  AND event_time >= toDateTime64(?, 3, 'UTC')
			  AND event_time < addDays(toDateTime64(?, 3, 'UTC'), 1)
			  AND status = 'ALLOW'
			GROUP BY employee_id, day
		)
		GROUP BY day
		ORDER BY day ASC`

	rows, err := r.chConn.Query(ctx, query, orgUnitIDs, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []PeriodAttendanceMetrics
	for rows.Next() {
		var m PeriodAttendanceMetrics
		var lateCount, headcount int32
		if err := rows.Scan(&m.PeriodStart, &m.PeriodEnd, &m.AvgHours, &m.LateRate, &lateCount, &headcount); err != nil {
			return nil, err
		}
		m.LateCount = int(lateCount)
		m.Headcount = int(headcount)
		results = append(results, m)
	}
	return results, rows.Err()
}

// PassbackSpikeRow is used for security alerting metrics.
type PassbackSpikeRow struct {
	DoorID   string
	DoorName string
	Count    uint64
}

// GetPassbackDenyCountsLastMinute returns ANTI_PASSBACK deny counts per door in the last minute.
func (r *ReportRepository) GetPassbackDenyCountsLastMinute(ctx context.Context) ([]PassbackSpikeRow, error) {
	since := time.Now().UTC().Add(-time.Minute)
	query := `
		SELECT
			toString(e.door_id) AS door_id,
			any(d.name) AS door_name,
			count() AS cnt
		FROM inout_events AS e
		LEFT JOIN access_control.door AS d ON e.door_id = d.id
		WHERE e.status = 'DENY'
		  AND e.reason = 'ANTI_PASSBACK'
		  AND e.event_time >= ?
		GROUP BY e.door_id`

	rows, err := r.chConn.Query(ctx, query, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PassbackSpikeRow
	for rows.Next() {
		var row PassbackSpikeRow
		if err := rows.Scan(&row.DoorID, &row.DoorName, &row.Count); err != nil {
			return nil, err
		}
		if row.DoorName == "" {
			row.DoorName = row.DoorID
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// MergeTrendsIntoPeriods attaches late rate and avg hours from daily trends into period reports.
func MergeTrendsIntoPeriods(periods []model.PeriodReport, daily []PeriodAttendanceMetrics, granularity string) []model.PeriodReport {
	if len(periods) == 0 || len(daily) == 0 {
		return periods
	}
	dayMap := make(map[string]PeriodAttendanceMetrics, len(daily))
	for _, d := range daily {
		dayMap[d.PeriodStart] = d
	}

	for i := range periods {
		p := &periods[i]
		switch granularity {
		case "monthly":
			var sumLate, sumHead int
			var sumHours float64
			var days int
			start, _ := time.Parse("2006-01-02", p.PeriodStart)
			end, _ := time.Parse("2006-01-02", p.PeriodEnd)
			for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
				key := d.Format("2006-01-02")
				if m, ok := dayMap[key]; ok {
					sumLate += m.LateCount
					sumHead += m.Headcount
					sumHours += m.AvgHours
					days++
				}
			}
			if sumHead > 0 {
				p.LateRate = float64(sumLate) / float64(sumHead)
			}
			if days > 0 {
				p.AvgHours = sumHours / float64(days)
			}
		case "weekly":
			var sumLate, sumHead int
			var sumHours float64
			var days int
			start, _ := time.Parse("2006-01-02", p.PeriodStart)
			end, _ := time.Parse("2006-01-02", p.PeriodEnd)
			for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
				key := d.Format("2006-01-02")
				if m, ok := dayMap[key]; ok {
					sumLate += m.LateCount
					sumHead += m.Headcount
					sumHours += m.AvgHours
					days++
				}
			}
			if sumHead > 0 {
				p.LateRate = float64(sumLate) / float64(sumHead)
			}
			if days > 0 {
				p.AvgHours = sumHours / float64(days)
			}
		default:
			if m, ok := dayMap[p.PeriodStart]; ok {
				p.LateRate = m.LateRate
				p.AvgHours = m.AvgHours
			}
		}
	}
	return periods
}

// MaxPassbackCount returns the highest per-door ANTI_PASSBACK count in the last minute.
func MaxPassbackCount(rows []PassbackSpikeRow) uint64 {
	var max uint64
	for _, r := range rows {
		if r.Count > max {
			max = r.Count
		}
	}
	return max
}
