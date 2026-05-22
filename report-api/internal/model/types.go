package model

import "time"

// ────────────────────────────────────────
// Request types
// ────────────────────────────────────────

// PersonalReportRequest for GET /reports/personal
type PersonalReportRequest struct {
	StartDate string `form:"startDate" binding:"required"` // YYYY-MM-DD
	EndDate   string `form:"endDate" binding:"required"`   // YYYY-MM-DD
}

// DepartmentReportRequest for GET /reports/department
type DepartmentReportRequest struct {
	OrgUnitID   string `form:"orgUnitId" binding:"required,uuid"`
	StartDate   string `form:"startDate" binding:"required"`
	EndDate     string `form:"endDate" binding:"required"`
	Granularity string `form:"granularity"` // daily | weekly | monthly | quarterly | yearly
}

// AuditLogRequest for GET /reports/audit
type AuditLogRequest struct {
	StartDate  string `form:"startDate" binding:"required"`
	EndDate    string `form:"endDate" binding:"required"`
	EmployeeID string `form:"employeeId"` // optional filter
	DoorID     string `form:"doorId"`     // optional filter
	Status     string `form:"status"`     // ALLOW / DENY
	Page       int    `form:"page,default=1"`
	PageSize   int    `form:"pageSize,default=50"`
}

// ExportRequest for GET /reports/export and POST /reports/export/jobs
// type: events (default) | personal | department
type ExportRequest struct {
	Type        string `form:"type" json:"type"` // events | personal | department
	OrgUnitID   string `form:"orgUnitId" json:"orgUnitId"`
	StartDate   string `form:"startDate" json:"startDate" binding:"required"`
	EndDate     string `form:"endDate" json:"endDate" binding:"required"`
	Granularity string `form:"granularity" json:"granularity"`
	Format      string `form:"format" json:"format" binding:"required,oneof=csv pdf"`
}

// ────────────────────────────────────────
// Response types
// ────────────────────────────────────────

// PersonalReportResponse is the response for GET /reports/personal.
type PersonalReportResponse struct {
	UserID       string        `json:"userId"`
	StartDate    string        `json:"startDate"`
	EndDate      string        `json:"endDate"`
	TotalDays    int           `json:"totalDays"`
	DailyRecords []DailyRecord `json:"dailyRecords"`
}

// DailyRecord represents one day in a personal attendance report.
type DailyRecord struct {
	Date         string  `json:"date"`
	FirstIn      string  `json:"firstIn,omitempty"`
	LastOut      string  `json:"lastOut,omitempty"`
	TotalEntries int     `json:"totalEntries"`
	TotalExits   int     `json:"totalExits"`
	HoursWorked  float64 `json:"hoursWorked"`
}

// DepartmentReportResponse is the response for GET /reports/department.
type DepartmentReportResponse struct {
	OrgUnitID   string            `json:"orgUnitId"`
	OrgUnitName string            `json:"orgUnitName"`
	StartDate   string            `json:"startDate"`
	EndDate     string            `json:"endDate"`
	Granularity string            `json:"granularity"`
	Summary     DepartmentSummary `json:"summary"`
	Periods     []PeriodReport    `json:"periods"`
	SubUnits    []SubUnitSummary  `json:"subUnits,omitempty"`
}

// DepartmentSummary is the overall summary section of a department report.
type DepartmentSummary struct {
	TotalEntries           int     `json:"totalEntries"`
	TotalExits             int     `json:"totalExits"`
	UniqueEmployees        int     `json:"uniqueEmployees"`
	Headcount              int     `json:"headcount"`              // active employees in subtree
	WorkforceUtilization   float64 `json:"workforceUtilization"`   // unique present / headcount (0–1)
	AvgHoursPerDay         float64 `json:"avgHoursPerDay"`
	LateRate               float64 `json:"lateRate"` // fraction 0–1 (first ALLOW IN after 09:00 UTC)
}

// PeriodReport represents one period (day/week/month) in a department report.
type PeriodReport struct {
	PeriodStart     string  `json:"periodStart"`
	PeriodEnd       string  `json:"periodEnd"`
	TotalEntries    int     `json:"totalEntries"`
	TotalExits      int     `json:"totalExits"`
	UniqueEmployees int     `json:"uniqueEmployees"`
	AvgHours        float64 `json:"avgHours"`
	LateRate        float64 `json:"lateRate"`
}

// SubUnitSummary shows a child org unit's summary inside a department report.
type SubUnitSummary struct {
	OrgUnitID    string `json:"orgUnitId"`
	OrgUnitName  string `json:"orgUnitName"`
	TotalEntries int    `json:"totalEntries"`
	TotalExits   int    `json:"totalExits"`
}

// SecurityDenySummary counts DENY events in a date range for an org subtree.
type SecurityDenySummary struct {
	AntiPassbackDenies  int `json:"antiPassbackDenies"`
	PermissionDenied    int `json:"permissionDenied"` // banned / blacklist attempts
}

// EmployeeReportRow is per-employee stats for PDF detail tables.
type EmployeeReportRow struct {
	EmployeeID          string  `json:"employeeId"`
	EmployeeName        string  `json:"employeeName"`
	TotalSwipes         int     `json:"totalSwipes"`
	TotalHours          float64 `json:"totalHours"`
	AntiPassbackDenies  int     `json:"antiPassbackDenies"`
	PermissionDenied    int     `json:"permissionDenied"`
	MissingPunchDays    int     `json:"missingPunchDays"`
}

