package export

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tsmc/report-api/internal/model"
)

func TestEventsDocument(t *testing.T) {
	reasonStr := "Unauthorized Card"
	events := []model.InOutEvent{
		{
			EventID:    "evt-123",
			EmployeeID: "emp-456",
			DoorID:     "door-789",
			Direction:  "IN",
			EventTime:  time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC),
			Status:     "DENIED",
			Reason:     &reasonStr,
			SourceIP:   "192.168.1.100",
		},
		{
			EventID:    "evt-124",
			EmployeeID: "emp-456",
			DoorID:     "door-789",
			Direction:  "OUT",
			EventTime:  time.Date(2026, 5, 30, 18, 0, 0, 0, time.UTC),
			Status:     "SUCCESS",
			Reason:     nil,
			SourceIP:   "192.168.1.100",
		},
	}

	doc := EventsDocument("FAB 12", "org-1", "2026-05-30", "2026-05-31", events)

	assert.Equal(t, "TSMC Access Control Report", doc.Title)
	assert.Equal(t, "Access Events Export", doc.Subtitle)
	assert.Len(t, doc.Meta, 3)
	assert.Equal(t, "Org Unit", doc.Meta[0].Label)
	assert.Equal(t, "FAB 12 (org-1)", doc.Meta[0].Value)
	assert.Len(t, doc.Rows, 2)
	assert.Equal(t, "evt-123", doc.Rows[0][0])
	assert.Equal(t, "Unauthorized Card", doc.Rows[0][6])
	assert.Equal(t, "", doc.Rows[1][6]) // Nil reason should map to empty string
}

func TestPersonalDocument(t *testing.T) {
	resp := &model.PersonalReportResponse{
		UserID:      "emp-456",
		StartDate:   "2026-05-30",
		EndDate:     "2026-05-30",
		TotalDays:   1,
		DailyRecords: []model.DailyRecord{
			{
				Date:         "2026-05-30",
				FirstIn:      "09:00:00",
				LastOut:      "18:00:00",
				TotalEntries: 1,
				TotalExits:   1,
				HoursWorked:  9.0,
			},
		},
	}

	doc := PersonalDocument(resp)

	assert.Equal(t, "TSMC Access Control Report", doc.Title)
	assert.Equal(t, "Personal Attendance", doc.Subtitle)
	assert.Len(t, doc.Meta, 3)
	assert.Equal(t, "Employee", doc.Meta[0].Label)
	assert.Equal(t, "emp-456", doc.Meta[0].Value)
	assert.Len(t, doc.Rows, 1)
	assert.Equal(t, "2026-05-30", doc.Rows[0][0])
	assert.Equal(t, "9.00", doc.Rows[0][5])
}

func TestDepartmentDocument(t *testing.T) {
	resp := &model.DepartmentReportResponse{
		OrgUnitID:   "org-1",
		OrgUnitName: "FAB 12",
		StartDate:   "2026-05-30",
		EndDate:     "2026-05-30",
		Granularity: "DAILY",
		Summary: model.DepartmentSummary{
			TotalEntries: 10,
			TotalExits:   10,
		},
		Periods: []model.PeriodReport{
			{
				PeriodStart:     "2026-05-30",
				PeriodEnd:       "2026-05-30",
				TotalEntries:    10,
				TotalExits:      10,
				UniqueEmployees: 5,
				AvgHours:        8.5,
				LateRate:        0.2,
			},
		},
	}

	doc := DepartmentDocument(resp)

	assert.Equal(t, "TSMC Access Control Report", doc.Title)
	assert.Equal(t, "Department Summary", doc.Subtitle)
	assert.Len(t, doc.Meta, 5)
	assert.Equal(t, "DAILY", doc.Meta[2].Value)
	assert.Len(t, doc.Rows, 1)
	assert.Equal(t, "2026-05-30", doc.Rows[0][0])
	assert.Equal(t, "20.00%", doc.Rows[0][6]) // Late rate formatted as %
}
