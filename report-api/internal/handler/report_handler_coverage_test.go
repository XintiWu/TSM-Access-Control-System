package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tsmc/report-api/internal/export"
	"github.com/tsmc/report-api/internal/model"
	"github.com/tsmc/report-api/internal/repository"
	"github.com/tsmc/report-api/internal/service"
)

func TestReportHandler_ResolveRequester_Failures(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("missing header", func(t *testing.T) {
		mockOrg := &MockOrgRepository{}
		svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, nil)
		h := NewReportReportHandler(svc, mockOrg)
		r := gin.New()
		r.GET("/test", func(c *gin.Context) {
			h.resolveRequester(c)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("invalid UUID", func(t *testing.T) {
		mockOrg := &MockOrgRepository{}
		svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, nil)
		h := NewReportReportHandler(svc, mockOrg)
		r := gin.New()
		r.GET("/test", func(c *gin.Context) {
			h.resolveRequester(c)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("X-User-ID", "not-a-uuid")
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("db error", func(t *testing.T) {
		mockOrg := &MockOrgRepository{
			GetEmployeeInfoFn: func(ctx context.Context, id string) (*repository.EmployeeInfo, error) {
				return nil, errors.New("db error")
			},
		}
		svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, nil)
		h := NewReportReportHandler(svc, mockOrg)
		r := gin.New()
		r.GET("/test", func(c *gin.Context) {
			h.resolveRequester(c)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("X-User-ID", uuid.New().String())
		r.ServeHTTP(w, req)
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("expected 503, got %d", w.Code)
		}
	})

	t.Run("no org unit assigned", func(t *testing.T) {
		mockOrg := &MockOrgRepository{
			GetEmployeeInfoFn: func(ctx context.Context, id string) (*repository.EmployeeInfo, error) {
				return &repository.EmployeeInfo{OrgUnitID: ""}, nil
			},
		}
		svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, nil)
		h := NewReportReportHandler(svc, mockOrg)
		r := gin.New()
		r.GET("/test", func(c *gin.Context) {
			h.resolveRequester(c)
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set("X-User-ID", uuid.New().String())
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})
}

func NewReportReportHandler(svc *service.ReportService, orgRepo repository.OrgRepository) *ReportHandler {
	return NewReportHandler(svc, orgRepo)
}

func TestReportHandler_PersonalReport_Failures(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("query bind error", func(t *testing.T) {
		mockOrg := managerOrg(uuid.New().String())
		svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, nil)
		h := NewReportReportHandler(svc, mockOrg)
		r := gin.New()
		r.GET("/personal", h.PersonalReport)

		w := httptest.NewRecorder()
		// Missing startDate and endDate to trigger query bind error due to missing required fields
		req, _ := http.NewRequest("GET", "/personal", nil)
		req.Header.Set("X-User-ID", uuid.New().String())
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("service error", func(t *testing.T) {
		orgID := uuid.New().String()
		mockOrg := managerOrg(orgID)
		mockInOut := &MockInOutRepository{
			GetPersonalEventsFn: func(ctx context.Context, employeeID, startDate, endDate string) ([]model.InOutEvent, error) {
				return nil, errors.New("service error")
			},
		}
		svc := service.NewReportService(mockOrg, &MockReportRepository{}, mockInOut, nil, nil)
		h := NewReportReportHandler(svc, mockOrg)
		r := gin.New()
		r.GET("/personal", h.PersonalReport)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/personal?startDate=2026-05-01&endDate=2026-05-02", nil)
		req.Header.Set("X-User-ID", uuid.New().String())
		r.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", w.Code)
		}
	})
}

func TestReportHandler_DepartmentReport_Failures(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("insufficient report role", func(t *testing.T) {
		orgID := uuid.New().String()
		mockOrg := &MockOrgRepository{
			GetEmployeeInfoFn: func(ctx context.Context, employeeID string) (*repository.EmployeeInfo, error) {
				return &repository.EmployeeInfo{OrgUnitID: orgID, ReportRole: "EMPLOYEE"}, nil
			},
		}
		svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, nil)
		h := NewReportReportHandler(svc, mockOrg)
		r := gin.New()
		r.GET("/department", h.DepartmentReport)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/department?orgUnitId="+orgID+"&startDate=2026-05-01&endDate=2026-05-02", nil)
		req.Header.Set("X-User-ID", uuid.New().String())
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})

	t.Run("access denied error", func(t *testing.T) {
		orgID := uuid.New().String()
		mockOrg := managerOrg(orgID)
		mockOrg.IsInSubtreeFn = func(ctx context.Context, req, target string) (bool, error) {
			return false, nil
		}
		svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, nil)
		h := NewReportReportHandler(svc, mockOrg)
		r := gin.New()
		r.GET("/department", h.DepartmentReport)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/department?orgUnitId="+uuid.New().String()+"&startDate=2026-05-01&endDate=2026-05-02", nil)
		req.Header.Set("X-User-ID", uuid.New().String())
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})

	t.Run("service internal error", func(t *testing.T) {
		orgID := uuid.New().String()
		mockOrg := managerOrg(orgID)
		mockOrg.GetSubtreeIDsFn = func(ctx context.Context, id string) ([]string, error) {
			return nil, errors.New("subtree error")
		}
		svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, nil)
		h := NewReportReportHandler(svc, mockOrg)
		r := gin.New()
		r.GET("/department", h.DepartmentReport)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/department?orgUnitId="+orgID+"&startDate=2026-05-01&endDate=2026-05-02", nil)
		req.Header.Set("X-User-ID", uuid.New().String())
		r.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", w.Code)
		}
	})
}

func TestReportHandler_AuditLog_Failures(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("service error", func(t *testing.T) {
		orgID := uuid.New().String()
		mockOrg := managerOrg(orgID)
		mockInOut := &MockInOutRepository{
			GetAuditEventsFn: func(ctx context.Context, f repository.AuditFilter) ([]model.InOutEvent, int, error) {
				return nil, 0, errors.New("audit error")
			},
		}
		svc := service.NewReportService(mockOrg, &MockReportRepository{}, mockInOut, nil, nil)
		h := NewReportReportHandler(svc, mockOrg)
		r := gin.New()
		r.GET("/audit", h.AuditLog)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/audit?startDate=2026-05-01&endDate=2026-05-02", nil)
		req.Header.Set("X-User-ID", uuid.New().String())
		r.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", w.Code)
		}
	})
}

func TestReportHandler_Export_Failures(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("insufficient role for department type", func(t *testing.T) {
		orgID := uuid.New().String()
		mockOrg := &MockOrgRepository{
			GetEmployeeInfoFn: func(ctx context.Context, employeeID string) (*repository.EmployeeInfo, error) {
				return &repository.EmployeeInfo{OrgUnitID: orgID, ReportRole: "EMPLOYEE"}, nil
			},
		}
		svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, nil)
		h := NewReportReportHandler(svc, mockOrg)
		r := gin.New()
		r.GET("/export", h.Export)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/export?type=department&startDate=2026-05-01&endDate=2026-05-02&format=csv", nil)
		req.Header.Set("X-User-ID", uuid.New().String())
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})

	t.Run("service access denied", func(t *testing.T) {
		orgID := uuid.New().String()
		mockOrg := managerOrg(orgID)
		mockOrg.IsInSubtreeFn = func(ctx context.Context, req, target string) (bool, error) {
			return false, nil
		}
		svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, nil)
		h := NewReportReportHandler(svc, mockOrg)
		r := gin.New()
		r.GET("/export", h.Export)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/export?type=events&orgUnitId="+uuid.New().String()+"&startDate=2026-05-01&endDate=2026-05-02&format=csv", nil)
		req.Header.Set("X-User-ID", uuid.New().String())
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})

	t.Run("service internal error", func(t *testing.T) {
		orgID := uuid.New().String()
		mockOrg := managerOrg(orgID)
		mockOrg.GetSubtreeIDsFn = func(ctx context.Context, id string) ([]string, error) {
			return nil, errors.New("error")
		}
		svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, nil)
		h := NewReportReportHandler(svc, mockOrg)
		r := gin.New()
		r.GET("/export", h.Export)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/export?type=events&orgUnitId="+orgID+"&startDate=2026-05-01&endDate=2026-05-02&format=csv", nil)
		req.Header.Set("X-User-ID", uuid.New().String())
		r.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", w.Code)
		}
	})

	t.Run("org unit name err in export filename", func(t *testing.T) {
		orgID := uuid.New().String()
		mockOrg := managerOrg(orgID)
		mockOrg.GetOrgUnitFn = func(ctx context.Context, id string) (*model.OrgUnit, error) {
			return nil, errors.New("org name err")
		}
		h := NewReportReportHandler(nil, mockOrg)
		fn := h.exportFilename(context.Background(), "events", model.ExportRequest{StartDate: "2026-05-01", EndDate: "2026-05-02"}, "MANAGER", orgID, ".csv")
		if !strings.Contains(fn, "access-report_MANAGER_events_2026-05-01-2026-05-02.csv") {
			t.Errorf("unexpected filename: %s", fn)
		}
	})
}

