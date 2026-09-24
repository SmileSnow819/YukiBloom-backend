package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SmileSnow819/YukiBloom-backend/internal/config"
	"github.com/SmileSnow819/YukiBloom-backend/internal/database"
	"github.com/SmileSnow819/YukiBloom-backend/internal/httpserver"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Getenv); err != nil {
		log.Printf("server stopped: %v", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, getenv func(string) string) error {
	cfg, err := config.Load(getenv)
	if err != nil {
		return err
	}

	startupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	pool, err := database.Open(startupCtx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("database startup: %w", err)
	}
	defer pool.Close()
	if err := database.Migrate(startupCtx, pool); err != nil {
		return fmt.Errorf("database migration: %w", err)
	}

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpserver.NewRouter(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- server.ListenAndServe() }()
	log.Printf("API listening on port %s", cfg.Port)

	select {
	case err := <-serveErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("HTTP server: %w", err)
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("HTTP shutdown: %w", err)
		}
		return nil
	}
}
