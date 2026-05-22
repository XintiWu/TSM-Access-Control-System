package export

import (
	"testing"

	"github.com/tsmc/report-api/internal/model"
)

func TestChartPNGNonEmpty(t *testing.T) {
	dept := &model.DepartmentReportResponse{
		OrgUnitName: "Team-A",
		Summary:     model.DepartmentSummary{TotalEntries: 246, TotalExits: 242},
		Periods: []model.PeriodReport{
			{PeriodStart: "2026-05-22", TotalEntries: 246, TotalExits: 242, AvgHours: 2.97, LateRate: 1},
		},
	}
	for name, fn := range map[string]func(*model.DepartmentReportResponse) ([]byte, error){
		"entries": DepartmentEntriesChartPNG,
		"hours":   DepartmentHoursChartPNG,
		"late":    DepartmentLateChartPNG,
	} {
		b, err := fn(dept)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(b) < 1000 {
			t.Fatalf("%s png too small: %d", name, len(b))
		}
	}
}
