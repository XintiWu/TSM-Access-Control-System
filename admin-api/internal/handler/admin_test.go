package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tsmc/admin-api/internal/handler"
	"github.com/tsmc/admin-api/internal/queue"
)

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
