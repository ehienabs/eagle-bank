package middleware

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ehienabs/eagle-bank/pkg/errors"
	"github.com/ehienabs/eagle-bank/pkg/metrics"
	"golang.org/x/time/rate"
)

// RateLimiter implements a per-client rate limiter
type RateLimiter struct {
	limiters map[string]*clientLimiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
	metrics  *metrics.Metrics
}

type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requestsPerSecond float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*clientLimiter),
		rate:     rate.Limit(requestsPerSecond),
		burst:    burst,
		metrics:  metrics.Default(),
	}

	// Clean up old limiters periodically
	go rl.cleanup()

	return rl
}

// getLimiter returns the rate limiter for a client
func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.limiters[key]
	if !exists {
		limiter = &clientLimiter{
			limiter:  rate.NewLimiter(rl.rate, rl.burst),
			lastSeen: time.Now(),
		}
		rl.limiters[key] = limiter
	} else {
		limiter.lastSeen = time.Now()
	}

	return limiter.limiter
}

// cleanup removes old rate limiters
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		for key, limiter := range rl.limiters {
			if time.Since(limiter.lastSeen) > 3*time.Minute {
				delete(rl.limiters, key)
			}
		}
		rl.mu.Unlock()
	}
}

// Middleware returns a rate limiting middleware
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Use user ID if authenticated, otherwise use IP
		key := c.ClientIP()
		userType := "anonymous"

		if userID := GetUserID(c); userID != "" {
			key = userID
			userType = "authenticated"
		}

		limiter := rl.getLimiter(key)

		if !limiter.Allow() {
			rl.metrics.RecordRateLimitHit(c.FullPath(), userType)
			respondError(c, errors.RateLimited("rate limit exceeded, please try again later"))
			c.Abort()
			return
		}

		c.Next()
	}
}

// RateLimitMiddleware creates a rate limiting middleware with config
func RateLimitMiddleware(requestsPerMinute int, burst int) gin.HandlerFunc {
	requestsPerSecond := float64(requestsPerMinute) / 60.0
	rl := NewRateLimiter(requestsPerSecond, burst)
	return rl.Middleware()
}
