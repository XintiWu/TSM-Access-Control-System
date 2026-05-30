package queue

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/tsmc/admin-api/internal/model"
)

// MockKafkaWriter is a mock of kafkaWriter interface
type MockKafkaWriter struct {
	mock.Mock
}

func (m *MockKafkaWriter) WriteMessages(ctx context.Context, msgs ...kafka.Message) error {
	args := m.Called(ctx, msgs)
	return args.Error(0)
}

func (m *MockKafkaWriter) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestNoopPublisher(t *testing.T) {
	var pub PermissionPublisher = NoopPublisher{}
	err := pub.Publish(context.Background(), model.PermissionEvent{UserID: "user-1"})
	if err != nil {
		t.Fatalf("NoopPublisher.Publish failed: %v", err)
	}
	err = pub.Close()
	if err != nil {
		t.Fatalf("NoopPublisher.Close failed: %v", err)
	}
}

func TestKafkaProducer_Publish_Success(t *testing.T) {
	mockWriter := new(MockKafkaWriter)
	mockWriter.On("WriteMessages", mock.Anything, mock.Anything).Return(nil).Once()
	mockWriter.On("Close").Return(nil).Once()

	p := &KafkaProducer{
		writer: mockWriter,
		retry:  make(chan model.PermissionEvent, 10),
		done:   make(chan struct{}),
	}
	// We don't start retryLoop to avoid background activities, or we can mock it
	p.wg.Add(1)
	go p.retryLoop()

	event := model.PermissionEvent{
		UserID: "user-1",
		Action: model.ActionBan,
	}

	err := p.Publish(context.Background(), event)
	assert.NoError(t, err)

	err = p.Close()
	assert.NoError(t, err)

	mockWriter.AssertExpectations(t)
}

func TestKafkaProducer_Publish_FailAndRetry(t *testing.T) {
	mockWriter := new(MockKafkaWriter)
	// First write fails
	mockWriter.On("WriteMessages", mock.Anything, mock.Anything).Return(errors.New("kafka connection failure")).Once()
	// Next write (in retry loop) succeeds
	var wgRetry sync.WaitGroup
	wgRetry.Add(1)
	mockWriter.On("WriteMessages", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		wgRetry.Done()
	}).Return(nil).Once()
	mockWriter.On("Close").Return(nil).Once()

	p := &KafkaProducer{
		writer: mockWriter,
		retry:  make(chan model.PermissionEvent, 10),
		done:   make(chan struct{}),
	}
	p.wg.Add(1)
	go p.retryLoop()

	event := model.PermissionEvent{
		UserID: "user-1",
		Action: model.ActionBan,
	}

	err := p.Publish(context.Background(), event)
	// Publish returns the original error when it queues for retry
	assert.Error(t, err)

	// Wait for retry loop to process the retry queue
	wgRetry.Wait()

	err = p.Close()
	assert.NoError(t, err)

	mockWriter.AssertExpectations(t)
}
