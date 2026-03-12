# Frontend Migration Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Migrate the frontend from orval + custom CSS to hey-api/openapi-ts + Tailwind + shadcn/ui + TanStack Query, while keeping Uppy for uploads.

**Architecture:** Replace orval with `@hey-api/openapi-ts` for cleaner generated types, layer Tailwind v4 + shadcn/ui over the existing Vite+React 19 setup, and swap all manual `useState/useEffect` data-fetching patterns for TanStack Query. The Uppy instances stay; their UI shell gets re-skinned with shadcn components.

**Tech Stack:** Vite 8, React 19, TypeScript 5.9, Tailwind CSS v4 (`@tailwindcss/vite`), shadcn/ui, TanStack Query v5, `@hey-api/openapi-ts`, `@hey-api/client-fetch`, `@uppy/core`, `@uppy/react`

---

## Context: Current state

| Concern | Current | Target |
|---|---|---|
| API codegen | orval + `orval.config.ts` | `@hey-api/openapi-ts` + `openapi-ts.config.ts` |
| Styling | Hand-rolled CSS in `index.css` / `App.css` | Tailwind v4 + shadcn/ui |
| Data fetching | `useState` + `useEffect` in pages | TanStack Query `useQuery` / `useMutation` |
| Upload UI | Raw Uppy `Dropzone`/`FilesList` in custom CSS shells | Uppy inside shadcn `Card` + `Dialog` |

Generated types today have long names like `GithubComAbhishekPenDriveBackendInternalApiDtoDuplicateConflictPolicy`. hey-api generates short, schema-derived names.

---

## Phase 1 — Replace orval with hey-api/openapi-ts

### Task 1: Install hey-api packages, remove orval

**Files:**
- Modify: `frontend/package.json`
- Delete: `frontend/orval.config.ts`
- Delete: `frontend/scripts/fix-folder-upload-client.mjs`
- Create: `frontend/openapi-ts.config.ts`

**Step 1: Remove orval, install hey-api**

```bash
cd frontend
npm uninstall orval
npm install --save-dev @hey-api/openapi-ts
npm install @hey-api/client-fetch
```

**Step 2: Create `frontend/openapi-ts.config.ts`**

```ts
import { defineConfig } from '@hey-api/openapi-ts';

export default defineConfig({
  input: '../backend/docs/openapi/swagger.json',
  output: {
    path: 'src/lib/api/generated',
    format: 'prettier',
  },
  plugins: [
    {
      name: '@hey-api/client-fetch',
      reuseRequestOptions: true,
    },
    '@hey-api/schemas',
    '@hey-api/sdk',
    {
      name: '@hey-api/typescript',
      enums: 'javascript',
    },
  ],
});
```

**Step 3: Update `package.json` scripts**

Replace the `api:generate` script:
```json
"api:generate": "openapi-ts"
```

**Step 4: Replace `frontend/src/lib/api/http.ts` with hey-api client init**

```ts
import { createClient } from '@hey-api/client-fetch';

export const apiClient = createClient({
  baseUrl: import.meta.env.VITE_API_BASE_URL ?? 'http://127.0.0.1:8080',
});
```

**Step 5: Regenerate the client**

```bash
npm run api:generate
```

Expected: `src/lib/api/generated/` is regenerated with clean type names (e.g. `DuplicateConflictPolicy`, `FileSystemEntry`, `FileListResponse`).

**Step 6: Verify it builds**

```bash
npm run build
```

Expected: TypeScript compiler output, no errors. If type errors appear, fix import paths (generated barrel exports now live in `src/lib/api/generated/index.ts`).

**Step 7: Commit**

```bash
git add frontend/openapi-ts.config.ts frontend/src/lib/api/http.ts frontend/package.json frontend/package-lock.json
git rm frontend/orval.config.ts frontend/scripts/fix-folder-upload-client.mjs
git add frontend/src/lib/api/generated/
git commit -m "feat: replace orval with hey-api/openapi-ts for cleaner generated types"
```

---

