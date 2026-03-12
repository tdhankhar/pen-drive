package files

import (
	"net/http"
	"strconv"

	"github.com/abhishek/pen-drive/backend/internal/api/dto"
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

func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, dto.ErrorResponse{
		Error: dto.ErrorPayload{
			Code:    code,
			Message: message,
		},
	})
}
