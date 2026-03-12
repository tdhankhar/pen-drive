# Implementation Plan: Stable Filename Sanitization Engine

**Reference**: Follow-up to upload issue `#1`
**Purpose**: Provide a reusable, deterministic filename sanitization engine with exhaustive TDD coverage before wider adoption.

## Executive Summary

This work should be treated as a standalone ticket. The goal is to define one stable filename-normalization contract and verify it thoroughly before plugging it into upload, folder upload, duplicate resolution, trash, and restore flows.

Issue `#1` should only implement minimal filename validation. This ticket owns the full mutation logic.

## Why Separate This

- filename behavior is user-visible and hard to change later
- inconsistent sanitization will leak into object keys, duplicate handling, and restore logic
- this is easiest to get right with tests written first against a locked contract

## Proposed Deliverable

Create a dedicated package-level engine, for example under:

- `backend/internal/files/filename.go`
- `backend/internal/files/filename_test.go`

Suggested API:

```go
type SanitizedFilename struct {
	Original string
	Stored   string
}

func SanitizeFilename(input string) (SanitizedFilename, error)
```

The implementation should be deterministic, side-effect free, and independent of HTTP or S3 concerns.

## Stable Behavior Contract

The sanitization engine should explicitly define behavior for:

- leading and trailing whitespace
- repeated internal whitespace
- tabs and newlines
- ASCII control characters
- path separators `/` and `\\`
- traversal-like fragments such as `..`
- repeated dots
- leading dots for hidden files
- filenames without extension
- multi-part extensions such as `.tar.gz`
- unicode characters and normalization policy
- reserved names or degenerate outputs
- names that become empty after normalization
- separator collapsing rules
- maximum-length policy if needed

## TDD Requirement

Write the tests first. The implementation should not start until the filename contract is represented in test cases.

The test suite should emphasize stability:

- the same input always produces the same output
- equivalent unsafe variants normalize the same way when intended
- extension preservation is explicit and tested
- invalid or degenerate cases return errors consistently

## Verifiable Phases

### Phase 1: Contract Definition
**Goal**: Lock the exact sanitization rules before coding  
**Deliverables**:

- documented normalization rules in the ticket doc or code comments
- list of accepted transformations
- list of rejection cases

**Verification Criteria**:

- team review of the rule table is complete
- no ambiguous behavior remains for whitespace, separators, dots, unicode, or extensions

**Completion Signal**: sanitization contract approved

### Phase 2: Test Matrix First
**Goal**: Encode the contract as exhaustive unit tests before implementation  
**Deliverables**:

- `filename_test.go` with table-driven tests
- explicit cases for mutation and rejection
- assertions for extension handling and degenerate outputs

**Verification Criteria**:

- `go test ./internal/files` fails only because implementation is not present yet or is incomplete
- every rule in the contract has direct test coverage

**Completion Signal**: test matrix merged or approved before implementation

### Phase 3: Engine Implementation
**Goal**: Implement the sanitizer to satisfy the tests  
**Deliverables**:

- `SanitizeFilename` implementation
- small helper functions if needed
- no HTTP, Gin, or S3 coupling

**Verification Criteria**:

- `go test ./internal/files` passes
- implementation remains deterministic and readable
- no untested branches in the sanitizer

**Completion Signal**: unit tests pass for the sanitizer

### Phase 4: Integration Wiring
**Goal**: Plug the sanitizer into upload and related flows without changing its contract  
**Deliverables**:

- upload service updated to call `SanitizeFilename`
- metadata uses `Original` and `Stored`
- follow-up adoption tickets for folder upload, duplicate handling, and restore

**Verification Criteria**:

- upload tests still pass with sanitizer wired in
- metadata reflects original vs stored filename correctly
- no change to the agreed sanitizer output contract

**Completion Signal**: upload path uses the shared sanitization engine

## Required Test Cases

At minimum, add table-driven coverage for:

- `" report.pdf "` -> trims correctly
- `"my   report.pdf"` -> repeated spaces normalize consistently
- `"a\tb\nc.txt"` -> control chars handled deterministically
- `"../../secret.txt"` -> path injection prevented
- `"dir/name.txt"` -> separators removed or normalized per contract
- `"..hidden"` -> leading-dot policy enforced
- `"archive.tar.gz"` -> extension handling preserved per contract
- `"photo."` -> trailing dot behavior locked
- `".gitignore"` -> hidden file policy explicit
- `"..."` -> degenerate result rejected
- `""` -> rejected
- `"   "` -> rejected
- unicode examples such as `"résumé.pdf"` and `"नमस्ते.txt"` -> policy explicit and tested

## Recommendation

Do not implement this engine opportunistically inside issue `#1`. Keep issue `#1` on minimal safety validation and ship this sanitizer only after the standalone test suite locks the behavior.
