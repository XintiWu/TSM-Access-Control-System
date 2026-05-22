package export

import (
	"testing"

	"github.com/tsmc/report-api/internal/model"
)

func TestWriteVisualReportPDFProducesBytes(t *testing.T) {
	bundle := VisualReportBundle{
		Meta: ReportPDFMeta{
			ReportTitle:     "TSMC Site Attendance & Gate Traffic Report",
			StatWindowStart: "2026-05-01 00:00:00",
			StatWindowEnd:   "2026-05-22 23:59:59",
			RequesterLabel:  "Team-A Manager / TEAM_MANAGER",
			OrgScopePath:    "/corp/team-a/",
			OrgUnitName:     "Team-A",
		},
		DetailRows: []model.ReportDetailRow{
			{EntityID: "e1", EntityName: "Demo User", EntityType: "Employee", TotalSwipes: 100, TotalHours: 8, AnomalyNotes: "OK"},
		},
		Department: &model.DepartmentReportResponse{
			OrgUnitID: "a0000000-0000-0000-0000-000000000003", OrgUnitName: "Team-A",
			StartDate: "2026-05-01", EndDate: "2026-05-22", Granularity: "daily",
			Summary: model.DepartmentSummary{
				TotalEntries: 100, TotalExits: 98, AvgHoursPerDay: 8.2, LateRate: 0.15,
			},
			Periods: []model.PeriodReport{
				{PeriodStart: "2026-05-21", PeriodEnd: "2026-05-21", TotalEntries: 50, TotalExits: 48, AvgHours: 8.0, LateRate: 0.1},
				{PeriodStart: "2026-05-22", PeriodEnd: "2026-05-22", TotalEntries: 50, TotalExits: 50, AvgHours: 8.5, LateRate: 0.2},
			},
		},
		Heatmap: &model.DoorHeatmapResponse{
			WindowMinutes: 60,
			Doors: []model.DoorTrafficRow{
				{DoorName: "Main Gate", SwipeCount: 120},
				{DoorName: "East Wing", SwipeCount: 80},
			},
		},
		Trends: &model.AttendanceTrendsResponse{
			OrgUnitName: "Team-A",
			Series: []model.AttendancePoint{
				{PeriodStart: "2026-05-21", AvgHours: 7.5, LateRate: 0.1},
				{PeriodStart: "2026-05-22", AvgHours: 8.2, LateRate: 0.2},
			},
		},
	}
	b, err := WriteVisualReportPDF(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 5000 {
		t.Fatalf("visual pdf too small: %d bytes", len(b))
	}
	if b[0] != '%' || b[1] != 'P' {
		t.Fatalf("not a PDF header: %q", b[:8])
	}
}
