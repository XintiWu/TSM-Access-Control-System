package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func setupRateLimitRouter(rps int) *gin.Engine {
	r := gin.New()
	r.Use(RateLimit(rps))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	return r
}

func TestRateLimit_FirstRequestAllowed(t *testing.T) {
	r := setupRateLimitRouter(10)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on first request, got %d", w.Code)
	}
}

func TestRateLimit_ExceedsLimit(t *testing.T) {
	r := setupRateLimitRouter(1)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "10.0.0.1:5678"

	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req)
	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200 on first request, got %d", w1.Code)
	}

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on second request, got %d", w2.Code)
	}
}

func TestRateLimit_TokenRefill(t *testing.T) {
	r := setupRateLimitRouter(5)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "10.0.0.99:9999"

	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after burst, got %d", w.Code)
	}

	time.Sleep(1100 * time.Millisecond)

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 after refill, got %d", w2.Code)
	}
}

func TestRateLimit_DifferentClientsIndependent(t *testing.T) {
	r := setupRateLimitRouter(1)

	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.RemoteAddr = "10.0.0.2:1111"
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("client1 first request: expected 200, got %d", w1.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.RemoteAddr = "10.0.0.3:2222"
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("client2 first request: expected 200, got %d", w2.Code)
	}
}