func TestReportHandler_ExportJobCreate_Failures(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("invalid json", func(t *testing.T) {
		mockOrg := managerOrg(uuid.New().String())
		svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, nil)
		h := NewReportReportHandler(svc, mockOrg)
		r := gin.New()
		r.POST("/export/jobs", h.ExportJobCreate)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/export/jobs", strings.NewReader("invalid-json"))
		req.Header.Set("X-User-ID", uuid.New().String())
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("jobs store nil", func(t *testing.T) {
		mockOrg := managerOrg(uuid.New().String())
		svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, nil)
		h := NewReportReportHandler(svc, mockOrg)
		r := gin.New()
		r.POST("/export/jobs", h.ExportJobCreate)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/export/jobs", strings.NewReader(`{"format":"csv","startDate":"2026-05-01","endDate":"2026-05-02"}`))
		req.Header.Set("X-User-ID", uuid.New().String())
		r.ServeHTTP(w, req)
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("expected 503, got %d", w.Code)
		}
	})

	t.Run("insufficient role", func(t *testing.T) {
		orgID := uuid.New().String()
		mockOrg := &MockOrgRepository{
			GetEmployeeInfoFn: func(ctx context.Context, employeeID string) (*repository.EmployeeInfo, error) {
				return &repository.EmployeeInfo{OrgUnitID: orgID, ReportRole: "EMPLOYEE"}, nil
			},
		}
		store, _ := export.NewJobStore(t.TempDir())
		svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, store)
		h := NewReportReportHandler(svc, mockOrg)
		r := gin.New()
		r.POST("/export/jobs", h.ExportJobCreate)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/export/jobs", strings.NewReader(`{"format":"csv","type":"events","startDate":"2026-05-01","endDate":"2026-05-02"}`))
		req.Header.Set("X-User-ID", uuid.New().String())
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})
}

