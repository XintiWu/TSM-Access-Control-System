package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"time"

	"github.com/tsmc/report-api/internal/auth"
	"github.com/tsmc/report-api/internal/cache"
	"github.com/tsmc/report-api/internal/export"
	"github.com/tsmc/report-api/internal/model"
	"github.com/tsmc/report-api/internal/repository"
)

// ReportService implements the business logic for all report endpoints.
type ReportService struct {
	orgRepo    *repository.OrgRepository
	reportRepo *repository.ReportRepository
	inoutRepo  *repository.InOutRepository
	cache      *cache.ReportCache
	jobs       *export.JobStore
}

// NewReportService creates a new ReportService with all required dependencies.
func NewReportService(
	orgRepo *repository.OrgRepository,
	reportRepo *repository.ReportRepository,
	inoutRepo *repository.InOutRepository,
	reportCache *cache.ReportCache,
	jobs *export.JobStore,
) *ReportService {
	return &ReportService{
		orgRepo:    orgRepo,
		reportRepo: reportRepo,
		inoutRepo:  inoutRepo,
		cache:      reportCache,
		jobs:       jobs,
	}
}

// ────────────────────────────────────────
// Personal Report
// ────────────────────────────────────────

// GetPersonalReport builds a daily attendance summary for a single employee.
func (s *ReportService) GetPersonalReport(ctx context.Context, userID, startDate, endDate string) (*model.PersonalReportResponse, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("report:personal:%s:%s:%s", userID, startDate, endDate)
	if cached, err := s.cache.Get(ctx, cacheKey); err == nil && cached != nil {
		var resp model.PersonalReportResponse
		if json.Unmarshal(cached, &resp) == nil {
			return &resp, nil
		}
	}

	events, err := s.inoutRepo.GetPersonalEvents(ctx, userID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("get personal events: %w", err)
	}

	// Group events by date
	dayMap := make(map[string]*model.DailyRecord)
	for _, e := range events {
		dateStr := e.EventTime.Format("2006-01-02")
		rec, ok := dayMap[dateStr]
		if !ok {
			rec = &model.DailyRecord{Date: dateStr}
			dayMap[dateStr] = rec
		}
		timeStr := e.EventTime.Format("15:04:05")
		if e.Direction == "IN" {
			rec.TotalEntries++
			if rec.FirstIn == "" || timeStr < rec.FirstIn {
				rec.FirstIn = timeStr
			}
		} else if e.Direction == "OUT" {
			rec.TotalExits++
			if rec.LastOut == "" || timeStr > rec.LastOut {
				rec.LastOut = timeStr
			}
		}
	}

	// Calculate hours worked per day
	for _, rec := range dayMap {
		if rec.FirstIn != "" && rec.LastOut != "" {
			firstIn, _ := time.Parse("15:04:05", rec.FirstIn)
			lastOut, _ := time.Parse("15:04:05", rec.LastOut)
			hours := lastOut.Sub(firstIn).Hours()
			if hours > 0 {
				rec.HoursWorked = math.Round(hours*100) / 100
			}
		}
	}

	// Sort by date
	start, _ := time.Parse("2006-01-02", startDate)
	end, _ := time.Parse("2006-01-02", endDate)
	var records []model.DailyRecord
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")
		if rec, ok := dayMap[dateStr]; ok {
			records = append(records, *rec)
		}
	}

	resp := &model.PersonalReportResponse{
		UserID:       userID,
		StartDate:    startDate,
		EndDate:      endDate,
		TotalDays:    len(records),
		DailyRecords: records,
	}

	// Write to cache (best-effort)
	if data, err := json.Marshal(resp); err == nil {
		if err := s.cache.Set(ctx, cacheKey, data); err != nil {
			log.Printf("cache set personal report: %v", err)
		}
	}

	return resp, nil
}

// ────────────────────────────────────────
// Department Report
// ────────────────────────────────────────

