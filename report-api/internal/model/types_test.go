package model

import (
	"testing"
	"time"
)

func TestRequestTypesInstantiation(t *testing.T) {
	personalReq := PersonalReportRequest{
		StartDate: "2026-05-01",
		EndDate:   "2026-05-30",
	}
	if personalReq.StartDate != "2026-05-01" {
		t.Error("unexpected PersonalReportRequest StartDate")
	}

	deptReq := DepartmentReportRequest{
		OrgUnitID:   "org-1",
		StartDate:   "2026-05-01",
		EndDate:     "2026-05-30",
		Granularity: "daily",
	}
	if deptReq.OrgUnitID != "org-1" {
		t.Error("unexpected DepartmentReportRequest OrgUnitID")
	}

	auditReq := AuditLogRequest{
		StartDate:  "2026-05-01",
		EndDate:    "2026-05-30",
		EmployeeID: "emp-1",
		DoorID:     "door-1",
		Status:     "ALLOW",
		Page:       1,
		PageSize:   50,
	}
	if auditReq.EmployeeID != "emp-1" {
		t.Error("unexpected AuditLogRequest EmployeeID")
	}

	exportReq := ExportRequest{
		Type:        "personal",
		OrgUnitID:   "org-2",
		StartDate:   "2026-05-01",
		EndDate:     "2026-05-30",
		Granularity: "weekly",
		Format:      "pdf",
	}
	if exportReq.Format != "pdf" {
		t.Error("unexpected ExportRequest Format")
	}
}

func TestResponseTypesInstantiation(t *testing.T) {
	personalResp := PersonalReportResponse{
		UserID:    "user-1",
		StartDate: "2026-05-01",
		EndDate:   "2026-05-30",
		TotalDays: 30,
		DailyRecords: []DailyRecord{
			{Date: "2026-05-01", FirstIn: "09:00", LastOut: "18:00", TotalEntries: 2, TotalExits: 2, HoursWorked: 9.0},
		},
	}
	if personalResp.UserID != "user-1" || len(personalResp.DailyRecords) != 1 {
		t.Error("unexpected PersonalReportResponse")
	}

	deptResp := DepartmentReportResponse{
		OrgUnitID:   "org-3",
		OrgUnitName: "Engineering",
		StartDate:   "2026-05-01",
		EndDate:     "2026-05-30",
		Granularity: "daily",
		Summary: DepartmentSummary{
			TotalEntries: 100, TotalExits: 100, UniqueEmployees: 10, Headcount: 12, WorkforceUtilization: 0.83, AvgHoursPerDay: 8.2, LateRate: 0.05,
		},
		Periods: []PeriodReport{
			{PeriodStart: "2026-05-01", PeriodEnd: "2026-05-01", TotalEntries: 50, TotalExits: 50, UniqueEmployees: 10, AvgHours: 8.1, LateRate: 0.0},
		},
		SubUnits: []SubUnitSummary{
			{OrgUnitID: "org-4", OrgUnitName: "QA", TotalEntries: 20, TotalExits: 20},
		},
	}
	if deptResp.OrgUnitName != "Engineering" || deptResp.Summary.TotalEntries != 100 || len(deptResp.Periods) != 1 || len(deptResp.SubUnits) != 1 {
		t.Error("unexpected DepartmentReportResponse")
	}
}

func TestOtherTypesInstantiation(t *testing.T) {
	sec := SecurityDenySummary{
		AntiPassbackDenies: 5,
		PermissionDenied:   2,
	}
	if sec.AntiPassbackDenies != 5 || sec.PermissionDenied != 2 {
		t.Error("unexpected SecurityDenySummary")
	}

	empRow := EmployeeReportRow{
		EmployeeID:         "emp-2",
		TotalSwipes:        20,
		TotalHours:         85.0,
		AntiPassbackDenies: 1,
		PermissionDenied:   0,
		MissingPunchDays:   2,
	}
	if empRow.EmployeeID != "emp-2" {
		t.Error("unexpected EmployeeReportRow")
	}

	row := AggregatedRow{
		ReportDate:      "2026-05-01",
		TotalEntries:    10,
		TotalExits:      10,
		UniqueEmployees: 5,
		AvgHours:        8.0,
	}
	if row.ReportDate != "2026-05-01" {
		t.Error("unexpected AggregatedRow")
	}

	evt := InOutEvent{
		EventID:    "evt-1",
		EmployeeID: "emp-1",
		DoorID:     "door-1",
		Direction:  "IN",
		EventTime:  time.Now(),
		Status:     "ALLOW",
		SourceIP:   "1.1.1.1",
	}
	if evt.EventID != "evt-1" {
		t.Error("unexpected InOutEvent")
	}

	detail := ReportDetailRow{
		EntityID:     "ent-1",
		EntityName:   "Ent 1",
		EntityType:   "Employee",
		TotalSwipes:  15,
		TotalHours:   40.0,
		AnomalyNotes: "None",
	}
	if detail.EntityID != "ent-1" {
		t.Error("unexpected ReportDetailRow")
	}
}
