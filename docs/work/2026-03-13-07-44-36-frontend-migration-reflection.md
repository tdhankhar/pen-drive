# Frontend Migration Reflection
**Branch:** `fix/fe-migratino`
**Date:** 2026-03-13
**Scope:** Tailwind v4 + shadcn/ui, hey-api/openapi-ts, TanStack Query, Uppy theming

---

## What Was Done

16 commits replacing the frontend's hand-rolled CSS, orval codegen, and manual `useState/useEffect` data fetching:

| Phase | Changes |
|---|---|
| 1 — hey-api | Replaced orval + `orval.config.ts` with `@hey-api/openapi-ts`. New fetch client with `{ data, error }` destructuring on all call sites. |
| 2 — Tailwind + shadcn | Tailwind v4 via `@tailwindcss/vite`. shadcn/ui (Radix, Default/Slate). Migrated auth pages, dashboard, upload panel from custom CSS to Tailwind + Card/Button/Badge/Dialog/Breadcrumb. |
| 3 — TanStack Query | Replaced `useState/useEffect` listing fetches in `dashboard-page.tsx` with `useQuery`. Session bootstrap in `auth-context.tsx` moved to `useQuery`. |
| 4 — Uppy theming | Imported Uppy v5 CSS, overrode palette vars to match shadcn/slate tokens. |

---

## Issues Faced

### 1. shadcn init chose `base-nova` instead of Default/Slate

**What happened:** `npx shadcn@latest init` silently picked the `base-nova` preset (built on `@base-ui/react`) rather than the Radix-based `default` style. Components looked correct but were backed by a completely different primitive layer (`@base-ui/react` vs `@radix-ui/*`), and the CSS design tokens were never injected into `index.css`.

**Impact:** All components would have rendered without any colors or backgrounds. Caught in code review before Tasks 6–8 started.

**Fix:** Re-ran with explicit flags: `npx shadcn@latest init -t vite -b radix --force --reinstall`. Uninstalled `@base-ui/react` and `tw-animate-css`, reinstalled Radix-based components.

**Lesson:** Always pass `-t vite -b radix` explicitly to shadcn init — the interactive defaults are not deterministic across registry versions.

### 2. hey-api type names not shorter than orval

**What happened:** The plan stated a key motivation for switching from orval to hey-api was cleaner, shorter generated type names. In practice, hey-api generates the same long names (e.g. `GithubComAbhishekPenDriveBackendInternalApiDtoDuplicateConflictPolicy`) because it faithfully reflects the schema's `$ref` names, which come from Go's fully-qualified type paths in the OpenAPI spec.

**Impact:** The verbose type names persist throughout the codebase. Aliasing (`as DuplicateConflictPolicy`) mitigates readability but doesn't eliminate them.

**Fix:** Accepted as-is for now. Proper fix requires backend changes (see Next Steps).

**Lesson:** hey-api is not a magic name-shortener — it reflects what's in the spec. Cleaner names require a cleaner spec.

### 3. `@hey-api/client-fetch` deprecated, type incompatibility

**What happened:** The plan instructed importing `createClient` from `@hey-api/client-fetch`. By v0.73.0 of `@hey-api/openapi-ts`, the fetch client is bundled inside the generated output. The separate package is deprecated and caused a deep `Client` type mismatch when passed as `client:` to SDK functions.

**Fix:** Changed `http.ts` to import `createClient` from `./generated/client` (the generated bundle) instead of the npm package.

**Lesson:** Check hey-api release notes before following package-level docs — the architecture shifted significantly at v0.73.0.

### 4. Enum keys changed to SCREAMING_SNAKE_CASE

**What happened:** orval generated `DuplicateConflictPolicyRename`; hey-api generates `DUPLICATE_CONFLICT_POLICY_RENAME`. All existing usages in `upload-panel.tsx` were using the old PascalCase keys.

**Fix:** Updated all enum references in `upload-panel.tsx` during Task 2. Caught in the first code quality review.

