package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tsmc/aggregation-worker/internal/model"
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
	orgRow   driver.Row
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
	return m.orgRow
}

func (m *mockConn) Ping(ctx context.Context) error {
	return m.pingErr
}

func (m *mockConn) Close() error {
	return m.closeErr
}

func TestNewInOutRepository(t *testing.T) {
	// Provide invalid connection string to test failure
	_, err := NewInOutRepository("invalid:port", "user", "pass")
	assert.Error(t, err)

	// Provide TLS port to cover tlsConfig setup
	_, err = NewInOutRepository("localhost:9440", "user", "pass")
	assert.Error(t, err) // Ping should fail since there's no DB
}

func TestInOutRepository_Insert(t *testing.T) {
	conn := &mockConn{}
	repo := &InOutRepository{chConn: conn}

	ctx := context.Background()
	reason := "some reason"
	e := model.InOutEvent{
		EventID:    uuid.New().String(),
		EmployeeID: uuid.New().String(),
		DoorID:     uuid.New().String(),
		Direction:  "IN",
		EventTime:  time.Now(),
		Status:     "SUCCESS",
		Reason:     &reason,
		SourceIP:   "127.0.0.1",
		CardUID:    "abcd",
	}

	// Success case
	err := repo.Insert(ctx, e, uuid.New().String())
	assert.NoError(t, err)

	// Error case - execution fails
	conn.execErr = errors.New("db error")
	err = repo.Insert(ctx, e, uuid.New().String())
	assert.Error(t, err)
	conn.execErr = nil // reset

	// Error case - invalid EventID
	e.EventID = "invalid"
	err = repo.Insert(ctx, e, uuid.New().String())
	assert.Error(t, err)

	// Error case - invalid EmployeeID
	e.EventID = uuid.New().String()
	e.EmployeeID = "invalid"
	err = repo.Insert(ctx, e, uuid.New().String())
	assert.Error(t, err)

	// Error case - invalid DoorID
	e.EmployeeID = uuid.New().String()
	e.DoorID = "invalid"
	err = repo.Insert(ctx, e, uuid.New().String())
	assert.Error(t, err)
}

func TestInOutRepository_GetEmployeeOrgUnitID(t *testing.T) {
	conn := &mockConn{}
	repo := &InOutRepository{chConn: conn}

	ctx := context.Background()
	empID := uuid.New().String()
	orgID := uuid.New()

	// Error case - invalid employee ID
	_, err := repo.GetEmployeeOrgUnitID(ctx, "invalid")
	assert.Error(t, err)

	// Success case - count > 0, found org unit
	conn.countRow = &mockRow{args: []any{uint64(1)}}
	conn.orgRow = &mockRow{args: []any{&orgID}}
	res, err := repo.GetEmployeeOrgUnitID(ctx, empID)
	assert.NoError(t, err)
	assert.Equal(t, orgID.String(), res)

	// Success case - count == 0
	conn.countRow = &mockRow{args: []any{uint64(0)}}
	res, err = repo.GetEmployeeOrgUnitID(ctx, empID)
	assert.NoError(t, err)
	assert.Equal(t, "", res)

	// Success case - count > 0, org unit not found (nil)
	conn.countRow = &mockRow{args: []any{uint64(1)}}
	conn.orgRow = &mockRow{args: []any{nil}}
	res, err = repo.GetEmployeeOrgUnitID(ctx, empID)
	assert.NoError(t, err)
	assert.Equal(t, "", res)

	// Error case - count query fails
	conn.countRow = &mockRow{err: errors.New("count error")}
	_, err = repo.GetEmployeeOrgUnitID(ctx, empID)
	assert.Error(t, err)

	// Error case - org query fails
	conn.countRow = &mockRow{args: []any{uint64(1)}}
	conn.orgRow = &mockRow{err: errors.New("org error")}
	_, err = repo.GetEmployeeOrgUnitID(ctx, empID)
	assert.Error(t, err)
}

func TestInOutRepository_Close(t *testing.T) {
	conn := &mockConn{}
	repo := &InOutRepository{chConn: conn}

	err := repo.Close()
	assert.NoError(t, err)

	conn.closeErr = errors.New("close error")
	err = repo.Close()
	assert.Error(t, err)
}
