package cache_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/tsmc/cache-invalidation-worker/internal/cache"
)

func startRedis(t *testing.T) (addr string, stop func()) {
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

func TestSetAndClearDenied(t *testing.T) {
	addr, stop := startRedis(t)
	defer stop()
	ctx := context.Background()
	userID := "22222222-2222-2222-2222-222222222222"

	rc := cache.NewRedisCache(addr)
	client := redis.NewClient(&redis.Options{Addr: addr})
	defer client.Close()

	if err := rc.SetDenied(ctx, userID); err != nil {
		t.Fatal(err)
	}
	n, err := client.Exists(ctx, fmt.Sprintf("perm:denied:%s", userID)).Result()
	if err != nil || n != 1 {
		t.Fatalf("expected deny key, exists=%d err=%v", n, err)
	}

	if err := rc.ClearDenied(ctx, userID); err != nil {
		t.Fatal(err)
	}
	n, err = client.Exists(ctx, fmt.Sprintf("perm:denied:%s", userID)).Result()
	if err != nil || n != 0 {
		t.Fatalf("expected key removed, exists=%d err=%v", n, err)
	}
}
