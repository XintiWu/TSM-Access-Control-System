package export

import (
	"testing"
	"time"

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

func TestWriteVisualReportPDF_CorporateOverview(t *testing.T) {
	bundle := VisualReportBundle{
		ShowCorpOverview: true,
		Meta: ReportPDFMeta{
			ReportTitle:     "Corp-wide Overview",
			StatWindowStart: "2026-05-01 00:00:00",
			StatWindowEnd:   "2026-05-22 23:59:59",
			GeneratedAtUTC:  time.Now(),
			RequesterLabel:  "CEO / EXEC",
			OrgScopePath:    "/corp",
			OrgUnitName:     "Corp",
		},
		DetailRows: []model.ReportDetailRow{
			{EntityID: "shortid", EntityName: "Short ID User", EntityType: "Employee", TotalSwipes: 10, TotalHours: 5, AnomalyNotes: "None"},
			{EntityID: "very_long_entity_id_that_exceeds_thirteen", EntityName: "Long ID User", EntityType: "Contractor", TotalSwipes: 20, TotalHours: 10, AnomalyNotes: "Some issues with passback patterns"},
		},
		Department: &model.DepartmentReportResponse{
			OrgUnitID: "a0000000-0000-0000-0000-000000000001", OrgUnitName: "Corp",
			StartDate: "2026-05-01", EndDate: "2026-05-22", Granularity: "daily",
			Summary: model.DepartmentSummary{
				TotalEntries: 1000, TotalExits: 990, AvgHoursPerDay: 7.9, LateRate: 0.05,
			},
			Periods: []model.PeriodReport{
				{PeriodStart: "2026-05-21", PeriodEnd: "2026-05-21", TotalEntries: 500, TotalExits: 495, AvgHours: 7.8, LateRate: 0.04},
				{PeriodStart: "2026-05-22", PeriodEnd: "2026-05-22", TotalEntries: 500, TotalExits: 495, AvgHours: 8.0, LateRate: 0.06},
			},
			SubUnits: []model.SubUnitSummary{
				{OrgUnitID: "subunit1", OrgUnitName: "Department of Very Long SubUnit Name That Truncates", TotalEntries: 400, TotalExits: 395},
				{OrgUnitID: "subunit2", OrgUnitName: "Sub 2", TotalEntries: 600, TotalExits: 595},
			},
		},
		Heatmap: &model.DoorHeatmapResponse{
			WindowMinutes: 60,
			Doors: []model.DoorTrafficRow{
				{DoorName: "Very Long Gate Name That Will Exceed Maximum Twenty Two Characters Limit", SwipeCount: 500},
				{DoorName: "", DoorID: "GateWithoutNameID", SwipeCount: 300},
				{DoorName: "Gate C", SwipeCount: 100},
				{DoorName: "Gate D", SwipeCount: 50},
				{DoorName: "Gate E", SwipeCount: 40},
				{DoorName: "Gate F (ignored in list)", SwipeCount: 10},
			},
		},
		Trends: &model.AttendanceTrendsResponse{
			OrgUnitName: "Corp",
			Series: []model.AttendancePoint{
				{PeriodStart: "2026-05-21", AvgHours: 7.8, LateRate: 0.04},
				{PeriodStart: "2026-05-22", AvgHours: 8.0, LateRate: 0.06},
			},
		},
	}

	b, err := WriteVisualReportPDF(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) == 0 {
		t.Fatal("generated pdf is empty")
	}
}

func TestWriteVisualReportPDF_SubUnitCmpOnly(t *testing.T) {
	bundle := VisualReportBundle{
		ShowCorpOverview: false,
		Meta: ReportPDFMeta{
			ReportTitle:     "SubUnit CMP",
			OrgUnitName:     "Parent",
		},
		Department: &model.DepartmentReportResponse{
			OrgUnitID: "parent", OrgUnitName: "Parent",
			StartDate: "2026-05-01", EndDate: "2026-05-02",
			Summary: model.DepartmentSummary{TotalEntries: 100, TotalExits: 100},
			SubUnits: []model.SubUnitSummary{
				{OrgUnitID: "su1", OrgUnitName: "SU 1", TotalEntries: 50, TotalExits: 50},
			},
		},
	}
	b, err := WriteVisualReportPDF(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) == 0 {
		t.Fatal("pdf is empty")
	}
}

func TestWriteVisualReportPDF_Compact(t *testing.T) {
	bundle := VisualReportBundle{
		ShowCorpOverview: false,
		Meta: ReportPDFMeta{
			ReportTitle: "Compact Report",
		},
		Department: &model.DepartmentReportResponse{
			OrgUnitName: "CompactUnit",
			Summary:     model.DepartmentSummary{TotalEntries: 10, TotalExits: 10},
			Periods: []model.PeriodReport{
				{PeriodStart: "2026-05-01", AvgHours: 8.0, LateRate: 0.1},
			},
		},
		Heatmap: &model.DoorHeatmapResponse{
			WindowMinutes: 30,
			Doors: []model.DoorTrafficRow{
				{DoorName: "Only Door", SwipeCount: 20},
			},
		},
		Trends: &model.AttendanceTrendsResponse{
			OrgUnitName: "CompactUnit",
			Series: []model.AttendancePoint{
				{PeriodStart: "2026-05-01", AvgHours: 8.0, LateRate: 0.1},
				{PeriodStart: "2026-05-02", AvgHours: 8.2, LateRate: 0.15},
			},
		},
	}
	b, err := WriteVisualReportPDF(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) == 0 {
		t.Fatal("pdf is empty")
	}
}

func TestWriteVisualReportPDF_RedundantTrends(t *testing.T) {
	bundle := VisualReportBundle{
		ShowCorpOverview: false,
		Meta: ReportPDFMeta{
			ReportTitle: "Redundant Trends Report",
		},
		Department: &model.DepartmentReportResponse{
			OrgUnitName: "Unit",
			Periods: []model.PeriodReport{
				{PeriodStart: "2026-05-01", AvgHours: 8.0, LateRate: 0.1},
			},
		},
		Trends: &model.AttendanceTrendsResponse{
			OrgUnitName: "Unit",
			Series: []model.AttendancePoint{
				{PeriodStart: "2026-05-01", AvgHours: 8.0, LateRate: 0.1},
			},
		},
	}
	b, err := WriteVisualReportPDF(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) == 0 {
		t.Fatal("pdf is empty")
	}
}

func TestWriteVisualReportPDF_NilDepartment(t *testing.T) {
	bundle := VisualReportBundle{
		Department: nil,
	}
	_, err := WriteVisualReportPDF(bundle)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestWriteVisualReportPDF_EmptyDataFallbacks(t *testing.T) {
	bundle := VisualReportBundle{
		Meta: ReportPDFMeta{
			ReportTitle: "Fallback Report",
		},
		Department: &model.DepartmentReportResponse{
			OrgUnitName: "FallbackUnit",
			Summary:     model.DepartmentSummary{TotalEntries: 0, TotalExits: 0},
			Periods:     nil,
		},
		Heatmap: nil,
		Trends:  nil,
	}
	b, err := WriteVisualReportPDF(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) == 0 {
		t.Fatal("pdf is empty")
	}
}

func TestTruncate_SmallMax(t *testing.T) {
	if got := truncate("hello", 2); got != "he" {
		t.Errorf("expected 'he', got %q", got)
	}
	if got := truncate("hello", 3); got != "hel" {
		t.Errorf("expected 'hel', got %q", got)
	}
	if got := truncate("hello", 5); got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
	if got := truncate("hello world", 8); got != "hello..." {
		t.Errorf("expected 'hello...', got %q", got)
	}
}
