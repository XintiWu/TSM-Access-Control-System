package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tsmc/report-api/internal/auth"
	"github.com/tsmc/report-api/internal/export"
	"github.com/tsmc/report-api/internal/model"
)

// ExportDepartmentVisualPDF builds PDF with metadata, summary, detail table, and charts.
func (s *ReportService) ExportDepartmentVisualPDF(
	ctx context.Context,
	req model.ExportRequest,
	userID string,
	requesterOrgUnitID string,
	role auth.ReportRole,
) ([]byte, error) {
	if !role.CanViewDepartmentReports() {
		return nil, fmt.Errorf("access denied: role %s cannot export department reports", role)
	}
	if req.OrgUnitID == "" {
		return nil, fmt.Errorf("orgUnitId is required")
	}
	granularity := req.Granularity
	if granularity == "" {
		granularity = "daily"
	}

	deptReq := model.DepartmentReportRequest{
		OrgUnitID: req.OrgUnitID, StartDate: req.StartDate, EndDate: req.EndDate,
		Granularity: granularity,
	}
	dept, err := s.GetDepartmentReport(ctx, deptReq, requesterOrgUnitID)
	if err != nil {
		return nil, err
	}

	orgUnit, err := s.orgRepo.GetOrgUnit(ctx, req.OrgUnitID)
	if err != nil || orgUnit == nil {
		return nil, fmt.Errorf("org unit not found: %s", req.OrgUnitID)
	}

	subtreeIDs, err := s.orgRepo.GetSubtreeIDs(ctx, req.OrgUnitID)
	if err != nil {
		return nil, fmt.Errorf("get subtree: %w", err)
	}

	requesterName, _ := s.orgRepo.GetEmployeeDisplayName(ctx, userID)
	if requesterName == "" {
		requesterName = userID
	}

	security, err := s.inoutRepo.GetSecurityDenySummary(ctx, subtreeIDs, req.StartDate, req.EndDate)
	if err != nil {
		return nil, fmt.Errorf("security summary: %w", err)
	}

	detailRows, err := s.buildReportDetailRows(ctx, dept, req.StartDate, req.EndDate)
	if err != nil {
		return nil, err
	}

	heatmap, err := s.GetDoorHeatmap(ctx, 60, role)
	if err != nil {
		return nil, err
	}

	trendReq := model.AttendanceTrendsRequest{
		OrgUnitID: req.OrgUnitID, StartDate: req.StartDate, EndDate: req.EndDate,
		Granularity: granularity,
	}
	trends, err := s.GetAttendanceTrends(ctx, trendReq, requesterOrgUnitID, role)
	if err != nil {
		return nil, err
	}

	showCorp := role.IsExecutive()
	if !showCorp {
		if rootID, err := s.orgRepo.GetRootOrgUnitID(ctx); err == nil && req.OrgUnitID == rootID {
			showCorp = true
		}
	}

	meta := export.ReportPDFMeta{
		ReportTitle:      "TSMC Site Attendance & Gate Traffic Report",
		StatWindowStart:  dateAtStart(req.StartDate),
		StatWindowEnd:    dateAtEnd(req.EndDate),
		GeneratedAtUTC:   time.Now().UTC(),
		RequesterLabel:   fmt.Sprintf("%s / %s", requesterName, role),
		OrgScopePath:     orgUnit.MaterializedPath,
		OrgUnitName:      orgUnit.Name,
	}

	return export.WriteVisualReportPDF(export.VisualReportBundle{
		Department:       dept,
		Heatmap:          heatmap,
		Trends:           trends,
		ShowCorpOverview: showCorp,
		Meta:             meta,
		Security:         security,
		DetailRows:       detailRows,
	})
}

func dateAtStart(d string) string {
	return d + " 00:00:00"
}

func dateAtEnd(d string) string {
	return d + " 23:59:59"
}

func (s *ReportService) buildReportDetailRows(
	ctx context.Context,
	dept *model.DepartmentReportResponse,
	startDate, endDate string,
) ([]model.ReportDetailRow, error) {
	if len(dept.SubUnits) > 0 {
		var rows []model.ReportDetailRow
		for _, su := range dept.SubUnits {
			childIDs, err := s.orgRepo.GetSubtreeIDs(ctx, su.OrgUnitID)
			if err != nil {
				continue
			}
			sec, _ := s.inoutRepo.GetSecurityDenySummary(ctx, childIDs, startDate, endDate)
			hours := s.sumEmployeeHours(ctx, childIDs, startDate, endDate)
			rows = append(rows, model.ReportDetailRow{
				EntityID:     su.OrgUnitID,
				EntityName:   su.OrgUnitName,
				EntityType:   "Department",
				TotalSwipes:  su.TotalEntries + su.TotalExits,
				TotalHours:   hours,
				AnomalyNotes: formatAnomalyNotes(sec, 0),
			})
		}
		return rows, nil
	}

	ids, err := s.orgRepo.GetSubtreeIDs(ctx, dept.OrgUnitID)
	if err != nil {
		return nil, err
	}
	empRows, err := s.inoutRepo.GetEmployeeReportRows(ctx, ids, startDate, endDate)
	if err != nil {
		return nil, err
	}
	var rows []model.ReportDetailRow
	for _, e := range empRows {
		name, _ := s.orgRepo.GetEmployeeDisplayName(ctx, e.EmployeeID)
		if name == "" {
			name = e.EmployeeID
		}
		sec := model.SecurityDenySummary{
			AntiPassbackDenies: e.AntiPassbackDenies,
			PermissionDenied:   e.PermissionDenied,
		}
		rows = append(rows, model.ReportDetailRow{
			EntityID:     e.EmployeeID,
			EntityName:   name,
			EntityType:   "Employee",
			TotalSwipes:  e.TotalSwipes,
			TotalHours:   e.TotalHours,
			AnomalyNotes: formatAnomalyNotes(sec, e.MissingPunchDays),
		})
	}
	return rows, nil
}

func (s *ReportService) sumEmployeeHours(ctx context.Context, orgUnitIDs []string, start, end string) float64 {
	rows, err := s.inoutRepo.GetEmployeeReportRows(ctx, orgUnitIDs, start, end)
	if err != nil {
		return 0
	}
	var sum float64
	for _, r := range rows {
		sum += r.TotalHours
	}
	return sum
}

func formatAnomalyNotes(sec model.SecurityDenySummary, missingPunchDays int) string {
	var parts []string
	if sec.AntiPassbackDenies > 0 {
		parts = append(parts, fmt.Sprintf("Anti-passback deny: %d", sec.AntiPassbackDenies))
	}
	if sec.PermissionDenied > 0 {
		parts = append(parts, fmt.Sprintf("Blacklist/banned swipe: %d", sec.PermissionDenied))
	}
	if missingPunchDays > 0 {
		parts = append(parts, fmt.Sprintf("Missing OUT punch days: %d", missingPunchDays))
	}
	if len(parts) == 0 {
		return "OK"
	}
	return strings.Join(parts, "; ")
}
