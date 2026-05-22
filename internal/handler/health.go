package handler

import (
	"context"
	"net/http"

	"social-notif/internal/config"
	"social-notif/internal/repository"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type HealthHandler struct {
	db    *gorm.DB
	redis *redis.Client
	dbCfg config.DatabaseConfig
}

func NewHealthHandler(db *gorm.DB, redisClient *redis.Client, dbCfg config.DatabaseConfig) *HealthHandler {
	return &HealthHandler{
		db:    db,
		redis: redisClient,
		dbCfg: dbCfg,
	}
}

func (h *HealthHandler) Liveness(c *gin.Context) {
	Respond(c, http.StatusOK, gin.H{
		"status": "ok",
	})
}

func (h *HealthHandler) Readiness(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.dbCfg.HealthCheckTimeout)
	defer cancel()

	if h.db != nil {
		if err := repository.CheckPostgres(ctx, h.db); err != nil {
			RespondError(c, http.StatusServiceUnavailable, "dependency_unavailable", "postgres is unavailable")
			return
		}
	}

	if h.redis != nil {
		if err := h.redis.Ping(ctx).Err(); err != nil {
			RespondError(c, http.StatusServiceUnavailable, "dependency_unavailable", "redis is unavailable")
			return
		}
	}

	Respond(c, http.StatusOK, gin.H{
		"status": "ready",
	})
}
