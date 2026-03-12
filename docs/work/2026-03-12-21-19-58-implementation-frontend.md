# Frontend Implementation Progress

## Scope

This file tracks frontend-only implementation progress, verification evidence, and commit points.

## Checkpoints

### Checkpoint 5: Upload UI and destination-path flow

Goal:

- add upload controls to the dashboard
- support file and folder selection
- preserve folder-relative paths under the selected destination
- integrate duplicate preview and rename-vs-replace choices with backend upload APIs

Planned scope:

- uploader entry point in dashboard shell
- selected destination folder derived from current browser path
- upload request wiring from generated API client
- duplicate-impact preview UI
- user choice for `rename` or `replace`
- optimistic refresh of the current listing after successful upload

Verification steps:

- regenerate backend OpenAPI
- regenerate frontend API client
- `make frontend-lint`
- `make frontend-build`
- browser verification: upload a non-conflicting file into root
- browser verification: upload a folder into a nested destination and preserve relative paths
- browser verification: duplicate preview appears and rename/replace both behave correctly

Verification result:

- pending

Commit:

- pending

### Checkpoint 4: File browser shell wired to list API

Goal:

- connect the dashboard to the backend list API
- show folders and files for the current path
- support folder navigation and basic loading/error states

Verification steps:

- regenerate backend OpenAPI
- regenerate frontend API client
- `make frontend-lint`
- `make frontend-build`
- browser verification: root listing renders
- browser verification: nested folder navigation renders correct entries

Verification result:

- passed on 2026-03-12
- `make backend-openapi`: passed
- `cd frontend && npm run api:generate`: passed
- `make frontend-lint`: passed
- `make frontend-build`: passed
- browser verification with Playwright: root listing rendered seeded folder and file entries
- browser verification with Playwright: navigating into `docs` rendered nested folder and file entries
- dashboard now uses the generated list client against the authenticated backend

Commit:

- `b0c8bb6 feat: wire dashboard file browser`

### Checkpoint 1: Frontend foundation scaffold

Goal:

- create a separate `frontend/` app with Vite + React + TypeScript
- enable Oxlint in the frontend workflow
- keep the scaffold buildable before backend API integration

Verification steps:

- `cd frontend && npm install`
- `make frontend-lint`
- `make frontend-build`

Verification result:

- passed on 2026-03-12
- `cd frontend && npm install`: passed
- `make frontend-lint`: passed
- `make frontend-build`: passed
- separate `frontend/` app scaffolded successfully with Vite + React + TypeScript
- Oxlint is active in the frontend lint workflow

Commit:

- `95d3a7c feat: scaffold frontend app`

### Checkpoint 2: Generated API client from backend OpenAPI

Goal:

- generate frontend TypeScript API types/client from backend OpenAPI artifacts
- keep the frontend contract-driven from backend DTOs

Verification steps:

- `make backend-openapi`
- `cd frontend && npm run api:generate`
- `make frontend-build`

Verification result:

- passed on 2026-03-12
- `make backend-openapi`: passed
- `cd frontend && npm run api:generate`: passed
- `make frontend-lint`: passed
- `make frontend-build`: passed
- generated client and model files are now sourced from `../backend/docs/openapi/swagger.json`

Commit:

- `422dfa3 feat: generate frontend api client`

### Checkpoint 3: Frontend auth shell

Goal:

- add login/signup routes and a protected dashboard route
- wire the generated auth client into a session layer
- persist tokens locally for this phase and restore via refresh on reload

Verification steps:

- `make backend-dev-up`
- start backend with local S3-compatible settings
- `make frontend-lint`
- `make frontend-build`
- verify unauthenticated `/app` redirects to `/login`
- verify signup reaches dashboard
- verify reload restores session through refresh flow

Verification result:

- passed on 2026-03-12
- `make backend-dev-up`: passed
- backend started locally against Postgres + MinIO: passed
- `make frontend-lint`: passed
- `make frontend-build`: passed
- browser verification with Playwright: unauthenticated `/app` redirected to `/login`
- browser verification with Playwright: signup reached `/app`
- browser verification with Playwright: reload restored session via refresh flow and stayed on `/app`
- TODO recorded: replace local-storage refresh token transport with secure HTTP-only cookies in a later phase

Commit:

- `1b96b79 feat: add frontend auth shell`
