package handlers

import (
	"context"
	"net/http"
	"sync"
	"time"

	"live-polling-tool/database"

	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
	mu           sync.RWMutex
	mongoInst    *database.MongoInstance
	redisInst    *database.RedisInstance
	lastCheck    time.Time
	cachedStatus map[string]string
	cacheTTL     time.Duration
}

func NewHealthHandler(mongoInst *database.MongoInstance, redisInst *database.RedisInstance) *HealthHandler {
	return &HealthHandler{
		mongoInst: mongoInst,
		redisInst: redisInst,
		cacheTTL:  60 * time.Second,
		cachedStatus: map[string]string{
			"database": "unknown",
			"redis":    "unknown",
		},
	}
}

// Healthz is a lightweight liveness probe (for Render/Kubernetes) with 0 database or Redis commands
func (h *HealthHandler) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"time":   time.Now().UTC(),
	})
}

// DetailedHealth checks database and Redis connectivity, caching results for 60 seconds to avoid ping spam
func (h *HealthHandler) DetailedHealth(c *gin.Context) {
	h.mu.RLock()
	isCacheValid := time.Since(h.lastCheck) < h.cacheTTL && h.cachedStatus["database"] != "unknown"
	dbStatus := h.cachedStatus["database"]
	redisStatus := h.cachedStatus["redis"]
	h.mu.RUnlock()

	if !isCacheValid {
		h.mu.Lock()
		if time.Since(h.lastCheck) >= h.cacheTTL || h.cachedStatus["database"] == "unknown" {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			dbStatus = "connected"
			if h.mongoInst == nil || h.mongoInst.Client == nil || h.mongoInst.Client.Ping(ctx, nil) != nil {
				dbStatus = "disconnected"
			}

			redisStatus = "connected"
			if h.redisInst == nil || h.redisInst.Client == nil || h.redisInst.Client.Ping(ctx).Err() != nil {
				redisStatus = "disconnected"
			}

			h.cachedStatus["database"] = dbStatus
			h.cachedStatus["redis"] = redisStatus
			h.lastCheck = time.Now()
		} else {
			dbStatus = h.cachedStatus["database"]
			redisStatus = h.cachedStatus["redis"]
		}
		h.mu.Unlock()
	}

	overallStatus := "healthy"
	if dbStatus != "connected" || redisStatus != "connected" {
		overallStatus = "degraded"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   overallStatus,
		"database": dbStatus,
		"redis":    redisStatus,
		"clientIp": c.ClientIP(),
		"cached":   isCacheValid,
		"time":     time.Now().UTC(),
	})
}
