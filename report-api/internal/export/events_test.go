package export

import (
	"testing"
	"time"

	"github.com/tsmc/report-api/internal/model"
)

func TestEventsDocument_Basic(t *testing.T) {
	reason := "ANTI_PASSBACK"
	events := []model.InOutEvent{
		{
			EventID:    "e1",
			EmployeeID: "emp1",
			DoorID:     "door1",
			Direction:  "IN",
			EventTime:  time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC),
			Status:     "ALLOW",
			SourceIP:   "10.0.0.1",
		},
		{
			EventID:    "e2",
			EmployeeID: "emp2",
			DoorID:     "door2",
			Direction:  "OUT",
			EventTime:  time.Date(2026, 1, 1, 17, 0, 0, 0, time.UTC),
			Status:     "DENY",
			Reason:     &reason,
			SourceIP:   "10.0.0.2",
		},
	}

	doc := EventsDocument("Engineering", "org-1", "2026-01-01", "2026-01-31", events)

	if doc.Title != "TSMC Access Control Report" {
		t.Errorf("Title = %q, want TSMC Access Control Report", doc.Title)
	}
	if doc.Subtitle != "Access Events Export" {
		t.Errorf("Subtitle = %q, want Access Events Export", doc.Subtitle)
	}
	if len(doc.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(doc.Rows))
	}
	if len(doc.Headers) != 8 {
		t.Errorf("expected 8 headers, got %d", len(doc.Headers))
	}
	if len(doc.Meta) != 3 {
		t.Errorf("expected 3 meta lines, got %d", len(doc.Meta))
	}
	// Verify reason field
	if doc.Rows[0][6] != "" {
		t.Errorf("row 0 reason = %q, want empty", doc.Rows[0][6])
	}
	if doc.Rows[1][6] != "ANTI_PASSBACK" {
		t.Errorf("row 1 reason = %q, want ANTI_PASSBACK", doc.Rows[1][6])
	}
}

func TestEventsDocument_Empty(t *testing.T) {
	doc := EventsDocument("HR", "org-2", "2026-01-01", "2026-01-31", nil)

	if len(doc.Rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(doc.Rows))
	}
	if doc.Meta[2].Value != "0" {
		t.Errorf("total events meta = %q, want 0", doc.Meta[2].Value)
	}
}

func TestPersonalDocument_Basic(t *testing.T) {
	resp := &model.PersonalReportResponse{
		UserID:    "user-1",
		StartDate: "2026-01-01",
		EndDate:   "2026-01-31",
		TotalDays: 2,
		DailyRecords: []model.DailyRecord{
			{Date: "2026-01-01", FirstIn: "08:30:00", LastOut: "17:30:00", TotalEntries: 1, TotalExits: 1, HoursWorked: 9.0},
			{Date: "2026-01-02", FirstIn: "09:00:00", LastOut: "18:00:00", TotalEntries: 2, TotalExits: 2, HoursWorked: 9.0},
		},
	}

	doc := PersonalDocument(resp)

	if doc.Subtitle != "Personal Attendance" {
		t.Errorf("Subtitle = %q, want Personal Attendance", doc.Subtitle)
	}
	if len(doc.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(doc.Rows))
	}
	if doc.Rows[0][0] != "2026-01-01" {
		t.Errorf("first row date = %q, want 2026-01-01", doc.Rows[0][0])
	}
	if len(doc.Headers) != 6 {
		t.Errorf("expected 6 headers, got %d", len(doc.Headers))
	}
}

func TestDepartmentDocument_Basic(t *testing.T) {
	resp := &model.DepartmentReportResponse{
		OrgUnitID:   "org-1",
		OrgUnitName: "Engineering",
		StartDate:   "2026-01-01",
		EndDate:     "2026-01-31",
		Granularity: "daily",
		Summary: model.DepartmentSummary{
			TotalEntries: 100,
			TotalExits:   95,
		},
		Periods: []model.PeriodReport{
			{PeriodStart: "2026-01-01", PeriodEnd: "2026-01-01", TotalEntries: 50, TotalExits: 48, UniqueEmployees: 10, AvgHours: 8.5, LateRate: 0.1},
			{PeriodStart: "2026-01-02", PeriodEnd: "2026-01-02", TotalEntries: 50, TotalExits: 47, UniqueEmployees: 12, AvgHours: 7.9, LateRate: 0.15},
		},
	}

	doc := DepartmentDocument(resp)

	if doc.Subtitle != "Department Summary" {
		t.Errorf("Subtitle = %q, want Department Summary", doc.Subtitle)
	}
	if len(doc.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(doc.Rows))
	}
	if len(doc.Headers) != 7 {
		t.Errorf("expected 7 headers, got %d", len(doc.Headers))
	}
	if len(doc.Meta) != 5 {
		t.Errorf("expected 5 meta lines, got %d", len(doc.Meta))
	}
}

func TestDepartmentDocument_EmptyPeriods(t *testing.T) {
	resp := &model.DepartmentReportResponse{
		OrgUnitID:   "org-1",
		OrgUnitName: "Engineering",
		Summary:     model.DepartmentSummary{},
	}

	doc := DepartmentDocument(resp)

	if len(doc.Rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(doc.Rows))
	}
}
