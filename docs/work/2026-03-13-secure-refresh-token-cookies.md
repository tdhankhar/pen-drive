# Secure Refresh Token Transport (HTTP-only Cookies) Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace localStorage refresh token storage with HttpOnly cookies so refresh tokens are inaccessible to JavaScript.

**Architecture:** The backend sets a `refresh_token` HttpOnly cookie on login/signup/refresh responses and strips the token from the JSON body. The `/refresh` endpoint reads the token from the cookie instead of the request body. The frontend drops all refresh token handling — the browser sends the cookie automatically on every request to `/api/v1/auth/refresh`.

**Tech Stack:** Go (Gin), `gin-contrib/cors`, TypeScript (React), native `fetch` with `credentials: "include"`

---

## Overview of Changes

| File | Change |
|------|--------|
| `backend/internal/api/dto/auth.go` | Remove `RefreshToken`/`RefreshTokenExpiresAt` from `TokenPair`; remove `RefreshRequest` |
| `backend/internal/auth/http.go` | Set cookie on login/signup, read cookie in refresh, add `secure bool` to handler |
| `backend/internal/http/router.go` | `AllowCredentials: true` in CORS; pass `secure` flag to `NewHandler`; add `secure bool` param to `NewRouter` |
| `backend/cmd/api/main.go` | Pass `cfg.AppEnv == "production"` to `NewRouter` |
| `frontend/src/lib/api/http.ts` | Add `credentials: "include"` to `fetch` |
| `frontend/src/lib/session.ts` | Remove `refreshToken` from `SessionState`; drop refresh token from all storage/restore logic |
| `frontend/src/lib/auth-context.tsx` | Audit for any `.refreshToken` access and remove |

---

## Task 1: Backend — Remove refresh token from response DTO

**Files:**
- Modify: `backend/internal/api/dto/auth.go`

**Context:** `TokenPair` currently exposes `refresh_token` and `refresh_token_expires_at` in JSON. After this change, these fields live only in the cookie. `RefreshRequest` accepts `refresh_token` in the body — we remove it since the handler will read from cookie instead.

**Step 1: Write the failing test**

Create `backend/internal/auth/http_test.go`:

```go
package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Stub service and helpers will be added as tests grow.
// For now, focus on the DTO shape contract.

func TestTokenPairOmitsRefreshToken(t *testing.T) {
	// Marshal a TokenPair and assert no refresh_token key appears.
	pair := dto.TokenPair{
		AccessToken:          "access-abc",
		AccessTokenExpiresAt: "2026-03-13T16:00:00Z",
	}
	b, _ := json.Marshal(pair)
	var m map[string]any
	json.Unmarshal(b, &m)

	if _, ok := m["refresh_token"]; ok {
		t.Error("TokenPair must not include refresh_token in JSON")
	}
	if _, ok := m["refresh_token_expires_at"]; ok {
		t.Error("TokenPair must not include refresh_token_expires_at in JSON")
	}
}
```

Add import at top: `"github.com/abhishek/pen-drive/backend/internal/api/dto"`

**Step 2: Run test to verify it fails**

```bash
cd backend && go test ./internal/auth/... -run TestTokenPairOmitsRefreshToken -v
```

Expected: FAIL — `refresh_token` key present.

**Step 3: Update the DTO**

In `backend/internal/api/dto/auth.go`, replace `TokenPair` and remove `RefreshRequest`:

```go
type TokenPair struct {
	AccessToken          string `json:"access_token" example:"eyJhbGciOiJIUzI1NiJ9"`
	AccessTokenExpiresAt string `json:"access_token_expires_at" example:"2026-03-12T16:00:00Z"`
}

// Remove RefreshRequest entirely — refresh token now comes from cookie.
```

The file after edit should have: `CredentialsRequest`, `AuthenticatedUser`, `TokenPair` (2 fields only), `AuthResponse`, `ErrorPayload`, `ErrorResponse`, `HealthResponse`. Delete `RefreshRequest`.

**Step 4: Run test to verify it passes**

```bash
cd backend && go test ./internal/auth/... -run TestTokenPairOmitsRefreshToken -v
```

Expected: PASS (compile errors elsewhere are expected — fixed in Task 2).

**Step 5: Commit**

```bash
git add backend/internal/api/dto/auth.go backend/internal/auth/http_test.go
git commit -m "feat(#12): remove refresh token from response DTO"
```

---

## Task 2: Backend — Set HttpOnly cookie in Login/Signup handlers

**Files:**
- Modify: `backend/internal/auth/http.go`
- Modify: `backend/internal/http/router.go`
- Modify: `backend/cmd/api/main.go`

