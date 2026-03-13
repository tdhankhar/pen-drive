# Updated Auth Plan

**Date:** 2026-03-13
**Status:** Done
**Priority:** MEDIUM

## Goal

Unify auth state access across React and non-React code, while aligning the frontend refactor with the planned move of refresh tokens to HttpOnly cookies.

## Key Inputs Considered

- Current frontend auth implementation in `frontend/src/lib/auth-context.tsx`, `session.ts`, `api/http.ts`, `upload-panel.tsx`, `app-router.tsx`, `login-page.tsx`, `signup-page.tsx`, and `dashboard-page.tsx`
- Existing auth context draft: `docs/work/2026-03-13-06-30-00-auth-context-refactor-plan.md`
- Cookie migration note: `docs/work/2026-03-13-deferred-secure-refresh-token-cookies.md`
- Full cookie plan: `docs/work/2026-03-13-01-00-27-secure-refresh-token-cookies.md`

## Current Problems

| Issue | Current Location | Why it matters |
|---|---|---|
| Flat auth context API | `frontend/src/lib/auth-context.tsx` | Harder to extend cleanly; does not separate state from actions |
| Session read from multiple places | `auth-context.tsx`, `api/http.ts`, `upload-panel.tsx` | Creates inconsistent access paths and repeated storage reads |
| Raw XHR upload path bypasses client interceptor | `frontend/src/components/upload-panel.tsx` | Upload requests must build auth headers manually |
| Refresh token still lives in `SessionState` | `frontend/src/lib/session.ts` | Conflicts with the cookie plan; refresh token should become inaccessible to JS |
| Existing consumer API would break | `app-router.tsx`, `login-page.tsx`, `signup-page.tsx`, `dashboard-page.tsx` | Refactor must explicitly migrate all consumers |

## Recommendation

Do the auth work in two coordinated phases:

1. **First align the session model with cookie-based refresh transport**
2. **Then restructure the React auth context around that new session model**

This avoids doing one refactor against a session shape that is about to change again.

## Target Architecture

### 1. Session module becomes the single non-React auth source

`frontend/src/lib/session.ts` should own:

- persistent access-token storage
- in-memory cached session access
- login/signup/restore/clear helpers
- the cookie-aware refresh flow

Recommended shape after cookie migration:

```ts
type SessionState = {
  accessToken: string;
  user: AuthenticatedUser;
};

let cachedSession: SessionState | null = null;

export function getSessionSnapshot(): SessionState | null;
export function readSession(): SessionState | null;
export function writeSession(session: SessionState): void;
export function clearSession(): void;
export async function login(...): Promise<SessionState>;
export async function signup(...): Promise<SessionState>;
export async function restoreSession(): Promise<SessionState | null>;
```

Notes:

- `readSession()` and `writeSession()` should keep the in-memory cache synchronized.
- Non-React code should read through the session module, not through React context.
- Do not make `AuthProvider` responsible for cache ownership.

### 2. Auth context uses `state/actions/meta`

Recommended context contract:

```tsx
interface AuthState {
  session: SessionState | null;
  isLoading: boolean;
}

interface AuthActions {
  login: (credentials: Credentials) => Promise<void>;
  signup: (credentials: Credentials) => Promise<void>;
  logout: () => void;
}

interface AuthMeta {
  isAuthenticated: boolean;
}

interface AuthContextValue {
  state: AuthState;
  actions: AuthActions;
  meta: AuthMeta;
}
```

Notes:

- This is a **breaking API change** for `useAuth()` consumers and should be treated as such.
- `getAccessToken()` is not necessary on the React context if non-React code reads from `session.ts`.

### 3. Transport model after cookie migration

- Access token remains available to frontend JavaScript for `Authorization` headers.
- Refresh token is removed from `SessionState` and from localStorage.
- Browser sends the refresh cookie automatically on `/api/v1/auth/refresh`.
- Frontend fetch client must use `credentials: "include"`.

## Implementation Plan

