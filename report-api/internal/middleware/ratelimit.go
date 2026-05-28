package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// clientTracker holds rate limiting state for a single client IP.
type clientTracker struct {
	tokens     int
	lastRefill time.Time
}

// RateLimit returns a Gin middleware for simple per-IP rate limiting (token bucket).
func RateLimit(rps int) gin.HandlerFunc {
	capacity := rps

	var mu sync.Mutex
	clients := make(map[string]*clientTracker)

	go func() {
		for {
			time.Sleep(5 * time.Minute)
			mu.Lock()
			now := time.Now()
			for ip, tracker := range clients {
				if now.Sub(tracker.lastRefill) > 5*time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

		mu.Lock()
		tracker, exists := clients[ip]
		if !exists {
			tracker = &clientTracker{
				tokens:     capacity - 1,
				lastRefill: now,
			}
			clients[ip] = tracker
			mu.Unlock()
			c.Next()
			return
		}

		elapsed := now.Sub(tracker.lastRefill)
		tokensToAdd := int(elapsed.Seconds() * float64(rps))
		if tokensToAdd > 0 {
			tracker.tokens += tokensToAdd
			if tracker.tokens > capacity {
				tracker.tokens = capacity
			}
			tracker.lastRefill = now
		}

		if tracker.tokens > 0 {
			tracker.tokens--
			mu.Unlock()
			c.Next()
		} else {
			mu.Unlock()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
		}
	}
}
