package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

type mockRow struct {
	err  error
	args []any
}

func (m *mockRow) Err() error {
	return m.err
}

func (m *mockRow) Scan(dest ...any) error {
	if m.err != nil {
		return m.err
	}
	for i, arg := range m.args {
		if arg == nil {
			continue
		}
		switch d := dest[i].(type) {
		case *uint64:
			*d = arg.(uint64)
		case *string:
			*d = arg.(string)
		case **string:
			*d = arg.(*string)
		case *uint8:
			*d = arg.(uint8)
		case **uuid.UUID:
			*d = arg.(*uuid.UUID)
		}
	}
	return nil
}

func (m *mockRow) ScanStruct(dest any) error {
	return m.err
}

type mockConn struct {
	driver.Conn
	execErr  error
	countRow driver.Row
	dataRow  driver.Row
	pingErr  error
	closeErr error
}

func (m *mockConn) Exec(ctx context.Context, query string, args ...any) error {
	return m.execErr
}

func (m *mockConn) QueryRow(ctx context.Context, query string, args ...any) driver.Row {
	if strings.Contains(query, "count()") {
		return m.countRow
	}
	return m.dataRow
}

func (m *mockConn) Ping(ctx context.Context) error {
	return m.pingErr
}

func (m *mockConn) Close() error {
	return m.closeErr
}

func TestNewEmployeeRepository(t *testing.T) {
	_, err := NewEmployeeRepository("invalid:port", "user", "pass")
	assert.Error(t, err)

	_, err = NewEmployeeRepository("localhost:9440", "user", "pass")
	assert.Error(t, err)
}

func TestEmployeeRepository_Ping(t *testing.T) {
	conn := &mockConn{}
	repo := &EmployeeRepository{chConn: conn}

	err := repo.Ping(context.Background())
	assert.NoError(t, err)

	conn.pingErr = errors.New("ping error")
	err = repo.Ping(context.Background())
	assert.Error(t, err)
}

func TestEmployeeRepository_Exists(t *testing.T) {
	conn := &mockConn{}
	repo := &EmployeeRepository{chConn: conn}

	ctx := context.Background()

	_, err := repo.Exists(ctx, "invalid")
	assert.Error(t, err)

	conn.countRow = &mockRow{args: []any{uint64(1)}}
	exists, err := repo.Exists(ctx, uuid.New().String())
	assert.NoError(t, err)
	assert.True(t, exists)

	conn.countRow = &mockRow{err: errors.New("query error")}
	_, err = repo.Exists(ctx, uuid.New().String())
	assert.Error(t, err)
}

func TestEmployeeRepository_SetActive(t *testing.T) {
	conn := &mockConn{}
	repo := &EmployeeRepository{chConn: conn}
	ctx := context.Background()
	userID := uuid.New().String()

	// Error case - invalid uuid
	err := repo.SetActive(ctx, "invalid", true)
	assert.Error(t, err)

	// Error case - getLatest error (count query fails)
	conn.countRow = &mockRow{err: errors.New("count error")}
	err = repo.SetActive(ctx, userID, true)
	assert.Error(t, err)

	// Error case - not found
	conn.countRow = &mockRow{args: []any{uint64(0)}}
	err = repo.SetActive(ctx, userID, true)
	assert.Error(t, err)

	// Error case - exec error
	conn.countRow = &mockRow{args: []any{uint64(1)}}
	orgID := uuid.New()
	cardUID := "abcd"
	conn.dataRow = &mockRow{args: []any{"John Doe", &cardUID, uint8(1), &orgID}}
	conn.execErr = errors.New("exec error")
	err = repo.SetActive(ctx, userID, true)
	assert.Error(t, err)

	// Success case
	conn.execErr = nil
	err = repo.SetActive(ctx, userID, false)
	assert.NoError(t, err)
}

func TestEmployeeRepository_Close(t *testing.T) {
	conn := &mockConn{}
	repo := &EmployeeRepository{chConn: conn}

	err := repo.Close()
	assert.NoError(t, err)

	conn.closeErr = errors.New("close error")
	err = repo.Close()
	assert.Error(t, err)
}

func TestEmployeeRepository_getLatest(t *testing.T) {
	conn := &mockConn{}
	repo := &EmployeeRepository{chConn: conn}
	ctx := context.Background()
	userID := uuid.New().String()

	// test data query error
	conn.countRow = &mockRow{args: []any{uint64(1)}}
	conn.dataRow = &mockRow{err: errors.New("data error")}
	_, err := repo.getLatest(ctx, userID)
	assert.Error(t, err)
}
