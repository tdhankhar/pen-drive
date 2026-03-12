package files

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/abhishek/pen-drive/backend/internal/api/dto"
	"github.com/abhishek/pen-drive/backend/internal/storage"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// List godoc
// @Summary List files
// @Description List folders and files under the authenticated user's bucket path.
// @Tags files
// @Produce json
// @Security BearerAuth
// @Param path query string false "Relative folder path"
// @Param continuation_token query string false "Continuation token from previous page"
// @Param limit query int false "Max keys to request from S3-compatible storage"
// @Success 200 {object} dto.FileListResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/v1/files [get]
func (h *Handler) List(c *gin.Context) {
	userID, _ := c.Get("auth.user_id")
	limit := int32(100)
	if rawLimit := c.Query("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 || parsed > 1000 {
			respondError(c, http.StatusBadRequest, "invalid_limit", "limit must be between 1 and 1000")
			return
		}
		limit = int32(parsed)
	}

	response, err := h.service.List(
		c.Request.Context(),
		userID.(string),
		c.Query("path"),
		c.Query("continuation_token"),
		limit,
	)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}

	c.JSON(http.StatusOK, response)
}

// Upload godoc
// @Summary Upload file
// @Description Upload a single file to a destination path in the user's bucket
// @Tags files
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "File to upload"
// @Param path formData string false "Destination path within bucket"
// @Param filename formData string false "Override filename"
// @Success 201 {object} dto.FileUploadResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 409 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/v1/files/upload [post]
func (h *Handler) Upload(c *gin.Context) {
	userID, _ := c.Get("auth.user_id")

	// Extract multipart file
	file, err := c.FormFile("file")
	if err != nil {
		respondError(c, http.StatusBadRequest, "missing_file", "file field is required")
		return
	}

	// Validate file size > 0
	if file.Size == 0 {
		respondError(c, http.StatusBadRequest, "zero_byte_file", "file cannot be empty")
		return
	}

	// Open the file
	src, err := file.Open()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "file_open_failed", err.Error())
		return
	}
	defer src.Close()

	// Get optional parameters
	destinationPath := c.PostForm("path")
	filenameOverride := c.PostForm("filename")

	// Use override if provided, otherwise use uploaded filename
	filename := filenameOverride
	if filename == "" {
		filename = file.Filename
	}

	// Call service layer
	result, err := h.service.Upload(
		c.Request.Context(),
		userID.(string),
		destinationPath,
		filename,
		file.Header.Get("Content-Type"),
		src,
		file.Size,
	)
	if err != nil {
		// Handle specific errors
		if strings.Contains(err.Error(), "already exists") {
			respondError(c, http.StatusConflict, "file_exists", err.Error())
			return
		}
		if strings.Contains(err.Error(), "filename") || strings.Contains(err.Error(), "path") {
			respondError(c, http.StatusBadRequest, "invalid_input", err.Error())
			return
		}
		if err == storage.ErrObjectAlreadyExists {
			respondError(c, http.StatusConflict, "file_exists", "file already exists at this path")
			return
		}
		respondError(c, http.StatusInternalServerError, "upload_failed", err.Error())
		return
	}

	// Return 201 Created with upload metadata
	c.JSON(http.StatusCreated, dto.FileUploadResponse{
		File: dto.UploadedFileInfo{
			Name:       result.Name,
			Path:       result.Path,
			Size:       result.Size,
			UploadedAt: result.UploadedAt,
		},
	})
}

func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, dto.ErrorResponse{
		Error: dto.ErrorPayload{
			Code:    code,
			Message: message,
		},
	})
}
