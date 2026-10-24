package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"gorm.io/gorm"

	"gptgo/internal/platform/config"
	"gptgo/internal/platform/middleware"
	"gptgo/internal/platform/routing"
)

type HTTPServer struct {
	srv *http.Server
}

func NewEngine(cfg *config.Config, log *slog.Logger) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	middleware.Install(engine, cfg, log)
	return engine
}

func NewHTTPServer(cfg *config.Config, engine *gin.Engine) *HTTPServer {
	return &HTTPServer{srv: &http.Server{
		Addr:              cfg.Address(),
		Handler:           engine,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout.Std(),
		IdleTimeout:       cfg.Server.IdleTimeout.Std(),
		MaxHeaderBytes:    cfg.Server.MaxHeaderBytes,
	}}
}

type mountParams struct {
	fx.In
	Engine     *gin.Engine
	DB         *gorm.DB
	Registrars []routing.Registrar `group:"registrars"`
}

func Mount(p mountParams) {
	p.Engine.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "live"})
	})
	p.Engine.GET("/health/ready", func(c *gin.Context) {
		sqlDB, err := p.DB.DB()
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := sqlDB.PingContext(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
	api := p.Engine.Group("/api")
	for _, registrar := range p.Registrars {
		registrar.Register(api)
	}
}

func Hook(lc fx.Lifecycle, srv *HTTPServer, log *slog.Logger, shutdowner fx.Shutdowner) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			ln, err := net.Listen("tcp", srv.srv.Addr)
			if err != nil {
				return err
			}
			log.Info("http listening", "addr", ln.Addr().String())
			go func() {
				err := srv.srv.Serve(ln)
				if err != nil && !errors.Is(err, http.ErrServerClosed) {
					log.Error("http server exited", "err", err.Error())
					_ = shutdowner.Shutdown()
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return srv.srv.Shutdown(ctx)
		},
	})
}