### Task 2: Adapt upload-panel.tsx to hey-api generated types

**Files:**
- Modify: `frontend/src/components/upload-panel.tsx`

hey-api SDK functions now take `{ client, body, query, ... }` option objects and return `{ data, error, response }` instead of `{ data, status }`.

**Step 1: Update all API call sites in upload-panel.tsx**

Old pattern:
```ts
const response = await postApiV1FilesUpload(body, { headers: { Authorization: `Bearer ${token}` } });
if (response.status !== 201) throw new Error(...);
```

New pattern (hey-api):
```ts
const { data, error, response } = await postApiV1FilesUpload({
  client: apiClient,
  body,
  headers: { Authorization: `Bearer ${token}` },
});
if (error) throw new Error(getErrorMessage(error, response.status));
```

Update every call: `postApiV1FilesUpload`, `postApiV1FilesUploadMultipartInitiate`, `postApiV1FilesUploadMultipartPart`, `postApiV1FilesUploadMultipartComplete`, `postApiV1FilesUploadMultipartAbort`, `postApiV1FilesDuplicatesPreview`.

Also update the `authorizedRequest` helper — it can become an inline `headers` object or a reusable small helper:

```ts
function authHeaders(token: string) {
  return { Authorization: `Bearer ${token}` };
}
```

Update the long type imports to use the cleaner hey-api generated names (check the generated `index.ts` for exact export names).

**Step 2: Verify build**

```bash
cd frontend && npm run build
```

**Step 3: Commit**

```bash
git add frontend/src/components/upload-panel.tsx
git commit -m "feat: adapt upload-panel to hey-api generated client"
```

---

### Task 3: Adapt dashboard-page.tsx to hey-api types

**Files:**
- Modify: `frontend/src/pages/dashboard-page.tsx`

**Step 1: Update the `getApiV1Files` call to hey-api shape**

Old:
```ts
const response = await getApiV1Files(activePath ? { path: activePath } : undefined, {
  headers: { Authorization: `Bearer ${accessToken}` },
});
if (response.status !== 200) { setError(...); }
setListing(response.data);
```

New:
```ts
const { data, error } = await getApiV1Files({
  client: apiClient,
  query: activePath ? { path: activePath } : undefined,
  headers: { Authorization: `Bearer ${accessToken}` },
});
if (error) { setError(...); return; }
setListing(data);
```

**Step 2: Update imported type names to match hey-api short names**

**Step 3: Verify build and dev server loads**

```bash
npm run build && npm run dev
```

**Step 4: Commit**

```bash
git add frontend/src/pages/dashboard-page.tsx
git commit -m "feat: adapt dashboard-page to hey-api generated client"
```

---

## Phase 2 — Tailwind CSS v4 + shadcn/ui

### Task 4: Install Tailwind v4

**Files:**
- Modify: `frontend/vite.config.ts`
- Modify: `frontend/src/index.css`

**Step 1: Install**

```bash
cd frontend
npm install tailwindcss @tailwindcss/vite
```

**Step 2: Add Tailwind Vite plugin to `vite.config.ts`**

```ts
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
})
```

**Step 3: Replace `src/index.css` top with Tailwind import**

Add at the very top of `src/index.css` (keep existing CSS variables below for now — they will be migrated in Task 6):

```css
@import "tailwindcss";
```

**Step 4: Verify Tailwind works**

```bash
npm run dev
```

Add a test class to any element (e.g. `className="text-red-500"` on the `<h1>` in `dashboard-page.tsx`), confirm it renders red, then remove it.

**Step 5: Commit**

```bash
git add frontend/vite.config.ts frontend/src/index.css frontend/package.json frontend/package-lock.json
git commit -m "feat: add tailwind css v4 via @tailwindcss/vite"
```

---

### Task 5: Initialize shadcn/ui

**Files:**
- Create: `frontend/components.json` (shadcn config, auto-generated)
- Modify: `frontend/src/index.css` (shadcn injects CSS variables)
- Create: `frontend/src/lib/utils.ts` (shadcn creates this)

