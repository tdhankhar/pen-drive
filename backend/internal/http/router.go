package http

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"github.com/abhishek/pen-drive/backend/internal/api/dto"
	"github.com/abhishek/pen-drive/backend/internal/auth"
	"github.com/abhishek/pen-drive/backend/internal/config"
	"github.com/abhishek/pen-drive/backend/internal/files"
	"github.com/abhishek/pen-drive/backend/internal/storage"
	"github.com/abhishek/pen-drive/backend/internal/users"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/abhishek/pen-drive/backend/docs/openapi"
)

func NewRouter(logger *slog.Logger, dbConn *sql.DB, storageClient *storage.Client, jwtConfig config.JWTConfig, secureCookies bool) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(RequestLogger(logger))
	router.Use(
		cors.New(cors.Config{
			AllowOrigins:     []string{"http://127.0.0.1:5173", "http://localhost:5173"},
			AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
			ExposeHeaders:    []string{"X-Request-Id"},
			AllowCredentials: true,
		}),
	)

	// Swagger UI is served directly from the generated backend contract.
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))

	userRepo := users.NewRepository(dbConn)
	authService := auth.NewService(dbConn, userRepo, storageClient, jwtConfig)
	authHandler := auth.NewHandler(authService, secureCookies)
	filesService := files.NewService(storageClient)
	filesHandler := files.NewHandler(filesService)

	// Healthz godoc
	// @Summary Liveness
	// @Description Basic liveness probe.
	// @Tags system
	// @Produce json
	// @Success 200 {object} dto.HealthResponse
	// @Router /healthz [get]
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, dto.HealthResponse{
			Status: "ok",
			Time:   time.Now().UTC().Format(time.RFC3339),
		})
	})

	// Readyz godoc
	// @Summary Readiness
	// @Description Validate database and object storage connectivity.
	// @Tags system
	// @Produce json
	// @Success 200 {object} dto.HealthResponse
	// @Failure 503 {object} dto.ErrorResponse
	// @Router /readyz [get]
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

		c.JSON(http.StatusOK, dto.HealthResponse{
			Status: "ready",
			Time:   time.Now().UTC().Format(time.RFC3339),
		})
	})

	api := router.Group("/api/v1")
	authGroup := api.Group("/auth")
	authGroup.POST("/signup", authHandler.Signup)
	authGroup.POST("/login", authHandler.Login)
	authGroup.POST("/refresh", authHandler.Refresh)

	api.GET("/me", authHandler.AuthMiddleware(), authHandler.Me)
	api.GET("/files", authHandler.AuthMiddleware(), filesHandler.List)
	api.POST("/files/duplicates/preview", authHandler.AuthMiddleware(), filesHandler.PreviewDuplicates)
	api.POST("/files/upload", authHandler.AuthMiddleware(), filesHandler.Upload)
	api.POST("/files/upload-folder", authHandler.AuthMiddleware(), filesHandler.UploadFolder)
	api.POST("/files/upload-multipart/initiate", authHandler.AuthMiddleware(), filesHandler.InitiateMultipartUpload)
	api.POST("/files/upload-multipart/part", authHandler.AuthMiddleware(), filesHandler.UploadMultipartPart)
	api.POST("/files/upload-multipart/complete", authHandler.AuthMiddleware(), filesHandler.CompleteMultipartUpload)
	api.POST("/files/upload-multipart/abort", authHandler.AuthMiddleware(), filesHandler.AbortMultipartUpload)

	return router
}
