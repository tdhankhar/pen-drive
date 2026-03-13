# Work Plan: `upload-folder` Progress

## Decision

Keep the `upload-folder` endpoint for folder uploads.

## Constraint

`upload-folder` sends one multipart HTTP request for the whole folder batch.
Because of that, the browser can report only aggregate upload progress for the request.
It cannot report truthful per-file upload progress from the frontend alone.

## Concrete Plan

1. Use Uppy for queue management and upload UI.
2. Preserve `relativePath` metadata for every file added from a folder.
3. Before upload starts, call duplicate preview for the queued folder paths.
4. If conflicts exist, block upload and resolve through the existing conflict dialog.
5. On confirm, build one `FormData` payload and send it to `POST /api/v1/files/upload-folder` with XHR.
6. Bind `xhr.upload.onprogress` to one aggregate folder progress value in Uppy state.
7. Show folder progress in the UI as aggregate progress, not as exact per-file progress.
8. Keep real per-file progress only for non-folder single-file and multipart uploads.

## UI Rule

For folder uploads:

- Show total bytes uploaded for the batch.
- Show one aggregate progress bar.
- Do not show exact per-file progress percentages.

## Verification

- Upload a small nested folder and verify aggregate progress moves from 0 to 100.
- Verify all `relativePath` values are preserved on the backend.
- Verify duplicate preview still blocks upload until a policy is selected.
- Verify success and failure states are reported clearly for the folder batch.

## Conclusion

`upload-folder` can provide useful progress, but only at the batch level.
If exact per-file folder progress becomes a hard requirement later, the architecture must change:
either upload files individually or add backend-driven per-file progress events.
