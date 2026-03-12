package http

import "github.com/gin-gonic/gin"

type errorResponse struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, errorResponse{
		Error: errorPayload{
			Code:    code,
			Message: message,
		},
	})
}
