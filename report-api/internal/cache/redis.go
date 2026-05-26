package cache

import (
	"context"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

const reportCacheTTL = 5 * time.Minute

// ReportCache provides Redis-based caching for pre-computed report responses.
type ReportCache struct {
	client redis.UniversalClient
}

// NewReportCache creates a new Redis-backed report cache.
func NewReportCache(addr string) *ReportCache {
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
	return &ReportCache{
		client: client,
	}
}

// Ping checks connectivity.
func (c *ReportCache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Get retrieves a cached report. Returns (nil, nil) on cache miss.
func (c *ReportCache) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := c.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return val, nil
}

// Set stores a report in cache with the default TTL.
func (c *ReportCache) Set(ctx context.Context, key string, data []byte) error {
	return c.client.Set(ctx, key, data, reportCacheTTL).Err()
}

// Invalidate removes all keys matching a pattern (e.g., "report:dept:orgId:*").
// Uses SCAN to avoid blocking Redis.
func (c *ReportCache) Invalidate(ctx context.Context, pattern string) error {
	iter := c.client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		if err := c.client.Del(ctx, iter.Val()).Err(); err != nil {
			return err
		}
	}
	return iter.Err()
}