### 5. Vite 8 peer dep conflicts with `@tailwindcss/vite` and shadcn

**What happened:** `@tailwindcss/vite@4.x` declares a peer dep of `vite@^5 || ^6 || ^7`. Vite 8 is not yet in the range. shadcn's `npm install` calls also fail for the same reason.

**Fix:** Added `.npmrc` with `legacy-peer-deps=true`. This is a packaging lag — the code works fine at runtime.

**Lesson:** When living on the bleeding edge of Vite (v8 at time of writing), expect peer dep metadata to lag.

### 6. auth-form.tsx CSS classes missed in first pass

**What happened:** The auth migration (Task 6) converted `auth-layout`/`auth-panel`/`auth-footer` but left `eyebrow`, `field`, and `form-error` custom CSS classes in `auth-form.tsx`. Caught in the final code review.

**Fix:** Converted the remaining three classes to Tailwind in the final cleanup commit.

**Lesson:** Component-level classes (inside `<AuthForm>`) are easy to miss when the migration task is described in terms of page-level layout. Spec reviews that grep for remaining custom classNames are essential.

### 7. oxlint incompatibility with ESM `__dirname` in vite.config.ts

**What happened:** `npm run lint` runs `oxlint . && eslint .`. oxlint tries to load `vite.config.ts` as a config file and crashes because `__dirname` is not defined in an ES module context.

**Status:** Pre-existing issue, not introduced by this migration. `eslint` passes cleanly. oxlint fix is a separate task.

---

## Next Steps

### High Priority

1. **Fix type names in the OpenAPI spec** — Add `x-go-name` annotations (or equivalent) to the backend's OpenAPI generation so Go type names are shortened before they hit the spec. Target: `DuplicateConflictPolicy`, `FileSystemEntry`, `FileListResponse`, etc. After fixing the spec, re-run `npm run api:generate` and update the dozen remaining verbose type references.

2. **Fix oxlint crash** — `vite.config.ts` uses `__dirname` (CommonJS API) in an ES module. Either switch to `import.meta.dirname` (Node 21.2+) / `fileURLToPath(import.meta.url)`, or configure oxlint to skip `vite.config.ts`. Unblocks `npm run lint` running cleanly end-to-end.

3. **Add `input` component from shadcn** — `auth-form.tsx` uses hand-styled `<input>` elements. Installing `npx shadcn@latest add input` and replacing the inline Tailwind `<input>` would make form styling consistent and accessible (focus rings, disabled states handled by the component).

### Medium Priority

4. **Add `form` component and React Hook Form** — The auth form uses manual `useState` for field values. shadcn's `form` component (built on react-hook-form + zod) would add validation feedback and reduce boilerplate. Pairs well with adding the `input` component.

5. **Loading states with shadcn Skeleton** — The dashboard file listing shows plain text "Loading folder contents..." during `isLoading`. Adding `npx shadcn@latest add skeleton` and a skeleton list would improve perceived performance.

6. **Pagination** — The backend returns `next_continuation_token` in the file listing response. The dashboard displays it as raw text but doesn't use it. Implement infinite scroll or a "Load more" button using `useInfiniteQuery` from TanStack Query.

7. **Upload progress** — Uppy supports per-file upload progress via `uppy.on('upload-progress', ...)`. Surface this in the UI using shadcn `Progress` component.

8. **Delete / rename / move file actions** — The file browser is currently read-only (upload + list). The backend likely supports delete and move operations. Add action buttons per entry using shadcn `DropdownMenu`.

### Low Priority

9. **Dark mode** — shadcn's CSS tokens include a full `.dark` variant. Adding a theme toggle (`npx shadcn@latest add toggle`) and persisting to localStorage / `prefers-color-scheme` is straightforward.

10. **Remove `App.css` import** — `App.css` is now empty. Remove the `import './App.css'` from `App.tsx` and delete the file to reduce dead imports.

11. **Move `QueryClient` to a dedicated file** — Currently created inline in `main.tsx`. Extracting to `src/lib/query-client.ts` makes it easier to import in tests.