### Phase 1: Cookie-aware session model

#### Backend prerequisites

Use the existing cookie plan as the source of truth for backend work:

- remove refresh token fields from auth DTO responses
- set HttpOnly refresh cookie on login/signup/refresh
- read refresh cookie in refresh handler
- enable credentialed CORS

Reference: `docs/work/2026-03-13-01-00-27-secure-refresh-token-cookies.md`

#### Frontend session changes

Update:

1. `frontend/src/lib/api/http.ts`
   - configure requests to send cookies with `credentials: "include"`
   - keep bearer token injection sourced from session module

2. `frontend/src/lib/session.ts`
   - remove `refreshToken` from `SessionState`
   - update `refreshSession()` to call refresh without a body token
   - keep module-level cache synchronized in `readSession`, `writeSession`, and `clearSession`
   - ensure failure paths clear both storage and cache

3. `frontend/src/components/upload-panel.tsx`
   - replace direct `readSession()` auth header usage with the shared session snapshot helper

4. `frontend/src/lib/api/http.ts`
   - replace direct `readSession()` access with the same shared session snapshot helper

### Phase 2: Context restructure

Update:

1. `frontend/src/lib/auth-context.tsx`
   - expose `state/actions/meta`
   - initialize session from session module
   - keep React state in sync with successful session helper calls
   - keep restore flow behavior unchanged

2. `frontend/src/lib/use-auth.ts`
   - keep the hook shape simple; return the typed context value

3. Consumers
   - `frontend/src/app-router.tsx`
   - `frontend/src/pages/login-page.tsx`
   - `frontend/src/pages/signup-page.tsx`
   - `frontend/src/pages/dashboard-page.tsx`

Examples:

```tsx
const {
  state: { session, isLoading },
  actions: { login, signup, logout },
} = useAuth();
```

### Phase 3: Cleanup and consistency

- remove any remaining direct session reads outside `session.ts`
- ensure all auth header creation goes through one accessor path
- confirm no frontend code references `refreshToken`

## File Impact Summary

### Frontend

- `frontend/src/lib/session.ts`
- `frontend/src/lib/auth-context.tsx`
- `frontend/src/lib/use-auth.ts`
- `frontend/src/lib/api/http.ts`
- `frontend/src/components/upload-panel.tsx`
- `frontend/src/app-router.tsx`
- `frontend/src/pages/login-page.tsx`
- `frontend/src/pages/signup-page.tsx`
- `frontend/src/pages/dashboard-page.tsx`

### Backend

- `backend/internal/api/dto/auth.go`
- `backend/internal/auth/http.go`
- `backend/internal/http/router.go`
- `backend/cmd/api/main.go`

## Acceptance Criteria

- `SessionState` no longer contains `refreshToken`
- refresh token is not readable from frontend JavaScript
- auth restore works through cookie-based refresh
- API client and upload XHR read auth state through the same session accessor path
- `useAuth()` exposes `state/actions/meta`
- `app-router`, login, signup, and dashboard all compile against the new context shape
- login, logout, signup, reload-based restore, and uploads continue to work

## Risks and Guardrails

- **Breaking consumer API:** all `useAuth()` consumers must be migrated in the same change set.
- **Split ownership risk:** avoid putting cache sync logic partly in context and partly in session helpers.
- **Cross-origin cookie risk:** cookie transport depends on both frontend `credentials: "include"` and backend credentialed CORS.
- **Upload path risk:** `XMLHttpRequest` upload code will not benefit from fetch interceptors, so it must use the shared session accessor explicitly.

## Suggested Execution Order

1. Implement cookie transport backend/frontend prerequisites
2. Simplify `SessionState` and centralize session cache ownership in `session.ts`
3. Update `api/http.ts` and `upload-panel.tsx` to consume the shared session snapshot helper
4. Refactor `AuthContext` to `state/actions/meta`
5. Migrate all `useAuth()` consumers in one pass
6. Run lint, typecheck, and build; then verify login/logout/reload/upload flows manually
