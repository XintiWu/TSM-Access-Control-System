package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tsmc/cache-invalidation-worker/internal/cache"
	"github.com/tsmc/cache-invalidation-worker/internal/config"
	"github.com/tsmc/cache-invalidation-worker/internal/consumer"
)

func main() {
	// Configure global slog JSON logger
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg := config.Load()

	redisCache := cache.NewRedisCache(cfg.RedisAddr)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := redisCache.Ping(ctx); err != nil {
		cancel()
		slog.Error("redis ping failed", "error", err)
		os.Exit(1)
	}
	cancel()

	w := consumer.NewWorker(cfg.KafkaBrokers, cfg.KafkaTopic, cfg.KafkaGroup, redisCache)
	defer w.Close()

	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()

	go func() {
		if err := w.Run(runCtx); err != nil {
			slog.Error("worker stopped with error", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	runCancel()
	time.Sleep(time.Second)
}
