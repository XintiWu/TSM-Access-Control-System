package cache

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

const deniedTTL = 24 * time.Hour

type RedisCache struct {
	client redis.UniversalClient
}

func NewRedisCache(addr string) *RedisCache {
	var client redis.UniversalClient
	if os.Getenv("REDIS_CLUSTER") == "true" {
		client = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs: []string{addr},
		})
	} else {
		client = redis.NewClient(&redis.Options{
			Addr: addr,
		})
	}
	return &RedisCache{
		client: client,
	}
}

func (c *RedisCache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

func (c *RedisCache) SetDenied(ctx context.Context, userID string) error {
	return c.client.Set(ctx, permDeniedKey(userID), "DENY", deniedTTL).Err()
}

func (c *RedisCache) ClearDenied(ctx context.Context, userID string) error {
	return c.client.Del(ctx, permDeniedKey(userID)).Err()
}

func permDeniedKey(userID string) string {
	return fmt.Sprintf("perm:denied:%s", userID)
}
