package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/abhishek/pen-drive/backend/internal/config"
	apphttp "github.com/abhishek/pen-drive/backend/internal/http"
	"github.com/abhishek/pen-drive/backend/internal/logging"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	logger, closeLogs, err := logging.New(cfg.Logging)
	if err != nil {
		log.Fatalf("init logging: %v", err)
	}
	defer closeLogs()

	router := apphttp.NewRouter(logger)

	server := &http.Server{
		Addr:              cfg.HTTP.Address(),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("server starting", "addr", server.Addr, "env", cfg.AppEnv)

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		logger.Info("server shutting down")
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("server shutdown failed", "error", err)
		}
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server stopped unexpectedly", "error", err)
		return
	}

	logger.Info("server stopped")
}
