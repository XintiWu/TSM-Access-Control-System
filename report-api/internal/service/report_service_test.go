package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tsmc/report-api/internal/auth"
	"github.com/tsmc/report-api/internal/export"
	"github.com/tsmc/report-api/internal/model"
	"github.com/tsmc/report-api/internal/repository"
)

func TestCalcUtilization_Detailed(t *testing.T) {
	tests := []struct {
		name          string
		uniquePresent int
		headcount     int
		want          float64
	}{
		{"negative headcount", 5, -1, 0},
		{"normal 50pct", 50, 100, 0.5},
		{"zero present", 0, 100, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calcUtilization(tt.uniquePresent, tt.headcount)
			if got != tt.want {
				t.Errorf("calcUtilization(%d, %d) = %f, want %f", tt.uniquePresent, tt.headcount, got, tt.want)
			}
		})
	}
}

func TestBuildPeriods_Daily(t *testing.T) {
	rows := []model.AggregatedRow{
		{ReportDate: "2026-01-01", TotalEntries: 10, TotalExits: 8},
		{ReportDate: "2026-01-02", TotalEntries: 15, TotalExits: 12},
	}
	periods := buildPeriods(rows, "daily", "2026-01-01", "2026-01-02")
	if len(periods) != 2 {
		t.Fatalf("expected 2 periods, got %d", len(periods))
	}
	if periods[0].TotalEntries != 10 {
		t.Errorf("period[0].TotalEntries = %d, want 10", periods[0].TotalEntries)
	}
}

func TestBuildPeriods_Empty(t *testing.T) {
	periods := buildPeriods(nil, "daily", "2026-01-01", "2026-01-02")
	if periods != nil {
		t.Fatalf("expected nil, got %v", periods)
	}
}

func TestBuildPeriods_Weekly(t *testing.T) {
	rows := []model.AggregatedRow{
		{ReportDate: "2026-01-01", TotalEntries: 10, TotalExits: 8},
		{ReportDate: "2026-01-03", TotalEntries: 5, TotalExits: 4},
		{ReportDate: "2026-01-08", TotalEntries: 20, TotalExits: 18},
	}
	periods := buildPeriods(rows, "weekly", "2026-01-01", "2026-01-14")
	if len(periods) == 0 {
		t.Fatal("expected non-empty periods for weekly")
	}
	if periods[0].TotalEntries != 15 {
		t.Errorf("week1 TotalEntries = %d, want 15", periods[0].TotalEntries)
	}
}

func TestBuildPeriods_Monthly(t *testing.T) {
	rows := []model.AggregatedRow{
		{ReportDate: "2026-01-05", TotalEntries: 10},
		{ReportDate: "2026-01-15", TotalEntries: 20},
		{ReportDate: "2026-02-01", TotalEntries: 5},
	}
	periods := buildPeriods(rows, "monthly", "2026-01-01", "2026-02-28")
	if len(periods) != 2 {
		t.Fatalf("expected 2 monthly periods, got %d", len(periods))
	}
	if periods[0].TotalEntries != 30 {
		t.Errorf("jan TotalEntries = %d, want 30", periods[0].TotalEntries)
	}
}

func TestGroupByDay_MergesSameDate(t *testing.T) {
	rows := []model.AggregatedRow{
		{ReportDate: "2026-01-01", TotalEntries: 5, TotalExits: 3, UniqueEmployees: 2},
		{ReportDate: "2026-01-01", TotalEntries: 10, TotalExits: 7, UniqueEmployees: 3},
	}
	periods := groupByDay(rows)
	if len(periods) != 1 {
		t.Fatalf("expected 1 merged period, got %d", len(periods))
	}
	if periods[0].TotalEntries != 15 {
		t.Errorf("merged TotalEntries = %d, want 15", periods[0].TotalEntries)
	}
}

func TestGroupByWeek_InvalidDate(t *testing.T) {
	result := groupByWeek(nil, "invalid", "also-invalid")
	if result != nil {
		t.Errorf("expected nil for invalid date, got %v", result)
	}
}

func TestGroupByMonth_InvalidRowDate(t *testing.T) {
	rows := []model.AggregatedRow{
		{ReportDate: "not-a-date", TotalEntries: 10},
	}
	result := groupByMonth(rows, "2026-01-01", "2026-01-31")
	if len(result) != 0 {
		t.Errorf("expected 0 periods for invalid row date, got %d", len(result))
	}
}

func TestAccessDeniedError(t *testing.T) {
	err := NewAccessDeniedError("test message")
	if err.Error() != "test message" {
		t.Errorf("Error() = %q, want %q", err.Error(), "test message")
	}

	ade, ok := err.(*AccessDeniedError)
	if !ok {
		t.Fatal("expected *AccessDeniedError type")
	}
	if !ade.Is(ErrAccessDenied) {
		t.Error("expected Is(ErrAccessDenied) to return true")
	}
}

func TestCalcUtilization_CapAtOne(t *testing.T) {
	got := calcUtilization(200, 100)
	if got != 1 {
		t.Errorf("calcUtilization(200, 100) = %f, want 1.0", got)
	}
}