// GetDepartmentReport builds a hierarchical department report.
// The requesterOrgUnitID is used for permission enforcement — the target orgUnitId
// must be within the requester's subtree.
func (s *ReportService) GetDepartmentReport(ctx context.Context, req model.DepartmentReportRequest, requesterOrgUnitID string) (*model.DepartmentReportResponse, error) {
	// Permission check
	inSubtree, err := s.orgRepo.IsInSubtree(ctx, requesterOrgUnitID, req.OrgUnitID)
	if err != nil {
		return nil, fmt.Errorf("check subtree: %w", err)
	}
	if !inSubtree {
		return nil, fmt.Errorf("access denied: orgUnitId %s is not in your subtree", req.OrgUnitID)
	}

	granularity := req.Granularity
	if granularity == "" {
		granularity = "daily"
	}

	// Check cache
	cacheKey := fmt.Sprintf("report:dept:%s:%s:%s:%s", req.OrgUnitID, req.StartDate, req.EndDate, granularity)
	if cached, err := s.cache.Get(ctx, cacheKey); err == nil && cached != nil {
		var resp model.DepartmentReportResponse
		if json.Unmarshal(cached, &resp) == nil {
			return &resp, nil
		}
	}

	// Get target org unit info
	orgUnit, err := s.orgRepo.GetOrgUnit(ctx, req.OrgUnitID)
	if err != nil || orgUnit == nil {
		return nil, fmt.Errorf("org unit not found: %s", req.OrgUnitID)
	}

	// Get all org units in the subtree
	subtreeIDs, err := s.orgRepo.GetSubtreeIDs(ctx, req.OrgUnitID)
	if err != nil {
		return nil, fmt.Errorf("get subtree: %w", err)
	}

	// Get aggregated data
	aggRows, err := s.reportRepo.GetAggregated(ctx, subtreeIDs, req.StartDate, req.EndDate)
	if err != nil {
		return nil, fmt.Errorf("get aggregated: %w", err)
	}

	// Build summary
	summary, err := s.reportRepo.GetSummary(ctx, subtreeIDs, req.StartDate, req.EndDate)
	if err != nil {
		return nil, fmt.Errorf("get summary: %w", err)
	}
	headcount, err := s.orgRepo.CountActiveEmployees(ctx, subtreeIDs)
	if err != nil {
		return nil, fmt.Errorf("headcount: %w", err)
	}
	summary.Headcount = headcount
	summary.WorkforceUtilization = calcUtilization(summary.UniqueEmployees, headcount)

	// Build periods based on granularity
	periods := buildPeriods(aggRows, granularity, req.StartDate, req.EndDate)
	if dailyTrends, err := s.reportRepo.GetAttendanceTrends(ctx, subtreeIDs, req.StartDate, req.EndDate); err == nil {
		periods = repository.MergeTrendsIntoPeriods(periods, dailyTrends, granularity)
	}

	// Build sub-unit summaries (direct children only)
	childUnits, err := s.orgRepo.GetChildUnits(ctx, req.OrgUnitID)
	if err != nil {
		return nil, fmt.Errorf("get children: %w", err)
	}
	var subUnits []model.SubUnitSummary
	for _, child := range childUnits {
		childSubtreeIDs, err := s.orgRepo.GetSubtreeIDs(ctx, child.ID)
		if err != nil {
			continue
		}
		childSummary, err := s.reportRepo.GetSummary(ctx, childSubtreeIDs, req.StartDate, req.EndDate)
		if err != nil {
			continue
		}
		subUnits = append(subUnits, model.SubUnitSummary{
			OrgUnitID:    child.ID,
			OrgUnitName:  child.Name,
			TotalEntries: childSummary.TotalEntries,
			TotalExits:   childSummary.TotalExits,
		})
	}

	resp := &model.DepartmentReportResponse{
		OrgUnitID:   req.OrgUnitID,
		OrgUnitName: orgUnit.Name,
		StartDate:   req.StartDate,
		EndDate:     req.EndDate,
		Granularity: granularity,
		Summary:     summary,
		Periods:     periods,
		SubUnits:    subUnits,
	}

	// Write to cache
	if data, err := json.Marshal(resp); err == nil {
		if err := s.cache.Set(ctx, cacheKey, data); err != nil {
			log.Printf("cache set dept report: %v", err)
		}
	}

	return resp, nil
}

// buildPeriods groups aggregated rows into periods based on granularity.
func buildPeriods(rows []model.AggregatedRow, granularity, startDate, endDate string) []model.PeriodReport {
	if len(rows) == 0 {
		return nil
	}

	switch granularity {
	case "weekly":
		return groupByWeek(rows, startDate, endDate)
	case "monthly":
		return groupByMonth(rows, startDate, endDate)
	case "quarterly":
		return groupByQuarter(rows, startDate, endDate)
	case "yearly":
		return groupByYear(rows, startDate, endDate)
	default: // daily
		return groupByDay(rows)
	}
}

func calcUtilization(uniquePresent, headcount int) float64 {
	if headcount <= 0 {
		return 0
	}
	u := float64(uniquePresent) / float64(headcount)
	if u > 1 {
		return 1
	}
	return math.Round(u*10000) / 10000
}

