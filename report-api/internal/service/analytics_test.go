package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/tsmc/report-api/internal/auth"
	"github.com/tsmc/report-api/internal/model"
	"github.com/tsmc/report-api/internal/repository"
)

type analyticsOrgRepo struct {
	orgUnitID string
}

func (a analyticsOrgRepo) Ping(ctx context.Context) error { return nil }
func (a analyticsOrgRepo) GetOrgUnit(ctx context.Context, id string) (*model.OrgUnit, error) {
	return &model.OrgUnit{ID: id, Name: "Analytics Org"}, nil
}
func (a analyticsOrgRepo) GetSubtreeIDs(ctx context.Context, orgUnitID string) ([]string, error) {
	return []string{orgUnitID}, nil
}
func (a analyticsOrgRepo) GetEmployeeDisplayName(ctx context.Context, employeeID string) (string, error) {
	return "User", nil
}
func (a analyticsOrgRepo) GetEmployeeInfo(ctx context.Context, employeeID string) (*repository.EmployeeInfo, error) {
	return &repository.EmployeeInfo{OrgUnitID: a.orgUnitID, ReportRole: "MANAGER"}, nil
}
func (a analyticsOrgRepo) GetEmployeeOrgUnitID(ctx context.Context, employeeID string) (string, error) {
	return a.orgUnitID, nil
}
func (a analyticsOrgRepo) GetRootOrgUnitID(ctx context.Context) (string, error) {
	return a.orgUnitID, nil
}
func (a analyticsOrgRepo) IsInSubtree(ctx context.Context, req, target string) (bool, error) {
	return req == target, nil
}
func (a analyticsOrgRepo) CountActiveEmployees(ctx context.Context, ids []string) (int, error) {
	return 20, nil
}
func (a analyticsOrgRepo) GetChildUnits(ctx context.Context, parentID string) ([]model.OrgUnit, error) {
	return nil, nil
}
func (a analyticsOrgRepo) Close() error { return nil }

type analyticsReportRepo struct {
	heatmapRows []repository.DoorHeatmapRow
	trendRows   []repository.PeriodAttendanceMetrics
	summary     model.DepartmentSummary
}

func (a analyticsReportRepo) GetAggregated(ctx context.Context, orgUnitIDs []string, startDate, endDate string) ([]model.AggregatedRow, error) {
	return nil, nil
}
func (a analyticsReportRepo) GetSummary(ctx context.Context, orgUnitIDs []string, startDate, endDate string) (model.DepartmentSummary, error) {
	return a.summary, nil
}
func (a analyticsReportRepo) GetOnSiteCount(ctx context.Context, orgUnitIDs []string) (int, error) {
	return 5, nil
}
func (a analyticsReportRepo) GetDoorHeatmap(ctx context.Context, orgUnitIDs []string, minutes int) ([]repository.DoorHeatmapRow, error) {
	return a.heatmapRows, nil
}
func (a analyticsReportRepo) GetAttendanceTrends(ctx context.Context, orgUnitIDs []string, startDate, endDate string) ([]repository.PeriodAttendanceMetrics, error) {
	return a.trendRows, nil
}
func (a analyticsReportRepo) GetPassbackDenyCountsLastMinute(ctx context.Context) ([]repository.PassbackDenyRow, error) {
	return nil, nil
}
func (a analyticsReportRepo) Close() error { return nil }

func TestGetDoorHeatmap_AccessDenied(t *testing.T) {
	orgID := uuid.New().String()
	svc := NewReportService(analyticsOrgRepo{orgUnitID: orgID}, analyticsReportRepo{}, &exportInOutRepo{}, nil, nil)
	_, err := svc.GetDoorHeatmap(context.Background(), orgID, 60, orgID, auth.RoleEmployee)
	if !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("expected ErrAccessDenied, got %v", err)
	}
}

func TestGetDoorHeatmap_Success(t *testing.T) {
	orgID := uuid.New().String()
	repo := analyticsReportRepo{
		heatmapRows: []repository.DoorHeatmapRow{
			{DoorID: uuid.New().String(), DoorName: "Gate A", SwipeCount: 10},
		},
	}
	svc := NewReportService(analyticsOrgRepo{orgUnitID: orgID}, repo, &exportInOutRepo{}, nil, nil)
	resp, err := svc.GetDoorHeatmap(context.Background(), orgID, 60, orgID, auth.RoleTeamManager)
	if err != nil {
		t.Fatalf("GetDoorHeatmap() error = %v", err)
	}
	if len(resp.Doors) != 1 {
		t.Fatalf("expected 1 door, got %d", len(resp.Doors))
	}
}

func TestGetAttendanceTrends_AccessDenied(t *testing.T) {
	orgID := uuid.New().String()
	svc := NewReportService(analyticsOrgRepo{orgUnitID: orgID}, analyticsReportRepo{}, &exportInOutRepo{}, nil, nil)
	req := model.AttendanceTrendsRequest{OrgUnitID: orgID, StartDate: "2026-05-01", EndDate: "2026-05-30"}
	_, err := svc.GetAttendanceTrends(context.Background(), req, orgID, auth.RoleEmployee)
	if !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("expected ErrAccessDenied, got %v", err)
	}
}

func TestGetAttendanceTrends_Success(t *testing.T) {
	orgID := uuid.New().String()
	repo := analyticsReportRepo{
		trendRows: []repository.PeriodAttendanceMetrics{
			{PeriodStart: "2026-05-01", AvgHours: 8, LateRate: 0.1},
		},
	}
	svc := NewReportService(analyticsOrgRepo{orgUnitID: orgID}, repo, &exportInOutRepo{}, nil, nil)
	req := model.AttendanceTrendsRequest{OrgUnitID: orgID, StartDate: "2026-05-01", EndDate: "2026-05-30"}
	resp, err := svc.GetAttendanceTrends(context.Background(), req, orgID, auth.RoleDirector)
	if err != nil {
		t.Fatalf("GetAttendanceTrends() error = %v", err)
	}
	if len(resp.Series) == 0 {
		t.Error("expected non-empty series")
	}
}

func TestGetWorkforceUtilization_AccessDenied(t *testing.T) {
	orgID := uuid.New().String()
	svc := NewReportService(analyticsOrgRepo{orgUnitID: orgID}, analyticsReportRepo{}, &exportInOutRepo{}, nil, nil)
	req := model.WorkforceUtilizationRequest{OrgUnitID: orgID, StartDate: "2026-05-01", EndDate: "2026-05-30"}
	_, err := svc.GetWorkforceUtilization(context.Background(), req, orgID, auth.RoleEmployee)
	if !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("expected ErrAccessDenied, got %v", err)
	}
}

func TestGetWorkforceUtilization_Success(t *testing.T) {
	orgID := uuid.New().String()
	repo := analyticsReportRepo{
		summary: model.DepartmentSummary{UniqueEmployees: 10},
	}
	svc := NewReportService(analyticsOrgRepo{orgUnitID: orgID}, repo, &exportInOutRepo{}, nil, nil)
	req := model.WorkforceUtilizationRequest{OrgUnitID: orgID, StartDate: "2026-05-01", EndDate: "2026-05-30"}
	resp, err := svc.GetWorkforceUtilization(context.Background(), req, orgID, auth.RoleVP)
	if err != nil {
		t.Fatalf("GetWorkforceUtilization() error = %v", err)
	}
	if resp.Headcount != 20 {
		t.Errorf("headcount = %d, want 20", resp.Headcount)
	}
}
