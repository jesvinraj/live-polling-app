package middleware

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"live-polling-tool/database"

	"github.com/gin-gonic/gin"
)

type memoryLimiter struct {
	sync.Mutex
	counts map[string]int
	expiry map[string]time.Time
}

var fallbackLimiter = &memoryLimiter{
	counts: make(map[string]int),
	expiry: make(map[string]time.Time),
}

func RateLimiter(redisInst *database.RedisInstance, prefix string, maxRequests int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		if clientIP == "" {
			clientIP = "unknown"
		}

		key := fmt.Sprintf("ratelimit:%s:%s", prefix, clientIP)

		// Try Redis first
		if redisInst != nil && redisInst.Client != nil {
			ctx := c.Request.Context()
			count, err := redisInst.Client.Incr(ctx, key).Result()
			if err == nil {
				if count == 1 {
					redisInst.Client.Expire(ctx, key, window)
				}
				if count > int64(maxRequests) {
					c.JSON(http.StatusTooManyRequests, gin.H{
						"error": "too many requests, please slow down and try again shortly",
					})
					c.Abort()
					return
				}
				c.Next()
				return
			}
		}

		// In-memory fallback if Redis is offline
		fallbackLimiter.Lock()
		now := time.Now()
		exp, exists := fallbackLimiter.expiry[key]
		if !exists || now.After(exp) {
			fallbackLimiter.counts[key] = 1
			fallbackLimiter.expiry[key] = now.Add(window)
		} else {
			fallbackLimiter.counts[key]++
		}
		currentCount := fallbackLimiter.counts[key]
		fallbackLimiter.Unlock()

		if currentCount > maxRequests {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "too many requests, please slow down and try again shortly",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
