package http

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"github.com/abhishek/pen-drive/backend/internal/auth"
	"github.com/abhishek/pen-drive/backend/internal/config"
	"github.com/abhishek/pen-drive/backend/internal/storage"
	"github.com/abhishek/pen-drive/backend/internal/users"
	"github.com/gin-gonic/gin"
)

func NewRouter(logger *slog.Logger, dbConn *sql.DB, storageClient *storage.Client, jwtConfig config.JWTConfig) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(RequestLogger(logger))

	userRepo := users.NewRepository(dbConn)
	authService := auth.NewService(dbConn, userRepo, storageClient, jwtConfig)
	authHandler := auth.NewHandler(authService)

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	})

	router.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()

		if err := dbConn.PingContext(ctx); err != nil {
			RespondError(c, http.StatusServiceUnavailable, "database_unavailable", err.Error())
			return
		}

		if err := storageClient.Ping(ctx); err != nil {
			RespondError(c, http.StatusServiceUnavailable, "storage_unavailable", err.Error())
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "ready",
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	})

	api := router.Group("/api/v1")
	authGroup := api.Group("/auth")
	authGroup.POST("/signup", authHandler.Signup)
	authGroup.POST("/login", authHandler.Login)
	authGroup.POST("/refresh", authHandler.Refresh)

	api.GET("/me", authHandler.AuthMiddleware(), authHandler.Me)

	return router
}
