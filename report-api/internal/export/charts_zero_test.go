package export

import (
	"testing"

	"github.com/tsmc/report-api/internal/model"
)

func TestChartsWithAllZeroValues(t *testing.T) {
	hm := &model.DoorHeatmapResponse{
		WindowMinutes: 60,
		Doors: []model.DoorTrafficRow{
			{DoorName: "Gate A", SwipeCount: 0},
			{DoorName: "Gate B", SwipeCount: 0},
		},
	}
	if _, err := DoorHeatmapChartPNG(hm); err != nil {
		t.Fatalf("DoorHeatmapChartPNG zero swipes: %v", err)
	}

	dept := &model.DepartmentReportResponse{
		OrgUnitName: "Team-A",
		Summary:     model.DepartmentSummary{},
		Periods: []model.PeriodReport{
			{PeriodStart: "2026-05-22", AvgHours: 0, LateRate: 0},
			{PeriodStart: "2026-05-23", AvgHours: 0, LateRate: 0},
		},
	}
	for name, fn := range map[string]func(*model.DepartmentReportResponse) ([]byte, error){
		"entries": DepartmentEntriesChartPNG,
		"hours":   DepartmentHoursChartPNG,
		"late":    DepartmentLateChartPNG,
	} {
		if _, err := fn(dept); err != nil {
			t.Fatalf("%s all zero: %v", name, err)
		}
	}

	if _, err := emptyChartPNG("No data"); err != nil {
		t.Fatalf("emptyChartPNG: %v", err)
	}
}
