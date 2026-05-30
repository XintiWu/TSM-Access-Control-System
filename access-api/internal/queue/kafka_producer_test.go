package queue

import (
	"context"
	"errors"
	"testing"

	"github.com/tsmc/access-api/internal/model"
)

func TestNoopPublisher(t *testing.T) {
	var pub EventPublisher = NoopPublisher{}
	err := pub.Publish(context.Background(), model.InOutEvent{EventID: "evt-1"})
	if err != nil {
		t.Fatalf("NoopPublisher.Publish failed: %v", err)
	}
	err = pub.Close()
	if err != nil {
		t.Fatalf("NoopPublisher.Close failed: %v", err)
	}
}

func TestReplayOutbox_NilOutbox(t *testing.T) {
	p := &KafkaProducer{outbox: nil}
	count, err := p.ReplayOutbox(context.Background())
	if err != nil {
		t.Fatalf("ReplayOutbox failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected count=0, got %d", count)
	}
}

func TestEnqueueRetry_BufferFull_WithOutbox(t *testing.T) {
	p := &KafkaProducer{
		retry: make(chan model.InOutEvent, 1),
	}

	// fill the queue
	p.retry <- model.InOutEvent{EventID: "first"}

	// attempt to enqueue when buffer is full
	err := p.enqueueRetry(model.InOutEvent{EventID: "second"}, errors.New("cause"))
	if err == nil {
		t.Error("expected error when outbox is nil and retry buffer is full")
	}
}
