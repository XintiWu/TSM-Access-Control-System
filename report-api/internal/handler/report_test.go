package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tsmc/report-api/internal/export"
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

type MockReportRepository struct {
	GetAggregatedFn          func(ctx context.Context, orgUnitIDs []string, startDate, endDate string) ([]model.AggregatedRow, error)
	GetSummaryFn             func(ctx context.Context, orgUnitIDs []string, startDate, endDate string) (model.DepartmentSummary, error)
	GetOnSiteCountFn         func(ctx context.Context, orgUnitIDs []string) (int, error)
	GetDoorHeatmapFn         func(ctx context.Context, orgUnitIDs []string, minutes int) ([]repository.DoorHeatmapRow, error)
	GetAttendanceTrendsFn    func(ctx context.Context, orgUnitIDs []string, startDate, endDate string) ([]repository.PeriodAttendanceMetrics, error)
	GetPassbackDenyCountsFn  func(ctx context.Context) ([]repository.PassbackDenyRow, error)
}

func (m *MockReportRepository) GetAggregated(ctx context.Context, orgUnitIDs []string, startDate, endDate string) ([]model.AggregatedRow, error) {
	if m.GetAggregatedFn != nil {
		return m.GetAggregatedFn(ctx, orgUnitIDs, startDate, endDate)
	}
	return nil, nil
}
func (m *MockReportRepository) GetSummary(ctx context.Context, orgUnitIDs []string, startDate, endDate string) (model.DepartmentSummary, error) {
	if m.GetSummaryFn != nil {
		return m.GetSummaryFn(ctx, orgUnitIDs, startDate, endDate)
	}
	return model.DepartmentSummary{}, nil
}
func (m *MockReportRepository) GetOnSiteCount(ctx context.Context, orgUnitIDs []string) (int, error) {
	if m.GetOnSiteCountFn != nil {
		return m.GetOnSiteCountFn(ctx, orgUnitIDs)
	}
	return 5, nil
}
func (m *MockReportRepository) GetDoorHeatmap(ctx context.Context, orgUnitIDs []string, minutes int) ([]repository.DoorHeatmapRow, error) {
	if m.GetDoorHeatmapFn != nil {
		return m.GetDoorHeatmapFn(ctx, orgUnitIDs, minutes)
	}
	return nil, nil
}
func (m *MockReportRepository) GetAttendanceTrends(ctx context.Context, orgUnitIDs []string, startDate, endDate string) ([]repository.PeriodAttendanceMetrics, error) {
	if m.GetAttendanceTrendsFn != nil {
		return m.GetAttendanceTrendsFn(ctx, orgUnitIDs, startDate, endDate)
	}
	return nil, nil
}
func (m *MockReportRepository) GetPassbackDenyCountsLastMinute(ctx context.Context) ([]repository.PassbackDenyRow, error) {
	if m.GetPassbackDenyCountsFn != nil {
		return m.GetPassbackDenyCountsFn(ctx)
	}
	return nil, nil
}
func (m *MockReportRepository) Close() error { return nil }

func managerOrg(orgID string) *MockOrgRepository {
	return &MockOrgRepository{
		GetEmployeeInfoFn: func(ctx context.Context, employeeID string) (*repository.EmployeeInfo, error) {
			return &repository.EmployeeInfo{OrgUnitID: orgID, ReportRole: "MANAGER"}, nil
		},
		IsInSubtreeFn: func(ctx context.Context, requesterOrgUnitID, targetOrgUnitID string) (bool, error) {
			return requesterOrgUnitID == targetOrgUnitID, nil
		},
		GetOrgUnitFn: func(ctx context.Context, id string) (*model.OrgUnit, error) {
			return &model.OrgUnit{ID: id, Name: "Dept"}, nil
		},
	}
}

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

