package cache_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/tsmc/cache-invalidation-worker/internal/cache"
)

// redisAddr returns the address of a live Redis instance.
// Prefers the REDIS_ADDR env variable (set in CI by docker compose),
// then falls back to the local default. Skips the test if unreachable.
func redisAddr(t *testing.T) string {
	t.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	// Verify connectivity before running; skip instead of fail if unavailable.
	client := redis.NewClient(&redis.Options{Addr: addr})
	defer client.Close()
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skipf("Redis not reachable at %s (%v) — skipping integration test", addr, err)
	}
	return addr
}

func TestSetAndClearDenied(t *testing.T) {
	addr := redisAddr(t)
	ctx := context.Background()
	userID := "22222222-2222-2222-2222-222222222222"

	rc := cache.NewRedisCache(addr)
	client := redis.NewClient(&redis.Options{Addr: addr})
	defer client.Close()

	// Clean up before and after to keep tests idempotent.
	_ = client.Del(ctx, fmt.Sprintf("perm:denied:%s", userID))
	t.Cleanup(func() {
		_ = client.Del(ctx, fmt.Sprintf("perm:denied:%s", userID))
	})

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

func TestRedisCache_Ping(t *testing.T) {
	addr := redisAddr(t)
	ctx := context.Background()
	rc := cache.NewRedisCache(addr)
	err := rc.Ping(ctx)
	if err != nil {
		t.Errorf("expected no error on ping, got %v", err)
	}
}

func TestNewRedisCache_Cluster(t *testing.T) {
	os.Setenv("REDIS_CLUSTER", "true")
	defer os.Unsetenv("REDIS_CLUSTER")

	rc := cache.NewRedisCache("localhost:6379")
	if rc == nil {
		t.Fatal("expected non-nil RedisCache")
	}
}
