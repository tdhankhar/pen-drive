# Work Plan: Tighten UploadPanel Around Uppy and Truthful Progress

## Goal

Refactor `UploadPanel` to use a more complete Uppy-driven upload experience without re-solving work that is already done. The codebase already uses `@uppy/core`, `@uppy/react`, and a custom `uppy.addUploader` bridge. The remaining work is to simplify the UI layer, reduce upload-state complexity, and make progress reporting accurate and defensible.

## Current State

The current implementation already has these pieces in place:

- Uppy instances for file and folder uploads.
- A custom uploader bridge via `uppy.addUploader`.
- Duplicate preview integration before upload starts.
- Multipart uploads routed through generated API client calls.
- Folder path preservation via `relativePath` metadata.

The current pain points are narrower than a full migration:

1. `UploadPanel` still renders a custom upload surface instead of using Uppy's more complete UI primitives.
2. Folder upload progress is still estimated by distributing aggregate batch progress across files. That heuristic is not trustworthy when file sizes differ.
3. Upload networking is still split:
   - Multipart control flow uses generated client calls.
   - Single-file and folder form uploads still rely on raw XHR/form-data helpers for progress events.
4. State and completion handling are more manual than they need to be because the component still manages two parallel surfaces with custom rendering.

## Non-Goals

- Do not rewrite the duplicate resolution dialog from scratch.
- Do not replace multipart upload backend semantics.
- Do not force all uploads through the generated client if that would sacrifice upload progress visibility.
- Do not broaden scope into remote sources, webcam, or other Uppy plugins.

## Proposed Direction

This is a focused refactor, not a greenfield migration.

- Keep Uppy core and the existing custom uploader model.
- Replace the bespoke `UppySurface` rendering with `@uppy/dashboard` if it provides the required folder-selection UX cleanly in this app.
- Preserve the existing conflict-preview step as a pre-upload gate inside the Uppy lifecycle.
- Make an explicit architectural decision for folder uploads:
  - Option A: Keep the current batch folder endpoint and accept that per-file progress is only approximate.
  - Option B: Upload folder files individually through Uppy so each file has truthful progress.

Recommended option: Option B, unless backend throughput requirements make the batch endpoint materially better. Accurate per-file progress is one of the main reasons to lean on Uppy in the first place.

## Key Decision

### Folder Upload Semantics

This is the main technical decision that determines the rest of the implementation.

#### Option A: Keep `/upload-folder`

Pros:

- Fewer network requests.
- Reuses existing backend folder upload endpoint.
- Likely lower implementation churn.

Cons:

- Per-file progress remains heuristic, not real.
- Dashboard UI may imply a level of progress fidelity we are not actually providing.
- Continues the current mismatch between Uppy's file model and one aggregate network stream.

#### Option B: Upload folder contents file-by-file

Pros:

- Real per-file progress.
- Cleaner alignment with Uppy's state model.
- Simpler reasoning about retries, failures, and status display.

Cons:

- More requests for large folders.
- Loses the existing batched folder upload path unless retained for fallback.

Recommendation:

- Default to Option B.
- Keep existing multipart branching for large files.
- Preserve `relativePath` metadata so backend folder structure is still reconstructed correctly.

## Implementation Plan

### Phase 1: Narrow the Surface Area

- Reframe the task as a refactor of `UploadPanel`, not an Uppy migration.
- Add `@uppy/dashboard` only if we decide to adopt Dashboard-based UI.
- Confirm that required Uppy CSS imports are present for the chosen UI components.

### Phase 2: Replace Custom Upload UI

- Remove the custom `UppySurface` list/progress rendering.
- Introduce `Dashboard` in inline mode if it fits the page layout and supports the expected file and folder flows.
- If Dashboard does not support the exact folder-picking UX we need, keep a minimal custom trigger for folder selection and let Dashboard own queue/progress/status presentation.
- Preserve the existing split between file uploads and folder uploads only if product UX still requires separate entry points.
- Otherwise, consider consolidating to one Uppy instance with clearer metadata and routing rules.

### Phase 3: Preserve and Clarify the Conflict Flow

- Keep duplicate preview before actual upload begins.
- Continue using the existing `ConflictPreviewDialog` as the decision UI.
- Treat user cancellation as a first-class upload cancellation path rather than a generic error where possible.
- Ensure the dialog still receives enough context for both plain files and preserved folder paths.

### Phase 4: Fix Progress Semantics

- Remove `distributeBatchProgress` if folder uploads move to file-by-file execution.
- If batch folder upload is retained, explicitly label its progress as aggregate progress and avoid presenting it as precise per-file progress.
- Keep multipart progress reporting per file.
- Keep raw XHR where necessary for upload-progress events unless the generated client can provide equivalent progress hooks for multipart/form-data requests.

### Phase 5: Simplify Networking Boundaries

- Keep generated client usage for duplicate preview and multipart control calls.
- Audit whether single-file upload can move to generated client calls without losing progress events.
- If not, document the intentional split:
  - generated client for JSON/control APIs
  - raw XHR for progress-sensitive form-data uploads

### Phase 6: Verification

- Verify single small-file uploads show real per-file progress.
- Verify multipart uploads show real per-file progress from start to completion.
- Verify folder uploads preserve nested paths on the backend.
- Verify duplicate preview still blocks upload until the user selects a policy.
- Verify cancel, retry, and partial-failure behavior at the file level.
- Verify the completion message and refresh logic still behave correctly when some files succeed and others fail.

## Target End State

- Uppy remains the source of truth for queue state and upload lifecycle.
- The UI is driven by Dashboard or another minimal Uppy-native surface rather than bespoke rendering.
- Progress semantics are honest:
  - per-file where truly supported
  - aggregate only where unavoidable and clearly represented
- Duplicate resolution remains an explicit step in the upload lifecycle.
- Networking boundaries are intentional and documented rather than accidentally mixed.

## Recommended Execution Order

1. Decide folder strategy: batch endpoint vs file-by-file uploads.
2. Prototype Dashboard integration against the current page layout.
3. Wire duplicate preview and conflict dialog into the updated UI flow.
4. Remove heuristic per-file progress if moving to file-by-file folder uploads.
5. Run focused verification on small files, multipart files, and nested folders.

## Success Criteria

- No custom progress-slicing heuristic remains for flows that claim per-file progress.
- Upload UI complexity is reduced compared with the current `UppySurface` implementation.
- Duplicate handling still works for both single files and nested folder uploads.
- Folder path preservation remains correct.
- The remaining raw-XHR usage, if any, is deliberate and justified by progress requirements.
