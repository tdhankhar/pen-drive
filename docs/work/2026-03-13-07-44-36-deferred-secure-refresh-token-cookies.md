# Deferred: Secure Refresh Token Cookies

**Status:** Done — implemented.

**Goal:** Replace localStorage refresh token with HttpOnly cookie so refresh tokens are inaccessible to JavaScript.

**Backend changes:**
- `backend/internal/api/dto/auth.go`: remove `RefreshToken`/`RefreshTokenExpiresAt` from `TokenPair`; remove `RefreshRequest`
- `backend/internal/auth/http.go`: set HttpOnly cookie on login/signup; read cookie in `/refresh` handler; add `secure bool` flag
- `backend/internal/http/router.go`: `AllowCredentials: true` in CORS; pass `secure` flag
- `backend/cmd/api/main.go`: pass `cfg.AppEnv == "production"` as secure flag

**Frontend changes:**
- `frontend/src/lib/api/http.ts`: add `credentials: "include"` to fetch config
- `frontend/src/lib/session.ts`: remove `refreshToken` from `SessionState`, drop all localStorage refresh token logic
- `frontend/src/lib/auth-context.tsx`: remove any `.refreshToken` access

**Full plan:** `docs/work/2026-03-13-01-00-27-secure-refresh-token-cookies.md`
