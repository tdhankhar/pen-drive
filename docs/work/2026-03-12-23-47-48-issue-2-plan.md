# Implementation Plan: Backend Folder Upload with Relative Path Preservation (Issue #2)

**Reference**: Phase 6 of the implementation plan  
**Related Issue**: GitHub Issue `#2`  
**Current Baseline**: issue `#1` merged on `main` with authenticated single-file upload, duplicate `409`, zero-byte rejection, destination path validation, metadata write, and generated frontend client support

## Scope

Implement a backend folder-upload endpoint that accepts a batch of files representing a folder, preserves each file's relative path under a selected destination folder, and uploads every file into the authenticated user's bucket through the Go backend.

This issue should build directly on the merged single-file upload contract instead of creating a second upload model.

## Locked Contract

- the endpoint accepts multiple files in one multipart request
- each uploaded file must include a relative path representing its position inside the selected folder
- `path` remains the destination folder in the user's bucket
- final object key = `destination path` + `relative path inside uploaded folder`
- every relative path must be validated and must reject traversal segments such as `..`
- zero-byte files remain rejected with `400`
- duplicate keys remain rejected with `409`
- object metadata must be written for every uploaded file using the existing upload metadata contract
- multipart part `Content-Type` must be preserved per file when present
- uploads are processed as a batch, but issue `#2` does not add duplicate-resolution UX or rollback semantics

## Explicit Non-Goals

- no multipart/resumable upload for files larger than 5 MB
- no duplicate-resolution rename/replace flow
- no trash move or restore behavior
- no full filename sanitization engine beyond the current minimal validation rule
- no frontend UI implementation in this issue

## API Contract Shape

Recommended route:

- `POST /api/v1/files/upload-folder`

Recommended multipart fields:

- `files`: repeated binary file parts
- `relative_paths`: repeated string fields aligned by file order
- `path`: optional destination folder in the user's bucket

Alternative acceptable shape:

- repeated file parts where each part includes the browser-provided relative path in metadata or paired form fields

Recommendation:

- use repeated `files` plus repeated `relative_paths` arrays
- keep ordering strict and reject mismatched lengths with `400`
- do not depend on browser-specific nonstandard multipart behavior if a simple explicit contract works

## Response Contract

Recommended success response: `201 Created`

```json
{
  "files": [
    {
      "name": "report.pdf",
      "path": "docs/q1/report.pdf",
      "size": 1024,
      "uploaded_at": "2026-03-12T16:00:00Z"
    },
    {
      "name": "chart.png",
      "path": "docs/q1/assets/chart.png",
      "size": 2048,
      "uploaded_at": "2026-03-12T16:00:01Z"
    }
  ]
}
```

Recommendation:

- return per-file results for all uploaded files
- keep response fields aligned with the single-file upload response shape

## Failure Matrix

| Input | Expected Result |
|------|-----------------|
| missing `files` | `400 missing_files` |
| no `relative_paths` provided | `400 invalid_input` |
| `files` count != `relative_paths` count | `400 invalid_input` |
| any zero-byte file | `400 zero_byte_file` |
| any `path` traversal in destination path | `400 invalid_input` |
| any traversal in a relative path | `400 invalid_input` |
| any relative path empty after normalization | `400 invalid_input` |
| any file name invalid under current minimal filename rule | `400 invalid_input` |
| any final object key already exists | `409 file_exists` |
| storage write failure | `500 upload_failed` |

## Cross-Layer Data Flow Requirements

### Destination path

- read from multipart form field `path`
- validate using the same destination-path rules as issue `#1`
- pass normalized destination path into folder-upload service logic

### Relative path

- read as a per-file relative path from multipart form
- normalize separators to `/`
- reject `..`, empty segments after normalization, and absolute-path patterns
- split into folder segments + basename

### Filename

- derive the basename from the validated relative path unless the chosen request shape requires a separate filename source
- apply the same minimal filename validation currently used by single-file upload

### Content type

- read per file from multipart headers
- pass through service layer into storage `PutObjectInput.ContentType`

### Metadata

For each uploaded file, write:

- `original-filename`
- `stored-filename`
- `uploaded-by-user-id`
- `uploaded-at`

Recommendation:

- `original-filename` should reflect the per-file basename before minimal validation
- do not store the full relative path as metadata unless later restore/trash design needs it

## Design Decisions

### Batch processing model

Recommendation:

- validate the entire batch first
- if any file has invalid input, fail the request before writing anything
- then upload files sequentially in a deterministic order

Why:

- simpler failure semantics
- easier to reason about for duplicate handling in later phases
- avoids partial success from malformed input

### Duplicate behavior

Recommendation:

- preserve the current issue `#1` contract
- if any final object key already exists, return `409` and do not begin writes

Why:

- keeps folder upload behavior consistent with single-file upload
- avoids partial-batch writes until issue `#4` defines conflict resolution

### Reuse strategy

Recommendation:

- extract shared upload-building helpers in the service layer rather than duplicating validation logic in the handler
- keep the single-file and folder-upload paths aligned around a common internal upload item model

Suggested internal model:

```go
type UploadItem struct {
	FinalPath    string
	Name         string
	Size         int64
	ContentType  string
	Metadata     map[string]string
	Body         io.Reader
}
```

## Verifiable Phases

### Phase 1: Contract and DTO Definition
**Goal**: define the request/response contract and lock the batch semantics  
**Deliverables**:

- request DTO documentation for multi-file folder upload
- response DTO containing per-file upload results
- OpenAPI annotations for the new endpoint
- explicit rules for `files`/`relative_paths` alignment

**Verification Criteria**:

