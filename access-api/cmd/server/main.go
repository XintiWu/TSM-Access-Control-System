package main

import (
	"context"
	"log"
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
	"github.com/tsmc/access-api/internal/queue"
	"github.com/tsmc/access-api/internal/repository"
	"github.com/tsmc/access-api/internal/service"
)

func main() {
	cfg := config.Load()

	redisCache := cache.NewRedisCache(cfg.RedisAddr)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := redisCache.Ping(ctx); err != nil {
		log.Fatalf("redis ping failed: %v", err)
	}

	publisher := queue.NewKafkaProducer(cfg.KafkaBrokers, cfg.KafkaTopic)
	defer publisher.Close()

	decisions := service.NewAccessDecisionService(redisCache)

	// Optional: DB fallback for when Redis is unavailable (§8 Resilience)
	if cfg.DBDSN != "" {
		repo, err := repository.NewEmployeeRepository(cfg.DBDSN)
		if err != nil {
			log.Printf("WARNING: DB fallback disabled — cannot connect to MariaDB: %v", err)
		} else {
			decisions.SetDBFallback(repo)
			defer repo.Close()
			log.Printf("DB fallback enabled for Redis-down resilience")
		}
	}

	accessHandler := handler.NewAccessHandler(decisions, redisCache, publisher)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())

	r.GET("/health", func(c *gin.Context) {
		if err := redisCache.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "redis": err.Error()})
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
		log.Printf("access-api listening on %s", cfg.HTTPAddr)
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
