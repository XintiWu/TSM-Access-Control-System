package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tsmc/aggregation-worker/internal/config"
	"github.com/tsmc/aggregation-worker/internal/consumer"
	"github.com/tsmc/aggregation-worker/internal/repository"
)

func main() {
	cfg := config.Load()

	var repo *repository.InOutRepository
	var err error
	for i := 0; i < 30; i++ {
		repo, err = repository.NewInOutRepository(cfg.ClickHouseAddr, cfg.ClickHouseUser, cfg.ClickHousePass)
		if err == nil {
			break
		}
		log.Printf("waiting for database: %v", err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer repo.Close()

	w := consumer.NewWorker(cfg.KafkaBrokers, cfg.KafkaTopic, cfg.KafkaGroup, repo)
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := w.Run(ctx); err != nil {
			log.Printf("worker stopped: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	cancel()
	time.Sleep(time.Second)
}
