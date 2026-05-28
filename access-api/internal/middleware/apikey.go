package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// APIKeyAuth returns a Gin middleware that validates the X-API-Key header.
// Requests to /health and /metrics are exempt from authentication.
// If apiKey is empty, the middleware is a no-op (auth disabled).
func APIKeyAuth(apiKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if apiKey == "" {
			c.Next()
			return
		}

		// Skip auth for health checks and metrics (infrastructure endpoints)
		path := c.Request.URL.Path
		if path == "/health" || path == "/metrics" {
			c.Next()
			return
		}

		provided := c.GetHeader("X-API-Key")
		if provided == "" {
			provided = extractBearerToken(c.GetHeader("Authorization"))
		}

		if provided == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "missing authentication: provide X-API-Key header or Bearer token",
			})
			return
		}

		if provided != apiKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid API key",
			})
			return
		}

		c.Next()
	}
}

// extractBearerToken extracts the token from an "Authorization: Bearer <token>" header.
func extractBearerToken(auth string) string {
	const prefix = "Bearer "
	if len(auth) > len(prefix) && strings.EqualFold(auth[:len(prefix)], prefix) {
		return auth[len(prefix):]
	}
	return ""
}
