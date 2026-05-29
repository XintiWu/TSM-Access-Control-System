package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// APIKeyAuth returns a Gin middleware that validates the X-API-Key header.
// Requests to /health and /metrics are always exempt from authentication.
// Additional path prefixes may be passed via extraBypassPrefixes (e.g. "/ui").
// If apiKey is empty, the middleware is a no-op (auth disabled).
func APIKeyAuth(apiKey string, extraBypassPrefixes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if apiKey == "" {
			c.Next()
			return
		}

		path := c.Request.URL.Path
		if path == "/health" || path == "/metrics" {
			c.Next()
			return
		}
		for _, prefix := range extraBypassPrefixes {
			if strings.HasPrefix(path, prefix) {
				c.Next()
				return
			}
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

		if subtle.ConstantTimeCompare([]byte(provided), []byte(apiKey)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "invalid API key",
			})
			return
		}

		c.Next()
	}
}

func extractBearerToken(auth string) string {
	const prefix = "Bearer "
	if len(auth) > len(prefix) && strings.EqualFold(auth[:len(prefix)], prefix) {
		return auth[len(prefix):]
	}
	return ""
}