func TestReportHandler_ExportJobGet_Failures(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("invalid jobId", func(t *testing.T) {
		mockOrg := managerOrg(uuid.New().String())
		svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, nil)
		h := NewReportReportHandler(svc, mockOrg)
		r := gin.New()
		r.GET("/export/jobs/:jobId", h.ExportJobGet)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/export/jobs/invalid-uuid", nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("jobs store nil", func(t *testing.T) {
		mockOrg := managerOrg(uuid.New().String())
		svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, nil)
		h := NewReportReportHandler(svc, mockOrg)
		r := gin.New()
		r.GET("/export/jobs/:jobId", h.ExportJobGet)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/export/jobs/"+uuid.New().String(), nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("expected 503, got %d", w.Code)
		}
	})

	t.Run("job not found", func(t *testing.T) {
		mockOrg := managerOrg(uuid.New().String())
		store, _ := export.NewJobStore(t.TempDir())
		svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, store)
		h := NewReportReportHandler(svc, mockOrg)
		r := gin.New()
		r.GET("/export/jobs/:jobId", h.ExportJobGet)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/export/jobs/"+uuid.New().String(), nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})

	t.Run("job failed", func(t *testing.T) {
		mockOrg := managerOrg(uuid.New().String())
		store, _ := export.NewJobStore(t.TempDir())
		svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, store)
		h := NewReportReportHandler(svc, mockOrg)
		r := gin.New()
		r.GET("/export/jobs/:jobId", h.ExportJobGet)

		jobID := store.Create("csv", "events")
		store.MarkFailed(jobID, "failed")

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/export/jobs/"+jobID, nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", w.Code)
		}
	})

	t.Run("job done, result not ready / open error", func(t *testing.T) {
		mockOrg := managerOrg(uuid.New().String())
		store, _ := export.NewJobStore(t.TempDir())
		svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, store)
		h := NewReportReportHandler(svc, mockOrg)
		r := gin.New()
		r.GET("/export/jobs/:jobId", h.ExportJobGet)

		jobID := store.Create("csv", "events")
		// Manually mark done without writing result to cause OpenResult error
		store.MarkDone(jobID)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/export/jobs/"+jobID, nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", w.Code)
		}
	})
}

