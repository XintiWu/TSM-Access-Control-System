package consumer

import (
	"context"
	"errors"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/tsmc/aggregation-worker/internal/model"
)

// MockKafkaReader is a mock of KafkaReader
type MockKafkaReader struct {
	mock.Mock
}

func (m *MockKafkaReader) FetchMessage(ctx context.Context) (kafka.Message, error) {
	args := m.Called(ctx)
	return args.Get(0).(kafka.Message), args.Error(1)
}

func (m *MockKafkaReader) CommitMessages(ctx context.Context, msgs ...kafka.Message) error {
	args := m.Called(ctx, msgs)
	return args.Error(0)
}

func (m *MockKafkaReader) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockRepository is a mock of Repository
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) Insert(ctx context.Context, e model.InOutEvent, orgUnitID string) error {
	args := m.Called(ctx, e, orgUnitID)
	return args.Error(0)
}

func (m *MockRepository) GetEmployeeOrgUnitID(ctx context.Context, employeeID string) (string, error) {
	args := m.Called(ctx, employeeID)
	return args.String(0), args.Error(1)
}

func TestWorker_Run_Success(t *testing.T) {
	mockReader := new(MockKafkaReader)
	mockRepo := new(MockRepository)

	eventJSON := `{"eventId":"550e8400-e29b-41d4-a716-446655440000","employeeId":"123e4567-e89b-12d3-a456-426614174000","doorId":"987f6543-e21b-34d5-c678-901234567890","direction":"IN","eventTime":"2026-05-30T10:00:00Z","status":"SUCCESS","sourceIp":"192.168.1.1","cardUid":"A1B2C3D4"}`
	msg := kafka.Message{
		Topic:     "access_events",
		Partition: 0,
		Offset:    1,
		Value:     []byte(eventJSON),
	}

	ctx, cancel := context.WithCancel(context.Background())

	mockReader.On("FetchMessage", mock.Anything).Return(msg, nil).Once()
	// Next call will block or return context canceled when cancel() is called.
	mockReader.On("FetchMessage", mock.Anything).Run(func(args mock.Arguments) {
		cancel()
	}).Return(kafka.Message{}, context.Canceled).Once()

	mockRepo.On("GetEmployeeOrgUnitID", mock.Anything, "123e4567-e89b-12d3-a456-426614174000").Return("org-123", nil).Once()
	mockRepo.On("Insert", mock.Anything, mock.Anything, "org-123").Return(nil).Once()
	mockReader.On("CommitMessages", mock.Anything, []kafka.Message{msg}).Return(nil).Once()

	w := &Worker{
		reader: mockReader,
		repo:   mockRepo,
	}

	err := w.Run(ctx)
	assert.NoError(t, err)

	mockReader.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

func TestWorker_Run_InvalidJSON(t *testing.T) {
	mockReader := new(MockKafkaReader)
	mockRepo := new(MockRepository)

	msg := kafka.Message{
		Topic:     "access_events",
		Partition: 0,
		Offset:    1,
		Value:     []byte(`{invalid-json}`),
	}

	ctx, cancel := context.WithCancel(context.Background())

	mockReader.On("FetchMessage", mock.Anything).Return(msg, nil).Once()
	mockReader.On("FetchMessage", mock.Anything).Run(func(args mock.Arguments) {
		cancel()
	}).Return(kafka.Message{}, context.Canceled).Once()

	mockReader.On("CommitMessages", mock.Anything, []kafka.Message{msg}).Return(nil).Once()

	w := &Worker{
		reader: mockReader,
		repo:   mockRepo,
	}

	err := w.Run(ctx)
	assert.NoError(t, err)

	mockReader.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

func TestWorker_Run_OrgLookupError(t *testing.T) {
	mockReader := new(MockKafkaReader)
	mockRepo := new(MockRepository)

	eventJSON := `{"eventId":"550e8400-e29b-41d4-a716-446655440000","employeeId":"123e4567-e89b-12d3-a456-426614174000","doorId":"987f6543-e21b-34d5-c678-901234567890","direction":"IN","eventTime":"2026-05-30T10:00:00Z","status":"SUCCESS","sourceIp":"192.168.1.1","cardUid":"A1B2C3D4"}`
	msg := kafka.Message{
		Topic:     "access_events",
		Partition: 0,
		Offset:    1,
		Value:     []byte(eventJSON),
	}

	ctx, cancel := context.WithCancel(context.Background())

	mockReader.On("FetchMessage", mock.Anything).Return(msg, nil).Once()
	mockReader.On("FetchMessage", mock.Anything).Run(func(args mock.Arguments) {
		cancel()
	}).Return(kafka.Message{}, context.Canceled).Once()

	mockRepo.On("GetEmployeeOrgUnitID", mock.Anything, "123e4567-e89b-12d3-a456-426614174000").Return("", errors.New("db lookup error")).Once()
	mockRepo.On("Insert", mock.Anything, mock.Anything, "").Return(nil).Once()
	mockReader.On("CommitMessages", mock.Anything, []kafka.Message{msg}).Return(nil).Once()

	w := &Worker{
		reader: mockReader,
		repo:   mockRepo,
	}

	err := w.Run(ctx)
	assert.NoError(t, err)

	mockReader.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

func TestWorker_Run_InsertError(t *testing.T) {
	mockReader := new(MockKafkaReader)
	mockRepo := new(MockRepository)

	eventJSON := `{"eventId":"550e8400-e29b-41d4-a716-446655440000","employeeId":"123e4567-e89b-12d3-a456-426614174000","doorId":"987f6543-e21b-34d5-c678-901234567890","direction":"IN","eventTime":"2026-05-30T10:00:00Z","status":"SUCCESS","sourceIp":"192.168.1.1","cardUid":"A1B2C3D4"}`
	msg := kafka.Message{
		Topic:     "access_events",
		Partition: 0,
		Offset:    1,
		Value:     []byte(eventJSON),
	}

	ctx, cancel := context.WithCancel(context.Background())

	mockReader.On("FetchMessage", mock.Anything).Return(msg, nil).Once()
	mockReader.On("FetchMessage", mock.Anything).Run(func(args mock.Arguments) {
		cancel()
	}).Return(kafka.Message{}, context.Canceled).Once()

	mockRepo.On("GetEmployeeOrgUnitID", mock.Anything, "123e4567-e89b-12d3-a456-426614174000").Return("org-123", nil).Once()
	mockRepo.On("Insert", mock.Anything, mock.Anything, "org-123").Return(errors.New("db insert error")).Once()

	w := &Worker{
		reader: mockReader,
		repo:   mockRepo,
	}

	err := w.Run(ctx)
	assert.NoError(t, err)

	mockReader.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

func TestWorker_Run_FetchErrorAndContextCancel(t *testing.T) {
	mockReader := new(MockKafkaReader)
	mockRepo := new(MockRepository)

	ctx, cancel := context.WithCancel(context.Background())

	mockReader.On("FetchMessage", mock.Anything).Return(kafka.Message{}, errors.New("some fetch error")).Once()
	mockReader.On("FetchMessage", mock.Anything).Run(func(args mock.Arguments) {
		cancel()
	}).Return(kafka.Message{}, context.Canceled).Once()

	w := &Worker{
		reader: mockReader,
		repo:   mockRepo,
	}

	err := w.Run(ctx)
	assert.NoError(t, err)

	mockReader.AssertExpectations(t)
}

func TestWorker_Close(t *testing.T) {
	mockReader := new(MockKafkaReader)
	mockReader.On("Close").Return(nil).Once()

	w := &Worker{
		reader: mockReader,
	}

	err := w.Close()
	assert.NoError(t, err)
	mockReader.AssertExpectations(t)
}
