package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const deniedTTL = 24 * time.Hour

type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(addr string) *RedisCache {
	return &RedisCache{
		client: redis.NewClient(&redis.Options{Addr: addr}),
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
