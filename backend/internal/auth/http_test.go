package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestSetRefreshCookieUsesHttpOnlyCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	handler := NewHandler(nil, true)

	handler.setRefreshCookie(context, "refresh-token", 2*time.Hour)

	response := recorder.Result()
	defer response.Body.Close()

	cookies := response.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}

	cookie := cookies[0]
	if cookie.Name != refreshCookieName {
		t.Fatalf("expected cookie %q, got %q", refreshCookieName, cookie.Name)
	}
	if cookie.Path != refreshCookiePath {
		t.Fatalf("expected path %q, got %q", refreshCookiePath, cookie.Path)
	}
	if !cookie.HttpOnly {
		t.Fatal("expected HttpOnly cookie")
	}
	if !cookie.Secure {
		t.Fatal("expected Secure cookie")
	}
}

func TestRefreshHandlerRequiresCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, false)
	router := gin.New()
	router.POST("/refresh", handler.Refresh)

	request := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), `"code":"missing_refresh_token"`) {
		t.Fatalf("expected missing_refresh_token error, got %s", response.Body.String())
	}
}

func TestToAuthResponseOmitsRefreshFields(t *testing.T) {
	response := toAuthResponse(
		AuthenticatedUser{ID: "user-123", Email: "user@example.com"},
		TokenPair{
			AccessToken:           "access-token",
			AccessTokenExpiresAt:  time.Unix(0, 0).UTC(),
			RefreshToken:          "refresh-token",
			RefreshTokenExpiresAt: time.Unix(3600, 0).UTC(),
		},
	)

	payload, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal auth response: %v", err)
	}

	body := string(payload)
	if strings.Contains(body, "refresh_token") {
		t.Fatalf("expected refresh_token to be omitted, got %s", body)
	}
	if strings.Contains(body, "refresh_token_expires_at") {
		t.Fatalf("expected refresh_token_expires_at to be omitted, got %s", body)
	}
}
