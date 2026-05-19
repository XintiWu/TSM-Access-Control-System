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
	"github.com/tsmc/admin-api/internal/config"
	"github.com/tsmc/admin-api/internal/handler"
	"github.com/tsmc/admin-api/internal/queue"
	"github.com/tsmc/admin-api/internal/repository"
)

func main() {
	cfg := config.Load()

	var repo *repository.EmployeeRepository
	var err error
	for i := 0; i < 30; i++ {
		repo, err = repository.NewEmployeeRepository(cfg.DBDSN)
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

	publisher := queue.NewKafkaProducer(cfg.KafkaBrokers, cfg.KafkaTopic)
	defer publisher.Close()

	adminHandler := handler.NewAdminHandler(repo, publisher)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())

	r.GET("/health", func(c *gin.Context) {
		if err := repo.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "db": err.Error()})
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
		log.Printf("admin-api listening on %s", cfg.HTTPAddr)
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
