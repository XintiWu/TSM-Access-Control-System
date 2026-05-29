package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tsmc/report-api/internal/model"
	"github.com/tsmc/report-api/internal/repository"
	"github.com/tsmc/report-api/internal/service"
)

// Mock objects
type MockOrgRepository struct {
	GetOrgUnitFn             func(ctx context.Context, id string) (*model.OrgUnit, error)
	GetSubtreeIDsFn           func(ctx context.Context, orgUnitID string) ([]string, error)
	GetEmployeeInfoFn         func(ctx context.Context, employeeID string) (*repository.EmployeeInfo, error)
	IsInSubtreeFn             func(ctx context.Context, requesterOrgUnitID, targetOrgUnitID string) (bool, error)
	CountActiveEmployeesFn   func(ctx context.Context, orgUnitIDs []string) (int, error)
	GetChildUnitsFn           func(ctx context.Context, parentID string) ([]model.OrgUnit, error)
	GetEmployeeDisplayNameFn func(ctx context.Context, employeeID string) (string, error)
	GetEmployeeOrgUnitIDFn   func(ctx context.Context, employeeID string) (string, error)
	GetRootOrgUnitIDFn       func(ctx context.Context) (string, error)
	PingFn                   func(ctx context.Context) error
	CloseFn                  func() error
}

func (m *MockOrgRepository) Ping(ctx context.Context) error {
	if m.PingFn != nil {
		return m.PingFn(ctx)
	}
	return nil
}
func (m *MockOrgRepository) GetOrgUnit(ctx context.Context, id string) (*model.OrgUnit, error) {
	if m.GetOrgUnitFn != nil {
		return m.GetOrgUnitFn(ctx, id)
	}
	return &model.OrgUnit{ID: id, Name: "Test Org"}, nil
}
func (m *MockOrgRepository) GetSubtreeIDs(ctx context.Context, orgUnitID string) ([]string, error) {
	if m.GetSubtreeIDsFn != nil {
		return m.GetSubtreeIDsFn(ctx, orgUnitID)
	}
	return []string{orgUnitID}, nil
}
func (m *MockOrgRepository) GetEmployeeDisplayName(ctx context.Context, employeeID string) (string, error) {
	if m.GetEmployeeDisplayNameFn != nil {
		return m.GetEmployeeDisplayNameFn(ctx, employeeID)
	}
	return "Test Employee", nil
}
func (m *MockOrgRepository) GetEmployeeInfo(ctx context.Context, employeeID string) (*repository.EmployeeInfo, error) {
	if m.GetEmployeeInfoFn != nil {
		return m.GetEmployeeInfoFn(ctx, employeeID)
	}
	return &repository.EmployeeInfo{OrgUnitID: uuid.New().String(), ReportRole: "EMPLOYEE"}, nil
}
func (m *MockOrgRepository) GetEmployeeOrgUnitID(ctx context.Context, employeeID string) (string, error) {
	if m.GetEmployeeOrgUnitIDFn != nil {
		return m.GetEmployeeOrgUnitIDFn(ctx, employeeID)
	}
	return uuid.New().String(), nil
}
func (m *MockOrgRepository) GetRootOrgUnitID(ctx context.Context) (string, error) {
	if m.GetRootOrgUnitIDFn != nil {
		return m.GetRootOrgUnitIDFn(ctx)
	}
	return uuid.New().String(), nil
}
func (m *MockOrgRepository) IsInSubtree(ctx context.Context, req, target string) (bool, error) {
	if m.IsInSubtreeFn != nil {
		return m.IsInSubtreeFn(ctx, req, target)
	}
	return req == target, nil
}
func (m *MockOrgRepository) CountActiveEmployees(ctx context.Context, ids []string) (int, error) {
	if m.CountActiveEmployeesFn != nil {
		return m.CountActiveEmployeesFn(ctx, ids)
	}
	return 10, nil
}
func (m *MockOrgRepository) GetChildUnits(ctx context.Context, parentID string) ([]model.OrgUnit, error) {
	if m.GetChildUnitsFn != nil {
		return m.GetChildUnitsFn(ctx, parentID)
	}
	return nil, nil
}
func (m *MockOrgRepository) Close() error {
	if m.CloseFn != nil {
		return m.CloseFn()
	}
	return nil
}

