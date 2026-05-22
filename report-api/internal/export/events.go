package export

import (
	"fmt"
	"time"

	"github.com/tsmc/report-api/internal/model"
)

// EventsDocument builds an access events export table.
func EventsDocument(orgUnitName, orgUnitID, startDate, endDate string, events []model.InOutEvent) Document {
	meta := []MetaLine{
		{Label: "Org Unit", Value: fmt.Sprintf("%s (%s)", orgUnitName, orgUnitID)},
		{Label: "Period", Value: fmt.Sprintf("%s to %s", startDate, endDate)},
		{Label: "Total Events", Value: fmt.Sprintf("%d", len(events))},
	}
	headers := []string{"Event ID", "Employee ID", "Door ID", "Direction", "Event Time", "Status", "Reason", "Source IP"}
	rows := make([][]string, 0, len(events))
	for _, e := range events {
		reason := ""
		if e.Reason != nil {
			reason = *e.Reason
		}
		rows = append(rows, []string{
			e.EventID, e.EmployeeID, e.DoorID, e.Direction,
			e.EventTime.Format(time.RFC3339), e.Status, reason, e.SourceIP,
		})
	}
	return Document{
		Title: "TSMC Access Control Report", Subtitle: "Access Events Export",
		GeneratedAt: time.Now().UTC(), Meta: meta, Headers: headers, Rows: rows,
		FooterNote: "Internal use only",
	}
}

func PersonalDocument(resp *model.PersonalReportResponse) Document {
	meta := []MetaLine{
		{Label: "Employee", Value: resp.UserID},
		{Label: "Period", Value: fmt.Sprintf("%s to %s", resp.StartDate, resp.EndDate)},
		{Label: "Days with activity", Value: fmt.Sprintf("%d", resp.TotalDays)},
	}
	headers := []string{"Date", "First In", "Last Out", "Entries", "Exits", "Hours Worked"}
	rows := make([][]string, 0, len(resp.DailyRecords))
	for _, d := range resp.DailyRecords {
		rows = append(rows, []string{
			d.Date, d.FirstIn, d.LastOut,
			fmt.Sprintf("%d", d.TotalEntries), fmt.Sprintf("%d", d.TotalExits),
			fmt.Sprintf("%.2f", d.HoursWorked),
		})
	}
	return Document{
		Title: "TSMC Access Control Report", Subtitle: "Personal Attendance",
		GeneratedAt: time.Now().UTC(), Meta: meta, Headers: headers, Rows: rows,
		FooterNote: "Internal use only",
	}
}

func DepartmentDocument(resp *model.DepartmentReportResponse) Document {
	meta := []MetaLine{
		{Label: "Org Unit", Value: fmt.Sprintf("%s (%s)", resp.OrgUnitName, resp.OrgUnitID)},
		{Label: "Period", Value: fmt.Sprintf("%s to %s", resp.StartDate, resp.EndDate)},
		{Label: "Granularity", Value: resp.Granularity},
		{Label: "Total Entries", Value: fmt.Sprintf("%d", resp.Summary.TotalEntries)},
		{Label: "Total Exits", Value: fmt.Sprintf("%d", resp.Summary.TotalExits)},
	}
	headers := []string{"Period Start", "Period End", "Entries", "Exits", "Unique Employees", "Avg Hours", "Late Rate"}
	rows := make([][]string, 0, len(resp.Periods))
	for _, p := range resp.Periods {
		rows = append(rows, []string{
			p.PeriodStart, p.PeriodEnd,
			fmt.Sprintf("%d", p.TotalEntries), fmt.Sprintf("%d", p.TotalExits),
			fmt.Sprintf("%d", p.UniqueEmployees), fmt.Sprintf("%.2f", p.AvgHours),
			fmt.Sprintf("%.2f%%", p.LateRate*100),
		})
	}
	return Document{
		Title: "TSMC Access Control Report", Subtitle: "Department Summary",
		GeneratedAt: time.Now().UTC(), Meta: meta, Headers: headers, Rows: rows,
		FooterNote: "Internal use only",
	}
}