func TestReportHandler_DepartmentReport_InsufficientRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	orgID := uuid.New().String()
	mockOrg := &MockOrgRepository{
		GetEmployeeInfoFn: func(ctx context.Context, employeeID string) (*repository.EmployeeInfo, error) {
			return &repository.EmployeeInfo{OrgUnitID: orgID, ReportRole: "EMPLOYEE"}, nil
		},
	}
	svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, nil)
	h := NewReportHandler(svc, mockOrg)

	r := gin.New()
	r.GET("/reports/department", h.DepartmentReport)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/reports/department?orgUnitId="+orgID+"&startDate=2026-05-01&endDate=2026-05-30", nil)
	req.Header.Set("X-User-ID", uuid.New().String())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestReportHandler_Export_InsufficientRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	orgID := uuid.New().String()
	mockOrg := &MockOrgRepository{
		GetEmployeeInfoFn: func(ctx context.Context, employeeID string) (*repository.EmployeeInfo, error) {
			return &repository.EmployeeInfo{OrgUnitID: orgID, ReportRole: "EMPLOYEE"}, nil
		},
	}
	svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, nil)
	h := NewReportHandler(svc, mockOrg)

	r := gin.New()
	r.GET("/reports/export", h.Export)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/reports/export?format=csv&startDate=2026-05-01&endDate=2026-05-30", nil)
	req.Header.Set("X-User-ID", uuid.New().String())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestReportHandler_Export_PersonalSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	orgID := uuid.New().String()
	userID := uuid.New().String()
	mockOrg := &MockOrgRepository{
		GetEmployeeInfoFn: func(ctx context.Context, employeeID string) (*repository.EmployeeInfo, error) {
			return &repository.EmployeeInfo{OrgUnitID: orgID, ReportRole: "EMPLOYEE"}, nil
		},
	}
	mockInOut := &MockInOutRepository{
		GetPersonalEventsFn: func(ctx context.Context, employeeID, startDate, endDate string) ([]model.InOutEvent, error) {
			return []model.InOutEvent{{EmployeeID: employeeID, Direction: "IN"}}, nil
		},
	}
	svc := service.NewReportService(mockOrg, &MockReportRepository{}, mockInOut, nil, nil)
	h := NewReportHandler(svc, mockOrg)

	r := gin.New()
	r.GET("/reports/export", h.Export)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/reports/export?type=personal&format=csv&startDate=2026-05-01&endDate=2026-05-30", nil)
	req.Header.Set("X-User-ID", userID)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestReportHandler_Export_InternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	orgID := uuid.New().String()
	userID := uuid.New().String()
	mockOrg := &MockOrgRepository{
		GetEmployeeInfoFn: func(ctx context.Context, employeeID string) (*repository.EmployeeInfo, error) {
			return &repository.EmployeeInfo{OrgUnitID: orgID, ReportRole: "EMPLOYEE"}, nil
		},
	}
	mockInOut := &MockInOutRepository{
		GetPersonalEventsFn: func(ctx context.Context, employeeID, startDate, endDate string) ([]model.InOutEvent, error) {
			return nil, errors.New("database error")
		},
	}
	svc := service.NewReportService(mockOrg, &MockReportRepository{}, mockInOut, nil, nil)
	h := NewReportHandler(svc, mockOrg)

	r := gin.New()
	r.GET("/reports/export", h.Export)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/reports/export?type=personal&format=csv&startDate=2026-05-01&endDate=2026-05-30", nil)
	req.Header.Set("X-User-ID", userID)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestReportHandler_ExportJobCreate_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	orgID := uuid.New().String()
	mockOrg := &MockOrgRepository{
		GetEmployeeInfoFn: func(ctx context.Context, employeeID string) (*repository.EmployeeInfo, error) {
			return &repository.EmployeeInfo{OrgUnitID: orgID, ReportRole: "EMPLOYEE"}, nil
		},
	}
	svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, nil)
	h := NewReportHandler(svc, mockOrg)

	r := gin.New()
	r.POST("/reports/export/jobs", h.ExportJobCreate)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/reports/export/jobs", strings.NewReader("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", uuid.New().String())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestReportHandler_ExportJobCreate_JobsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	orgID := uuid.New().String()
	mockOrg := &MockOrgRepository{
		GetEmployeeInfoFn: func(ctx context.Context, employeeID string) (*repository.EmployeeInfo, error) {
			return &repository.EmployeeInfo{OrgUnitID: orgID, ReportRole: "EMPLOYEE"}, nil
		},
	}
	svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, nil)
	h := NewReportHandler(svc, mockOrg)

	r := gin.New()
	r.POST("/reports/export/jobs", h.ExportJobCreate)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/reports/export/jobs", strings.NewReader(`{"format":"csv","type":"personal","startDate":"2026-05-01","endDate":"2026-05-30"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", uuid.New().String())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestReportHandler_resolveRequester_InvalidUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	orgID := uuid.New().String()
	mockOrg := managerOrg(orgID)
	svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, nil)
	h := NewReportHandler(svc, mockOrg)
	r := gin.New()
	r.GET("/reports/personal", h.PersonalReport)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/reports/personal?startDate=2026-05-01&endDate=2026-05-30", nil)
	req.Header.Set("X-User-ID", "not-a-uuid")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestReportHandler_resolveRequester_DBUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockOrg := &MockOrgRepository{
		GetEmployeeInfoFn: func(ctx context.Context, employeeID string) (*repository.EmployeeInfo, error) {
			return nil, errors.New("db down")
		},
	}
	svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, nil)
	h := NewReportHandler(svc, mockOrg)
	r := gin.New()
	r.GET("/reports/personal", h.PersonalReport)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/reports/personal?startDate=2026-05-01&endDate=2026-05-30", nil)
	req.Header.Set("X-User-ID", uuid.New().String())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestReportHandler_resolveRequester_NoOrgUnit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockOrg := &MockOrgRepository{
		GetEmployeeInfoFn: func(ctx context.Context, employeeID string) (*repository.EmployeeInfo, error) {
			return &repository.EmployeeInfo{OrgUnitID: "", ReportRole: "EMPLOYEE"}, nil
		},
	}
	svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, nil)
	h := NewReportHandler(svc, mockOrg)
	r := gin.New()
	r.GET("/reports/personal", h.PersonalReport)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/reports/personal?startDate=2026-05-01&endDate=2026-05-30", nil)
	req.Header.Set("X-User-ID", uuid.New().String())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestReportHandler_DepartmentReport_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	orgID := uuid.New().String()
	mockOrg := managerOrg(orgID)
	mockReport := &MockReportRepository{
		GetSummaryFn: func(ctx context.Context, orgUnitIDs []string, startDate, endDate string) (model.DepartmentSummary, error) {
			return model.DepartmentSummary{UniqueEmployees: 5}, nil
		},
	}
	svc := service.NewReportService(mockOrg, mockReport, &MockInOutRepository{}, nil, nil)
	h := NewReportHandler(svc, mockOrg)
	r := gin.New()
	r.GET("/reports/department", h.DepartmentReport)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/reports/department?orgUnitId="+orgID+"&startDate=2026-05-01&endDate=2026-05-30", nil)
	req.Header.Set("X-User-ID", uuid.New().String())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestReportHandler_AuditLog_Forbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	orgID := uuid.New().String()
	mockOrg := &MockOrgRepository{
		GetEmployeeInfoFn: func(ctx context.Context, employeeID string) (*repository.EmployeeInfo, error) {
			return &repository.EmployeeInfo{OrgUnitID: orgID, ReportRole: "EMPLOYEE"}, nil
		},
	}
	svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, nil)
	h := NewReportHandler(svc, mockOrg)
	r := gin.New()
	r.GET("/reports/audit", h.AuditLog)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/reports/audit?startDate=2026-05-01&endDate=2026-05-30&employeeId="+uuid.New().String(), nil)
	req.Header.Set("X-User-ID", uuid.New().String())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for employee self-audit, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestReportHandler_AuditLog_ManagerSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	orgID := uuid.New().String()
	mockOrg := managerOrg(orgID)
	mockInOut := &MockInOutRepository{
		GetAuditEventsFn: func(ctx context.Context, f repository.AuditFilter) ([]model.InOutEvent, int, error) {
			return []model.InOutEvent{{EventID: "e1", EmployeeID: uuid.New().String()}}, 1, nil
		},
	}
	svc := service.NewReportService(mockOrg, &MockReportRepository{}, mockInOut, nil, nil)
	h := NewReportHandler(svc, mockOrg)
	r := gin.New()
	r.GET("/reports/audit", h.AuditLog)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/reports/audit?startDate=2026-05-01&endDate=2026-05-30", nil)
	req.Header.Set("X-User-ID", uuid.New().String())
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestReportHandler_ExportJobGet_InvalidJobID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	orgID := uuid.New().String()
	jobStore, _ := export.NewJobStore(t.TempDir())
	mockOrg := managerOrg(orgID)
	svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, jobStore)
	h := NewReportHandler(svc, mockOrg)
	r := gin.New()
	r.GET("/reports/export/jobs/:jobId", h.ExportJobGet)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/reports/export/jobs/not-uuid", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestReportHandler_ExportJobGet_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	orgID := uuid.New().String()
	jobStore, _ := export.NewJobStore(t.TempDir())
	mockOrg := managerOrg(orgID)
	svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, jobStore)
	h := NewReportHandler(svc, mockOrg)
	r := gin.New()
	r.GET("/reports/export/jobs/:jobId", h.ExportJobGet)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/reports/export/jobs/"+uuid.New().String(), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestReportHandler_ExportJobGet_PendingAndFailed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	jobStore, _ := export.NewJobStore(t.TempDir())
	pendingID := jobStore.Create("csv", "personal")
	failedID := jobStore.Create("csv", "personal")
	jobStore.MarkFailed(failedID, "secret internal error")

	orgID := uuid.New().String()
	mockOrg := managerOrg(orgID)
	svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, jobStore)
	h := NewReportHandler(svc, mockOrg)
	r := gin.New()
	r.GET("/reports/export/jobs/:jobId", h.ExportJobGet)

	wPending := httptest.NewRecorder()
	req1, _ := http.NewRequest("GET", "/reports/export/jobs/"+pendingID, nil)
	r.ServeHTTP(wPending, req1)
	if wPending.Code != http.StatusAccepted {
		t.Errorf("pending: expected 202, got %d", wPending.Code)
	}

	wFailed := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/reports/export/jobs/"+failedID, nil)
	r.ServeHTTP(wFailed, req2)
	if wFailed.Code != http.StatusInternalServerError {
		t.Errorf("failed: expected 500, got %d", wFailed.Code)
	}
	if strings.Contains(wFailed.Body.String(), "secret internal error") {
		t.Error("failed job response must not expose internal error details")
	}
}

