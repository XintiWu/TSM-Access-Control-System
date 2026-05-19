package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/tsmc/access-api/internal/model"
)

const (
	passbackTTL = 24 * time.Hour
	doorTTL     = 30 * time.Second
)

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

func (c *RedisCache) IsDenied(ctx context.Context, userID string) (bool, error) {
	n, err := c.client.Exists(ctx, permDeniedKey(userID)).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (c *RedisCache) GetPassback(ctx context.Context, userID string) (model.PassbackState, error) {
	val, err := c.client.Get(ctx, passbackKey(userID)).Result()
	if err == redis.Nil {
		return model.PassbackNone, nil
	}
	if err != nil {
		return model.PassbackNone, err
	}
	switch val {
	case string(model.PassbackIN):
		return model.PassbackIN, nil
	case string(model.PassbackOUT):
		return model.PassbackOUT, nil
	default:
		return model.PassbackNone, nil
	}
}

func (c *RedisCache) SetPassback(ctx context.Context, userID string, state model.PassbackState) error {
	return c.client.Set(ctx, passbackKey(userID), string(state), passbackTTL).Err()
}

func (c *RedisCache) GetDoorStatus(ctx context.Context, doorID string) (string, error) {
	val, err := c.client.Get(ctx, doorStatusKey(doorID)).Result()
	if err == redis.Nil {
		return "OFFLINE", nil
	}
	if err != nil {
		return "", err
	}
	return val, nil
}

func (c *RedisCache) SetDoorStatus(ctx context.Context, doorID, status string) error {
	return c.client.Set(ctx, doorStatusKey(doorID), status, doorTTL).Err()
}

func permDeniedKey(userID string) string {
	return fmt.Sprintf("perm:denied:%s", userID)
}

func passbackKey(userID string) string {
	return fmt.Sprintf("passback:%s", userID)
}

func doorStatusKey(doorID string) string {
	return fmt.Sprintf("door:status:%s", doorID)
}
