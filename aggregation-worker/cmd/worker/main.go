package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tsmc/aggregation-worker/internal/config"
	"github.com/tsmc/aggregation-worker/internal/consumer"
	"github.com/tsmc/aggregation-worker/internal/repository"
)

func main() {
	// Configure global slog JSON logger
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg := config.Load()

	var repo *repository.InOutRepository
	var err error
	for i := 0; i < 30; i++ {
		repo, err = repository.NewInOutRepository(cfg.ClickHouseAddr, cfg.ClickHouseUser, cfg.ClickHousePass)
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

	w := consumer.NewWorker(cfg.KafkaBrokers, cfg.KafkaTopic, cfg.KafkaGroup, repo)
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := w.Run(ctx); err != nil {
			slog.Error("worker stopped with error", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	cancel()
	time.Sleep(time.Second)
}