- DTOs compile
- OpenAPI generation succeeds
- generated spec clearly shows the new route and fields
- failure cases for missing files and mismatched arrays are documented

**Completion Signal**: API contract is generated and reviewable before service logic is finalized

### Phase 2: Relative Path Validation Engine
**Goal**: add deterministic validation for relative file paths within a folder upload  
**Deliverables**:

- helper to validate destination path
- helper to validate relative path
- helper to build final object key from destination path + relative path
- unit tests for normalization and traversal rejection

**Verification Criteria**:

- `docs/reports/file.pdf` maps correctly under root and nested destination paths
- `../file.pdf` is rejected
- `folder/../../file.pdf` is rejected
- empty or directory-only relative paths are rejected
- backslash-separated input is normalized and validated correctly

**Completion Signal**: path helper tests pass and all key-building rules are explicit

### Phase 3: Service Layer Batch Upload
**Goal**: implement batch folder upload by reusing single-file upload semantics  
**Deliverables**:

- service method for folder upload
- whole-batch validation before any writes
- duplicate pre-check across final keys
- per-file metadata and content type propagation
- deterministic upload order

**Verification Criteria**:

- all final keys are computed before writing
- any duplicate existing key returns `409` with no writes started
- per-file `ContentType` reaches storage input
- metadata is populated for every item
- service tests cover mixed nested paths and duplicate conflicts

**Completion Signal**: service tests prove batch validation and write behavior

### Phase 4: HTTP Handler and Status Mapping
**Goal**: implement the multipart handler for folder upload  
**Deliverables**:

- authenticated handler for folder upload
- multipart parsing for repeated files and repeated relative paths
- `400` mapping for malformed batch input
- `409` mapping for duplicate keys
- `201` response with per-file results

**Verification Criteria**:

- request with aligned arrays succeeds
- request with mismatched arrays returns `400`
- request with path traversal returns `400`
- request with duplicate target key returns `409`
- request returns per-file uploaded paths

**Completion Signal**: handler tests prove the HTTP contract

### Phase 5: Frontend-Unblock Contract Verification
**Goal**: ensure frontend issue work can begin immediately on the new route  
**Deliverables**:

- regenerated OpenAPI artifacts
- regenerated TS client
- upload-folder models/types available under `frontend/src/lib/api/generated`

**Verification Criteria**:

- `make backend-openapi` succeeds
- `cd frontend && npm run api:generate` succeeds
- `cd frontend && npm run build` succeeds
- generated client exposes the folder-upload endpoint

**Completion Signal**: frontend can consume the endpoint without handwritten client code

### Phase 6: End-to-End Verification
**Goal**: verify the folder upload flow against real local services  
**Deliverables**:

- local MinIO-backed verification run
- authenticated user with a seeded bucket
- uploaded nested folder example

**Verification Criteria**:

- `make backend-dev-up`
- backend starts against MinIO
- `go test ./...`
- upload a folder-shaped multipart request with nested relative paths
- list endpoint shows uploaded files under the correct nested structure
- duplicate upload to the same final keys returns `409`

**Completion Signal**: nested uploaded files are visible in the list API with the expected structure

## Required Tests Per Phase

### Service tests

- destination path normalization reused from issue `#1`
- relative path validation table tests
- final key generation table tests
- duplicate pre-check across multiple items
- metadata creation per file
- content type propagation per file

### HTTP tests

- missing files -> `400`
- mismatched files/relative_paths -> `400`
- traversal in destination path -> `400`
- traversal in relative path -> `400`
- duplicate existing key -> `409`
- successful batch upload -> `201` with correct result paths

### Integration tests

- upload root-level folder
- upload nested folder into non-root destination
- list API reflects the relative folder structure

## Suggested File Touchpoints

- `backend/internal/api/dto/files.go`
- `backend/internal/files/http.go`
- `backend/internal/files/service.go`
- `backend/internal/files/http_test.go`
- `backend/internal/files/service_test.go`
- `backend/internal/http/router.go`
- `backend/docs/openapi/*`
- `frontend/src/lib/api/generated/*`

## Open Questions

1. Should the route be `POST /api/v1/files/upload-folder`, or should folder upload share `POST /api/v1/files/upload` with a batch-capable request shape?
Recommendation: use a separate `upload-folder` route to avoid ambiguous multipart parsing and to keep issue `#1` stable.

2. Should folder upload fail the entire batch if any file conflicts or is invalid, or allow partial success?
Recommendation: fail the whole batch for now; partial success complicates duplicate handling and frontend UX.

3. Should we sort uploads by relative path before writing?
Recommendation: yes, to keep results deterministic and easier to test.

4. Do we need to persist the original relative path in metadata for future restore/trash features?
Recommendation: not yet; defer unless issue `#7` or `#9` proves the need.

## Success Criteria Summary

| Criterion | Status | Verification |
|-----------|--------|---------------|
| Folder upload accepts multiple files | Required | multipart request with repeated files succeeds |
| Relative paths preserved | Required | nested files appear under matching keys |
| Destination path respected | Required | nested upload under chosen folder works |
| Traversal rejected | Required | invalid destination or relative path returns `400` |
| Duplicate keys rejected | Required | existing target key returns `409` |
| Per-file metadata written | Required | storage input contains metadata for each file |
| Per-file content type preserved | Required | storage input `ContentType` matches multipart header |
| OpenAPI updated | Required | generated spec contains folder upload route |
| Frontend client updated | Required | Orval output contains folder upload call |
| Tests pass | Required | `go test ./...` all green |
| Builds pass | Required | backend and frontend build succeed |
