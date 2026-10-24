package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"gptgo/internal/platform/config"
	"gptgo/internal/platform/domain"
)

const requestIDHeader = "X-Request-Id"

func Install(engine *gin.Engine, cfg *config.Config, log *slog.Logger) {
	if err := engine.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		log.Error("trusted proxies rejected", "err", err.Error())
	}
	engine.Use(RequestID())
	engine.Use(Recovery(log))
	engine.Use(SecurityHeaders())
	engine.Use(AccessLog(log))
	engine.Use(MaxBody(cfg.Server.MaxBodyBytes))
	engine.Use(Deadline(cfg))
	engine.Use(LocalActor())
}

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.GetHeader(requestIDHeader))
		if id == "" || len(id) > 80 {
			id = newID()
		}
		c.Writer.Header().Set(requestIDHeader, id)
		c.Set("request_id", id)
		c.Next()
	}
}

func LocalActor() gin.HandlerFunc {
	return func(c *gin.Context) {
		id, _ := c.Get("request_id")
		requestID, _ := id.(string)
		ctx := domain.WithScope(c.Request.Context(), domain.Scope{ActorID: 1, RequestID: requestID})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func Recovery(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error("panic recovered", "err", rec, "path", c.Request.URL.Path)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "服务内部错误"})
			}
		}()
		c.Next()
	}
}

func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; font-src 'self' data:; script-src 'self'; connect-src 'self'; base-uri 'self'; frame-ancestors 'none'")
		h.Set("Permissions-Policy", "camera=(), microphone=(self), geolocation=()")
		c.Next()
	}
}

func AccessLog(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		path := c.Request.URL.Path
		if path == "/health/live" || path == "/health/ready" {
			return
		}
		log.Info("request",
			"method", c.Request.Method,
			"path", c.FullPath(),
			"route", path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", c.GetString("request_id"),
		)
	}
}

func MaxBody(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limit > 0 && c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		}
		c.Next()
	}
}

func Deadline(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		timeout := cfg.Server.RequestTimeout.Std()
		if strings.HasPrefix(c.Request.URL.Path, "/api/chat/stream") {
			timeout = cfg.Server.SSETimeout.Std()
		}
		ctx, cancel := contextWithTimeout(c, timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func newID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "rid"
	}
	return hex.EncodeToString(buf)
}
