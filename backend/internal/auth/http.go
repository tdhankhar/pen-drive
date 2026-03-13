package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/abhishek/pen-drive/backend/internal/api/dto"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
	secure  bool
}

func NewHandler(service *Service, secure bool) *Handler {
	return &Handler{service: service, secure: secure}
}

// Signup godoc
// @Summary Sign up
// @Description Create a user, provision a bucket, and issue auth tokens.
// @Tags auth
// @Accept json
// @Produce json
// @Param payload body dto.CredentialsRequest true "Signup payload"
// @Success 201 {object} dto.AuthResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/v1/auth/signup [post]
func (h *Handler) Signup(c *gin.Context) {
	var req dto.CredentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}

	user, tokens, err := h.service.Signup(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		respondError(c, signupStatus(err), signupCode(err), err.Error())
		return
	}

	h.setRefreshCookie(c, tokens.RefreshToken, time.Until(tokens.RefreshTokenExpiresAt))
	c.JSON(http.StatusCreated, toAuthResponse(user, tokens))
}

// Login godoc
// @Summary Log in
// @Description Authenticate a user and issue auth tokens.
// @Tags auth
// @Accept json
// @Produce json
// @Param payload body dto.CredentialsRequest true "Login payload"
// @Success 200 {object} dto.AuthResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/v1/auth/login [post]
func (h *Handler) Login(c *gin.Context) {
	var req dto.CredentialsRequest
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

	h.setRefreshCookie(c, tokens.RefreshToken, time.Until(tokens.RefreshTokenExpiresAt))
	c.JSON(http.StatusOK, toAuthResponse(user, tokens))
}

// Refresh godoc
// @Summary Refresh tokens
// @Description Rotate the refresh token cookie and issue a new access token.
// @Tags auth
// @Produce json
// @Success 200 {object} dto.AuthResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/v1/auth/refresh [post]
func (h *Handler) Refresh(c *gin.Context) {
	refreshToken, err := c.Cookie(refreshCookieName)
	if err != nil || strings.TrimSpace(refreshToken) == "" {
		h.clearRefreshCookie(c)
		respondError(c, http.StatusUnauthorized, "missing_refresh_token", "refresh token cookie is required")
		return
	}

	user, tokens, err := h.service.Refresh(c.Request.Context(), refreshToken)
	if err != nil {
		h.clearRefreshCookie(c)
		status := http.StatusUnauthorized
		code := "invalid_refresh_token"
		if !errors.Is(err, ErrInvalidToken) {
			status = http.StatusInternalServerError
			code = "refresh_failed"
		}
		respondError(c, status, code, err.Error())
		return
	}

	h.setRefreshCookie(c, tokens.RefreshToken, time.Until(tokens.RefreshTokenExpiresAt))
	c.JSON(http.StatusOK, toAuthResponse(user, tokens))
}

// Me godoc
// @Summary Current user
// @Description Return the authenticated user from the access token.
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.AuthenticatedUser
// @Failure 401 {object} dto.ErrorResponse
// @Router /api/v1/me [get]
func (h *Handler) Me(c *gin.Context) {
	userID, _ := c.Get(userIDContextKey)
	email, _ := c.Get(userEmailContextKey)

	c.JSON(http.StatusOK, dto.AuthenticatedUser{
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
	refreshCookieName   = "refresh_token"
	refreshCookiePath   = "/api/v1/auth/refresh"
	userIDContextKey    = "auth.user_id"
	userEmailContextKey = "auth.user_email"
)

func (h *Handler) setRefreshCookie(c *gin.Context, token string, ttl time.Duration) {
	maxAge := int(ttl.Seconds())
	if maxAge < 0 {
		maxAge = 0
	}

	c.SetSameSite(h.refreshCookieSameSite())
	c.SetCookie(refreshCookieName, token, maxAge, refreshCookiePath, "", h.secure, true)
}

func (h *Handler) clearRefreshCookie(c *gin.Context) {
	c.SetSameSite(h.refreshCookieSameSite())
	c.SetCookie(refreshCookieName, "", -1, refreshCookiePath, "", h.secure, true)
}

func (h *Handler) refreshCookieSameSite() http.SameSite {
	if h.secure {
		return http.SameSiteNoneMode
	}

	return http.SameSiteLaxMode
}

func respondError(c *gin.Context, status int, code, message string) {
	c.JSON(status, dto.ErrorResponse{
		Error: dto.ErrorPayload{
			Code:    code,
			Message: message,
		},
	})
}

func toAuthResponse(user AuthenticatedUser, tokens TokenPair) dto.AuthResponse {
	return dto.AuthResponse{
		User: dto.AuthenticatedUser{
			ID:    user.ID,
			Email: user.Email,
		},
		Tokens: dto.TokenPair{
			AccessToken:          tokens.AccessToken,
			AccessTokenExpiresAt: tokens.AccessTokenExpiresAt.UTC().Format(http.TimeFormat),
		},
	}
}
