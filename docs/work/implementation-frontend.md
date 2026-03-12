# Frontend Implementation Progress

## Scope

This file tracks frontend-only implementation progress, verification evidence, and commit points.

## Checkpoints

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
