# Reflection: Issue #1 Planning Gaps and How to Plan Better

**Date**: 2026-03-12  
**Subject**: `#1` backend single-file upload endpoint

## Context

Issue `#1` was merged successfully, but the first implementation still missed a few important requirements that had to be fixed afterward:

- destination path traversal rejection was not enforced
- multipart `Content-Type` was not carried through to S3 object writes
- the feature initially had no backend tests despite the plan calling for verifiable phases

The root cause was not only implementation quality. The plan itself left too much room between intent and proof.

## What The Plan Got Right

- it broke the work into storage, service, HTTP, router, and verification phases
- it identified duplicate handling as out of scope for `#1`
- it captured important product decisions like `409` on duplicate keys, zero-byte rejection, and object metadata
- it gave a reasonable implementation path for the backend

## What The Plan Missed

### 1. Critical rules were described, but not locked as a testable contract

The plan said "path validation" and "metadata/content type," but those requirements were still prose. That made it possible to implement something that looked reasonable while still violating the contract.

Examples:

- "validate destination path" was not enough; the plan should have explicitly said that any `..` segment in `path` must return `400`
- "set content type from multipart header" was not enough; the plan should have said that the handler must read it and the storage layer must receive it

### 2. The plan lacked a failure matrix

The happy path was defined, but the negative cases were not formalized into a compact input-to-output table.

That made it easier for implementation and review to focus on:

- request accepted
- object uploaded
- response returned

instead of:

- `path=../x` must fail with `400`
- duplicate key must fail with `409`
- zero-byte file must fail with `400`

### 3. Phase verification was too broad

The plan said phases were verifiable, but the exit criteria were still generic enough that a phase could be considered "done" without proving the exact behaviors the frontend or later phases depend on.

Examples:

- `go test ./...` passing is not meaningful if no tests exist for the feature
- "handler unit tests pass" is not enough unless the expected cases are named

### 4. The cross-layer data flow was not specified explicitly

Some requirements start in one layer and must arrive intact in another:

- multipart file header `Content-Type`
- destination `path`
- original filename
- stored filename
- object metadata keys

The plan did not force a layer-by-layer trace from HTTP -> service -> storage -> response. That gap is what allowed `Content-Type` to be dropped.

### 5. Frontend-readiness was implied, not explicit

Issue `#1` is backend-only, but it unblocks frontend upload work. The plan should therefore have treated the generated API contract as part of completion:

- backend route exists
- OpenAPI is regenerated
- TS client is regenerated
- frontend build still passes

That was handled eventually, but not as a hard phase gate from the start.

## How To Write A Better Plan Next Time

## 1. Start with a Locked Contract section

Every important rule should be one sentence, deterministic, and directly testable.

Example:

- `path` is a destination folder only
- any `..` segment in `path` returns `400`
- zero-byte uploads return `400`
- existing key returns `409`
- multipart part `Content-Type` must be written to S3 `ContentType`
- uploaded object metadata must include the exact required keys

If a statement cannot be turned directly into a test, it is too vague.

## 2. Add a Failure Matrix

Plans should include a compact table of invalid or edge-case inputs and the required result.

Example:

| Input | Expected Result |
|------|-----------------|
| missing `file` | `400 missing_file` |
| zero-byte file | `400 zero_byte_file` |
| `path=../docs` | `400 invalid_input` |
| filename contains `/` | `400 invalid_input` |
| duplicate key | `409 file_exists` |

This forces the implementation and the review to check the failure contract, not just the happy path.

## 3. Add Proof Obligations per phase

Each phase should say exactly what must be proven before it is complete.

Bad:

- path validation implemented

Better:

- service phase is incomplete until a unit test proves `../docs` is rejected
- storage phase is incomplete until a test proves the passed `ContentType` reaches `PutObject`
- handler phase is incomplete until a test proves duplicate errors map to `409`

## 4. Require named tests, not generic test success

Instead of:

- `go test ./...`

use:

- add `service_test.go` cases for traversal rejection, duplicate mapping, metadata population
- add `http_test.go` cases for `400`, `409`, and `201`
- then run `go test ./...`

This closes the loophole where a phase "passes" with zero coverage.

## 5. Add a data-flow checklist for cross-layer fields

For any field that matters across layers, the plan should list every hop it must take.

Example:

### `Content-Type`

- extracted from multipart part in HTTP handler
- passed into service method
- included in storage `PutObjectInput`
- written to S3 `PutObject`

### Metadata keys

- constructed in service
- passed to storage client
- written onto the object unchanged

If a value is important enough to mention, it is important enough to trace.

## 6. Add a frontend-unblock checklist for backend issues

For backend issues that unblock UI work, completion should include:

- route and payload shape locked
- OpenAPI regenerated
- generated frontend client updated
- frontend build still passes

That turns "frontend can now start" into a real completion criterion.

## 7. Make deferred work sharply explicit

The filename-sanitization split was the right decision, but it should be written as a strict non-goal:

- full sanitization engine is out of scope
- `#1` only performs minimal validation
- implementation must not invent extra sanitization behavior beyond the locked temporary rule

This prevents scope creep and inconsistent behavior.

## Recommended Plan Template Going Forward

For backend feature issues like `#2`, `#3`, `#4`, and `#7`, the plan should use this structure:

1. Scope
2. Locked Contract
3. Explicit Non-Goals
4. Request/Response Contract
5. Failure Matrix
6. Cross-Layer Data Flow Requirements
7. Verifiable Phases
8. Required Tests Per Phase
9. Frontend-Unblock Checklist
10. Open Questions

## One Rule To Keep

If a reviewer cannot turn a sentence in the plan directly into a test case, that sentence should be rewritten before implementation starts.
