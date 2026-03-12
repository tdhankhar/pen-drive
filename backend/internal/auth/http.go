package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

type credentialsRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type authResponse struct {
	User   AuthenticatedUser `json:"user"`
	Tokens TokenPair         `json:"tokens"`
}

func (h *Handler) Signup(c *gin.Context) {
	var req credentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}

	user, tokens, err := h.service.Signup(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		respondError(c, signupStatus(err), signupCode(err), err.Error())
		return
	}

	c.JSON(http.StatusCreated, authResponse{User: user, Tokens: tokens})
}

func (h *Handler) Login(c *gin.Context) {
	var req credentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}

	user, tokens, err := h.service.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		status := http.StatusUnauthorized
		code := "invalid_credentials"
		if !errors.Is(err, ErrInvalidCredentials) {
			status = http.StatusInternalServerError
			code = "login_failed"
		}
		respondError(c, status, code, err.Error())
		return
	}

	c.JSON(http.StatusOK, authResponse{User: user, Tokens: tokens})
}

func (h *Handler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}

	user, tokens, err := h.service.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		status := http.StatusUnauthorized
		code := "invalid_refresh_token"
		if !errors.Is(err, ErrInvalidToken) {
			status = http.StatusInternalServerError
			code = "refresh_failed"
		}
		respondError(c, status, code, err.Error())
		return
	}

	c.JSON(http.StatusOK, authResponse{User: user, Tokens: tokens})
}

func (h *Handler) Me(c *gin.Context) {
	userID, _ := c.Get(userIDContextKey)
	email, _ := c.Get(userEmailContextKey)

	c.JSON(http.StatusOK, AuthenticatedUser{
		ID:    userID.(string),
		Email: email.(string),
	})
}

func (h *Handler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			respondError(c, http.StatusUnauthorized, "missing_authorization", "authorization header is required")
			c.Abort()
			return
		}

		token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer"))
		if token == "" || token == header {
			respondError(c, http.StatusUnauthorized, "invalid_authorization", "bearer token is required")
			c.Abort()
			return
		}

		claims, err := h.service.ParseAccessToken(token)
		if err != nil {
			respondError(c, http.StatusUnauthorized, "invalid_access_token", "access token is invalid")
			c.Abort()
			return
		}

		c.Set(userIDContextKey, claims.Subject)
		c.Set(userEmailContextKey, claims.Email)
		c.Next()
	}
}

func signupStatus(err error) int {
	if strings.Contains(err.Error(), "email is invalid") || strings.Contains(err.Error(), "password must be") {
		return http.StatusBadRequest
	}

	return http.StatusInternalServerError
}

func signupCode(err error) string {
	if strings.Contains(err.Error(), "email is invalid") || strings.Contains(err.Error(), "password must be") {
		return "invalid_signup"
	}

	return "signup_failed"
}

const (
	userIDContextKey    = "auth.user_id"
	userEmailContextKey = "auth.user_email"
)

func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	})
}
