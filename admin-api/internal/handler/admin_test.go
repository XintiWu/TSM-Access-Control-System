package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tsmc/admin-api/internal/handler"
	"github.com/tsmc/admin-api/internal/model"
	"github.com/tsmc/admin-api/internal/queue"
	"github.com/tsmc/admin-api/internal/repository"
)

type mockEmployeeRepo struct {
	existsFn    func(ctx context.Context, userID string) (bool, error)
	setActiveFn func(ctx context.Context, userID string, active bool) error
}

func (m *mockEmployeeRepo) Exists(ctx context.Context, userID string) (bool, error) {
	if m.existsFn != nil {
		return m.existsFn(ctx, userID)
	}
	return false, nil
}

func (m *mockEmployeeRepo) SetActive(ctx context.Context, userID string, active bool) error {
	if m.setActiveFn != nil {
		return m.setActiveFn(ctx, userID, active)
	}
	return nil
}

type mockPublisher struct {
	publishFn func(ctx context.Context, event model.PermissionEvent) error
}

func (m *mockPublisher) Publish(ctx context.Context, event model.PermissionEvent) error {
	if m.publishFn != nil {
		return m.publishFn(ctx, event)
	}
	return nil
}

func (m *mockPublisher) Close() error { return nil }

func TestBan_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userID := uuid.New().String()

	repo := &mockEmployeeRepo{
		existsFn: func(ctx context.Context, u string) (bool, error) {
			return true, nil
		},
		setActiveFn: func(ctx context.Context, u string, active bool) error {
			if active != false {
				t.Errorf("expected active=false for ban")
			}
			return nil
		},
	}

	pubCalled := false
	pub := &mockPublisher{
		publishFn: func(ctx context.Context, event model.PermissionEvent) error {
			if event.UserID != userID || event.Action != model.ActionBan {
				t.Errorf("unexpected event: %+v", event)
			}
			pubCalled = true
			return nil
		},
	}

	h := handler.NewAdminHandler(repo, pub)
	r := gin.New()
	r.POST("/admin/employees/:userId/ban", h.Ban)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/employees/"+userID+"/ban", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if !pubCalled {
		t.Error("expected publish to be called")
	}
}

func TestUnban_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userID := uuid.New().String()

	repo := &mockEmployeeRepo{
		existsFn: func(ctx context.Context, u string) (bool, error) {
			return true, nil
		},
		setActiveFn: func(ctx context.Context, u string, active bool) error {
			if active != true {
				t.Errorf("expected active=true for unban")
			}
			return nil
		},
	}

	pubCalled := false
	pub := &mockPublisher{
		publishFn: func(ctx context.Context, event model.PermissionEvent) error {
			if event.UserID != userID || event.Action != model.ActionUnban {
				t.Errorf("unexpected event: %+v", event)
			}
			pubCalled = true
			return nil
		},
	}

	h := handler.NewAdminHandler(repo, pub)
	r := gin.New()
	r.POST("/admin/employees/:userId/unban", h.Unban)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/employees/"+userID+"/unban", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if !pubCalled {
		t.Error("expected publish to be called")
	}
}

func TestBan_EmployeeNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userID := uuid.New().String()

	repo := &mockEmployeeRepo{
		existsFn: func(ctx context.Context, u string) (bool, error) {
			return false, nil
		},
	}

	h := handler.NewAdminHandler(repo, &mockPublisher{})
	r := gin.New()
	r.POST("/admin/employees/:userId/ban", h.Ban)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/employees/"+userID+"/ban", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestBan_DBError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userID := uuid.New().String()

	repo := &mockEmployeeRepo{
		existsFn: func(ctx context.Context, u string) (bool, error) {
			return false, errors.New("db error")
		},
	}

	h := handler.NewAdminHandler(repo, &mockPublisher{})
	r := gin.New()
	r.POST("/admin/employees/:userId/ban", h.Ban)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/employees/"+userID+"/ban", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestBan_SetActiveError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userID := uuid.New().String()

	repo := &mockEmployeeRepo{
		existsFn: func(ctx context.Context, u string) (bool, error) {
			return true, nil
		},
		setActiveFn: func(ctx context.Context, u string, active bool) error {
			return errors.New("set active error")
		},
	}

	h := handler.NewAdminHandler(repo, &mockPublisher{})
	r := gin.New()
	r.POST("/admin/employees/:userId/ban", h.Ban)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/employees/"+userID+"/ban", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestBan_SetActiveNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userID := uuid.New().String()

	repo := &mockEmployeeRepo{
		existsFn: func(ctx context.Context, u string) (bool, error) {
			return true, nil
		},
		setActiveFn: func(ctx context.Context, u string, active bool) error {
			return repository.ErrNotFound
		},
	}

	h := handler.NewAdminHandler(repo, &mockPublisher{})
	r := gin.New()
	r.POST("/admin/employees/:userId/ban", h.Ban)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/employees/"+userID+"/ban", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestBan_PublishError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	userID := uuid.New().String()

	repo := &mockEmployeeRepo{
		existsFn: func(ctx context.Context, u string) (bool, error) {
			return true, nil
		},
		setActiveFn: func(ctx context.Context, u string, active bool) error {
			return nil
		},
	}

	pub := &mockPublisher{
		publishFn: func(ctx context.Context, event model.PermissionEvent) error {
			return errors.New("pub error")
		},
	}

	h := handler.NewAdminHandler(repo, pub)
	r := gin.New()
	r.POST("/admin/employees/:userId/ban", h.Ban)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/employees/"+userID+"/ban", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestBanInvalidUUID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := handler.NewAdminHandler(nil, queue.NoopPublisher{})

	r := gin.New()
	r.POST("/admin/employees/:userId/ban", h.Ban)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/employees/not-a-uuid/ban", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestBanInvalidUUIDUnban(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := handler.NewAdminHandler(nil, queue.NoopPublisher{})

	r := gin.New()
	r.POST("/admin/employees/:userId/unban", h.Unban)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/employees/bad/unban", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d", w.Code)
	}
}

func TestNoopPublisher(t *testing.T) {
	p := queue.NoopPublisher{}
	event := model.PermissionEvent{
		UserID:    uuid.New().String(),
		Action:    model.ActionBan,
		EventTime: time.Now().UTC(),
	}
	if err := p.Publish(context.Background(), event); err != nil {
		t.Errorf("NoopPublisher.Publish() = %v, want nil", err)
	}
	if err := p.Close(); err != nil {
		t.Errorf("NoopPublisher.Close() = %v, want nil", err)
	}
}

func TestErrNotFound(t *testing.T) {
	if repository.ErrNotFound == nil {
		t.Fatal("ErrNotFound should not be nil")
	}
	if !errors.Is(repository.ErrNotFound, repository.ErrNotFound) {
		t.Error("ErrNotFound should match itself")
	}
}

func TestPermissionEventModel(t *testing.T) {
	now := time.Now().UTC()
	event := model.PermissionEvent{
		UserID:    "user-1",
		Action:    model.ActionBan,
		EventTime: now,
	}
	if event.Action != model.ActionBan {
		t.Errorf("Action = %q, want BAN", event.Action)
	}

	event.Action = model.ActionUnban
	if event.Action != model.ActionUnban {
		t.Errorf("Action = %q, want UNBAN", event.Action)
	}
}

func TestPermissionResponseModel(t *testing.T) {
	resp := model.PermissionResponse{
		UserID: "user-1",
		Action: model.ActionBan,
		Status: "accepted",
	}
	if resp.Status != "accepted" {
		t.Errorf("Status = %q, want accepted", resp.Status)
	}
}
