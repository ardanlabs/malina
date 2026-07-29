// Package runner configures and runs the Malina HTTP service.
package runner

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	service "github.com/ardanlabs/malina/cmd/server/api/services/malina/owner"
	"github.com/ardanlabs/malina/cmd/server/api/services/malina/static"
	"github.com/ardanlabs/malina/cmd/server/app/domain"
)

// Config controls server startup, request limits, and shutdown.
type Config struct {
	Host             string
	LibPath          string
	ModelPath        string
	QueueDepth       int
	ReadTimeout      time.Duration
	WriteTimeout     time.Duration
	IdleTimeout      time.Duration
	InferenceTimeout time.Duration
	ShutdownTimeout  time.Duration
	BUI              bool
}

// DefaultConfig returns environment-aware production defaults.
func DefaultConfig() Config {
	return Config{Host: env("MALINA_API_HOST", "127.0.0.1:8080"), LibPath: os.Getenv("MALINA_LIB"), ModelPath: os.Getenv("MALINA_MODEL"), QueueDepth: envInt("MALINA_QUEUE_DEPTH", 2), ReadTimeout: envDuration("MALINA_READ_TIMEOUT", 30*time.Second), WriteTimeout: envDuration("MALINA_WRITE_TIMEOUT", 60*time.Minute), IdleTimeout: envDuration("MALINA_IDLE_TIMEOUT", 2*time.Minute), InferenceTimeout: envDuration("MALINA_INFERENCE_TIMEOUT", 60*time.Minute), ShutdownTimeout: envDuration("MALINA_SHUTDOWN_TIMEOUT", 2*time.Minute), BUI: envBool("MALINA_BUI", true)}
}

// Validate checks whether Config can safely run the HTTP service.
func (cfg Config) Validate() error {
	if cfg.Host == "" {
		return errors.New("server: host is required")
	}
	if cfg.LibPath == "" {
		return errors.New("server: library path is required")
	}
	if cfg.QueueDepth < 1 {
		return errors.New("server: queue depth must be positive")
	}
	if cfg.ReadTimeout <= 0 || cfg.WriteTimeout <= 0 || cfg.IdleTimeout <= 0 || cfg.InferenceTimeout <= 0 || cfg.ShutdownTimeout <= 0 {
		return errors.New("server: timeouts must be positive")
	}
	return nil
}

// Run starts the server and gracefully drains it on cancellation or signals.
func Run(ctx context.Context, cfg Config) (runErr error) {
	if err := cfg.Validate(); err != nil {
		return err
	}
	svc, err := service.New(cfg.LibPath, cfg.QueueDepth)
	if err != nil {
		return fmt.Errorf("server: initializing service: %w", err)
	}
	defer func() {
		if _, ok := svc.Status(); !ok {
			return
		}
		unloadCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		runErr = errors.Join(runErr, svc.Unload(unloadCtx))
	}()
	if cfg.ModelPath != "" {
		loadCtx, cancel := context.WithTimeout(ctx, cfg.InferenceTimeout)
		err = svc.Load(loadCtx, cfg.ModelPath, cfg.QueueDepth)
		cancel()
		if err != nil {
			return fmt.Errorf("server: loading startup model: %w", err)
		}
	}
	var admin http.Handler
	if cfg.BUI {
		admin = static.Handler()
	}
	handler := domain.NewMux(svc, admin)
	httpServer := http.Server{Addr: cfg.Host, Handler: withTimeout(handler, cfg.InferenceTimeout), ReadTimeout: cfg.ReadTimeout, WriteTimeout: cfg.WriteTimeout, IdleTimeout: cfg.IdleTimeout}
	runCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	errCh := make(chan error, 1)
	go func() { errCh <- httpServer.ListenAndServe() }()
	select {
	case err = <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("server: serving: %w", err)
		}
		return nil
	case <-runCtx.Done():
	}
	httpCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	shutdownErr := httpServer.Shutdown(httpCtx)
	cancel()
	if shutdownErr != nil {
		shutdownErr = errors.Join(shutdownErr, httpServer.Close())
	}
	return shutdownErr
}

func withTimeout(next http.Handler, timeout time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
func envInt(name string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(name))
	if err == nil {
		return value
	}
	return fallback
}
func envDuration(name string, fallback time.Duration) time.Duration {
	value, err := time.ParseDuration(os.Getenv(name))
	if err == nil {
		return value
	}
	return fallback
}
func envBool(name string, fallback bool) bool {
	value, err := strconv.ParseBool(os.Getenv(name))
	if err == nil {
		return value
	}
	return fallback
}
