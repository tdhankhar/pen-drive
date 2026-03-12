package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/abhishek/pen-drive/backend/internal/config"
	"github.com/abhishek/pen-drive/backend/internal/db"
	apphttp "github.com/abhishek/pen-drive/backend/internal/http"
	"github.com/abhishek/pen-drive/backend/internal/logging"
	"github.com/abhishek/pen-drive/backend/internal/storage"
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

	dbConn, err := db.Open(cfg.Database)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		return
	}
	defer dbConn.Close()

	if err := db.RunMigrations(dbConn); err != nil {
		logger.Error("database migrations failed", "error", err)
		return
	}

	storageClient, err := storage.NewClient(context.Background(), cfg.S3)
	if err != nil {
		logger.Error("storage client initialization failed", "error", err)
		return
	}

	router := apphttp.NewRouter(logger, dbConn, storageClient)

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
