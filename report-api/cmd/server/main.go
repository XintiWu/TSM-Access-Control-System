package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tsmc/report-api/internal/cache"
	"github.com/tsmc/report-api/internal/config"
	"github.com/tsmc/report-api/internal/export"
	"github.com/tsmc/report-api/internal/handler"
	"github.com/tsmc/report-api/internal/metrics"
	"github.com/tsmc/report-api/internal/middleware"
	"github.com/tsmc/report-api/internal/repository"
	"github.com/tsmc/report-api/internal/service"
)

func main() {
	// Configure global slog JSON logger
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg := config.Load()

	reportCache := cache.NewReportCache(cfg.RedisAddr)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := reportCache.Ping(ctx); err != nil {
		slog.Warn("Redis unavailable — report cache disabled", "error", err)
	}

	var orgRepo repository.OrgRepository
	var err error
	for i := 0; i < 30; i++ {
		orgRepo, err = repository.NewOrgRepository(cfg.ClickHouseAddr, cfg.ClickHouseUser, cfg.ClickHousePass)
		if err == nil {
			break
		}
		slog.Warn("waiting for ClickHouse", "error", err, "attempt", i+1)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		slog.Error("org repository connection failed", "error", err)
		os.Exit(1)
	}
	defer orgRepo.Close()

	reportRepo, err := repository.NewReportRepository(cfg.ClickHouseAddr, cfg.ClickHouseUser, cfg.ClickHousePass)
	if err != nil {
		slog.Error("report repository connection failed", "error", err)
		os.Exit(1)
	}
	defer reportRepo.Close()

	inoutRepo, err := repository.NewInOutRepository(cfg.ClickHouseAddr, cfg.ClickHouseUser, cfg.ClickHousePass)
	if err != nil {
		slog.Error("inout repository connection failed", "error", err)
		os.Exit(1)
	}
	defer inoutRepo.Close()

	jobStore, err := export.NewJobStore(cfg.ExportDir)
	if err != nil {
		slog.Error("export jobs initialization failed", "error", err)
		os.Exit(1)
	}

	svc := service.NewReportService(orgRepo, reportRepo, inoutRepo, reportCache, jobStore)
	h := handler.NewReportHandler(svc, orgRepo)

	metricsCtx, metricsCancel := context.WithCancel(context.Background())
	defer metricsCancel()
	metrics.StartPassbackPoller(metricsCtx, reportRepo, 15*time.Second)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())
	r.Use(middleware.APIKeyAuth(cfg.APIKey))
	if cfg.RateLimitRPS > 0 {
		r.Use(middleware.RateLimit(cfg.RateLimitRPS))
	}

	// Interactive charts UI (department / heatmap / attendance / personal)
	r.Static("/ui", "./report-ui")
	r.GET("/ui", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/ui/")
	})

	r.GET("/health", func(c *gin.Context) {
		if err := orgRepo.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "clickhouse": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	api := r.Group("/reports")
	{
		api.GET("/personal", h.PersonalReport)
		api.GET("/department", h.DepartmentReport)
		api.GET("/audit", h.AuditLog)
		api.GET("/export", h.Export)
		api.POST("/export/jobs", h.ExportJobCreate)
		api.GET("/export/jobs/:jobId", h.ExportJobGet)
		api.GET("/analytics/door-heatmap", h.DoorHeatmap)
		api.GET("/analytics/attendance-trends", h.AttendanceTrends)
		api.GET("/analytics/workforce-utilization", h.WorkforceUtilization)
	}

	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: r}

	go func() {
		slog.Info("report-api listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
}
