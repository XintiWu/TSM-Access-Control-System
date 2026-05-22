package service

import "errors"

// ErrCacheUnavailable is returned when Redis is down and no DB fallback is configured.
var ErrCacheUnavailable = errors.New("cache unavailable and no DB fallback configured")
