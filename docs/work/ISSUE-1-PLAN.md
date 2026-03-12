# Implementation Plan: Backend Single-File Upload Endpoint (Issue #1)

**Reference**: Phase 6 of the implementation plan
**Related Issue**: GitHub Issue #1
**Current Checkpoint**: Phase 5 completion (File listing API working)

## Executive Summary

Implement a single-file upload endpoint that accepts authenticated multipart uploads and streams files to S3-compatible storage. This endpoint forms the foundation for the upload pipeline (Phases 6-8).

## Locked Decisions

- Existing-key upload behavior for issue `#1`: return `409 Conflict`; duplicate resolution remains a later phase.
- Filename sanitization engine: defer full sanitization logic to a dedicated follow-up ticket with TDD-first coverage.
- Filename handling in issue `#1`: use a temporary minimal safety rule only, sufficient to prevent path injection and empty names.
- `path` semantics: treat `path` as a destination folder only, not a full object key.
- Zero-byte uploads: reject with `400 Bad Request`.
- Object metadata: persist a minimal backend-owned metadata set on every uploaded object.

## Requirements Analysis

### From Issue Scope
- Accept multipart upload via authenticated endpoint
- Stream file to S3-compatible storage through the Go backend
- Respect destination path parameter within the user's bucket
- Add DTOs and OpenAPI annotations for the upload request/response
- Wire through the existing storage interface in `backend/internal/storage`

### Acceptance Criteria
- Non-conflicting single-file upload succeeds
- File appears at correct path in user bucket
- `go test ./...` passes
- `make backend-build` passes

## Design Decisions

### Endpoint Design
**Route**: `POST /api/v1/files/upload`
**Auth**: Required (Bearer token via `AuthMiddleware`)
**Payload**: `multipart/form-data`
  - Field `file`: binary file content (required)
  - Field `path`: destination path within bucket (optional, defaults to root)
  - Field `filename`: override detected filename (optional)

**Response**: `201 Created`
```json
{
  "file": {
    "name": "document.pdf",
    "path": "uploads/document.pdf",
    "size": 1024576,
    "uploaded_at": "2026-03-12T16:00:00Z"
  }
}
```

### Error Cases
- **400 Bad Request**: No file provided, invalid path format, zero-byte file, file too large
- **401 Unauthorized**: Missing/invalid auth token
- **409 Conflict**: File already exists; duplicate-resolution UX remains a later phase
- **500 Internal Server Error**: S3/storage failures

### Path Validation Rules
1. Destination path must be within user's bucket (no `..` traversal)
2. Empty path uploads to bucket root
3. `path` is always treated as a folder prefix; filename comes from multipart input or `filename` override
4. Path with trailing `/` treated as folder prefix (append validated filename)
5. Path must be normalized (forward slashes, no double slashes)

### Filename Handling for Issue #1
This issue should not implement the full sanitization engine. That work is important and should be isolated so it can be developed test-first and reviewed independently.

For issue `#1`, use a temporary minimal filename rule:

1. Use the multipart filename unless `filename` override is provided
2. Reject path separators and traversal fragments such as `/`, `\`, and `..`
3. Trim surrounding whitespace
4. Reject empty filenames after trimming/validation
5. Keep the filename otherwise unchanged for now

### Filename Sanitization Follow-Up
The full stable-filename sanitization engine should be handled in a separate ticket and then plugged into upload, folder upload, duplicate handling, and restore flows.

That follow-up should be implemented with TDD first and must include exhaustive tests for:

- leading/trailing whitespace
- repeated internal whitespace
- control characters
- path separators
- traversal-like sequences
- multiple dots
- hidden-file prefixes
- missing extension
- multi-part extensions like `.tar.gz`
- unicode normalization policy
- reserved or degenerate names
- filenames that become empty after normalization

### Object Metadata Contract
Write the following metadata on each uploaded object:

- `original-filename`: filename before sanitization
- `stored-filename`: final filename written to storage
- `uploaded-by-user-id`: authenticated user ID
- `uploaded-at`: backend-generated UTC RFC3339 timestamp

Also set the S3 `ContentType` field from the multipart header when present, otherwise default to `application/octet-stream`.

### File Size Strategy
- Single upload handles any file size via streaming
- Multipart upload (Phase 3) will handle files > 5 MB with resumable chunks
- For Phase 6, no artificial limits; S3 handles streaming naturally
- Zero-byte uploads are not allowed in this phase

## Implementation Breakdown

### Phase 1: Storage Layer Enhancements

**File**: `backend/internal/storage/client.go`

Add method to upload file to S3:
```go
func (c *Client) PutObject(ctx context.Context, bucket, key string, body io.Reader, size int64) error
```

**Responsibilities**:
- Stream body to S3
- Handle S3 errors (404, 403, 413, etc.)
- Accept object metadata and content type as part of the write contract
- Return error with context

**Verification**:
- ✓ Code compiles
- ✓ Method signature matches interface pattern
- ✓ Unit test with mock S3 client

---

### Phase 2: Service Layer Implementation

**File**: `backend/internal/files/service.go`

Add method to upload file:
```go
func (s *Service) Upload(ctx context.Context, userID, destinationPath, filename string, body io.Reader, size int64) (UploadResult, error)
```

**Responsibilities**:
- Validate destination path (no traversal, normalize)
- Apply the temporary minimal filename validation rule
- Resolve final S3 key from path + validated filename
- Check whether the destination key already exists and surface a conflict
- Prepare object metadata and content type for storage
- Call storage layer to write
- Return upload result metadata

**Verification**:
- ✓ Code compiles
- ✓ Path normalization handles edge cases (empty, trailing /, etc.)
- ✓ Unit tests for path resolution logic
- ✓ Unit tests for service call to storage layer

---

### Phase 3: HTTP Handler and DTOs

**File**: `backend/internal/api/dto/files.go`

Add request/response DTOs:
```go
type FileUploadRequest struct {
  // Implicit in multipart form
  // file: binary
  // path: string
  // filename: string
}

type FileUploadResponse struct {
  File UploadedFileInfo `json:"file"`
}

