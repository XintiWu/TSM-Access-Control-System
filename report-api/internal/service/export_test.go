package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/tsmc/report-api/internal/auth"
	"github.com/tsmc/report-api/internal/export"
	"github.com/tsmc/report-api/internal/model"
	"github.com/tsmc/report-api/internal/repository"
)

type exportInOutRepo struct {
	getPersonalEventsFn func(ctx context.Context, employeeID, startDate, endDate string) ([]model.InOutEvent, error)
}

func (m *exportInOutRepo) GetPersonalEvents(ctx context.Context, employeeID, startDate, endDate string) ([]model.InOutEvent, error) {
	if m.getPersonalEventsFn != nil {
		return m.getPersonalEventsFn(ctx, employeeID, startDate, endDate)
	}
	return nil, nil
}
func (m *exportInOutRepo) GetAuditEvents(ctx context.Context, f repository.AuditFilter) ([]model.InOutEvent, int, error) {
	return nil, 0, nil
}
func (m *exportInOutRepo) GetEventsForExport(ctx context.Context, orgUnitIDs []string, startDate, endDate string) ([]model.InOutEvent, error) {
	return nil, nil
}
func (m *exportInOutRepo) GetSecurityDenySummary(ctx context.Context, orgUnitIDs []string, startDate, endDate string) (model.SecurityDenySummary, error) {
	return model.SecurityDenySummary{}, nil
}
func (m *exportInOutRepo) GetEmployeeReportRows(ctx context.Context, orgUnitIDs []string, startDate, endDate string) ([]model.EmployeeReportRow, error) {
	return nil, nil
}
func (m *exportInOutRepo) Close() error { return nil }

type exportOrgRepo struct{}

func (exportOrgRepo) Ping(ctx context.Context) error { return nil }
func (exportOrgRepo) GetOrgUnit(ctx context.Context, id string) (*model.OrgUnit, error) {
	return &model.OrgUnit{ID: id, Name: "Test Org"}, nil
}
func (exportOrgRepo) GetSubtreeIDs(ctx context.Context, orgUnitID string) ([]string, error) {
	return []string{orgUnitID}, nil
}
func (exportOrgRepo) GetEmployeeDisplayName(ctx context.Context, employeeID string) (string, error) {
	return "Test Employee", nil
}
func (exportOrgRepo) GetEmployeeInfo(ctx context.Context, employeeID string) (*repository.EmployeeInfo, error) {
	return &repository.EmployeeInfo{OrgUnitID: uuid.New().String(), ReportRole: "EMPLOYEE"}, nil
}
func (exportOrgRepo) GetEmployeeOrgUnitID(ctx context.Context, employeeID string) (string, error) {
	return uuid.New().String(), nil
}
func (exportOrgRepo) GetRootOrgUnitID(ctx context.Context) (string, error) {
	return uuid.New().String(), nil
}
func (exportOrgRepo) IsInSubtree(ctx context.Context, req, target string) (bool, error) {
	return req == target, nil
}
func (exportOrgRepo) CountActiveEmployees(ctx context.Context, ids []string) (int, error) {
	return 10, nil
}
func (exportOrgRepo) GetChildUnits(ctx context.Context, parentID string) ([]model.OrgUnit, error) {
	return nil, nil
}
func (exportOrgRepo) Close() error { return nil }

type exportReportRepo struct{}

func (exportReportRepo) GetAggregated(ctx context.Context, orgUnitIDs []string, startDate, endDate string) ([]model.AggregatedRow, error) {
	return nil, nil
}
func (exportReportRepo) GetSummary(ctx context.Context, orgUnitIDs []string, startDate, endDate string) (model.DepartmentSummary, error) {
	return model.DepartmentSummary{}, nil
}
func (exportReportRepo) GetOnSiteCount(ctx context.Context, orgUnitIDs []string) (int, error) {
	return 0, nil
}
func (exportReportRepo) GetDoorHeatmap(ctx context.Context, orgUnitIDs []string, minutes int) ([]repository.DoorHeatmapRow, error) {
	return nil, nil
}
func (exportReportRepo) GetAttendanceTrends(ctx context.Context, orgUnitIDs []string, startDate, endDate string) ([]repository.PeriodAttendanceMetrics, error) {
	return nil, nil
}
func (exportReportRepo) GetPassbackDenyCountsLastMinute(ctx context.Context) ([]repository.PassbackDenyRow, error) {
	return nil, nil
}
func (exportReportRepo) Close() error { return nil }

func TestBuildExportDocument_Personal(t *testing.T) {
	userID := uuid.New().String()
	inout := &exportInOutRepo{
		getPersonalEventsFn: func(ctx context.Context, employeeID, startDate, endDate string) ([]model.InOutEvent, error) {
			return []model.InOutEvent{{EmployeeID: employeeID, Direction: "IN"}}, nil
		},
	}
	svc := NewReportService(exportOrgRepo{}, exportReportRepo{}, inout, nil, nil)

	req := model.ExportRequest{Type: "personal", StartDate: "2026-05-01", EndDate: "2026-05-30"}
	doc, err := svc.BuildExportDocument(context.Background(), req, userID, uuid.New().String(), auth.RoleEmployee)
	if err != nil {
		t.Fatalf("BuildExportDocument() error = %v", err)
	}
	if doc.Title == "" {
		t.Error("expected non-empty document title")
	}
}

func TestBuildExportDocument_AccessDenied(t *testing.T) {
	svc := NewReportService(exportOrgRepo{}, exportReportRepo{}, &exportInOutRepo{}, nil, nil)

	req := model.ExportRequest{Type: "events", StartDate: "2026-05-01", EndDate: "2026-05-30", OrgUnitID: uuid.New().String()}
	_, err := svc.BuildExportDocument(context.Background(), req, uuid.New().String(), uuid.New().String(), auth.RoleEmployee)
	if !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("expected ErrAccessDenied, got %v", err)
	}
}

func TestRenderExport_CSV(t *testing.T) {
	doc := export.Document{
		Title:   "Test Report",
		Headers: []string{"Col1", "Col2"},
		Rows:    [][]string{{"a", "b"}},
	}
	data, ext, err := RenderExport(doc, "csv")
	if err != nil {
		t.Fatalf("RenderExport(csv) error = %v", err)
	}
	if ext != ".csv" {
		t.Errorf("ext = %q, want .csv", ext)
	}
	if len(data) == 0 {
		t.Error("expected non-empty CSV data")
	}
}

func TestRenderExport_PDF(t *testing.T) {
	doc := export.Document{
		Title:   "Test Report",
		Headers: []string{"Col1", "Col2"},
		Rows:    [][]string{{"a", "b"}},
	}
	data, ext, err := RenderExport(doc, "pdf")
	if err != nil {
		t.Fatalf("RenderExport(pdf) error = %v", err)
	}
	if ext != ".pdf" {
		t.Errorf("ext = %q, want .pdf", ext)
	}
	if len(data) == 0 {
		t.Error("expected non-empty PDF data")
	}
}
