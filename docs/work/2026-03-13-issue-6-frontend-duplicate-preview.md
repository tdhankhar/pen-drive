# Issue 6: Frontend Duplicate Preview and Rename/Replace UI

## Scope implemented

- preview duplicate conflicts before upload batches start
- show impacted existing paths and backend-proposed rename targets
- allow the user to choose `rename` or `replace`
- pass the selected `conflict_policy` into single-file and multipart upload requests

## Implementation notes

- duplicate preview is triggered from the upload panel before queued uploads begin
- the current frontend upload architecture still uploads folder items individually, so preview runs against the queued batch and the selected policy is reused per file request
- multipart sequencing remains in the upload panel, but duplicate resolution now uses the same backend contract as simple upload

## Verification

- `make frontend-lint` passed
- `make frontend-build` passed

## Remaining verification gap

- browser-level end-to-end validation against a live backend is still pending for actual rename and replace behavior
