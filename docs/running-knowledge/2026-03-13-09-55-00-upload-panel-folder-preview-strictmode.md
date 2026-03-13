# Running Knowledge: Folder Upload Preview False Conflicts in Dev

## Summary

`UploadPanel` had a dev-only false-conflict bug during folder uploads.

The symptoms looked like duplicate detection was broken:

- first-time folder uploads showed the conflict dialog
- duplicate preview sometimes reported `has_conflicts: true` even though the files did not exist before the upload
- files still uploaded successfully

## Actual Cause

The real bug was frontend-only: `configureUppy(...)` was being run twice in development under React `StrictMode`, which attached Uppy handlers twice to the same instance.

That caused:

- duplicate `file-added` logs
- duplicate `uploader-start` executions for one click
- first preview pass: `has_conflicts = false`
- first upload pass completes
- second preview pass runs immediately after and sees the just-uploaded files
- second preview pass reports `has_conflicts = true`

So the duplicate-resolution flow was triggered by the client running the uploader twice, not by backend duplicate detection returning a false positive.

## Fix

Two frontend fixes were required in `frontend/src/components/upload-panel.tsx`:

1. Keep folder-relative paths intact.

- `webkitRelativePath` should be normalized for separators only
- the selected folder name remains part of the relative path
- example: `scannedcopies/1.jpg` must stay `scannedcopies/1.jpg`

2. Guard Uppy configuration so each Uppy instance is configured only once:

- use refs to avoid registering `file-added` and `addUploader` handlers twice under `StrictMode`

## Debug Signal

If this regresses, check the browser console:

- `uppy-file-added` should appear once per queued file
- `uploader-start` should appear once per upload click
- first-time upload should show `preview-result { hasConflicts: false }`

If `uploader-start` appears twice for one click, the conflict dialog is likely a client-side duplicate execution artifact rather than a backend conflict bug.
