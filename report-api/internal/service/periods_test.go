package service

import (
	"testing"

	"github.com/tsmc/report-api/internal/model"
)

func TestBuildPeriods_Quarterly(t *testing.T) {
	rows := []model.AggregatedRow{
		{ReportDate: "2026-01-15", TotalEntries: 10, UniqueEmployees: 5},
		{ReportDate: "2026-04-10", TotalEntries: 20, UniqueEmployees: 8},
	}
	periods := buildPeriods(rows, "quarterly", "2026-01-01", "2026-06-30")
	if len(periods) != 2 {
		t.Fatalf("expected 2 quarters, got %d", len(periods))
	}
	if periods[0].TotalEntries != 10 || periods[1].TotalEntries != 20 {
		t.Fatalf("unexpected totals: %+v", periods)
	}
}

func TestBuildPeriods_Yearly(t *testing.T) {
	rows := []model.AggregatedRow{
		{ReportDate: "2025-06-01", TotalEntries: 100},
		{ReportDate: "2026-03-01", TotalEntries: 50},
	}
	periods := buildPeriods(rows, "yearly", "2025-01-01", "2026-12-31")
	if len(periods) != 2 {
		t.Fatalf("expected 2 years, got %d", len(periods))
	}
}

func TestCalcUtilization(t *testing.T) {
	if calcUtilization(50, 100) != 0.5 {
		t.Fatal("expected 0.5")
	}
	if calcUtilization(0, 0) != 0 {
		t.Fatal("expected 0 for zero headcount")
	}
	if calcUtilization(150, 100) != 1 {
		t.Fatal("expected cap at 1")
	}
}
