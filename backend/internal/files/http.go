package files

import (
	"errors"
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

// UploadFolder godoc
// @Summary Upload folder
// @Description Upload multiple files to a destination folder in the user's bucket, preserving relative paths
// @Tags files
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param files formData file true "Files to upload (repeated)"
// @Param relative_paths formData string true "Relative path for each file (repeated, must match files count)"
// @Param path formData string false "Destination folder path within bucket"
// @Success 201 {object} dto.FolderUploadResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 409 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/v1/files/upload-folder [post]
func (h *Handler) UploadFolder(c *gin.Context) {
	userID, _ := c.Get("auth.user_id")

	// Parse multipart form
	err := c.Request.ParseMultipartForm(10 << 20) // 10MB limit for all files
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_multipart", "failed to parse multipart form")
		return
	}

	// Extract files
	files := c.Request.MultipartForm.File["files"]
	if len(files) == 0 {
		respondError(c, http.StatusBadRequest, "missing_files", "files field is required")
		return
	}

	// Extract relative paths
	relativePaths := c.Request.Form["relative_paths"]
	if len(relativePaths) == 0 {
		respondError(c, http.StatusBadRequest, "invalid_input", "relative_paths field is required")
		return
	}

	// Validate array alignment
	if len(files) != len(relativePaths) {
		respondError(c, http.StatusBadRequest, "invalid_input", "files and relative_paths counts must match")
		return
	}

	// Get destination path
	destinationPath := c.PostForm("path")

	// Call service layer
	results, err := h.service.UploadFolder(
		c.Request.Context(),
		userID.(string),
		destinationPath,
		files,
		relativePaths,
	)
	if err != nil {
		if errors.Is(err, storage.ErrObjectAlreadyExists) {
			respondError(c, http.StatusConflict, "file_exists", err.Error())
			return
		}
		if errors.Is(err, ErrInvalidUploadInput) || errors.Is(err, ErrZeroByteFile) || errors.Is(err, ErrDuplicateBatchKey) {
			respondError(c, http.StatusBadRequest, "invalid_input", err.Error())
			return
		}
		respondError(c, http.StatusInternalServerError, "upload_failed", err.Error())
		return
	}

	// Return 201 Created with upload metadata
	uploadedFiles := make([]dto.UploadedFileInfo, len(results))
	for i, result := range results {
		uploadedFiles[i] = dto.UploadedFileInfo{
			Name:       result.Name,
			Path:       result.Path,
			Size:       result.Size,
			UploadedAt: result.UploadedAt,
		}
	}

	c.JSON(http.StatusCreated, dto.FolderUploadResponse{
		Files: uploadedFiles,
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