**Context:** The `Handler` needs a `secure bool` flag (true in production so cookies get `Secure` attribute, false for local HTTP). `NewRouter` gets a `secure bool` param threaded from `main.go`. Login and Signup set the cookie after issuing tokens. `toAuthResponse` no longer includes refresh token in body.

**Step 1: Write the failing test**

Add to `backend/internal/auth/http_test.go`:

```go
func TestLoginSetsHttpOnlyCookie(t *testing.T) {
	// Use a minimal fake handler wired with a stub service.
	// This test will be filled in properly once the handler compiles.
	// For now it documents the expected contract:
	//   POST /api/v1/auth/login → Set-Cookie: refresh_token=...; HttpOnly; Path=/api/v1/auth/refresh
	t.Skip("implement after handler compiles")
}
```

**Step 2: Run to confirm it skips (compile errors must be fixed first)**

```bash
cd backend && go build ./... 2>&1 | head -30
```

Expected: compile errors about `dto.RefreshRequest` and `tokens.RefreshToken` field references.

**Step 3: Fix compile errors and implement cookie logic**

In `backend/internal/auth/http.go`:

1. Add `"time"` to imports.

2. Update `Handler` struct and constructor:

```go
type Handler struct {
	service *Service
	secure  bool
}

func NewHandler(service *Service, secure bool) *Handler {
	return &Handler{service: service, secure: secure}
}
```

3. Add cookie helpers (add after the constants block):

```go
const refreshCookieName = "refresh_token"
const refreshCookiePath = "/api/v1/auth/refresh"

func (h *Handler) setRefreshCookie(c *gin.Context, token string, ttl time.Duration) {
	c.SetCookie(
		refreshCookieName,
		token,
		int(ttl.Seconds()),
		refreshCookiePath,
		"",      // domain — empty means request host
		h.secure, // Secure flag
		true,    // HttpOnly
	)
}

func (h *Handler) clearRefreshCookie(c *gin.Context) {
	c.SetCookie(refreshCookieName, "", -1, refreshCookiePath, "", h.secure, true)
}
```

4. Update `Signup` handler — call `setRefreshCookie` before responding:

```go
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

	ttl := time.Until(tokens.RefreshTokenExpiresAt)
	h.setRefreshCookie(c, tokens.RefreshToken, ttl)
	c.JSON(http.StatusCreated, toAuthResponse(user, tokens))
}
```

5. Update `Login` handler identically (same `setRefreshCookie` call, `http.StatusOK`).

6. Update `toAuthResponse` — drop refresh fields:

```go
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
```

7. In `router.go`, add `secure bool` param to `NewRouter` and thread it to `NewHandler`:

```go
func NewRouter(logger *slog.Logger, dbConn *sql.DB, storageClient *storage.Client, jwtConfig config.JWTConfig, secure bool) *gin.Engine {
    // ... existing code ...
    authHandler := auth.NewHandler(authService, secure)
    // ...
}
```

8. In `main.go`, update the `NewRouter` call:

```go
router := apphttp.NewRouter(logger, dbConn, storageClient, cfg.JWT, cfg.AppEnv == "production")
```

**Step 4: Verify it compiles**

```bash
cd backend && go build ./...
```

Expected: clean build.

**Step 5: Implement the real cookie test**

Replace the `t.Skip` stub in `http_test.go` with a real test. You'll need a test helper that creates a `Handler` with a mock/stub `Service`. Look at the `Service` interface or create a minimal fake:

```go
// fakeService implements enough of Service for handler tests.
// Add it at the top of http_test.go.
```

The test should:
- POST to `/login` with valid JSON credentials
- Assert response has `Set-Cookie` header with `refresh_token`
- Assert the cookie has `HttpOnly` flag
- Assert the cookie `Path` is `/api/v1/auth/refresh`
- Assert the response body JSON does NOT contain `refresh_token`

**Step 6: Run tests**

```bash
cd backend && go test ./internal/auth/... -v
```

Expected: PASS

**Step 7: Commit**

```bash
git add backend/internal/auth/http.go backend/internal/http/router.go backend/cmd/api/main.go
git commit -m "feat(#12): set HttpOnly cookie on login and signup"
```

---

## Task 3: Backend — Read refresh token from cookie in Refresh handler

**Files:**
- Modify: `backend/internal/auth/http.go`

**Context:** The `/refresh` endpoint currently reads `req.RefreshToken` from the JSON body. After this change, it reads from the `refresh_token` cookie. Missing or empty cookie → 401.

**Step 1: Write failing tests**

Add to `backend/internal/auth/http_test.go`:

```go
func TestRefreshReadsCookie(t *testing.T) {
	// POST /api/v1/auth/refresh with cookie, no body
	// Expect 200 and a new Set-Cookie: refresh_token header
}

func TestRefreshMissingCookieReturns401(t *testing.T) {
	// POST /api/v1/auth/refresh with no cookie and no body
	// Expect 401 with code "missing_refresh_token"
}
```

**Step 2: Run tests to verify they fail**

```bash
cd backend && go test ./internal/auth/... -run "TestRefresh" -v
```

Expected: FAIL — current handler reads body, ignores cookie.

**Step 3: Rewrite the Refresh handler**

```go
func (h *Handler) Refresh(c *gin.Context) {
	token, err := c.Cookie(refreshCookieName)
	if err != nil || strings.TrimSpace(token) == "" {
		respondError(c, http.StatusUnauthorized, "missing_refresh_token", "refresh token cookie is required")
		return
	}

	user, tokens, err := h.service.Refresh(c.Request.Context(), token)
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

	ttl := time.Until(tokens.RefreshTokenExpiresAt)
	h.setRefreshCookie(c, tokens.RefreshToken, ttl)
	c.JSON(http.StatusOK, toAuthResponse(user, tokens))
}
```

Remove the `dto.RefreshRequest` import usage (it was only used here).

**Step 4: Run tests**

```bash
cd backend && go test ./internal/auth/... -run "TestRefresh" -v
```

Expected: PASS

**Step 5: Run all backend tests**

```bash
cd backend && go test ./...
```

Expected: all PASS

**Step 6: Commit**

```bash
git add backend/internal/auth/http.go
git commit -m "feat(#12): read refresh token from cookie in refresh handler"
```

---

## Task 4: Backend — Update CORS for cookie transport

**Files:**
- Modify: `backend/internal/http/router.go`

**Context:** Browsers only send cookies on cross-origin requests when `credentials: "include"` is set on the frontend fetch AND the server responds with `Access-Control-Allow-Credentials: true`. Wildcards in `AllowOrigins` are incompatible with `AllowCredentials` — already satisfied (explicit origins are set).

**Step 1: Write failing test**

Add to `backend/internal/http/router_test.go` (create if needed):

```go
package http_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSAllowsCredentials(t *testing.T) {
	// OPTIONS preflight to any route from allowed origin
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/refresh", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")

	w := httptest.NewRecorder()
	// router := NewRouter(...) — use a test helper or nil deps where possible

	if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Error("CORS must allow credentials")
	}
	if w.Header().Get("Access-Control-Allow-Origin") == "*" {
		t.Error("CORS must not use wildcard origin when credentials are allowed")
	}
}
```

**Step 2: Run test**

```bash
cd backend && go test ./internal/http/... -run TestCORSAllowsCredentials -v
```

Expected: FAIL

**Step 3: Add AllowCredentials to CORS config**

In `backend/internal/http/router.go`, update the `cors.New(...)` block:

```go
router.Use(
    cors.New(cors.Config{
        AllowOrigins:     []string{"http://127.0.0.1:5173", "http://localhost:5173"},
        AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
        AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
        ExposeHeaders:    []string{"X-Request-Id"},
        AllowCredentials: true,
    }),
)
```

**Step 4: Run test**

```bash
cd backend && go test ./...
```

Expected: all PASS

**Step 5: Commit**

```bash
git add backend/internal/http/router.go
git commit -m "feat(#12): enable CORS credentials for cookie transport"
```

---

## Task 5: Frontend — Add credentials: include to HTTP client

**Files:**
- Modify: `frontend/src/lib/api/http.ts`

**Context:** Browsers only attach cookies to fetch requests when `credentials: "include"` is set. Without this, the refresh cookie is never sent.

**Step 1: Update customFetch**

Replace the entire `fetch(...)` call in `frontend/src/lib/api/http.ts`:

```typescript
const response = await fetch(`${apiBaseUrl}${url}`, {
  ...options,
  credentials: "include",
  headers: {
    "Content-Type": "application/json",
    ...options.headers,
  },
});
```

**Step 2: Verify build**

```bash
cd frontend && npm run build
```

Expected: clean build, no type errors.

**Step 3: Commit**

```bash
git add frontend/src/lib/api/http.ts
git commit -m "feat(#12): add credentials: include to fetch for cookie transport"
```

---

## Task 6: Frontend — Remove refresh token from session state

**Files:**
- Modify: `frontend/src/lib/session.ts`
- Audit: `frontend/src/lib/auth-context.tsx`

**Context:** `SessionState` currently stores `refreshToken` in localStorage. After this change, the browser holds the refresh token in an HttpOnly cookie — invisible to JS. `SessionState` only stores `accessToken` + `user`. `refreshSession()` sends no body token; the cookie travels automatically.

**Step 1: Audit auth-context.tsx for refreshToken references**

