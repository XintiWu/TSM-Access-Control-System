package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tsmc/report-api/internal/cache"
	"github.com/tsmc/report-api/internal/config"
	"github.com/tsmc/report-api/internal/export"
	"github.com/tsmc/report-api/internal/handler"
	"github.com/tsmc/report-api/internal/repository"
	"github.com/tsmc/report-api/internal/service"
)

func main() {
	cfg := config.Load()

	// Redis (Report Cache)
	reportCache := cache.NewReportCache(cfg.RedisAddr)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := reportCache.Ping(ctx); err != nil {
		log.Printf("WARNING: Redis unavailable — report cache disabled: %v", err)
	}

	// MariaDB — wait up to 60s for DB readiness
	var db *sql.DB
	var dbErr error
	for i := 0; i < 30; i++ {
		db, dbErr = sql.Open("mysql", cfg.DBDSN)
		if dbErr == nil {
			dbErr = db.Ping()
		}
		if dbErr == nil {
			break
		}
		log.Printf("waiting for database: %v", dbErr)
		time.Sleep(2 * time.Second)
	}
	if dbErr != nil {
		log.Fatalf("database: %v", dbErr)
	}
	defer db.Close()

	// Repositories (share the same DSN, each manages its own pool)
	orgRepo, err := repository.NewOrgRepository(cfg.DBDSN)
	if err != nil {
		log.Fatalf("org repository: %v", err)
	}
	defer orgRepo.Close()

	reportRepo, err := repository.NewReportRepository(cfg.ClickHouseAddr, cfg.ClickHouseUser, cfg.ClickHousePass)
	if err != nil {
		log.Fatalf("report repository: %v", err)
	}
	defer reportRepo.Close()

	inoutRepo, err := repository.NewInOutRepository(cfg.ClickHouseAddr, cfg.ClickHouseUser, cfg.ClickHousePass)
	if err != nil {
		log.Fatalf("inout repository: %v", err)
	}
	defer inoutRepo.Close()

	jobStore, err := export.NewJobStore(cfg.ExportDir)
	if err != nil {
		log.Fatalf("export jobs: %v", err)
	}

	// Service + Handler
	svc := service.NewReportService(orgRepo, reportRepo, inoutRepo, reportCache, jobStore)
	h := handler.NewReportHandler(svc, orgRepo)

	// Router
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())

	r.GET("/health", func(c *gin.Context) {
		if err := db.Ping(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "db": err.Error()})
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
	}

	// Graceful shutdown
	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: r}

	go func() {
		log.Printf("report-api listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
