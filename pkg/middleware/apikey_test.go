package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupAPIKeyRouter(apiKey string, extraBypass ...string) *gin.Engine {
	r := gin.New()
	r.Use(APIKeyAuth(apiKey, extraBypass...))
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/metrics", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/ui/dashboard", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/reports/personal", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	return r
}

func TestAPIKeyAuth_NotConfigured(t *testing.T) {
	r := setupAPIKeyRouter("")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/reports/personal", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when auth not configured, got %d", w.Code)
	}
}

func TestAPIKeyAuth_ValidKey(t *testing.T) {
	r := setupAPIKeyRouter("test-key")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/reports/personal", nil)
	req.Header.Set("X-API-Key", "test-key")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid key, got %d", w.Code)
	}
}

func TestAPIKeyAuth_InvalidKey(t *testing.T) {
	r := setupAPIKeyRouter("test-key")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/reports/personal", nil)
	req.Header.Set("X-API-Key", "wrong-key")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with invalid key, got %d", w.Code)
	}
}

func TestAPIKeyAuth_MissingKey(t *testing.T) {
	r := setupAPIKeyRouter("test-key")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/reports/personal", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with missing key, got %d", w.Code)
	}
}

func TestAPIKeyAuth_HealthBypass(t *testing.T) {
	r := setupAPIKeyRouter("test-key")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for /health bypass, got %d", w.Code)
	}
}

func TestAPIKeyAuth_MetricsBypass(t *testing.T) {
	r := setupAPIKeyRouter("test-key")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for /metrics bypass, got %d", w.Code)
	}
}

func TestAPIKeyAuth_UIBypass(t *testing.T) {
	r := setupAPIKeyRouter("test-key", "/ui")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ui/dashboard", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for /ui prefix bypass, got %d", w.Code)
	}
}

func TestAPIKeyAuth_UINotBypassedWithoutPrefix(t *testing.T) {
	r := setupAPIKeyRouter("test-key")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ui/dashboard", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for /ui without bypass prefix, got %d", w.Code)
	}
}

func TestAPIKeyAuth_BearerToken(t *testing.T) {
	r := setupAPIKeyRouter("test-key")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/reports/personal", nil)
	req.Header.Set("Authorization", "Bearer test-key")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid bearer token, got %d", w.Code)
	}
}

func TestAPIKeyAuth_InvalidBearerToken(t *testing.T) {
	r := setupAPIKeyRouter("test-key")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/reports/personal", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with invalid bearer, got %d", w.Code)
	}
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name string
		auth string
		want string
	}{
		{"empty", "", ""},
		{"valid", "Bearer my-token", "my-token"},
		{"case insensitive", "bearer my-token", "my-token"},
		{"no space", "Bearermy-token", ""},
		{"too short", "Bear", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractBearerToken(tt.auth)
			if got != tt.want {
				t.Errorf("extractBearerToken(%q) = %q, want %q", tt.auth, got, tt.want)
			}
		})
	}
}