**Step 1: Run shadcn init**

```bash
cd frontend
npx shadcn@latest init
```

When prompted:
- Style: **Default**
- Base color: **Slate** (or your preference)
- CSS variables: **Yes**

This creates `components.json`, updates `src/index.css` with shadcn design tokens, and creates `src/lib/utils.ts` with the `cn()` helper.

**Step 2: Install the components we'll need**

```bash
npx shadcn@latest add button badge card dialog separator breadcrumb
```

Components land in `src/components/ui/`.

**Step 3: Verify build**

```bash
npm run build
```

**Step 4: Commit**

```bash
git add frontend/components.json frontend/src/components/ui/ frontend/src/lib/utils.ts frontend/src/index.css frontend/package.json frontend/package-lock.json
git commit -m "feat: initialize shadcn/ui with button, badge, card, dialog, separator, breadcrumb"
```

---

### Task 6: Migrate auth pages to shadcn

**Files:**
- Modify: `frontend/src/components/auth-form.tsx`
- Modify: `frontend/src/pages/login-page.tsx`
- Modify: `frontend/src/pages/signup-page.tsx`

**Step 1: Replace custom button in auth-form.tsx with shadcn Button**

```tsx
import { Button } from './ui/button';
// replace <button className="primary-button" ...> with:
<Button type="submit" disabled={isSubmitting}>
  {submitLabel}
</Button>
```

**Step 2: Replace layout classes with Tailwind utilities**

Example for `login-page.tsx`:
```tsx
// replace className="auth-layout" with:
<section className="min-h-screen flex items-center justify-center gap-12 p-8">
```