func groupByDay(rows []model.AggregatedRow) []model.PeriodReport {
	// Merge rows with the same date (from different org units)
	dayMap := make(map[string]*model.PeriodReport)
	var order []string
	for _, r := range rows {
		p, ok := dayMap[r.ReportDate]
		if !ok {
			p = &model.PeriodReport{PeriodStart: r.ReportDate, PeriodEnd: r.ReportDate}
			dayMap[r.ReportDate] = p
			order = append(order, r.ReportDate)
		}
		p.TotalEntries += r.TotalEntries
		p.TotalExits += r.TotalExits
		p.UniqueEmployees += r.UniqueEmployees
		if r.AvgHours > 0 {
			p.AvgHours = r.AvgHours // simplified: last wins
		}
	}
	var periods []model.PeriodReport
	for _, d := range order {
		periods = append(periods, *dayMap[d])
	}
	return periods
}

func groupByWeek(rows []model.AggregatedRow, startDate, endDate string) []model.PeriodReport {
	start, _ := time.Parse("2006-01-02", startDate)
	end, _ := time.Parse("2006-01-02", endDate)

	// Build week boundaries
	type weekBucket struct {
		start time.Time
		end   time.Time
		p     model.PeriodReport
	}
	var buckets []weekBucket
	ws := start
	for ws.Before(end) || ws.Equal(end) {
		we := ws.AddDate(0, 0, 6)
		if we.After(end) {
			we = end
		}
		buckets = append(buckets, weekBucket{start: ws, end: we, p: model.PeriodReport{
			PeriodStart: ws.Format("2006-01-02"),
			PeriodEnd:   we.Format("2006-01-02"),
		}})
		ws = we.AddDate(0, 0, 1)
	}

	for _, r := range rows {
		rd, _ := time.Parse("2006-01-02", r.ReportDate)
		for i := range buckets {
			if (rd.Equal(buckets[i].start) || rd.After(buckets[i].start)) &&
				(rd.Equal(buckets[i].end) || rd.Before(buckets[i].end)) {
				buckets[i].p.TotalEntries += r.TotalEntries
				buckets[i].p.TotalExits += r.TotalExits
				buckets[i].p.UniqueEmployees += r.UniqueEmployees
				break
			}
		}
	}

	var periods []model.PeriodReport
	for _, b := range buckets {
		periods = append(periods, b.p)
	}
	return periods
}

func groupByMonth(rows []model.AggregatedRow, startDate, endDate string) []model.PeriodReport {
	monthMap := make(map[string]*model.PeriodReport) // key: "2006-01"
	var order []string
	for _, r := range rows {
		rd, _ := time.Parse("2006-01-02", r.ReportDate)
		monthKey := rd.Format("2006-01")
		p, ok := monthMap[monthKey]
		if !ok {
			firstDay := time.Date(rd.Year(), rd.Month(), 1, 0, 0, 0, 0, time.UTC)
			lastDay := firstDay.AddDate(0, 1, -1)
			p = &model.PeriodReport{
				PeriodStart: firstDay.Format("2006-01-02"),
				PeriodEnd:   lastDay.Format("2006-01-02"),
			}
			monthMap[monthKey] = p
			order = append(order, monthKey)
		}
		p.TotalEntries += r.TotalEntries
		p.TotalExits += r.TotalExits
		p.UniqueEmployees += r.UniqueEmployees
	}
	var periods []model.PeriodReport
	for _, k := range order {
		periods = append(periods, *monthMap[k])
	}
	return periods
}

func groupByQuarter(rows []model.AggregatedRow, startDate, endDate string) []model.PeriodReport {
	quarterMap := make(map[string]*model.PeriodReport)
	var order []string
	for _, r := range rows {
		rd, _ := time.Parse("2006-01-02", r.ReportDate)
		q := (int(rd.Month())-1)/3 + 1
		key := fmt.Sprintf("%d-Q%d", rd.Year(), q)
		p, ok := quarterMap[key]
		if !ok {
			monthStart := time.Date(rd.Year(), time.Month((q-1)*3+1), 1, 0, 0, 0, 0, time.UTC)
			monthEnd := monthStart.AddDate(0, 3, -1)
			p = &model.PeriodReport{
				PeriodStart: monthStart.Format("2006-01-02"),
				PeriodEnd:   monthEnd.Format("2006-01-02"),
			}
			quarterMap[key] = p
			order = append(order, key)
		}
		p.TotalEntries += r.TotalEntries
		p.TotalExits += r.TotalExits
		p.UniqueEmployees += r.UniqueEmployees
	}
	var periods []model.PeriodReport
	for _, k := range order {
		periods = append(periods, *quarterMap[k])
	}
	_ = startDate
	_ = endDate
	return periods
}

