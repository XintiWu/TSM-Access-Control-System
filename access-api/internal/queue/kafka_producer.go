package queue

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/tsmc/access-api/internal/model"
)

type EventPublisher interface {
	Publish(ctx context.Context, event model.InOutEvent) error
	Close() error
}

type KafkaProducer struct {
	writer *kafka.Writer
	retry  chan model.InOutEvent
	outbox *FileOutbox
	done   chan struct{}
	wg     sync.WaitGroup
}

func NewKafkaProducer(brokers []string, topic string) *KafkaProducer {
	return NewKafkaProducerWithOutbox(brokers, topic, nil)
}

// NewKafkaProducerWithOutbox creates a producer; optional outbox persists failed publishes to disk.
func NewKafkaProducerWithOutbox(brokers []string, topic string, outbox *FileOutbox) *KafkaProducer {
	w := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
		Async:        false, // Changed to false for synchronous publishing to capture errors
	}
	p := &KafkaProducer{
		writer: w,
		retry:  make(chan model.InOutEvent, 1024),
		outbox: outbox,
		done:   make(chan struct{}),
	}
	p.wg.Add(1)
	go p.retryLoop()
	return p
}

func (p *KafkaProducer) enqueueRetry(event model.InOutEvent, cause error) error {
	select {
	case p.retry <- event:
		slog.Warn("kafka publish queued for retry", "eventId", event.EventID, "error", cause)
		return cause
	default:
		if p.outbox != nil {
			if err := p.outbox.Append(event); err != nil {
				slog.Error("kafka publish failed; outbox append failed", "eventId", event.EventID, "error", err)
				return err
			}
			slog.Warn("kafka publish spooled to outbox", "eventId", event.EventID, "error", cause)
			return nil
		}
		slog.Error("kafka publish failed and retry buffer full", "eventId", event.EventID, "error", cause)
		return cause
	}
}

func (p *KafkaProducer) Publish(ctx context.Context, event model.InOutEvent) error {
	// Set a timeout of 100ms for publishing to avoid blocking API latency SLA
	writeCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	
	if err := p.write(writeCtx, event); err != nil {
		return p.enqueueRetry(event, err)
	}
	return nil
}

// ReplayOutbox drains the on-disk outbox (call at startup).
func (p *KafkaProducer) ReplayOutbox(ctx context.Context) (int, error) {
	if p.outbox == nil {
		return 0, nil
	}
	return p.outbox.Replay(ctx, p.write)
}

func (p *KafkaProducer) write(ctx context.Context, event model.InOutEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(event.EventID),
		Value: body,
	})
}

func (p *KafkaProducer) retryLoop() {
	defer p.wg.Done()
	for {
		select {
		case <-p.done:
			return
		case event := <-p.retry:
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			if err := p.write(ctx, event); err != nil {
				_ = p.enqueueRetry(event, err)
			} else {
				slog.Info("kafka retry publish succeeded", "eventId", event.EventID)
			}
			cancel()
		}
	}
}

func (p *KafkaProducer) Close() error {
	close(p.done)
	p.wg.Wait()
	return p.writer.Close()
}

// NoopPublisher for tests or when Kafka is unavailable at startup.
type NoopPublisher struct{}

func (NoopPublisher) Publish(context.Context, model.InOutEvent) error { return nil }
func (NoopPublisher) Close() error                                   { return nil }
