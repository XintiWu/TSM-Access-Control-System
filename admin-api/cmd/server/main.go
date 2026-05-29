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
	"github.com/tsmc/admin-api/internal/config"
	"github.com/tsmc/admin-api/internal/handler"
	"github.com/tsmc/platform-middleware"
	"github.com/tsmc/admin-api/internal/queue"
	"github.com/tsmc/admin-api/internal/repository"
)

func main() {
	// Configure global slog JSON logger
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg := config.Load()

	var repo *repository.EmployeeRepository
	var err error
	for i := 0; i < 30; i++ {
		repo, err = repository.NewEmployeeRepository(cfg.ClickHouseAddr, cfg.ClickHouseUser, cfg.ClickHousePass)
		if err == nil {
			break
		}
		slog.Warn("waiting for database", "error", err, "attempt", i+1)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		slog.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer repo.Close()

	publisher := queue.NewKafkaProducer(cfg.KafkaBrokers, cfg.KafkaTopic)
	defer publisher.Close()

	adminHandler := handler.NewAdminHandler(repo, publisher)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())
	r.Use(middleware.APIKeyAuth(cfg.APIKey))
	if cfg.RateLimitRPS > 0 {
		r.Use(middleware.RateLimit(cfg.RateLimitRPS))
	}

	r.GET("/health", func(c *gin.Context) {
		if err := repo.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "error": "database unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	admin := r.Group("/admin")
	{
		admin.POST("/employees/:userId/ban", adminHandler.Ban)
		admin.POST("/employees/:userId/unban", adminHandler.Unban)
	}

	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: r}

	go func() {
		slog.Info("admin-api listening", "addr", cfg.HTTPAddr)
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
