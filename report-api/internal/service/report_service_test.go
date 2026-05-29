package service

import (
	"testing"

	"github.com/tsmc/report-api/internal/model"
)

func TestCalcUtilization_Detailed(t *testing.T) {
	tests := []struct {
		name          string
		uniquePresent int
		headcount     int
		want          float64
	}{
		{"negative headcount", 5, -1, 0},
		{"normal 50pct", 50, 100, 0.5},
		{"zero present", 0, 100, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calcUtilization(tt.uniquePresent, tt.headcount)
			if got != tt.want {
				t.Errorf("calcUtilization(%d, %d) = %f, want %f", tt.uniquePresent, tt.headcount, got, tt.want)
			}
		})
	}
}

func TestBuildPeriods_Daily(t *testing.T) {
	rows := []model.AggregatedRow{
		{ReportDate: "2026-01-01", TotalEntries: 10, TotalExits: 8},
		{ReportDate: "2026-01-02", TotalEntries: 15, TotalExits: 12},
	}
	periods := buildPeriods(rows, "daily", "2026-01-01", "2026-01-02")
	if len(periods) != 2 {
		t.Fatalf("expected 2 periods, got %d", len(periods))
	}
	if periods[0].TotalEntries != 10 {
		t.Errorf("period[0].TotalEntries = %d, want 10", periods[0].TotalEntries)
	}
}

func TestBuildPeriods_Empty(t *testing.T) {
	periods := buildPeriods(nil, "daily", "2026-01-01", "2026-01-02")
	if periods != nil {
		t.Fatalf("expected nil, got %v", periods)
	}
}

func TestBuildPeriods_Weekly(t *testing.T) {
	rows := []model.AggregatedRow{
		{ReportDate: "2026-01-01", TotalEntries: 10, TotalExits: 8},
		{ReportDate: "2026-01-03", TotalEntries: 5, TotalExits: 4},
		{ReportDate: "2026-01-08", TotalEntries: 20, TotalExits: 18},
	}
	periods := buildPeriods(rows, "weekly", "2026-01-01", "2026-01-14")
	if len(periods) == 0 {
		t.Fatal("expected non-empty periods for weekly")
	}
	if periods[0].TotalEntries != 15 {
		t.Errorf("week1 TotalEntries = %d, want 15", periods[0].TotalEntries)
	}
}

func TestBuildPeriods_Monthly(t *testing.T) {
	rows := []model.AggregatedRow{
		{ReportDate: "2026-01-05", TotalEntries: 10},
		{ReportDate: "2026-01-15", TotalEntries: 20},
		{ReportDate: "2026-02-01", TotalEntries: 5},
	}
	periods := buildPeriods(rows, "monthly", "2026-01-01", "2026-02-28")
	if len(periods) != 2 {
		t.Fatalf("expected 2 monthly periods, got %d", len(periods))
	}
	if periods[0].TotalEntries != 30 {
		t.Errorf("jan TotalEntries = %d, want 30", periods[0].TotalEntries)
	}
}

func TestGroupByDay_MergesSameDate(t *testing.T) {
	rows := []model.AggregatedRow{
		{ReportDate: "2026-01-01", TotalEntries: 5, TotalExits: 3, UniqueEmployees: 2},
		{ReportDate: "2026-01-01", TotalEntries: 10, TotalExits: 7, UniqueEmployees: 3},
	}
	periods := groupByDay(rows)
	if len(periods) != 1 {
		t.Fatalf("expected 1 merged period, got %d", len(periods))
	}
	if periods[0].TotalEntries != 15 {
		t.Errorf("merged TotalEntries = %d, want 15", periods[0].TotalEntries)
	}
}

func TestGroupByWeek_InvalidDate(t *testing.T) {
	result := groupByWeek(nil, "invalid", "also-invalid")
	if result != nil {
		t.Errorf("expected nil for invalid date, got %v", result)
	}
}

func TestGroupByMonth_InvalidRowDate(t *testing.T) {
	rows := []model.AggregatedRow{
		{ReportDate: "not-a-date", TotalEntries: 10},
	}
	result := groupByMonth(rows, "2026-01-01", "2026-01-31")
	if len(result) != 0 {
		t.Errorf("expected 0 periods for invalid row date, got %d", len(result))
	}
}

func TestAccessDeniedError(t *testing.T) {
	err := NewAccessDeniedError("test message")
	if err.Error() != "test message" {
		t.Errorf("Error() = %q, want %q", err.Error(), "test message")
	}

	ade, ok := err.(*AccessDeniedError)
	if !ok {
		t.Fatal("expected *AccessDeniedError type")
	}
	if !ade.Is(ErrAccessDenied) {
		t.Error("expected Is(ErrAccessDenied) to return true")
	}
}
