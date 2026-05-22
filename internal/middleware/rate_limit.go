package middleware

import (
	"net/http"
	"sync"
	"time"

	"social-notif/internal/handler"

	"github.com/gin-gonic/gin"
)

type visitor struct {
	count      int
	windowEnds time.Time
}

func RateLimit(limitPerMinute int) gin.HandlerFunc {
	if limitPerMinute <= 0 {
		return func(c *gin.Context) { c.Next() }
	}

	var mu sync.Mutex
	visitors := make(map[string]*visitor)
	window := time.Minute

	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			mu.Lock()
			now := time.Now()
			for key, value := range visitors {
				if now.After(value.windowEnds) {
					delete(visitors, key)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		key := c.ClientIP()
		now := time.Now()

		mu.Lock()
		v, exists := visitors[key]
		if !exists || now.After(v.windowEnds) {
			v = &visitor{windowEnds: now.Add(window)}
			visitors[key] = v
		}
		v.count++
		allowed := v.count <= limitPerMinute
		reset := time.Until(v.windowEnds).Seconds()
		mu.Unlock()

		if !allowed {
			c.Header("Retry-After", strconvFormatFloat(reset))
			handler.RespondError(c, http.StatusTooManyRequests, "rate_limited", "rate limit exceeded")
			c.Abort()
			return
		}

		c.Next()
	}
}

func strconvFormatFloat(v float64) string {
	return time.Duration(v * float64(time.Second)).Round(time.Second).String()
}
