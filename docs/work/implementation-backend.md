# Backend Implementation Progress

## Scope

This file tracks backend-only implementation progress, verification evidence, and commit points.

## Checkpoints

### Checkpoint 4: Backend DTO contract and OpenAPI generation

Goal:

- move API contracts into explicit Go DTO structs
- annotate handlers for OpenAPI generation
- generate OpenAPI artifacts from backend code
- serve Swagger UI from the backend

Verification steps:

- `make backend-tidy`
- `make backend-build`
- `make backend-openapi`
- `go test ./...`
- start backend
- call `GET /swagger/doc.json`

Verification result:

- passed on 2026-03-12
- `make backend-tidy`: passed
- `make backend-build`: passed
- `make backend-openapi`: passed and generated `docs/openapi/docs.go`, `swagger.json`, and `swagger.yaml`
- `go test ./...`: passed
- backend served `GET /swagger/doc.json`
- verified spec metadata and DTO-backed auth routes in generated document

Commit:

- `ef35ebf feat: add backend openapi contract`

### Checkpoint 1: Backend foundation boot

Goal:

- scaffold Go + Gin backend
- load runtime config
- expose `GET /healthz`
- write structured logs to terminal and `logs/backend.log`
- add request ID propagation and request logging

Verification steps:

- `make backend-tidy`
- `make backend-build`
- start backend with example env
- call `GET /healthz`
- confirm log file is written under `logs/`

Verification result:

- passed on 2026-03-12
- `make backend-tidy`: passed
- `make backend-build`: passed
- `go test ./...`: passed
- started backend with default example env values: passed
- `GET /healthz`: returned `200 OK` with JSON status payload and `X-Request-Id`
- confirmed structured JSON logs in terminal and `logs/backend.log`

Commit:

- `15a67e4 chore: scaffold backend foundation`

### Checkpoint 2: Backend core platform

Goal:

- connect to PostgreSQL on startup
- run SQL migrations automatically
- initialize S3-compatible client against provider endpoint
- expose `GET /readyz` that validates database and storage connectivity
- add local dev services for self-verification

Verification steps:

- `make backend-dev-up`
- `make backend-tidy`
- `make backend-build`
- `go test ./...`
- start backend with `.env.local` R2 credentials auto-loaded
- call `GET /readyz`
- confirm migrations created expected tables in Postgres
- confirm readiness succeeds against the configured R2 bucket

Verification result:

- passed on 2026-03-12
- `make backend-dev-up`: passed after rebinding local Postgres container to host port `5433`
- `make backend-tidy`: passed
- `make backend-build`: passed
- `go test ./...`: passed
- backend auto-loaded root `.env.local` for R2 credentials and bucket selection: passed
- `GET /readyz`: returned `200 OK`
- confirmed Postgres migrations created `users`, `refresh_tokens`, and `schema_migrations`
- confirmed storage readiness against the configured R2 bucket via `HeadBucket`

Commit:

- `41e81cb feat: add backend platform wiring`

### Checkpoint 3: Auth and user provisioning

Goal:

- implement signup, login, refresh, and protected `me` endpoint
- hash passwords with bcrypt
- issue JWT access tokens and opaque refresh tokens
- persist and rotate refresh tokens in Postgres
- provision one storage bucket per user on signup

Verification steps:

- `make backend-dev-up`
- `make backend-build`
- `go test ./...`
- start backend with `.env.local` R2 credentials auto-loaded
- call signup, login, refresh, and protected `me` endpoints
- confirm user and refresh token rows exist in Postgres
- confirm signup-created user bucket exists in R2

Verification result:

- passed on 2026-03-12 against local Postgres + local MinIO
- `make backend-dev-up`: passed
- `make backend-build`: passed
- `go test ./...`: passed
- signup endpoint returned `201 Created`
- login endpoint returned `200 OK`
- refresh endpoint returned `200 OK`
- protected `GET /api/v1/me` returned the authenticated user
- confirmed user row exists in Postgres
- confirmed refresh token rows exist in Postgres and rotation adds a new token row
- confirmed signup-created bucket exists on S3-compatible storage via `head-bucket`
- note: the provided R2 credentials successfully support readiness checks on the existing bucket, but `CreateBucket` against R2 returned `403 AccessDenied`, so bucket-provisioning verification for this checkpoint used local MinIO instead

Commit:

- `0f9458b feat: add backend auth and provisioning`