func TestCalcUtilization_ZeroHeadcount(t *testing.T) {
	got := calcUtilization(10, 0)
	if got != 0 {
		t.Errorf("calcUtilization(10, 0) = %f, want 0", got)
	}
}

func TestCalcUtilization_Normal(t *testing.T) {
	got := calcUtilization(3, 4)
	if got != 0.75 {
		t.Errorf("calcUtilization(3, 4) = %f, want 0.75", got)
	}
}

func TestBuildPeriods_QuarterlyMore(t *testing.T) {
	rows := []model.AggregatedRow{
		{ReportDate: "2026-01-15", TotalEntries: 10},
		{ReportDate: "2026-02-10", TotalEntries: 20},
		{ReportDate: "2026-04-01", TotalEntries: 5},
	}
	periods := buildPeriods(rows, "quarterly", "2026-01-01", "2026-06-30")
	if len(periods) != 2 {
		t.Fatalf("expected 2 quarterly periods, got %d", len(periods))
	}
	if periods[0].TotalEntries != 30 {
		t.Errorf("Q1 TotalEntries = %d, want 30", periods[0].TotalEntries)
	}
	if periods[1].TotalEntries != 5 {
		t.Errorf("Q2 TotalEntries = %d, want 5", periods[1].TotalEntries)
	}
}

func TestBuildPeriods_YearlyMore(t *testing.T) {
	rows := []model.AggregatedRow{
		{ReportDate: "2025-06-15", TotalEntries: 100},
		{ReportDate: "2026-01-01", TotalEntries: 50},
		{ReportDate: "2026-06-01", TotalEntries: 30},
	}
	periods := buildPeriods(rows, "yearly", "2025-01-01", "2026-12-31")
	if len(periods) != 2 {
		t.Fatalf("expected 2 yearly periods, got %d", len(periods))
	}
	if periods[0].TotalEntries != 100 {
		t.Errorf("2025 TotalEntries = %d, want 100", periods[0].TotalEntries)
	}
	if periods[1].TotalEntries != 80 {
		t.Errorf("2026 TotalEntries = %d, want 80", periods[1].TotalEntries)
	}
}

func TestGroupByQuarter_InvalidDate(t *testing.T) {
	rows := []model.AggregatedRow{
		{ReportDate: "not-a-date", TotalEntries: 10},
	}
	result := groupByQuarter(rows, "2026-01-01", "2026-12-31")
	if len(result) != 0 {
		t.Errorf("expected 0 periods for invalid row date, got %d", len(result))
	}
}

func TestGroupByYear_InvalidDate(t *testing.T) {
	rows := []model.AggregatedRow{
		{ReportDate: "bad-date", TotalEntries: 10},
	}
	result := groupByYear(rows, "2026-01-01", "2026-12-31")
	if len(result) != 0 {
		t.Errorf("expected 0 periods for invalid row date, got %d", len(result))
	}
}

func TestBuildPeriods_DefaultDaily(t *testing.T) {
	rows := []model.AggregatedRow{
		{ReportDate: "2026-01-01", TotalEntries: 10},
	}
	periods := buildPeriods(rows, "unknown-granularity", "2026-01-01", "2026-01-01")
	if len(periods) != 1 {
		t.Fatalf("expected 1 daily period for unknown granularity, got %d", len(periods))
	}
}

func TestGroupByDay_Order(t *testing.T) {
	rows := []model.AggregatedRow{
		{ReportDate: "2026-01-03", TotalEntries: 3},
		{ReportDate: "2026-01-01", TotalEntries: 1},
		{ReportDate: "2026-01-02", TotalEntries: 2},
	}
	periods := groupByDay(rows)
	if len(periods) != 3 {
		t.Fatalf("expected 3 periods, got %d", len(periods))
	}
	// Order should match insertion order
	if periods[0].PeriodStart != "2026-01-03" {
		t.Errorf("first period = %s, want 2026-01-03", periods[0].PeriodStart)
	}
}

func TestGroupByDay_AvgHours(t *testing.T) {
	rows := []model.AggregatedRow{
		{ReportDate: "2026-01-01", TotalEntries: 5, AvgHours: 8.5},
		{ReportDate: "2026-01-01", TotalEntries: 3, AvgHours: 7.0},
	}
	periods := groupByDay(rows)
	if len(periods) != 1 {
		t.Fatalf("expected 1 merged period, got %d", len(periods))
	}
	if periods[0].AvgHours != 7.0 {
		t.Errorf("AvgHours = %f, want 7.0 (last wins)", periods[0].AvgHours)
	}
}

