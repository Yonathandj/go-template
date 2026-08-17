package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/supernurture/go-template/internal/api/server"
	"github.com/supernurture/go-template/internal/config"
	"github.com/supernurture/go-template/internal/container"
)

const (
	shutdownTimeout = 8 * time.Second
	idleTimeout     = 60 * time.Second
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() (err error) {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	deps, err := container.NewContainer(cfg)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := deps.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close dependencies: %w", closeErr)
		}
	}()

	router, err := server.NewRouter(cfg, deps)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Handler:           router,
		ReadHeaderTimeout: cfg.Server.Timeout,
		IdleTimeout:       idleTimeout,
	}

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srvErr := make(chan error, 1)
	go func() {
		if serveErr := srv.Serve(listener); !errors.Is(serveErr, http.ErrServerClosed) {
			srvErr <- serveErr
		}
	}()
	deps.Logger.Info("server listening", map[string]any{"addr": listener.Addr().String()})

	select {
	case serveErr := <-srvErr:
		return fmt.Errorf("serve: %w", serveErr)
	case <-ctx.Done():
	}
	stop()

	deps.Logger.Info("shutting down", map[string]any{"timeout": shutdownTimeout.String()})

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if shutdownErr := srv.Shutdown(shutdownCtx); shutdownErr != nil {
		return fmt.Errorf("shutdown server: %w", shutdownErr)
	}

	return nil
}
