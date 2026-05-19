package consumer

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/tsmc/cache-invalidation-worker/internal/cache"
	"github.com/tsmc/cache-invalidation-worker/internal/model"
)

type Worker struct {
	reader *kafka.Reader
	cache  *cache.RedisCache
}

func NewWorker(brokers []string, topic, group string, c *cache.RedisCache) *Worker {
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
		cache: c,
	}
}

func (w *Worker) Run(ctx context.Context) error {
	log.Printf("cache invalidation worker started")
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

		var event model.PermissionEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			log.Printf("invalid message: %v", err)
			_ = w.reader.CommitMessages(ctx, msg)
			continue
		}

		opCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		switch event.Action {
		case model.ActionBan:
			err = w.cache.SetDenied(opCtx, event.UserID)
		case model.ActionUnban:
			err = w.cache.ClearDenied(opCtx, event.UserID)
		default:
			log.Printf("unknown action %q for userId=%s", event.Action, event.UserID)
			err = nil
		}
		cancel()

		if err != nil {
			log.Printf("redis update failed userId=%s action=%s: %v", event.UserID, event.Action, err)
			continue
		}

		if err := w.reader.CommitMessages(ctx, msg); err != nil {
			log.Printf("commit failed: %v", err)
		} else {
			log.Printf("applied %s for userId=%s", event.Action, event.UserID)
		}
	}
}

func (w *Worker) Close() error {
	return w.reader.Close()
}
