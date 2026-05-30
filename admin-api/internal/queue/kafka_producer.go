package queue

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/tsmc/admin-api/internal/model"
)

type PermissionPublisher interface {
	Publish(ctx context.Context, event model.PermissionEvent) error
	Close() error
}

type kafkaWriter interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

type KafkaProducer struct {
	writer kafkaWriter
	retry  chan model.PermissionEvent
	done   chan struct{}
	wg     sync.WaitGroup
}

func NewKafkaProducer(brokers []string, topic string) *KafkaProducer {
	w := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
		Async:        false,
	}
	p := &KafkaProducer{
		writer: w,
		retry:  make(chan model.PermissionEvent, 256),
		done:   make(chan struct{}),
	}
	p.wg.Add(1)
	go p.retryLoop()
	return p
}

func (p *KafkaProducer) Publish(ctx context.Context, event model.PermissionEvent) error {
	if err := p.write(ctx, event); err != nil {
		select {
		case p.retry <- event:
			log.Printf("kafka publish queued for retry userId=%s: %v", event.UserID, err)
		default:
			return err
		}
		return err
	}
	return nil
}

func (p *KafkaProducer) write(ctx context.Context, event model.PermissionEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(event.UserID),
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
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := p.write(ctx, event); err != nil {
				log.Printf("kafka retry failed userId=%s: %v", event.UserID, err)
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

type NoopPublisher struct{}

func (NoopPublisher) Publish(context.Context, model.PermissionEvent) error { return nil }
func (NoopPublisher) Close() error                                       { return nil }