type auditInOutRepo struct {
	getAuditEventsFn func(ctx context.Context, f repository.AuditFilter) ([]model.InOutEvent, int, error)
}
func (m *auditInOutRepo) GetPersonalEvents(ctx context.Context, employeeID, startDate, endDate string) ([]model.InOutEvent, error) { return nil, nil }
func (m *auditInOutRepo) GetAuditEvents(ctx context.Context, f repository.AuditFilter) ([]model.InOutEvent, int, error) {
	if m.getAuditEventsFn != nil {
		return m.getAuditEventsFn(ctx, f)
	}
	return nil, 0, nil
}
func (m *auditInOutRepo) GetEventsForExport(ctx context.Context, orgUnitIDs []string, startDate, endDate string) ([]model.InOutEvent, error) { return nil, nil }
func (m *auditInOutRepo) GetSecurityDenySummary(ctx context.Context, orgUnitIDs []string, startDate, endDate string) (model.SecurityDenySummary, error) { return model.SecurityDenySummary{}, nil }
func (m *auditInOutRepo) GetEmployeeReportRows(ctx context.Context, orgUnitIDs []string, startDate, endDate string) ([]model.EmployeeReportRow, error) { return nil, nil }
func (m *auditInOutRepo) Close() error { return nil }

func TestGetAuditLog_Director(t *testing.T) {
	reason := "some reason"
	mockInOut := &auditInOutRepo{
		getAuditEventsFn: func(ctx context.Context, f repository.AuditFilter) ([]model.InOutEvent, int, error) {
			if f.EmployeeID != "" {
				t.Errorf("expected no EmployeeID filter for director, got %q", f.EmployeeID)
			}
			return []model.InOutEvent{
				{EventID: "evt-1", EmployeeID: "emp-1", DoorID: "door-1", Direction: "IN", EventTime: time.Now(), Status: "ALLOW", Reason: &reason, SourceIP: "1.2.3.4"},
			}, 1, nil
		},
	}
	svc := NewReportService(exportOrgRepo{}, exportReportRepo{}, mockInOut, nil, nil)
	req := model.AuditLogRequest{
		StartDate: "2026-05-01",
		EndDate:   "2026-05-30",
	}
	resp, err := svc.GetAuditLog(context.Background(), req, "dir-1", uuid.New().String(), auth.RoleDirector)
	if err != nil {
		t.Fatalf("GetAuditLog: %v", err)
	}
	if len(resp.Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(resp.Events))
	}
	if resp.Events[0].Reason != "some reason" {
		t.Errorf("expected reason 'some reason', got %q", resp.Events[0].Reason)
	}
}

func TestGetAuditLog_Employee(t *testing.T) {
	mockInOut := &auditInOutRepo{
		getAuditEventsFn: func(ctx context.Context, f repository.AuditFilter) ([]model.InOutEvent, int, error) {
			if f.EmployeeID != "emp-1" {
				t.Errorf("expected EmployeeID to be filtered to emp-1, got %q", f.EmployeeID)
			}
			return nil, 0, nil
		},
	}
	svc := NewReportService(exportOrgRepo{}, exportReportRepo{}, mockInOut, nil, nil)
	req := model.AuditLogRequest{
		StartDate: "2026-05-01",
		EndDate:   "2026-05-30",
	}
	_, err := svc.GetAuditLog(context.Background(), req, "emp-1", uuid.New().String(), auth.RoleEmployee)
	if err != nil {
		t.Fatalf("GetAuditLog: %v", err)
	}
}

func TestExportSync_CSV(t *testing.T) {
	inout := &exportInOutRepo{
		getPersonalEventsFn: func(ctx context.Context, employeeID, startDate, endDate string) ([]model.InOutEvent, error) {
			return []model.InOutEvent{{EmployeeID: employeeID, Direction: "IN"}}, nil
		},
	}
	svc := NewReportService(exportOrgRepo{}, exportReportRepo{}, inout, nil, nil)
	req := model.ExportRequest{
		Type:      "personal",
		StartDate: "2026-05-01",
		EndDate:   "2026-05-30",
		Format:    "csv",
	}
	data, ext, err := svc.ExportSync(context.Background(), req, "emp-1", uuid.New().String(), auth.RoleEmployee)
	if err != nil {
		t.Fatalf("ExportSync CSV: %v", err)
	}
	if ext != ".csv" {
		t.Errorf("expected .csv extension, got %q", ext)
	}
	if len(data) == 0 {
		t.Error("expected non-empty CSV data")
	}
}

func TestRunExportJob(t *testing.T) {
	inout := &exportInOutRepo{
		getPersonalEventsFn: func(ctx context.Context, employeeID, startDate, endDate string) ([]model.InOutEvent, error) {
			return []model.InOutEvent{{EmployeeID: employeeID, Direction: "IN"}}, nil
		},
	}
	store, err := export.NewJobStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJobStore: %v", err)
	}
	svc := NewReportService(exportOrgRepo{}, exportReportRepo{}, inout, nil, store)

	jobID := store.Create("csv", "personal")
	req := model.ExportRequest{
		Type:      "personal",
		StartDate: "2026-05-01",
		EndDate:   "2026-05-30",
		Format:    "csv",
	}
	svc.RunExportJob(jobID, req, "emp-1", uuid.New().String(), auth.RoleEmployee)

	// Wait up to 1 second for job completion
	var job *export.Job
	var found bool
	for i := 0; i < 100; i++ {
		job, found = store.Get(jobID)
		if found && job.Status != export.JobPending {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if !found {
		t.Fatal("job not found in store")
	}
	if job.Status != export.JobDone {
		t.Errorf("expected job status 'done', got %q error=%q", job.Status, job.Error)
	}
}

