package api

import (
	"net/http"

	"social-notif/internal/config"
	"social-notif/internal/handler"
	"social-notif/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Dependencies struct {
	Config         *config.Config
	Logger         *zap.Logger
	DB             *gorm.DB
	Redis          *redis.Client
	MessageHandler *handler.MessageHandler
}

func NewRouter(deps Dependencies) http.Handler {
	if deps.Config.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	if len(deps.Config.HTTP.TrustedProxies) > 0 {
		if err := router.SetTrustedProxies(deps.Config.HTTP.TrustedProxies); err != nil {
			deps.Logger.Warn("failed to set trusted proxies", zap.Error(err))
		}
	}

	router.Use(middleware.RequestID())
	router.Use(middleware.Recovery(deps.Logger))
	router.Use(middleware.Logger(deps.Logger))
	router.Use(middleware.Timeout(deps.Config.HTTP.RequestTimeout))
	router.Use(middleware.BodySizeLimit(deps.Config.Security.MaxRequestBytes))
	router.Use(middleware.RateLimit(deps.Config.Security.RateLimitPerMin))

	healthHandler := handler.NewHealthHandler(deps.DB, deps.Redis, deps.Config.Database)

	router.GET("/healthz", healthHandler.Liveness)
	router.GET("/readyz", healthHandler.Readiness)

	v1 := router.Group("/api/v1")
	v1.Use(middleware.APIKeyAuth(deps.Config.Security.APIKey, deps.Logger))
	{
		v1.GET("/health", healthHandler.Readiness)

		if deps.MessageHandler != nil {
			v1.POST("/messages/whatsapp", deps.MessageHandler.CreateWhatsApp)
		}
	}

	return router
}
