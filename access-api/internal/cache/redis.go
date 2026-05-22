package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
	"github.com/tsmc/access-api/internal/model"
)

const (
	passbackTTL = 24 * time.Hour
	cardTTL     = 24 * time.Hour
	doorTTL     = 30 * time.Second
)

var (
	cacheOps = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "access_api_cache_ops_total",
		Help: "Redis cache operations by result",
	}, []string{"op", "result"}) // result: hit, miss, error
)

type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(addr string) *RedisCache {
	return &RedisCache{
		client: redis.NewClient(&redis.Options{
			Addr:         addr,
			PoolSize:     128,
			MinIdleConns: 32,
			PoolTimeout:  2 * time.Second,
		}),
	}
}

func (c *RedisCache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

func (c *RedisCache) IsDenied(ctx context.Context, userID string) (bool, error) {
	n, err := c.client.Exists(ctx, permDeniedKey(userID)).Result()
	if err != nil {
		cacheOps.WithLabelValues("is_denied", "error").Inc()
		return false, err
	}
	if n > 0 {
		cacheOps.WithLabelValues("is_denied", "hit").Inc()
	} else {
		cacheOps.WithLabelValues("is_denied", "miss").Inc()
	}
	return n > 0, nil
}

func (c *RedisCache) GetPassback(ctx context.Context, userID string) (model.PassbackState, error) {
	val, err := c.client.Get(ctx, passbackKey(userID)).Result()
	if err == redis.Nil {
		cacheOps.WithLabelValues("get_passback", "miss").Inc()
		return model.PassbackNone, nil
	}
	if err != nil {
		cacheOps.WithLabelValues("get_passback", "error").Inc()
		return model.PassbackNone, err
	}
	cacheOps.WithLabelValues("get_passback", "hit").Inc()
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

// LookupCard returns the userId mapped to the given cardUID.
// Returns ("", nil) if the card is not found in cache.
func (c *RedisCache) LookupCard(ctx context.Context, cardUID string) (string, error) {
	val, err := c.client.Get(ctx, cardKey(cardUID)).Result()
	if err == redis.Nil {
		cacheOps.WithLabelValues("lookup_card", "miss").Inc()
		return "", nil
	}
	if err != nil {
		cacheOps.WithLabelValues("lookup_card", "error").Inc()
		return "", err
	}
	cacheOps.WithLabelValues("lookup_card", "hit").Inc()
	return val, nil
}

// SetCardMapping writes a card→userId mapping into Redis cache.
func (c *RedisCache) SetCardMapping(ctx context.Context, cardUID, userID string) error {
	return c.client.Set(ctx, cardKey(cardUID), userID, cardTTL).Err()
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

func cardKey(cardUID string) string {
	return fmt.Sprintf("card:%s", cardUID)
}

func doorStatusKey(doorID string) string {
	return fmt.Sprintf("door:status:%s", doorID)
}