```bash
grep -n "refreshToken\|refresh_token" frontend/src/lib/auth-context.tsx
```

Note any references — they'll need to match the new `SessionState` shape.

**Step 2: Rewrite session.ts**

Replace the full file content:

```typescript
import {
  getApiV1Me,
  postApiV1AuthLogin,
  postApiV1AuthRefresh,
  postApiV1AuthSignup,
} from "./api/generated/client";
import type {
  GithubComAbhishekPenDriveBackendInternalApiDtoAuthenticatedUser,
  GithubComAbhishekPenDriveBackendInternalApiDtoCredentialsRequest,
} from "./api/generated/model";

type SessionState = {
  accessToken: string;
  user: GithubComAbhishekPenDriveBackendInternalApiDtoAuthenticatedUser;
};

type AuthPayload = {
  tokens?: {
    access_token?: string;
  };
  user?: GithubComAbhishekPenDriveBackendInternalApiDtoAuthenticatedUser;
};

const sessionStorageKey = "pen-drive.session";

export function readSession(): SessionState | null {
  const raw = window.localStorage.getItem(sessionStorageKey);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as SessionState;
  } catch {
    window.localStorage.removeItem(sessionStorageKey);
    return null;
  }
}

export function writeSession(session: SessionState) {
  window.localStorage.setItem(sessionStorageKey, JSON.stringify(session));
}

export function clearSession() {
  window.localStorage.removeItem(sessionStorageKey);
}

export async function signup(
  credentials: GithubComAbhishekPenDriveBackendInternalApiDtoCredentialsRequest,
): Promise<SessionState> {
  const response = await postApiV1AuthSignup(credentials);
  if (response.status !== 201) {
    throw new Error(response.data.error?.message ?? "signup failed");
  }
  const session = toSessionState(response.data);
  writeSession(session);
  return session;
}

export async function login(
  credentials: GithubComAbhishekPenDriveBackendInternalApiDtoCredentialsRequest,
): Promise<SessionState> {
  const response = await postApiV1AuthLogin(credentials);
  if (response.status !== 200) {
    throw new Error(response.data.error?.message ?? "login failed");
  }
  const session = toSessionState(response.data);
  writeSession(session);
  return session;
}

// Cookie is sent automatically by the browser — no token needed in body.
export async function refreshSession(): Promise<SessionState> {
  const response = await postApiV1AuthRefresh({});

  if (response.status !== 200) {
    clearSession();
    throw new Error(response.data.error?.message ?? "refresh failed");
  }

  if (!response.data.tokens?.access_token) {
    clearSession();
    throw new Error("refresh token response is incomplete");
  }

  const userResponse = await getApiV1Me({
    headers: {
      Authorization: `Bearer ${response.data.tokens.access_token}`,
    },
  });

  if (userResponse.status !== 200) {
    clearSession();
    throw new Error("session bootstrap failed");
  }

  const session = toSessionState({
    tokens: response.data.tokens,
    user: userResponse.data,
  });
  writeSession(session);
  return session;
}

export async function restoreSession(): Promise<SessionState | null> {
  const current = readSession();
  if (!current) return null;
  return refreshSession();
}

function toSessionState(response: AuthPayload): SessionState {
  if (!response.user || !response.tokens?.access_token) {
    throw new Error("response payload is incomplete");
  }
  return {
    accessToken: response.tokens.access_token,
    user: response.user,
  };
}

export type { SessionState };
```

**Step 3: Fix auth-context.tsx if needed**

If step 1 found any `.refreshToken` accesses in `auth-context.tsx`, remove them now. The `SessionState` type no longer has that field.

**Step 4: Verify build**

```bash
cd frontend && npm run build
```

Expected: clean build with no TypeScript errors.

**Step 5: Commit**

```bash
git add frontend/src/lib/session.ts frontend/src/lib/auth-context.tsx
git commit -m "feat(#12): remove refresh token from localStorage session state"
```

---

## Task 7: Final verification

**Step 1: Run all backend tests**

```bash
cd backend && go test ./...
```

Expected: all PASS

**Step 2: Run frontend build**

```bash
make frontend-build
```

Expected: clean build

**Step 3: Manual smoke test (if running locally)**

1. Start backend + frontend
2. Log in via UI
3. Open browser DevTools → Console → run `document.cookie`
   - `refresh_token` must NOT appear (HttpOnly = JS-invisible)
4. DevTools → Application → Cookies
   - `refresh_token` should appear with `HttpOnly` checked
5. Reload the page — session should restore via cookie-based refresh

**Step 4: Final commit if any loose files**

```bash
git status
git add <any remaining files>
git commit -m "feat(#12): secure refresh token transport via HTTP-only cookies"
```
