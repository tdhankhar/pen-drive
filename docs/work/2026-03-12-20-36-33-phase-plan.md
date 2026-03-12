# Phase-Wise Implementation Plan

## Locked Decisions

- Database: PostgreSQL
- Frontend: React + TypeScript + Vite + Oxlint
- Backend: Go + Gin
- Object storage: S3-compatible via standard AWS S3 SDK, targeting Cloudflare R2 initially
- User ID: ULID
- Upload path: browser -> Go backend -> S3-compatible storage
- Auth: email/password with bcrypt, access token + refresh token
- Bucket model: one bucket per user, where `bucket_name = user_id`
- Delete model: soft delete to `trash/<original-path>` inside the user's bucket
- Future restore model: move from `trash/<original-path>` back to `<original-path>`, using the same duplicate-resolution rules as upload

## Recommended API Contract Workflow

Use Go request/response DTO structs as the contract source, generate OpenAPI from the Gin backend, and generate TypeScript client/types for the frontend from that OpenAPI spec.

Recommended stack:

- Backend spec generation: `swaggo/swag`
- OpenAPI artifact committed/generated from backend DTOs and handler annotations
- Frontend type/client generation: `orval` from generated OpenAPI
- Frontend data layer: generated client consumed through TanStack Query

Why this path:

- keeps backend DTOs as the source of truth
- follows common Gin/OpenAPI practice in Go projects
- gives generated TS request/response types automatically
- reduces manual React plumbing for API calls

## Parallel Workstreams

These can start independently once Phase 0 is complete:

- Backend foundation and API contract setup
- Frontend foundation and auth/file-browser shell

These should not be blocked on each other except at contract-integration checkpoints:

- frontend app shell, routing, auth screens, file-browser UI shell
- backend project layout, config, logging, auth skeleton, storage abstraction

## Phase 0: Foundation and Repo Layout

Goal:

- create the base monorepo layout and engineering conventions

Deliverables:

- `frontend/` Vite React TS app with Oxlint
- `backend/` Go module with Gin app skeleton
- shared root docs for setup and local development
- `logs/` directory policy defined
- `.env.example` files for frontend and backend
- make/task runner commands for common workflows

Suggested structure:

- `frontend/`
- `backend/cmd/api`
- `backend/internal/config`
- `backend/internal/http`
- `backend/internal/auth`
- `backend/internal/storage`
- `backend/internal/db`
- `backend/internal/users`
- `backend/internal/files`
- `backend/internal/logging`
- `backend/internal/api/dto`
- `backend/docs/openapi`
- `docs/work`
- `logs/`

Self-verification:

- frontend boots with `npm run dev`
- backend boots with `go run ./cmd/api`
- lint commands run on both apps
- backend writes structured logs to terminal and `logs/`

## Phase 1: Backend Core Platform

Depends on:

- Phase 0

Goal:

- establish backend primitives that every feature builds on

Deliverables:

- config loader for Postgres, JWT, R2/S3 endpoint, credentials, and app env
- structured logging using Go `slog` JSON output
- log fan-out to stdout and file in `logs/`
- Postgres connection and migration setup
- AWS SDK v2 S3-compatible client configured for R2/custom endpoint
- health endpoint
- consistent error envelope and request ID middleware

Technical notes:

- prefer `slog` + `lumberjack` for terminal + rolling file logs
- keep S3 access behind an internal storage interface so provider swaps stay localized
- add DB migrations from the start

Self-verification:

- health endpoint returns success
- startup fails clearly with invalid env
- app can connect to Postgres
- app can issue a bucket operation against the configured S3-compatible endpoint
- log lines contain timestamp, level, request ID, and message in both stdout and `logs/`

## Phase 2: Auth and User Provisioning

Depends on:

- Phase 1

Goal:

- implement secure authentication and per-user bucket provisioning

Deliverables:

- `users` table with ULID primary key and unique email
- signup endpoint
- login endpoint
- refresh endpoint
- bcrypt password hashing
- JWT access token issuance
- refresh token persistence and rotation strategy
- auth middleware for protected routes
- bucket provisioning on signup where `bucket_name = user_id`

Data model baseline:

- `users(id, email, password_hash, created_at, updated_at)`
- `refresh_tokens(id, user_id, token_hash, expires_at, created_at, revoked_at)`

Self-verification:

- signup creates user row with ULID
- signup provisions user bucket
- login returns valid access + refresh tokens
- protected route rejects missing/invalid token
- refresh rotates token successfully

## Phase 3: OpenAPI Contract and Generated Frontend Client

Depends on:

- Phase 1

Can run in parallel with:

- Phase 2 implementation after endpoint shapes are stable enough
- Phase 4 frontend shell work

Goal:

- make backend DTOs drive the API contract and frontend types

Deliverables:

- annotated Gin handlers and DTOs
- generated OpenAPI spec committed under `backend/docs/openapi`
- `orval` config in frontend
- generated TS API client and types
- generation commands wired into scripts

Recommended rule:

- no handwritten frontend API request/response types for backend endpoints

Self-verification:

- spec generation runs cleanly
- frontend client generation runs cleanly
- a contract change in Go DTOs produces updated TS types
- frontend builds against generated client without manual edits

## Phase 4: Frontend Foundation

Depends on:

- Phase 0

Can run in parallel with:

- Phase 1
- most of Phase 2
- initial Phase 3 setup

Goal:

- establish the frontend application shell without waiting on full backend completion

Deliverables:

- app routing
- auth pages: signup and login
- protected dashboard route
- layout for file-system style browser
- API layer integration using generated client
- TanStack Query setup
- basic session handling for access + refresh workflow

Self-verification:

- app routes render correctly
- unauthenticated users are redirected to login
- auth forms submit to mocked or live endpoints
- lint/build succeed

## Phase 5: File Listing and Browser UI

Depends on:

- Phase 2
- Phase 3
- Phase 4

Goal:

- allow a user to browse their bucket like a simple file system

Deliverables:

- paginated backend list API over the user's bucket
- path-aware folder traversal
- dashboard file/folder listing UI
- download action using presigned URL from backend
- pagination or incremental loading strategy that fits S3 listing constraints

Design note:

- backend should own translation from flat object keys to folder/file view
- preserve future support for deep navigation and large buckets

Self-verification:

- root folder listing works for authenticated user
- navigating into nested folders works
- download URL is returned and usable
- pagination token flow works across multiple pages

## Phase 6: Upload Pipeline

Depends on:

- Phase 2
- Phase 3
- Phase 4
- Phase 5 for destination selection and list refresh

Goal:

- support file and folder upload into the user's bucket through the Go backend

Deliverables:

- frontend upload UI using Uppy or equivalent
- file upload through backend to S3-compatible storage
- folder upload preserving relative paths under selected destination
- multipart upload handling for files larger than 5 MB
- destination path selection within the user's root
- upload status and error reporting

Important rule:

- all uploads resolve paths relative to the selected target folder in the user's bucket

Self-verification:

- single file upload succeeds
- folder upload preserves relative paths
- files larger than 5 MB use multipart flow
- uploaded files appear correctly in the list API

## Phase 7: Duplicate Handling, Trash, and Delete Semantics

Depends on:

- Phase 5
- Phase 6

Goal:

- implement conflict-aware writes and soft delete behavior

Deliverables:

- duplicate detection before upload overwrite
- impacted-file preview returned to frontend
- duplicate choice: create renamed copy or replace existing
- rename strategy: `filename_(1).ext`
- replace strategy: move existing object(s) to `trash/<original-path>` before writing new object(s)
- delete action that moves file/object tree to trash instead of hard delete
- path rules designed to support future restore flow

Notes:

- for folder collisions, detect impacted object keys recursively
- replacement must be safe for both single-file and folder uploads
- trash path mapping must preserve original path exactly

Self-verification:

- duplicate preview shows impacted paths
- rename mode creates deterministic suffixed names
- replace mode moves originals into trash before upload completes
- delete action moves objects into trash without data loss

## Phase 8: Restore-Ready Trash Design

Depends on:

- Phase 7

Goal:

- finish the implementation details needed so a future restore feature is low risk

Deliverables:

- explicit trash path contract documented in backend code and API docs
- restore conflict algorithm defined to mirror duplicate upload behavior
- metadata decisions documented if needed for future restore UX

Self-verification:

- any trashed object can be mapped back to its original path deterministically
- conflict rules for restore are documented and testable

## Phase 9: Testing, Hardening, and Developer Experience

Depends on:

- Phases 1 through 8

Goal:

- make the system reliable enough to iterate on safely

Deliverables:

- backend unit tests for auth, path resolution, duplicate naming, and trash mapping
- backend integration tests for Postgres + S3-compatible flows
- frontend tests for auth flow and file-browser behavior
- seeded local/dev environment
- CI commands for lint, test, and spec generation

Self-verification:

- core test suite passes locally
- generated OpenAPI and frontend client are checked or reproducible in CI
- critical flows work end-to-end: signup, login, list, upload, delete

## Recommended Execution Order

1. Phase 0
2. Phase 1 and Phase 4 in parallel
3. Phase 2 and Phase 3 in parallel once backend foundation exists
4. Phase 5
5. Phase 6
6. Phase 7
7. Phase 8
8. Phase 9

## Minimum Viable Milestones

Milestone A:

- repo bootstrapped
- backend logging/config/db/storage working
- frontend shell working

Milestone B:

- signup/login/refresh complete
- bucket provisioning complete
- OpenAPI to TS generation working

Milestone C:

- authenticated file browser with list + download

Milestone D:

- file/folder upload working with multipart > 5 MB

Milestone E:

- duplicate handling and trash semantics working

## Open Questions To Resolve During Implementation

- whether refresh tokens should be stored as opaque random values or JWTs; current recommendation is opaque random tokens stored hashed in Postgres
- whether folder deletion should be exposed as a recursive delete API or inferred from selected prefix in the UI
- whether upload progress and large-upload retry behavior need to be included in the first release or handled after the happy path