type MockInOutRepository struct {
	GetPersonalEventsFn func(ctx context.Context, employeeID, startDate, endDate string) ([]model.InOutEvent, error)
	GetAuditEventsFn    func(ctx context.Context, f repository.AuditFilter) ([]model.InOutEvent, int, error)
}

func (m *MockInOutRepository) GetPersonalEvents(ctx context.Context, employeeID, startDate, endDate string) ([]model.InOutEvent, error) {
	if m.GetPersonalEventsFn != nil {
		return m.GetPersonalEventsFn(ctx, employeeID, startDate, endDate)
	}
	return nil, nil
}
func (m *MockInOutRepository) GetAuditEvents(ctx context.Context, f repository.AuditFilter) ([]model.InOutEvent, int, error) {
	if m.GetAuditEventsFn != nil {
		return m.GetAuditEventsFn(ctx, f)
	}
	return nil, 0, nil
}
func (m *MockInOutRepository) GetEventsForExport(ctx context.Context, orgUnitIDs []string, startDate, endDate string) ([]model.InOutEvent, error) {
	return nil, nil
}
func (m *MockInOutRepository) GetSecurityDenySummary(ctx context.Context, orgUnitIDs []string, startDate, endDate string) (model.SecurityDenySummary, error) {
	return model.SecurityDenySummary{}, nil
}
func (m *MockInOutRepository) GetEmployeeReportRows(ctx context.Context, orgUnitIDs []string, startDate, endDate string) ([]model.EmployeeReportRow, error) {
	return nil, nil
}
func (m *MockInOutRepository) Close() error { return nil }

type MockReportRepository struct{}

func (m *MockReportRepository) GetAggregated(ctx context.Context, orgUnitIDs []string, startDate, endDate string) ([]model.AggregatedRow, error) {
	return nil, nil
}
func (m *MockReportRepository) GetSummary(ctx context.Context, orgUnitIDs []string, startDate, endDate string) (model.DepartmentSummary, error) {
	return model.DepartmentSummary{}, nil
}
func (m *MockReportRepository) GetOnSiteCount(ctx context.Context, orgUnitIDs []string) (int, error) {
	return 5, nil
}
func (m *MockReportRepository) GetDoorHeatmap(ctx context.Context, orgUnitIDs []string, minutes int) ([]repository.DoorHeatmapRow, error) {
	return nil, nil
}
func (m *MockReportRepository) GetAttendanceTrends(ctx context.Context, orgUnitIDs []string, startDate, endDate string) ([]repository.PeriodAttendanceMetrics, error) {
	return nil, nil
}
func (m *MockReportRepository) GetPassbackDenyCountsLastMinute(ctx context.Context) ([]repository.PassbackDenyRow, error) {
	return nil, nil
}
func (m *MockReportRepository) Close() error { return nil }

func TestReportHandler_PersonalReport(t *testing.T) {
	gin.SetMode(gin.TestMode)

	orgID := uuid.New().String()
	mockOrg := &MockOrgRepository{
		GetEmployeeInfoFn: func(ctx context.Context, employeeID string) (*repository.EmployeeInfo, error) {
			return &repository.EmployeeInfo{
				OrgUnitID:  orgID,
				ReportRole: "EMPLOYEE",
			}, nil
		},
	}

	mockInOut := &MockInOutRepository{
		GetPersonalEventsFn: func(ctx context.Context, employeeID, startDate, endDate string) ([]model.InOutEvent, error) {
			return []model.InOutEvent{
				{
					EventID:    "123",
					EmployeeID: employeeID,
					Direction:  "IN",
					EventTime:  time.Now().UTC(),
					Status:     "ALLOW",
				},
			}, nil
		},
	}

	mockReport := &MockReportRepository{}

	svc := service.NewReportService(mockOrg, mockReport, mockInOut, nil, nil)
	h := NewReportHandler(svc, mockOrg)

	r := gin.New()
	r.GET("/reports/personal", h.PersonalReport)

	t.Run("Missing X-User-ID", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/reports/personal", nil)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})

	t.Run("Valid request", func(t *testing.T) {
		w := httptest.NewRecorder()
		userID := uuid.New().String()
		req, _ := http.NewRequest("GET", "/reports/personal?startDate=2026-05-01&endDate=2026-05-30", nil)
		req.Header.Set("X-User-ID", userID)
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		}
	})
}