type UploadedFileInfo struct {
  Name       string `json:"name"`
  Path       string `json:"path"`
  Size       int64  `json:"size"`
  UploadedAt string `json:"uploaded_at"`
}
```

**File**: `backend/internal/files/http.go`

Add handler method:
```go
func (h *Handler) Upload(c *gin.Context) // POST /api/v1/files/upload
```

**Responsibilities**:
- Extract multipart form file
- Parse path and filename from form
- Validate file size (reject empty files)
- Call service layer
- Respond with 201 + metadata
- Handle errors with appropriate status codes

**OpenAPI Annotations**:
```go
// @Summary Upload file
// @Description Upload a single file to a destination path in the user's bucket
// @Tags files
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "File to upload"
// @Param path formData string false "Destination path within bucket"
// @Param filename formData string false "Override filename"
// @Success 201 {object} dto.FileUploadResponse
// @Failure 400 {object} dto.ErrorResponse
// @Failure 401 {object} dto.ErrorResponse
// @Failure 500 {object} dto.ErrorResponse
// @Router /api/v1/files/upload [post]
```

**Verification**:
- ✓ Code compiles
- ✓ Handler extracts multipart correctly
- ✓ Handler calls service with correct parameters
- ✓ Unit test with mock service layer
- ✓ OpenAPI spec generates correctly

---

### Phase 4: Router Registration

**File**: `backend/internal/http/router.go`

Register upload handler:
```go
api.POST("/files/upload", authHandler.AuthMiddleware(), filesHandler.Upload)
```

**Verification**:
- ✓ Route appears in generated OpenAPI spec
- ✓ Route is protected by AuthMiddleware

---

### Phase 5: Testing and Verification

**Files to create/modify**:
- `backend/internal/storage/client_test.go` (if not exists)
- `backend/internal/files/service_test.go` (if not exists)
- `backend/internal/files/http_test.go` (if not exists)

**Test Coverage**:
1. **Storage layer**:
   - PutObject succeeds with mock S3
   - Error handling for S3 failures

2. **Service layer**:
   - Path normalization (empty, root, nested, trailing slashes)
   - No path traversal (../.. rejected)
   - Minimal filename validation rejects separators, traversal, and empty names
   - S3 key generation correct
   - Existing key returns conflict
   - Metadata payload populated correctly
   - Service calls storage layer correctly

3. **HTTP layer**:
   - Multipart parsing works
   - Returns 201 with correct metadata
   - Returns 400 for missing file
   - Returns 400 for zero-byte file
   - Returns 400 for invalid path
   - Returns 401 without auth token
   - Returns 409 for duplicate key
   - Returns 500 on storage error

4. **End-to-End** (via shell script or test client):
   - `make backend-dev-up` starts Postgres + MinIO
   - Create test user via signup endpoint
   - Upload file to test user's bucket
   - Verify file exists in MinIO via list endpoint
   - Verify response contains correct path and size

**Verification Script Checklist**:
```bash
make backend-build              # ✓ builds
go test ./...                   # ✓ all tests pass
make backend-dev-up             # ✓ services up
make backend-run                # ✓ backend starts
# Manual: curl signup to create user
# Manual: curl upload with test file
# Manual: curl list to verify file appears
# Manual: check backend logs for request IDs
make backend-dev-down           # ✓ cleanup
```

---

## Verifiable Phases

### Phase 1: Storage Layer (Storage Interface Extension)
**Goal**: Implement `PutObject` in storage client  
**Deliverables**:
- `func (c *Client) PutObject(...)` method in `storage/client.go`
- Proper error handling for S3 errors
- No OpenAPI annotation yet

**Verification Criteria**:
- Code compiles: `make backend-build`
- Lints pass: `make backend-lint`
- Unit test passes: `go test ./internal/storage`

**Completion Signal**: PR or commit with storage layer tests passing

---

### Phase 2: Service Layer (File Service Upload Logic)
**Goal**: Add upload logic, path validation, S3 key generation  
**Deliverables**:
- `func (s *Service) Upload(...)` method in `files/service.go`
- Path normalization and validation helpers
- Proper error propagation from storage

**Verification Criteria**:
- Code compiles: `make backend-build`
- Path validation tests pass: `go test ./internal/files`
- Minimal filename validation tests pass: `go test ./internal/files`
- Service calls storage correctly

**Completion Signal**: Unit tests for path normalization pass; service layer integrated

---

### Phase 3: HTTP Handler and DTOs (API Layer)
**Goal**: Implement multipart handler, add DTOs, wire OpenAPI  
**Deliverables**:
- UploadRequest, UploadResponse DTOs in `api/dto/files.go`
- Handler method in `files/http.go` with OpenAPI annotations
- Route registered in `http/router.go`

**Verification Criteria**:
- Code compiles: `make backend-build`
- OpenAPI generation works: `make backend-openapi`
- HTTP handler unit tests pass: `go test ./internal/files`
- Generated OpenAPI includes upload endpoint
- Duplicate upload returns `409 Conflict`

**Completion Signal**: `make backend-openapi` succeeds; spec includes upload route

---

### Phase 4: Integration Testing (End-to-End)
**Goal**: Verify full upload flow with real S3/MinIO  
**Deliverables**:
- Docker MinIO + Postgres up
- Backend running against MinIO
- Test script or manual curl sequence

**Verification Criteria**:
- `make backend-dev-up` succeeds
- Backend boots: `make backend-run`
- All unit tests pass: `go test ./...`
- Upload integration test succeeds:
  - User signup
  - File upload to root and nested path
  - File appears in list endpoint
  - Duplicate upload to the same key returns 409
  - Zero-byte upload returns 400
  - Object metadata matches (`original-filename`, `stored-filename`, `uploaded-by-user-id`, `uploaded-at`)

**Completion Signal**: End-to-end test passes; file appears in bucket

---

## Remaining Open Questions

1. **File Size Limit**:
   - Should we enforce a maximum upload size in this phase?
   - Recommendation: No hard limit for now; rely on S3 streaming. Larger files will be handled by multipart upload in Phase 3.

2. **Resumable Uploads**:
   - Should we support resumable/chunked uploads in this endpoint?
   - Recommendation: No. This is single-file non-resumable. Multipart/resumable in Phase 3.

3. **Progress Reporting**:
   - Should the endpoint support streaming upload progress (e.g., `Expect: 100-continue`)?
   - Recommendation: Not in Phase 6. Future enhancement if needed.

4. **Dedicated Sanitization Ticket Scope**:
   - Should the standalone sanitization engine also define unicode normalization and reserved-name policy?
   - Recommendation: Yes. That ticket should fully lock the stable-filename contract before broader reuse.

---

## Dependencies and Blockers

- ✓ Phase 5 (File listing) already complete
- ✓ Phase 2 (Auth) already complete
- ✓ Phase 1 (Storage client) already complete
- ✓ Multipart parsing available via Gin framework
- ⚠️ No external dependencies needed beyond existing AWS SDK

**No blockers identified.**

---

## Success Criteria Summary

| Criterion | Status | Verification |
|-----------|--------|---------------|
| Single-file upload succeeds | Required | `curl -F file=@test.txt` returns 201 |
| File appears at correct S3 key | Required | List API shows uploaded file |
| Path parameter respected | Required | Upload to nested path works |
| Destination path validated | Required | `../etc/passwd` upload rejected with 400 |
| Auth required | Required | Unauthenticated request returns 401 |
| Multipart parsing works | Required | Form file extracted correctly |
| Duplicate key rejected | Required | Second upload to same key returns 409 |
| Zero-byte upload rejected | Required | Empty upload returns 400 |
| Metadata returned | Required | Response includes name, path, size |
| Metadata stored on object | Required | Object metadata includes upload audit fields |
| OpenAPI spec updated | Required | `make backend-openapi` includes endpoint |
| Tests pass | Required | `go test ./...` all green |
| Build passes | Required | `make backend-build` succeeds |

---

## Timeline Estimate

- **Phase 1 (Storage)**: 30-60 min
- **Phase 2 (Service)**: 60-90 min
- **Phase 3 (Handler + DTOs)**: 60-90 min
- **Phase 4 (Router + OpenAPI)**: 15-30 min
- **Phase 5 (Testing + E2E)**: 60-120 min

**Total**: 4-6 hours of focused development

---

## Related Tasks

- **Blocks Phase 2**: Backend folder upload (will reuse this endpoint foundation)
- **Blocks Phase 3**: Multipart upload for files > 5 MB
- **Blocks Phase 4**: Duplicate detection and conflict resolution
- **Blocks Phase 5**: Delete-to-trash semantics (will use path patterns established here)

---

## Commit Strategy

Suggested commit path:
1. `feat(backend): add PutObject to storage client`
2. `feat(backend): implement file upload service`
3. `feat(backend): add upload DTOs and HTTP handler`
4. `feat(backend): wire upload route and OpenAPI annotations`
5. `test(backend): add upload integration tests`

Final commit message:
```
feat(backend): single-file upload endpoint

- Add PutObject method to S3-compatible storage client
- Implement upload service with path validation
- Add multipart upload HTTP handler at POST /api/v1/files/upload
- Include path parameter and filename override support
- Reject duplicate keys with 409 until duplicate-resolution flow exists
- Reject zero-byte uploads
- Store upload audit metadata and content type on S3 objects
- Add comprehensive path normalization (no traversal, no double slashes)
- Update OpenAPI spec with upload endpoint
- Add unit and integration tests
- Verify with MinIO + Postgres in local dev environment

Closes #1
```
