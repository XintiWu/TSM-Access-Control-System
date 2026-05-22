package handler

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/tsmc/report-api/internal/model"
	"github.com/tsmc/report-api/internal/repository"
	"github.com/tsmc/report-api/internal/service"
)

var (
	requestTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "report_api_requests_total",
		Help: "Total HTTP requests by endpoint and status",
	}, []string{"endpoint", "status"})

	requestLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "report_api_request_duration_ms",
		Help:    "Request latency in milliseconds by endpoint",
		Buckets: prometheus.ExponentialBuckets(1, 2, 14), // 1ms → ~16s
	}, []string{"endpoint"})
)

// ReportHandler handles all report-related HTTP endpoints.
type ReportHandler struct {
	svc     *service.ReportService
	orgRepo *repository.OrgRepository
}

// NewReportHandler creates a new handler with service and org repository.
func NewReportHandler(svc *service.ReportService, orgRepo *repository.OrgRepository) *ReportHandler {
	return &ReportHandler{svc: svc, orgRepo: orgRepo}
}

// resolveRequester extracts user identity from X-User-ID header and resolves their org_unit.
func (h *ReportHandler) resolveRequester(c *gin.Context) (userID, orgUnitID string, ok bool) {
	userID = c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing X-User-ID header"})
		return "", "", false
	}
	if _, err := uuid.Parse(userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid X-User-ID"})
		return "", "", false
	}

	orgUnitID, err := h.orgRepo.GetEmployeeOrgUnitID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database unavailable"})
		return "", "", false
	}
	if orgUnitID == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "employee has no org unit assigned"})
		return "", "", false
	}
	return userID, orgUnitID, true
}

// PersonalReport handles GET /reports/personal
func (h *ReportHandler) PersonalReport(c *gin.Context) {
	start := time.Now()
	defer func() {
		requestLatency.WithLabelValues("personal").Observe(float64(time.Since(start).Milliseconds()))
	}()

	userID, _, ok := h.resolveRequester(c)
	if !ok {
		requestTotal.WithLabelValues("personal", "4xx").Inc()
		return
	}

	var req model.PersonalReportRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		requestTotal.WithLabelValues("personal", "400").Inc()
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.svc.GetPersonalReport(c.Request.Context(), userID, req.StartDate, req.EndDate)
	if err != nil {
		requestTotal.WithLabelValues("personal", "500").Inc()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	requestTotal.WithLabelValues("personal", "200").Inc()
	c.JSON(http.StatusOK, resp)
}

// DepartmentReport handles GET /reports/department
func (h *ReportHandler) DepartmentReport(c *gin.Context) {
	start := time.Now()
	defer func() {
		requestLatency.WithLabelValues("department").Observe(float64(time.Since(start).Milliseconds()))
	}()

	_, orgUnitID, ok := h.resolveRequester(c)
	if !ok {
		requestTotal.WithLabelValues("department", "4xx").Inc()
		return
	}

	var req model.DepartmentReportRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		requestTotal.WithLabelValues("department", "400").Inc()
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.svc.GetDepartmentReport(c.Request.Context(), req, orgUnitID)
	if err != nil {
		if strings.Contains(err.Error(), "access denied") {
			requestTotal.WithLabelValues("department", "403").Inc()
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		requestTotal.WithLabelValues("department", "500").Inc()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	requestTotal.WithLabelValues("department", "200").Inc()
	c.JSON(http.StatusOK, resp)
}

// AuditLog handles GET /reports/audit
func (h *ReportHandler) AuditLog(c *gin.Context) {
	start := time.Now()
	defer func() {
		requestLatency.WithLabelValues("audit").Observe(float64(time.Since(start).Milliseconds()))
	}()

	_, orgUnitID, ok := h.resolveRequester(c)
	if !ok {
		requestTotal.WithLabelValues("audit", "4xx").Inc()
		return
	}

	var req model.AuditLogRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		requestTotal.WithLabelValues("audit", "400").Inc()
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.svc.GetAuditLog(c.Request.Context(), req, orgUnitID)
	if err != nil {
		requestTotal.WithLabelValues("audit", "500").Inc()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	requestTotal.WithLabelValues("audit", "200").Inc()
	c.JSON(http.StatusOK, resp)
}

// Export handles GET /reports/export (CSV download, PDF TODO)
func (h *ReportHandler) Export(c *gin.Context) {
	start := time.Now()
	defer func() {
		requestLatency.WithLabelValues("export").Observe(float64(time.Since(start).Milliseconds()))
	}()

	_, orgUnitID, ok := h.resolveRequester(c)
	if !ok {
		requestTotal.WithLabelValues("export", "4xx").Inc()
		return
	}

	var req model.ExportRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		requestTotal.WithLabelValues("export", "400").Inc()
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Format == "pdf" {
		requestTotal.WithLabelValues("export", "501").Inc()
		c.JSON(http.StatusNotImplemented, gin.H{"error": "PDF export not yet implemented"})
		return
	}

	reader, err := h.svc.ExportCSV(c.Request.Context(), req, orgUnitID)
	if err != nil {
		if strings.Contains(err.Error(), "access denied") {
			requestTotal.WithLabelValues("export", "403").Inc()
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		requestTotal.WithLabelValues("export", "500").Inc()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	filename := fmt.Sprintf("report_%s_%s_%s.csv", req.OrgUnitID, req.StartDate, req.EndDate)
	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	requestTotal.WithLabelValues("export", "200").Inc()
	io.Copy(c.Writer, reader)
}
