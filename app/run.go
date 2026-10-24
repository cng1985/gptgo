package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

// Run 打开本地数据库并启动页面服务。
// 监听地址默认 :8080，可用 GPTGO_ADDR 覆盖；数据库默认 data/gptgo.db，可用 GPTGO_DB 覆盖。
func Run() {
	if err := run(); err != nil {
		slog.Error("GPTGO 退出", "err", err)
		os.Exit(1)
	}
}

func run() error {
	addr := getenv("GPTGO_ADDR", ":8080")
	dbPath := getenv("GPTGO_DB", filepath.Join("data", "gptgo.db"))

	store, err := Open(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()

	server, err := NewServer(store)
	if err != nil {
		return err
	}

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("GPTGO 正在监听", "addr", addr)
		errCh <- httpServer.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(stop)

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-stop:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpServer.Shutdown(ctx)
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
