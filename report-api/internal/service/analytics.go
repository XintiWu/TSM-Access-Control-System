package service

import (
	"context"
	"fmt"

	"github.com/tsmc/report-api/internal/auth"
	"github.com/tsmc/report-api/internal/model"
	"github.com/tsmc/report-api/internal/repository"
)

// GetDoorHeatmap returns real-time door swipe ranking for the requester's company scope.
func (s *ReportService) GetDoorHeatmap(ctx context.Context, minutes int, role auth.ReportRole) (*model.DoorHeatmapResponse, error) {
	if !role.CanViewDepartmentReports() {
		return nil, fmt.Errorf("access denied: role %s cannot view door analytics", role)
	}
	if minutes < 1 {
		minutes = 60
	}
	if minutes > 24*60 {
		minutes = 24 * 60
	}
	rows, err := s.reportRepo.GetDoorHeatmap(ctx, minutes)
	if err != nil {
		return nil, fmt.Errorf("door heatmap: %w", err)
	}
	doors := make([]model.DoorTrafficRow, 0, len(rows))
	for _, r := range rows {
		doors = append(doors, model.DoorTrafficRow{
			DoorID: r.DoorID, DoorName: r.DoorName, Site: r.Site, SwipeCount: r.SwipeCount,
		})
	}
	return &model.DoorHeatmapResponse{WindowMinutes: minutes, Doors: doors}, nil
}

// GetAttendanceTrends returns avg hours and late rate series for charts.
func (s *ReportService) GetAttendanceTrends(ctx context.Context, req model.AttendanceTrendsRequest, requesterOrgUnitID string, role auth.ReportRole) (*model.AttendanceTrendsResponse, error) {
	if !role.CanViewDepartmentReports() {
		return nil, fmt.Errorf("access denied: role %s cannot view attendance trends", role)
	}
	inSubtree, err := s.orgRepo.IsInSubtree(ctx, requesterOrgUnitID, req.OrgUnitID)
	if err != nil {
		return nil, err
	}
	if !inSubtree {
		return nil, fmt.Errorf("access denied: orgUnitId %s is not in your subtree", req.OrgUnitID)
	}
	granularity := req.Granularity
	if granularity == "" {
		granularity = "monthly"
	}
	orgUnit, err := s.orgRepo.GetOrgUnit(ctx, req.OrgUnitID)
	if err != nil || orgUnit == nil {
		return nil, fmt.Errorf("org unit not found: %s", req.OrgUnitID)
	}
	subtreeIDs, err := s.orgRepo.GetSubtreeIDs(ctx, req.OrgUnitID)
	if err != nil {
		return nil, fmt.Errorf("get subtree: %w", err)
	}
	daily, err := s.reportRepo.GetAttendanceTrends(ctx, subtreeIDs, req.StartDate, req.EndDate)
	if err != nil {
		return nil, fmt.Errorf("attendance trends: %w", err)
	}
	aggRows := make([]model.AggregatedRow, len(daily))
	for i, d := range daily {
		aggRows[i] = model.AggregatedRow{
			ReportDate: d.PeriodStart,
			AvgHours:   d.AvgHours,
		}
	}
	periods := buildPeriods(aggRows, granularity, req.StartDate, req.EndDate)
	periods = repository.MergeTrendsIntoPeriods(periods, daily, granularity)

	series := make([]model.AttendancePoint, 0, len(periods))
	for _, p := range periods {
		series = append(series, model.AttendancePoint{
			PeriodStart: p.PeriodStart,
			PeriodEnd:   p.PeriodEnd,
			AvgHours:    p.AvgHours,
			LateRate:    p.LateRate,
		})
	}
	return &model.AttendanceTrendsResponse{
		OrgUnitID: req.OrgUnitID, OrgUnitName: orgUnit.Name,
		StartDate: req.StartDate, EndDate: req.EndDate,
		Granularity: granularity, Series: series,
	}, nil
}