Use Tailwind utilities for spacing, typography, flex layout. Remove corresponding CSS rules from `index.css` / `App.css` as you go (keep the file clean — don't leave dead rules).

**Step 3: Verify dev server renders login/signup correctly**

```bash
npm run dev
# navigate to /login and /signup
```

**Step 4: Commit**

```bash
git add frontend/src/components/auth-form.tsx frontend/src/pages/login-page.tsx frontend/src/pages/signup-page.tsx frontend/src/index.css frontend/src/App.css
git commit -m "feat: migrate auth pages to shadcn button + tailwind layout"
```

---

### Task 7: Migrate dashboard-page.tsx to shadcn + Tailwind

**Files:**
- Modify: `frontend/src/pages/dashboard-page.tsx`

**Step 1: Replace layout/panel classes with Tailwind + shadcn Card**

```tsx
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card';
import { Button } from '../components/ui/button';
import { Badge } from '../components/ui/badge';
import {
  Breadcrumb, BreadcrumbItem, BreadcrumbList, BreadcrumbSeparator,
} from '../components/ui/breadcrumb';

// dashboard-shell → <main className="min-h-screen p-6 space-y-6">
// dashboard-header → <header className="flex items-center justify-between">
// panel → <Card>
// eyebrow → <p className="text-sm text-muted-foreground">
// secondary-button → <Button variant="outline">
// entry-badge folder → <Badge variant="secondary">DIR</Badge>
// entry-badge file → <Badge>FILE</Badge>
// crumb-button → BreadcrumbItem > <button>
```

**Step 2: Clean up dead CSS rules from index.css / App.css**

Delete any CSS selectors that no longer have matching elements.

**Step 3: Verify dev server renders dashboard correctly**

```bash
npm run dev
# log in and check the dashboard
```

**Step 4: Commit**

```bash
git add frontend/src/pages/dashboard-page.tsx frontend/src/index.css frontend/src/App.css
git commit -m "feat: migrate dashboard-page to shadcn card, badge, breadcrumb + tailwind"
```

---

### Task 8: Migrate upload-panel.tsx to shadcn + Tailwind

**Files:**
- Modify: `frontend/src/components/upload-panel.tsx`

**Step 1: Replace upload-card, upload-copy, upload-toolbar with Card + Tailwind**

```tsx
import { Card, CardContent, CardHeader, CardTitle } from './ui/card';
import { Button } from './ui/button';

// upload-card → <Card className="flex flex-col gap-4 p-6">
// upload-copy → <CardHeader>
// upload-toolbar → <div className="flex gap-2">
// secondary-button → <Button variant="outline">
// uppy-shell → <div className="flex flex-col gap-2">
// upload-grid → <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
```

**Step 2: Wrap ConflictPreviewDialog with shadcn Dialog**

```tsx
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter,
} from './ui/dialog';

// Replace the custom backdrop/div with:
<Dialog open={conflictDialog !== null} onOpenChange={(open) => !open && resolveConflictDecision(null)}>
  <DialogContent>
    <DialogHeader>
      <DialogTitle id="conflict-dialog-title">Conflicts found in this upload</DialogTitle>
    </DialogHeader>
    {/* conflict items list */}
    <DialogFooter>
      <Button variant="outline" onClick={onCancel}>Cancel upload</Button>
      <Button variant="outline" onClick={() => onSelect(DuplicateConflictPolicy.DuplicateConflictPolicyRename)}>
        Create renamed copies
      </Button>
      <Button onClick={() => onSelect(DuplicateConflictPolicy.DuplicateConflictPolicyReplace)}>
        Replace existing files
      </Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
```

**Step 3: Verify upload works end-to-end**

```bash
npm run dev
# log in, try uploading a file and a folder, trigger a conflict
```

**Step 4: Clean remaining dead CSS**

Once all components are migrated, delete `App.css` entirely if empty, and remove leftover rules from `index.css`.

**Step 5: Commit**

```bash
git add frontend/src/components/upload-panel.tsx frontend/src/index.css frontend/src/App.css
git commit -m "feat: migrate upload-panel to shadcn card, button, dialog + tailwind"
```

---

## Phase 3 — TanStack Query

### Task 9: Install and configure TanStack Query

**Files:**
- Modify: `frontend/package.json`
- Modify: `frontend/src/main.tsx`

**Step 1: Install**

```bash
cd frontend
npm install @tanstack/react-query
npm install --save-dev @tanstack/react-query-devtools
```

**Step 2: Wrap app with QueryClientProvider in `src/main.tsx`**

```tsx
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ReactQueryDevtools } from '@tanstack/react-query-devtools';

const queryClient = new QueryClient();

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <AppRouter />
      </AuthProvider>
      <ReactQueryDevtools initialIsOpen={false} />
    </QueryClientProvider>
  </StrictMode>
);
```

**Step 3: Verify build**

```bash
npm run build
```

**Step 4: Commit**

```bash
git add frontend/src/main.tsx frontend/package.json frontend/package-lock.json
git commit -m "feat: add tanstack query with QueryClientProvider"
```

---

### Task 10: Convert file listing to useQuery

**Files:**
- Modify: `frontend/src/pages/dashboard-page.tsx`

Replace the manual `useState` + `useEffect` + `loadListing` pattern with `useQuery`.

**Step 1: Add useQuery for file listing**

```tsx
import { useQuery, useQueryClient } from '@tanstack/react-query';

// Inside DashboardPage:
const queryClient = useQueryClient();

const { data: listing, isLoading, error } = useQuery({
  queryKey: ['files', currentPath, session?.accessToken],
  queryFn: async () => {
    if (!session) throw new Error('no session');
    const { data, error } = await getApiV1Files({
      client: apiClient,
      query: currentPath ? { path: currentPath } : undefined,
      headers: { Authorization: `Bearer ${session.accessToken}` },
    });
    if (error) throw new Error(error.error?.message ?? 'listing failed');
    return data;
  },
  enabled: !!session,
});

// Pass to UploadPanel:
<UploadPanel
  accessToken={session.accessToken}
  currentPath={currentPath}
  onUploaded={() => queryClient.invalidateQueries({ queryKey: ['files', currentPath] })}
/>
```

Remove: `isLoading` state, `error` state, `loadListing` function, the manual `useEffect`.

**Step 2: Update error/loading rendering**

```tsx
// error is now Error | null from useQuery
{error ? <p className="...">{error.message}</p> : null}
```

**Step 3: Verify dev server — listing loads, uploading refreshes list**

```bash
npm run dev
```

**Step 4: Commit**

```bash
git add frontend/src/pages/dashboard-page.tsx
git commit -m "feat: replace manual data fetching with tanstack query useQuery in dashboard"
```

---

### Task 11: Convert auth session restore to useQuery

**Files:**
- Modify: `frontend/src/lib/auth-context.tsx`

**Step 1: Replace bootstrap useEffect with useQuery**

```tsx
import { useQuery } from '@tanstack/react-query';

// Replace the hasBootstrapped ref + useEffect with:
const { isLoading } = useQuery({
  queryKey: ['session-restore'],
  queryFn: async () => {
    const existing = readSession();
    if (!existing) return null;
    try {
      const restored = await restoreSession();
      setSession(restored);
      return restored;
    } catch {
      clearSession();
      setSession(null);
      return null;
    }
  },
  staleTime: Infinity,   // only run once per mount
  gcTime: 0,
});
```

**Step 2: Verify login/logout/session restore still works**

```bash
npm run dev
# log in, refresh page (session should restore), log out
```

**Step 3: Commit**

```bash
git add frontend/src/lib/auth-context.tsx
git commit -m "feat: replace session bootstrap useEffect with tanstack query"
```

---

## Phase 4 — Uppy UI polish (optional, do last)

### Task 12: Style Uppy dropzone with shadcn tokens

**Files:**
- Modify: `frontend/src/components/upload-panel.tsx`
- Modify: `frontend/src/index.css`

Uppy ships its own CSS. Override its variables using shadcn CSS custom properties so the dropzone matches the app theme.

**Step 1: Import Uppy CSS (if not already imported)**

In `src/main.tsx` or `src/index.css`:

```css
@import '@uppy/core/dist/style.min.css';
@import '@uppy/react/dist/style.min.css';
```

**Step 2: Override Uppy CSS variables to match shadcn theme**

In `src/index.css`, after the Tailwind import:

```css
:root {
  --uppy-color-accent: hsl(var(--primary));
  --uppy-color-accent-hover: hsl(var(--primary) / 0.9);
  --uppy-color-text: hsl(var(--foreground));
  --uppy-color-border: hsl(var(--border));
  --uppy-color-bg: hsl(var(--card));
}
```

(Exact variable names depend on Uppy version — check `node_modules/@uppy/core/dist/style.min.css` for available overrides.)

**Step 3: Verify Uppy dropzone looks consistent with the rest of the UI**

```bash
npm run dev
```

**Step 4: Commit**

```bash
git add frontend/src/index.css frontend/src/main.tsx
git commit -m "feat: override uppy css variables to match shadcn theme"
```

---

## Cleanup checklist

After all tasks:

- [ ] `App.css` is deleted or empty
- [ ] `index.css` has only `@import "tailwindcss"`, Uppy overrides, and any remaining global resets — no component-level CSS
- [ ] `orval.config.ts` and `scripts/fix-folder-upload-client.mjs` are deleted
- [ ] `npm run build` exits clean
- [ ] `npm run lint` exits clean
- [ ] Dev server: login → dashboard → browse folders → upload file → upload folder → trigger conflict dialog all work

---

## Notes for the implementer

- **hey-api response shape:** `{ data, error, response }` — never `.status` directly. `error` is non-null on failure.
- **hey-api query params:** pass as `query: { ... }` not positionally.
- **shadcn components** land in `src/components/ui/` — don't modify them; extend by wrapping.
- **Tailwind v4** does not use `tailwind.config.ts` — configuration lives in CSS via `@theme` blocks if customization needed.
- **TanStack Query v5** — `cacheTime` is now `gcTime`. `onSuccess`/`onError` callbacks on `useQuery` are removed; use `useEffect` watching `data`/`error` if side effects needed.