func groupByYear(rows []model.AggregatedRow, startDate, endDate string) []model.PeriodReport {
	yearMap := make(map[string]*model.PeriodReport)
	var order []string
	for _, r := range rows {
		rd, _ := time.Parse("2006-01-02", r.ReportDate)
		key := fmt.Sprintf("%d", rd.Year())
		p, ok := yearMap[key]
		if !ok {
			p = &model.PeriodReport{
				PeriodStart: fmt.Sprintf("%d-01-01", rd.Year()),
				PeriodEnd:   fmt.Sprintf("%d-12-31", rd.Year()),
			}
			yearMap[key] = p
			order = append(order, key)
		}
		p.TotalEntries += r.TotalEntries
		p.TotalExits += r.TotalExits
		p.UniqueEmployees += r.UniqueEmployees
	}
	var periods []model.PeriodReport
	for _, k := range order {
		periods = append(periods, *yearMap[k])
	}
	_ = startDate
	_ = endDate
	return periods
}

// ────────────────────────────────────────
// Audit Log
// ────────────────────────────────────────

// GetAuditLog returns a paginated list of raw events filtered by the requester's org subtree.
func (s *ReportService) GetAuditLog(ctx context.Context, req model.AuditLogRequest, requesterUserID, requesterOrgUnitID string, role auth.ReportRole) (*model.AuditLogResponse, error) {
	var subtreeIDs []string
	var err error
	if role.CanViewFullAudit() {
		subtreeIDs, err = s.orgRepo.GetSubtreeIDs(ctx, requesterOrgUnitID)
		if err != nil {
			return nil, fmt.Errorf("get subtree: %w", err)
		}
	} else {
		// Employees may only audit their own swipe events.
		req.EmployeeID = requesterUserID
		subtreeIDs, err = s.orgRepo.GetSubtreeIDs(ctx, requesterOrgUnitID)
		if err != nil {
			return nil, fmt.Errorf("get subtree: %w", err)
		}
	}

	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 200 {
		req.PageSize = 50
	}

	filter := repository.AuditFilter{
		StartDate:  req.StartDate,
		EndDate:    req.EndDate,
		EmployeeID: req.EmployeeID,
		DoorID:     req.DoorID,
		Status:     req.Status,
		OrgUnitIDs: subtreeIDs,
		Page:       req.Page,
		PageSize:   req.PageSize,
	}

	events, totalCount, err := s.inoutRepo.GetAuditEvents(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("get audit events: %w", err)
	}

	auditEvents := make([]model.AuditEvent, 0, len(events))
	for _, e := range events {
		ae := model.AuditEvent{
			EventID:    e.EventID,
			EmployeeID: e.EmployeeID,
			DoorID:     e.DoorID,
			Direction:  e.Direction,
			EventTime:  e.EventTime.Format(time.RFC3339),
			Status:     e.Status,
		}
		if e.Reason != nil {
			ae.Reason = *e.Reason
		}
		ae.SourceIP = e.SourceIP
		auditEvents = append(auditEvents, ae)
	}

	return &model.AuditLogResponse{
		Events:     auditEvents,
		Page:       req.Page,
		PageSize:   req.PageSize,
		TotalCount: totalCount,
	}, nil
}

// ────────────────────────────────────────
// CSV Export
// ────────────────────────────────────────

// ExportCSV generates a CSV file for events within the requester's org subtree.
func (s *ReportService) ExportCSV(ctx context.Context, req model.ExportRequest, requesterOrgUnitID string, role auth.ReportRole) (io.Reader, error) {
	if !role.CanViewDepartmentReports() {
		return nil, fmt.Errorf("access denied: role %s cannot export org reports", role)
	}
	// Permission check
	inSubtree, err := s.orgRepo.IsInSubtree(ctx, requesterOrgUnitID, req.OrgUnitID)
	if err != nil {
		return nil, fmt.Errorf("check subtree: %w", err)
	}
	if !inSubtree {
		return nil, fmt.Errorf("access denied: orgUnitId %s is not in your subtree", req.OrgUnitID)
	}

	subtreeIDs, err := s.orgRepo.GetSubtreeIDs(ctx, req.OrgUnitID)
	if err != nil {
		return nil, fmt.Errorf("get subtree: %w", err)
	}

	events, err := s.inoutRepo.GetEventsForExport(ctx, subtreeIDs, req.StartDate, req.EndDate)
	if err != nil {
		return nil, fmt.Errorf("get export events: %w", err)
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	// Header
	_ = w.Write([]string{"EventID", "EmployeeID", "DoorID", "Direction", "EventTime", "Status", "Reason", "SourceIP"})

	for _, e := range events {
		reason := ""
		if e.Reason != nil {
			reason = *e.Reason
		}
		_ = w.Write([]string{
			e.EventID,
			e.EmployeeID,
			e.DoorID,
			e.Direction,
			e.EventTime.Format(time.RFC3339),
			e.Status,
			reason,
			e.SourceIP,
		})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("csv write: %w", err)
	}

	return &buf, nil
}


// Jobs returns the async export job store (may be nil).
func (s *ReportService) Jobs() *export.JobStore {
	return s.jobs
}
