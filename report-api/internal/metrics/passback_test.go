package metrics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tsmc/report-api/internal/model"
	"github.com/tsmc/report-api/internal/repository"
)

type mockReportRepo struct {
	getPassbackFn func(ctx context.Context) ([]repository.PassbackDenyRow, error)
}

func (m mockReportRepo) GetAggregated(ctx context.Context, orgUnitIDs []string, startDate, endDate string) ([]model.AggregatedRow, error) {
	return nil, nil
}
func (m mockReportRepo) GetSummary(ctx context.Context, orgUnitIDs []string, startDate, endDate string) (model.DepartmentSummary, error) {
	return model.DepartmentSummary{}, nil
}
func (m mockReportRepo) GetOnSiteCount(ctx context.Context, orgUnitIDs []string) (int, error) {
	return 0, nil
}
func (m mockReportRepo) GetDoorHeatmap(ctx context.Context, orgUnitIDs []string, minutes int) ([]repository.DoorHeatmapRow, error) {
	return nil, nil
}
func (m mockReportRepo) GetAttendanceTrends(ctx context.Context, orgUnitIDs []string, startDate, endDate string) ([]repository.PeriodAttendanceMetrics, error) {
	return nil, nil
}
func (m mockReportRepo) GetPassbackDenyCountsLastMinute(ctx context.Context) ([]repository.PassbackDenyRow, error) {
	if m.getPassbackFn != nil {
		return m.getPassbackFn(ctx)
	}
	return nil, nil
}
func (m mockReportRepo) Close() error { return nil }

func TestPoll_Success(t *testing.T) {
	mock := mockReportRepo{
		getPassbackFn: func(ctx context.Context) ([]repository.PassbackDenyRow, error) {
			return []repository.PassbackDenyRow{
				{DoorID: "door-1", DoorName: "Front Gate", Count: 5},
				{DoorID: "door-2", DoorName: "Back Gate", Count: 12},
			}, nil
		},
	}

	poll(context.Background(), mock)
	// Just verify it executed without panic or issue
}

func TestPoll_Error(t *testing.T) {
	mock := mockReportRepo{
		getPassbackFn: func(ctx context.Context) ([]repository.PassbackDenyRow, error) {
			return nil, errors.New("db error")
		},
	}

	poll(context.Background(), mock)
	// Verify it executes without panic when db returns an error
}

func TestStartPassbackPoller(t *testing.T) {
	mock := mockReportRepo{
		getPassbackFn: func(ctx context.Context) ([]repository.PassbackDenyRow, error) {
			return nil, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	StartPassbackPoller(ctx, mock, 5*time.Millisecond)

	time.Sleep(15 * time.Millisecond)
	cancel()
	time.Sleep(5 * time.Millisecond)
}
