package consumer

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/tsmc/aggregation-worker/internal/model"
	"github.com/tsmc/aggregation-worker/internal/repository"
)

type Worker struct {
	reader *kafka.Reader
	repo   *repository.InOutRepository
}

func NewWorker(brokers []string, topic, group string, repo *repository.InOutRepository) *Worker {
	return &Worker{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:        brokers,
			Topic:          topic,
			GroupID:        group,
			MinBytes:       1,
			MaxBytes:       10e6,
			CommitInterval: time.Second,
			StartOffset:    kafka.FirstOffset,
		}),
		repo: repo,
	}
}

func (w *Worker) Run(ctx context.Context) error {
	log.Printf("aggregation worker started")
	for {
		msg, err := w.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			log.Printf("fetch error: %v", err)
			time.Sleep(time.Second)
			continue
		}

		var event model.InOutEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			log.Printf("invalid message: %v", err)
			_ = w.reader.CommitMessages(ctx, msg)
			continue
		}

		insertCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		if err := w.repo.Insert(insertCtx, event); err != nil {
			cancel()
			log.Printf("insert failed eventId=%s: %v", event.EventID, err)
			continue
		}
		cancel()

		if err := w.reader.CommitMessages(ctx, msg); err != nil {
			log.Printf("commit failed: %v", err)
		}
	}
}

func (w *Worker) Close() error {
	return w.reader.Close()
}
