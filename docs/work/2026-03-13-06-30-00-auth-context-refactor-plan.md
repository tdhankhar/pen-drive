# Auth Context Refactor Plan

**Date:** 2026-03-13
**Status:** Done
**Priority:** MEDIUM

## Problem Statement

The auth context follows a flat interface pattern instead of the recommended `state/actions/meta` structure. Additionally, there's a dual access pattern for session data:

- React components → `useAuth().state.session`
- Non-React code (XHR) → `readSession()` (direct localStorage read)

This creates inconsistency and potential for stale data.

## Current State Analysis

### What's Already Good

| Aspect | Status |
|--------|--------|
| `AuthProvider` exists | YES - wraps app, provides context |
| `useAuth()` hook | YES - components access via hook, not props |
| `DashboardPage` uses `useAuth()` | YES - no prop drilling for session |

### What's Missing / Could Improve

| Issue | Location | Problem |
|-------|----------|---------|
| Context interface not structured | `auth-context.tsx` | Flat object `{ session, login, logout, signup, isLoading }` - doesn't follow `state/actions/meta` pattern |
| `readSession()` called directly | `upload-panel.tsx:authHeaders()` | Bypasses context, directly reads localStorage |
| Custom XHR bypasses interceptor | `upload-panel.tsx:xhrJsonRequest()` | Uses raw XMLHttpRequest for progress, so can't use `apiClient` interceptor |

## Implementation Plan

### Step 1: Restructure Context Interface

Refactor `AuthContextValue` to follow `state/actions/meta` pattern:

```tsx
// Current (flat)
type AuthContextValue = {
  isLoading: boolean;
  session: SessionState | null;
  login: (credentials: Credentials) => Promise<void>;
  signup: (credentials: Credentials) => Promise<void>;
  logout: () => void;
};

// After (state/actions/meta pattern)
interface AuthState {
  session: SessionState | null;
  isLoading: boolean;
}

interface AuthActions {
  login: (credentials: Credentials) => Promise<void>;
  signup: (credentials: Credentials) => Promise<void>;
  logout: () => void;
  getAccessToken: () => string | null;  // New: for non-React contexts
}

interface AuthMeta {
  // Future: refs, feature flags, etc.
}

interface AuthContextValue {
  state: AuthState;
  actions: AuthActions;
  meta: AuthMeta;
}
```

### Step 2: Add Cached Session Accessor

Add a module-level cache that `AuthProvider` keeps in sync:

```tsx
// session.ts
let cachedSession: SessionState | null = null;

export function getCachedSession(): SessionState | null {
  if (!cachedSession) {
    cachedSession = readSession();
  }
  return cachedSession;
}

export function setCachedSession(session: SessionState | null) {
  cachedSession = session;
}
```

### Step 3: Update AuthProvider

Update `AuthProvider` to:
1. Use the new interface structure
2. Call `setCachedSession()` whenever session changes

```tsx
// auth-context.tsx
export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<SessionState | null>(() => {
    const initial = readSession();
    setCachedSession(initial);  // Initialize cache
    return initial;
  });

  // ... existing query for session restore ...

  const value: AuthContextValue = {
    state: { session, isLoading },
    actions: {
      login: async (credentials) => {
        const nextSession = await loginRequest(credentials);
        setSession(nextSession);
        setCachedSession(nextSession);  // Keep cache in sync
      },
      signup: async (credentials) => {
        const nextSession = await signupRequest(credentials);
        setSession(nextSession);
        setCachedSession(nextSession);  // Keep cache in sync
      },
      logout: () => {
        clearSession();
        setSession(null);
        setCachedSession(null);  // Keep cache in sync
      },
      getAccessToken: () => session?.accessToken ?? null,
    },
    meta: {},
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
```

### Step 4: Update Consumers

Update components that access `auth.session`:

```tsx
// Before
const session = auth.session;

// After
const { state: { session } } = useAuth();
```

Update `authHeaders()` in upload-panel:

```tsx
// Before
function authHeaders(): { Authorization: string } {
  const session = readSession();  // Direct localStorage read
  ...
}

// After
import { getCachedSession } from "../lib/session";

function authHeaders(): { Authorization: string } {
  const session = getCachedSession();  // Uses cached value
  ...
}
```

## Files to Modify

1. `frontend/src/lib/session.ts` - Add `getCachedSession()` and `setCachedSession()`
2. `frontend/src/lib/auth-context.tsx` - Restructure interface, sync cache
3. `frontend/src/lib/use-auth.ts` - Update hook if needed (likely minimal change)
4. `frontend/src/components/upload-panel.tsx` - Use `getCachedSession()` in `authHeaders()`
5. `frontend/src/pages/dashboard-page.tsx` - Update to use `state.session` pattern
6. `frontend/src/app-router.tsx` - Update if accessing `auth.session` directly

## Benefits

| Improvement | Why |
|-------------|-----|
| `state/actions/meta` structure | Clear separation, follows skill pattern, easier to extend |
| `getAccessToken()` on context | Components have a clean API, not reaching into `session.accessToken` |
| `getCachedSession()` | Non-React code (XHR callbacks) can access session without localStorage reads |
| Single source of truth | AuthProvider updates cache, everything else reads from cache |

## Testing Checklist

- [ ] Login flow works
- [ ] Logout clears session
- [ ] Session restore on page reload
- [ ] Upload panel still gets auth headers
- [ ] Build passes
- [ ] No TypeScript errors

## Notes

- This is a **non-breaking refactor** - the data is the same, just structured differently
- The `getCachedSession()` pattern is safe because it's only written by React state updates (single source of truth)
- Future consideration: move refresh token to HTTP-only cookies (noted in `session.ts` TODO)
