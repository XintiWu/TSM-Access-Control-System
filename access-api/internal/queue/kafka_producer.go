package queue

import (
	"context"
	"encoding/json"
	"log"
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
	done   chan struct{}
	wg     sync.WaitGroup
}

func NewKafkaProducer(brokers []string, topic string) *KafkaProducer {
	w := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
		Async:        true,
	}
	p := &KafkaProducer{
		writer: w,
		retry:  make(chan model.InOutEvent, 1024),
		done:   make(chan struct{}),
	}
	p.wg.Add(1)
	go p.retryLoop()
	return p
}

func (p *KafkaProducer) Publish(ctx context.Context, event model.InOutEvent) error {
	if err := p.write(ctx, event); err != nil {
		select {
		case p.retry <- event:
			log.Printf("kafka publish queued for retry eventId=%s: %v", event.EventID, err)
		default:
			log.Printf("kafka publish failed and retry buffer full eventId=%s: %v", event.EventID, err)
		}
		return err
	}
	return nil
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
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := p.write(ctx, event); err != nil {
				log.Printf("kafka retry failed eventId=%s: %v", event.EventID, err)
				go func(ev model.InOutEvent) {
					time.Sleep(2 * time.Second)
					select {
					case p.retry <- ev:
					default:
					}
				}(event)
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