// ReportDetailRow is one line in the structured detail table (dept or employee).
type ReportDetailRow struct {
	EntityID     string
	EntityName   string
	EntityType   string // Department | Employee
	TotalSwipes  int
	TotalHours   float64
	AnomalyNotes string
}

// AuditLogResponse is the response for GET /reports/audit.
type AuditLogResponse struct {
	Events     []AuditEvent `json:"events"`
	Page       int          `json:"page"`
	PageSize   int          `json:"pageSize"`
	TotalCount int          `json:"totalCount"`
}

// AuditEvent represents a single raw inout event in the audit log.
type AuditEvent struct {
	EventID    string `json:"eventId"`
	EmployeeID string `json:"employeeId"`
	DoorID     string `json:"doorId"`
	Direction  string `json:"direction"`
	EventTime  string `json:"eventTime"`
	Status     string `json:"status"`
	Reason     string `json:"reason,omitempty"`
	SourceIP   string `json:"sourceIp,omitempty"`
}

// DoorHeatmapRequest for GET /reports/analytics/door-heatmap
type DoorHeatmapRequest struct {
	OrgUnitID string `form:"orgUnitId"` // optional; defaults to requester's org
	Minutes   int    `form:"minutes"`   // default 60
}

// WorkforceUtilizationRequest for GET /reports/analytics/workforce-utilization
type WorkforceUtilizationRequest struct {
	OrgUnitID string `form:"orgUnitId" binding:"required,uuid"`
	StartDate string `form:"startDate" binding:"required"`
	EndDate   string `form:"endDate" binding:"required"`
}

// WorkforceUtilizationResponse reports workforce utilization for an org subtree.
type WorkforceUtilizationResponse struct {
	OrgUnitID            string  `json:"orgUnitId"`
	OrgUnitName          string  `json:"orgUnitName"`
	StartDate            string  `json:"startDate"`
	EndDate              string  `json:"endDate"`
	Headcount            int     `json:"headcount"`
	UniquePresent        int     `json:"uniquePresent"`
	WorkforceUtilization float64 `json:"workforceUtilization"`
	OnSiteNow            int     `json:"onSiteNow"`
	OnSiteRate           float64 `json:"onSiteRate"`
}

// DoorHeatmapResponse lists doors ranked by swipe volume.
type DoorHeatmapResponse struct {
	WindowMinutes int              `json:"windowMinutes"`
	Doors         []DoorTrafficRow `json:"doors"`
}

// DoorTrafficRow is one door in the heatmap ranking.
type DoorTrafficRow struct {
	DoorID     string `json:"doorId"`
	DoorName   string `json:"doorName"`
	Site       string `json:"site"`
	SwipeCount uint64 `json:"swipeCount"`
}

// AttendanceTrendsRequest for GET /reports/analytics/attendance-trends
type AttendanceTrendsRequest struct {
	OrgUnitID   string `form:"orgUnitId" binding:"required,uuid"`
	StartDate   string `form:"startDate" binding:"required"`
	EndDate     string `form:"endDate" binding:"required"`
	Granularity string `form:"granularity"` // daily | weekly | monthly | quarterly | yearly
}

// AttendanceTrendsResponse is time-series data for charts (avg hours + late rate).
type AttendanceTrendsResponse struct {
	OrgUnitID   string             `json:"orgUnitId"`
	OrgUnitName string             `json:"orgUnitName"`
	StartDate   string             `json:"startDate"`
	EndDate     string             `json:"endDate"`
	Granularity string             `json:"granularity"`
	Series      []AttendancePoint  `json:"series"`
}

// AttendancePoint is one point on the attendance trend chart.
type AttendancePoint struct {
	PeriodStart string  `json:"periodStart"`
	PeriodEnd   string  `json:"periodEnd"`
	AvgHours    float64 `json:"avgHours"`
	LateRate    float64 `json:"lateRate"`
}

// ────────────────────────────────────────
// DB entity types
// ────────────────────────────────────────

// OrgUnit represents a row in the org_unit table.
type OrgUnit struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	ParentID         string `json:"parentId,omitempty"`
	Depth            int    `json:"depth"`
	MaterializedPath string `json:"materializedPath"`
}

// AggregatedRow represents a row from pre_aggregated_reports.
type AggregatedRow struct {
	OrgUnitID       string  `json:"orgUnitId"`
	ReportDate      string  `json:"reportDate"`
	TotalEntries    int     `json:"totalEntries"`
	TotalExits      int     `json:"totalExits"`
	UniqueEmployees int     `json:"uniqueEmployees"`
	AvgHours        float64 `json:"avgHours"`
}

// InOutEvent represents a row from inout_events (for audit / personal report).
type InOutEvent struct {
	EventID    string    `json:"eventId"`
	EmployeeID string    `json:"employeeId"`
	DoorID     string    `json:"doorId"`
	Direction  string    `json:"direction"`
	EventTime  time.Time `json:"eventTime"`
	Status     string    `json:"status"`
	Reason     *string   `json:"reason"`
	CardUID    string    `json:"cardUid"`
	SourceIP   string    `json:"sourceIp"`
}
