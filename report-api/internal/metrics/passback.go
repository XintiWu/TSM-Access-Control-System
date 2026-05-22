package metrics

import (
	"context"
	"log"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/tsmc/report-api/internal/repository"
)

var (
	passbackDeny1m = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "report_passback_deny_1m",
		Help: "ANTI_PASSBACK deny events in the last minute per door (from ClickHouse)",
	}, []string{"door_id", "door_name"})

	passbackDenyMax1m = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "report_passback_deny_1m_max",
		Help: "Maximum ANTI_PASSBACK deny count across doors in the last minute",
	})
)

// StartPassbackPoller periodically refreshes passback deny gauges from ClickHouse.
func StartPassbackPoller(ctx context.Context, repo *repository.ReportRepository, interval time.Duration) {
	if interval < 5*time.Second {
		interval = 15 * time.Second
	}
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				poll(ctx, repo)
			}
		}
	}()
}

func poll(ctx context.Context, repo *repository.ReportRepository) {
	rows, err := repo.GetPassbackDenyCountsLastMinute(ctx)
	if err != nil {
		log.Printf("passback metrics poll: %v", err)
		return
	}
	passbackDeny1m.Reset()
	var max uint64
	for _, row := range rows {
		passbackDeny1m.WithLabelValues(row.DoorID, row.DoorName).Set(float64(row.Count))
		if row.Count > max {
			max = row.Count
		}
	}
	passbackDenyMax1m.Set(float64(max))
}
