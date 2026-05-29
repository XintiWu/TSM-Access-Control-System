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
	"github.com/tsmc/access-api/internal/cache"
	"github.com/tsmc/access-api/internal/config"
	"github.com/tsmc/access-api/internal/handler"
	"github.com/tsmc/platform-middleware"
	"github.com/tsmc/access-api/internal/queue"
	"github.com/tsmc/access-api/internal/repository"
	"github.com/tsmc/access-api/internal/service"
)

func main() {
	// Configure global slog JSON logger
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg := config.Load()

	redisCache := cache.NewRedisCache(cfg.RedisAddr)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := redisCache.Ping(ctx); err != nil {
		slog.Error("redis ping failed", "error", err)
		os.Exit(1)
	}

	outbox, err := queue.NewFileOutbox(cfg.OutboxDir)
	if err != nil {
		slog.Error("outbox initialization failed", "error", err)
		os.Exit(1)
	}
	publisher := queue.NewKafkaProducerWithOutbox(cfg.KafkaBrokers, cfg.KafkaTopic, outbox)
	defer publisher.Close()

	replayCtx, replayCancel := context.WithTimeout(context.Background(), 30*time.Second)
	if n, err := publisher.ReplayOutbox(replayCtx); err != nil {
		slog.Warn("outbox replay warning", "error", err)
	} else if n > 0 {
		slog.Info("replayed events from outbox", "count", n)
	}
	replayCancel()

	decisions := service.NewAccessDecisionService(redisCache)

	// Optional: ClickHouse fallback for when Redis is unavailable (§8 Resilience)
	if cfg.ClickHouseAddr != "" {
		repo, err := repository.NewEmployeeRepository(cfg.ClickHouseAddr, cfg.ClickHouseUser, cfg.ClickHousePass)
		if err != nil {
			slog.Warn("DB fallback disabled — cannot connect to ClickHouse", "error", err)
		} else {
			decisions.SetDBFallback(repo)
			defer repo.Close()
			slog.Info("ClickHouse fallback enabled for Redis-down resilience")
		}
	}

	accessHandler := handler.NewAccessHandler(decisions, redisCache, publisher)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	// Recovery only on hot path — gin.Logger adds I/O latency under shift-change load.
	r.Use(gin.Recovery())
	r.Use(middleware.APIKeyAuth(cfg.APIKey))
	if cfg.RateLimitRPS > 0 {
		r.Use(middleware.RateLimit(cfg.RateLimitRPS))
	}

	r.GET("/health", func(c *gin.Context) {
		if err := redisCache.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "error": "redis unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	api := r.Group("/access")
	{
		api.POST("/swipe", accessHandler.Swipe)
		api.GET("/employee/:userId/state", accessHandler.EmployeeState)
		api.GET("/door/:doorId/status", accessHandler.DoorStatus)
	}

	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: r}

	go func() {
		slog.Info("access-api listening", "addr", cfg.HTTPAddr)
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