func TestReportHandler_DoorHeatmap_Failures(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("service access denied", func(t *testing.T) {
		orgID := uuid.New().String()
		mockOrg := managerOrg(orgID)
		mockOrg.IsInSubtreeFn = func(ctx context.Context, req, target string) (bool, error) {
			return false, nil
		}
		svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, nil)
		h := NewReportReportHandler(svc, mockOrg)
		r := gin.New()
		r.GET("/door-heatmap", h.DoorHeatmap)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/door-heatmap?orgUnitId="+uuid.New().String(), nil)
		req.Header.Set("X-User-ID", uuid.New().String())
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})

	t.Run("service internal error", func(t *testing.T) {
		orgID := uuid.New().String()
		mockOrg := managerOrg(orgID)
		mockReport := &MockReportRepository{
			GetDoorHeatmapFn: func(ctx context.Context, ids []string, minutes int) ([]repository.DoorHeatmapRow, error) {
				return nil, errors.New("heatmap error")
			},
		}
		svc := service.NewReportService(mockOrg, mockReport, &MockInOutRepository{}, nil, nil)
		h := NewReportReportHandler(svc, mockOrg)
		r := gin.New()
		r.GET("/door-heatmap", h.DoorHeatmap)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/door-heatmap?orgUnitId="+orgID, nil)
		req.Header.Set("X-User-ID", uuid.New().String())
		r.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", w.Code)
		}
	})
}

func TestReportHandler_AttendanceTrends_Failures(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("service access denied", func(t *testing.T) {
		orgID := uuid.New().String()
		mockOrg := managerOrg(orgID)
		mockOrg.IsInSubtreeFn = func(ctx context.Context, req, target string) (bool, error) {
			return false, nil
		}
		svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, nil)
		h := NewReportReportHandler(svc, mockOrg)
		r := gin.New()
		r.GET("/attendance-trends", h.AttendanceTrends)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/attendance-trends?orgUnitId="+uuid.New().String()+"&startDate=2026-05-01&endDate=2026-05-02", nil)
		req.Header.Set("X-User-ID", uuid.New().String())
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})

	t.Run("service internal error", func(t *testing.T) {
		orgID := uuid.New().String()
		mockOrg := managerOrg(orgID)
		mockReport := &MockReportRepository{
			GetAttendanceTrendsFn: func(ctx context.Context, ids []string, start, end string) ([]repository.PeriodAttendanceMetrics, error) {
				return nil, errors.New("trends error")
			},
		}
		svc := service.NewReportService(mockOrg, mockReport, &MockInOutRepository{}, nil, nil)
		h := NewReportReportHandler(svc, mockOrg)
		r := gin.New()
		r.GET("/attendance-trends", h.AttendanceTrends)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/attendance-trends?orgUnitId="+orgID+"&startDate=2026-05-01&endDate=2026-05-02", nil)
		req.Header.Set("X-User-ID", uuid.New().String())
		r.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", w.Code)
		}
	})
}

func TestReportHandler_WorkforceUtilization_Failures(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("service access denied", func(t *testing.T) {
		orgID := uuid.New().String()
		mockOrg := managerOrg(orgID)
		mockOrg.IsInSubtreeFn = func(ctx context.Context, req, target string) (bool, error) {
			return false, nil
		}
		svc := service.NewReportService(mockOrg, &MockReportRepository{}, &MockInOutRepository{}, nil, nil)
		h := NewReportReportHandler(svc, mockOrg)
		r := gin.New()
		r.GET("/workforce-utilization", h.WorkforceUtilization)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/workforce-utilization?orgUnitId="+uuid.New().String()+"&startDate=2026-05-01&endDate=2026-05-02", nil)
		req.Header.Set("X-User-ID", uuid.New().String())
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})

	t.Run("service internal error", func(t *testing.T) {
		orgID := uuid.New().String()
		mockOrg := managerOrg(orgID)
		mockReport := &MockReportRepository{
			GetSummaryFn: func(ctx context.Context, ids []string, start, end string) (model.DepartmentSummary, error) {
				return model.DepartmentSummary{}, errors.New("utilization error")
			},
		}
		svc := service.NewReportService(mockOrg, mockReport, &MockInOutRepository{}, nil, nil)
		h := NewReportReportHandler(svc, mockOrg)
		r := gin.New()
		r.GET("/workforce-utilization", h.WorkforceUtilization)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/workforce-utilization?orgUnitId="+orgID+"&startDate=2026-05-01&endDate=2026-05-02", nil)
		req.Header.Set("X-User-ID", uuid.New().String())
		r.ServeHTTP(w, req)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected 500, got %d", w.Code)
		}
	})
}