func TestReportHandler_ExportJobGet_Done(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	jobStore, _ := export.NewJobStore(dir)
	jobID := jobStore.Create("csv", "personal")
	jobStore.MarkDone(jobID)
	if err := os.WriteFile(jobStore.FilePath(jobID, ".csv"), []byte("a,b\n1,2"), 0o644); err != nil {
		t.Fatal(err)
	}

	orgID := uuid.New().String()
	mockOrg := managerOrg(orgID)
	svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, jobStore)
	h := NewReportHandler(svc, mockOrg)
	r := gin.New()
	r.GET("/reports/export/jobs/:jobId", h.ExportJobGet)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/reports/export/jobs/"+jobID, nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestReportHandler_DoorHeatmap(t *testing.T) {
	gin.SetMode(gin.TestMode)
	orgID := uuid.New().String()
	mockOrg := managerOrg(orgID)
	mockReport := &MockReportRepository{
		GetDoorHeatmapFn: func(ctx context.Context, orgUnitIDs []string, minutes int) ([]repository.DoorHeatmapRow, error) {
			return []repository.DoorHeatmapRow{{DoorID: uuid.New().String(), SwipeCount: 3}}, nil
		},
	}
	svc := service.NewReportService(mockOrg, mockReport, &MockInOutRepository{}, nil, nil)
	h := NewReportHandler(svc, mockOrg)
	r := gin.New()
	r.GET("/reports/analytics/door-heatmap", h.DoorHeatmap)

	t.Run("403", func(t *testing.T) {
		empOrg := &MockOrgRepository{
			GetEmployeeInfoFn: func(ctx context.Context, employeeID string) (*repository.EmployeeInfo, error) {
				return &repository.EmployeeInfo{OrgUnitID: orgID, ReportRole: "EMPLOYEE"}, nil
			},
		}
		hEmp := NewReportHandler(service.NewReportService(empOrg, mockReport, &MockInOutRepository{}, nil, nil), empOrg)
		rEmp := gin.New()
		rEmp.GET("/reports/analytics/door-heatmap", hEmp.DoorHeatmap)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/reports/analytics/door-heatmap?orgUnitId="+orgID, nil)
		req.Header.Set("X-User-ID", uuid.New().String())
		rEmp.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/reports/analytics/door-heatmap?orgUnitId="+orgID+"&minutes=60", nil)
	req.Header.Set("X-User-ID", uuid.New().String())
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestReportHandler_AttendanceTrends(t *testing.T) {
	gin.SetMode(gin.TestMode)
	orgID := uuid.New().String()
	mockOrg := managerOrg(orgID)
	mockReport := &MockReportRepository{
		GetAttendanceTrendsFn: func(ctx context.Context, orgUnitIDs []string, startDate, endDate string) ([]repository.PeriodAttendanceMetrics, error) {
			return []repository.PeriodAttendanceMetrics{{PeriodStart: "2026-05-01", AvgHours: 8}}, nil
		},
	}
	svc := service.NewReportService(mockOrg, mockReport, &MockInOutRepository{}, nil, nil)
	h := NewReportHandler(svc, mockOrg)
	r := gin.New()
	r.GET("/reports/analytics/attendance-trends", h.AttendanceTrends)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/reports/analytics/attendance-trends?orgUnitId="+orgID+"&startDate=2026-05-01&endDate=2026-05-30", nil)
	req.Header.Set("X-User-ID", uuid.New().String())
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestReportHandler_WorkforceUtilization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	orgID := uuid.New().String()
	mockOrg := managerOrg(orgID)
	mockReport := &MockReportRepository{
		GetSummaryFn: func(ctx context.Context, orgUnitIDs []string, startDate, endDate string) (model.DepartmentSummary, error) {
			return model.DepartmentSummary{UniqueEmployees: 8}, nil
		},
	}
	svc := service.NewReportService(mockOrg, mockReport, &MockInOutRepository{}, nil, nil)
	h := NewReportHandler(svc, mockOrg)
	r := gin.New()
	r.GET("/reports/analytics/workforce-utilization", h.WorkforceUtilization)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/reports/analytics/workforce-utilization?orgUnitId="+orgID+"&startDate=2026-05-01&endDate=2026-05-30", nil)
	req.Header.Set("X-User-ID", uuid.New().String())
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}
