//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/tsmc/access-api/internal/cache"
	"github.com/tsmc/access-api/internal/handler"
	"github.com/tsmc/access-api/internal/model"
	"github.com/tsmc/access-api/internal/queue"
	"github.com/tsmc/access-api/internal/service"
)

func startRedis(t *testing.T) (addr string, terminate func()) {
	t.Helper()
	ctx := context.Background()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "redis:7-alpine",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForLog("Ready to accept connections"),
		},
		Started: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	host, _ := c.Host(ctx)
	port, _ := c.MappedPort(ctx, "6379")
	return host + ":" + port.Port(), func() { _ = c.Terminate(ctx) }
}

func TestSwipeIntegration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	addr, stop := startRedis(t)
	defer stop()

	redisCache := cache.NewRedisCache(addr)
	svc := service.NewAccessDecisionService(redisCache)
	h := handler.NewAccessHandler(svc, redisCache, queue.NoopPublisher{})

	r := gin.New()
	r.POST("/access/swipe", h.Swipe)

	userID := "22222222-2222-2222-2222-222222222222"
	doorID := "11111111-1111-1111-1111-111111111111"

	client := redis.NewClient(&redis.Options{Addr: addr})
	_ = client.Del(context.Background(), "passback:"+userID).Err()

	swipe := func(dir string) model.SwipeResponse {
		body, _ := json.Marshal(map[string]interface{}{
			"userId": userID, "doorId": doorID, "direction": dir,
			"cardUid": "CARD001", "timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/access/swipe", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
		var resp model.SwipeResponse
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		return resp
	}

	if r := swipe("IN"); r.Decision != model.DecisionAllow {
		t.Fatalf("first IN: %+v", r)
	}
	if r := swipe("IN"); r.Decision != model.DecisionDeny {
		t.Fatalf("second IN: %+v", r)
	}
	if r := swipe("OUT"); r.Decision != model.DecisionAllow {
		t.Fatalf("OUT: %+v", r)
	}
}
